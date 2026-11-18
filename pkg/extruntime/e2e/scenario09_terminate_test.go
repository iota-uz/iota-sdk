package e2e_test

import (
	"net/http"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

// Scenario 9: an instance is terminated through the management API and its
// work is continued as a new instance of a newer version, linked to its
// predecessor.
func TestScenario09TerminateAndStartV2(t *testing.T) {
	e := newEnv(t)
	admin := e.admin
	const ns = "e2e.versioned"
	e.deploy(admin, ns, "1.0.0", versionedTree(t, "v1", false))
	v1, _ := e.startJob(admin, "carry-me")
	e.waitStatus(admin, v1, "waiting")
	e.deploy(admin, ns, "2.0.0", versionedTree(t, "v2", true))

	// Not terminated: a successor cannot be started from a live instance.
	e.mgmt(admin, http.MethodPost, "/workflows/"+v1.String()+"/start-v2", map[string]string{"version": "2.0.0"}, http.StatusConflict, nil)

	// Rights: the management API checks them.
	reader := e.newActor("reader", "Extensions.Read")
	e.mgmt(reader, http.MethodPost, "/workflows/"+v1.String()+"/terminate", nil, http.StatusForbidden, nil)

	e.mgmt(admin, http.MethodPost, "/workflows/"+v1.String()+"/terminate", nil, http.StatusNoContent, nil)
	term := e.workflow(admin, v1)
	require.Equal(t, "terminated", term.Instance.Status)
	for _, o := range term.Occurrences {
		if o.Name == "go" {
			require.Equal(t, "cancelled", o.Status, "its open wait was cancelled")
		}
	}
	// A terminated instance accepts no signals and does not run.
	e.mgmt(admin, http.MethodPost, "/workflows/"+v1.String()+"/signal", map[string]any{"name": "go", "payload": map[string]any{}, "key": "late"}, http.StatusConflict, nil)

	// Start v2: a new instance of the chosen version, linked to the predecessor.
	var out struct {
		Instance string `json:"instance"`
	}
	e.mgmt(admin, http.MethodPost, "/workflows/"+v1.String()+"/start-v2", map[string]string{"version": "2.0.0"}, http.StatusCreated, &out)
	next := uuid.MustParse(out.Instance)
	require.NotEqual(t, v1, next)
	inst := e.waitStatus(admin, next, "waiting")
	require.Equal(t, "2.0.0", inst.Version)
	require.NotNil(t, inst.PredecessorID)
	require.Equal(t, v1, *inst.PredecessorID)
	require.JSONEq(t, `{"label":"carry-me"}`, string(inst.Input), "the successor starts from the predecessor's input")
	require.Nil(t, inst.StartKey, "a v2 start has no start key")
	require.Equal(t, "terminated", e.workflow(admin, v1).Instance.Status, "the predecessor stays terminated")

	e.signal(admin, next, "go", map[string]any{}, "n1")
	done := e.waitStatus(admin, next, "completed")
	require.JSONEq(t, `{"tag":"v2","label":"carry-me"}`, string(done.Result))

	// The list shows both; filters work.
	require.Len(t, e.workflows(admin, "workflow="+ns+".job"), 2)
	require.Len(t, e.workflows(admin, "workflow="+ns+".job&status=terminated"), 1)
	require.Len(t, e.workflows(admin, "workflow="+ns+".job&status=completed"), 1)
}
