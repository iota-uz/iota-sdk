package automations_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/iota-uz/iota-sdk/modules/automations/presentation/graphs"
	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/user"
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/permission"
	"github.com/iota-uz/iota-sdk/modules/core/domain/value_objects/internet"
	"github.com/iota-uz/iota-sdk/modules/core/infrastructure/persistence"
	"github.com/iota-uz/iota-sdk/pkg/automation/engine"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/composition"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/events"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/workflow"
	"github.com/iota-uz/iota-sdk/pkg/itf"
)

// The module composes the engine as the RunStarter of start_automation and run_automation: a graph's
// run_automation starts the child graph's run and continues with the output the child's end gives, by
// the child's output schema.
func TestRunAutomationStartsTheChildGraphInTheComposedModule(t *testing.T) {
	s, container := app(t, allAutomations...)
	// The engine resolves the run-as from the database: the acting user is a committed one.
	s = s.AsUser(committedUser(t, s, allAutomations...))

	require.True(t, composition.Has[engine.RunStarter](container), "the module provides the engine's RunStarter")
	starter, err := composition.Resolve[engine.RunStarter](container)
	require.NoError(t, err)
	rt, ok := starter.(*engine.Runtime)
	require.True(t, ok, "the RunStarter is the module's engine, %T", starter)

	create := func(g map[string]any) string {
		t.Helper()
		var created graphs.SaveResult
		require.Nil(t, rpc(t, s, graphs.MethodCreate, map[string]any{"graph": g, "layout": map[string]any{}}, &created).Error)
		require.True(t, created.OK, "%+v", created)
		var review graphs.ReviewResponse
		require.Nil(t, rpc(t, s, graphs.MethodEnableReview, map[string]any{"id": created.ID, "version": created.Version}, &review).Error)
		var enabled graphs.EnableResponse
		require.Nil(t, rpc(t, s, graphs.MethodEnable, map[string]any{"id": created.ID, "version": created.Version, "code_digest": review.CodeDigest}, &enabled).Error)
		require.True(t, enabled.OK, "%+v", enabled)
		return created.ID
	}
	edge := func(from, outcome, to string) map[string]any {
		return map[string]any{"from": map[string]any{"node": from, "outcome": outcome}, "to": to}
	}

	// A save checks the end's output against the graph's output schema.
	greeting := map[string]any{"type": "object", "properties": map[string]any{"greeting": map[string]any{"type": "string"}},
		"required": []any{"greeting"}, "additionalProperties": false}
	child := func(output map[string]any) map[string]any {
		return map[string]any{
			"name": "Greeter", "run_as": "owner", "output_schema": greeting,
			"triggers": []any{map[string]any{"id": "called", "event": engine.EventCalled}},
			"nodes":    []any{map[string]any{"id": "fin", "type": "end", "params": map[string]any{"reason": "greeted", "output": output}}},
			"edges":    []any{edge("called", "fired", "fin")},
		}
	}
	var refused graphs.SaveResult
	require.Nil(t, rpc(t, s, graphs.MethodCreate, map[string]any{"graph": child(map[string]any{"other": "'x'"}), "layout": map[string]any{}}, &refused).Error)
	require.False(t, refused.OK, "an output the schema does not take is refused: %+v", refused)
	childID := create(child(map[string]any{"greeting": `"hello " + string(event.input.name)`}))

	parentID := create(map[string]any{
		"name": "Caller", "run_as": "owner",
		"triggers": []any{map[string]any{"id": "start", "event": engine.EventManual}},
		"nodes": []any{
			map[string]any{"id": "call", "type": "run_automation", "params": map[string]any{"automation_id": childID, "input": map[string]any{"name": "'Ann'"}, "timeout": "5m"}},
			map[string]any{"id": "ok", "type": "end", "params": map[string]any{"output": map[string]any{"got": "nodes.call.output.output.greeting"}}},
			map[string]any{"id": "bad", "type": "end", "params": map[string]any{"reason": "the child failed"}},
		},
		"edges": []any{edge("start", "fired", "call"), edge("call", "done", "ok"), edge("call", "failed", "bad"), edge("call", "timeout", "bad")},
	})

	var started graphs.RunManualResponse
	require.Nil(t, rpc(t, s, graphs.MethodRunManual, map[string]any{"id": parentID, "trigger_id": "start", "input": map[string]any{}}, &started).Error)
	require.True(t, started.OK, "%+v", started)
	runID, err := uuid.Parse(started.RunID)
	require.NoError(t, err)

	// The process's engine, workflow engine and outbox, passed by hand (the module runs without background).
	wf, err := composition.Resolve[*workflow.Engine](container)
	require.NoError(t, err)
	disp, err := composition.Resolve[*events.Dispatcher](container)
	require.NoError(t, err)
	pool := s.Env().Pool
	ctx := context.Background()
	var status string
	var output []byte
	deadline := time.Now().Add(60 * time.Second)
	for time.Now().Before(deadline) {
		_, err := disp.RunOnce(ctx)
		require.NoError(t, err)
		_, err = wf.RunOnce(ctx)
		require.NoError(t, err)
		require.NoError(t, rt.Maintain(ctx))
		var run graphs.RunData
		require.Nil(t, rpc(t, s, graphs.MethodRun, map[string]any{"run_id": runID.String()}, &run).Error)
		status = string(run.Run.Status)
		if status == "blocked" {
			b, err := json.Marshal(run)
			require.NoError(t, err)
			t.Fatalf("the run is blocked: %s", b)
		}
		if status == string(engine.RunDone) || status == string(engine.RunFailed) {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	require.Equal(t, string(engine.RunDone), status, "the parent run ends")
	require.NoError(t, pool.QueryRow(ctx, `SELECT output FROM automation.runs WHERE id = $1`, runID).Scan(&output))
	var got map[string]any
	require.NoError(t, json.Unmarshal(output, &got))
	assert.Equal(t, map[string]any{"got": "hello Ann"}, got, "the parent read the child's output")

	var childRuns int
	var childOutput []byte
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*), max(output::text) FROM automation.runs WHERE automation_id = $1`, childID).Scan(&childRuns, &childOutput))
	assert.Equal(t, 1, childRuns, "the child graph ran once")
	assert.JSONEq(t, `{"greeting": "hello Ann"}`, string(childOutput))
}

// committedUser is a user of the suite's tenant with perms, committed so the engine's own connections
// load it as a run-as.
func committedUser(t *testing.T, s *itf.Suite, perms ...permission.Permission) user.User {
	t.Helper()
	ctx := composables.WithPool(composables.WithTenantID(context.Background(), s.Env().Tenant.ID), s.Env().Pool)
	permRepo := persistence.NewPermissionRepository()
	for _, p := range perms {
		require.NoError(t, permRepo.Save(ctx, p))
	}
	email, err := internet.NewEmail("runner-" + uuid.NewString()[:8] + "@example.test")
	require.NoError(t, err)
	u, err := persistence.NewUserRepository(persistence.NewUploadRepository()).Create(ctx,
		user.New("Runner", "Test", email, user.UILanguageEN, user.WithTenantID(s.Env().Tenant.ID), user.WithPermissions(perms)))
	require.NoError(t, err)
	return u
}
