package e2e_test

import (
	"net/http"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

// versionedTree is the "versioned" fixture stamped with a tag; v2 also evolves
// the entity with an optional field its workflow writes.
func versionedTree(t *testing.T, tag string, evolve bool) tree {
	t.Helper()
	tr := loadTree(t, "versioned").with("src/server.ts", "__TAG__", tag)
	if evolve {
		tr = tr.with("src/contracts.ts", "// EXTRA_FIELD", "extra: field.text().optional(),").
			with("src/server.ts", "/* EXTRA_WRITE */", `, extra: "written by " + TAG`)
	}
	return tr
}

func (e *env) startJob(a *actor, label string) (uuid.UUID, string) {
	e.t.Helper()
	var out struct {
		Instance  string `json:"instance"`
		StartedBy string `json:"startedBy"`
	}
	e.mustRPC(a, "command", "e2e.versioned.start", map[string]any{"label": label}, &out)
	id, err := uuid.Parse(out.Instance)
	require.NoError(e.t, err)
	return id, out.StartedBy
}

func (e *env) signal(a *actor, id uuid.UUID, name string, payload any, key string) {
	e.t.Helper()
	e.mgmt(a, http.MethodPost, "/workflows/"+id.String()+"/signal", map[string]any{"name": name, "payload": payload, "key": key}, http.StatusNoContent, nil)
}

// Scenario 7: publishing and activating v2 of a package while a v1 workflow
// instance waits does not move that instance: it finishes on v1 code, while
// instances started afterwards run v2.
func TestScenario07InstancesStayPinnedToTheirVersion(t *testing.T) {
	e := newEnv(t)
	admin := e.admin
	const ns = "e2e.versioned"
	e.deploy(admin, ns, "1.0.0", versionedTree(t, "v1", false))

	a, by := e.startJob(admin, "old")
	require.Equal(t, "v1", by)
	instA := e.waitStatus(admin, a, "waiting")
	require.Equal(t, "1.0.0", instA.Version)

	// v2 changes the code and the entity's schema; activation is allowed
	// because the running instance's pinned schema stays compatible.
	act := e.deploy(admin, ns, "2.0.0", versionedTree(t, "v2", true))
	require.NotEmpty(t, act.SchemaSteps, "the new optional column was added")

	b, by := e.startJob(admin, "new")
	require.Equal(t, "v2", by, "commands serve the new version")
	instB := e.waitStatus(admin, b, "waiting")
	require.Equal(t, "2.0.0", instB.Version)
	require.NotEqual(t, instA.ArtifactHash, instB.ArtifactHash)
	require.Equal(t, "1.0.0", e.workflow(admin, a).Instance.Version, "the waiting v1 instance was not moved")

	e.signal(admin, a, "go", map[string]any{}, "k-a")
	e.signal(admin, b, "go", map[string]any{"note": "n"}, "k-b")
	doneA := e.waitStatus(admin, a, "completed")
	doneB := e.waitStatus(admin, b, "completed")
	require.JSONEq(t, `{"tag":"v1","label":"old"}`, string(doneA.Result), "v1 continued on v1 code")
	require.JSONEq(t, `{"tag":"v2","label":"new"}`, string(doneB.Result))
	require.Equal(t, "1.0.0", doneA.Version)
	require.Equal(t, "2.0.0", doneB.Version)

	// Each wrote through its own code against the one evolved table.
	require.Empty(t, e.scalarString(`SELECT extra FROM x_e2e_versioned.log WHERE label = 'old'`), "v1 knows no extra field")
	require.Equal(t, "written by v2", e.scalarString(`SELECT extra FROM x_e2e_versioned.log WHERE label = 'new'`))
	require.Equal(t, "v1", e.scalarString(`SELECT tag FROM x_e2e_versioned.log WHERE label = 'old'`))

	// A signal to a finished instance is refused.
	e.mgmt(admin, http.MethodPost, "/workflows/"+a.String()+"/signal", map[string]any{"name": "go", "payload": map[string]any{}, "key": "k-a2"}, http.StatusConflict, nil)
}
