package mcptools_test

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/user"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/mcptools"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/spi"
	"github.com/iota-uz/iota-sdk/pkg/mcp"
	"github.com/iota-uz/iota-sdk/pkg/mcp/rbacauth"
)

// guarded marks one instance and one task as a person's to act on.
// blocked is a guarded task the caller may not decide.
type guarded struct{ instance, task, blocked uuid.UUID }

func (g guarded) Instance(_ context.Context, _, id uuid.UUID) (string, bool, error) {
	return "https://app.test/runs/" + id.String(), id == g.instance, nil
}

func (guarded) Contract(context.Context, string) (bool, error) { return false, nil }

func (g guarded) Task(_ context.Context, _ spi.Subject, id uuid.UUID) (string, bool, bool, error) {
	switch id {
	case g.task:
		return "https://app.test/runs/of-task", true, true, nil
	case g.blocked:
		return "", true, false, nil
	}
	return "", false, false, nil
}

func TestHumanOnlyInstancesRefuseActionsButNotReads(t *testing.T) {
	e := newEnv(t)
	g := guarded{instance: uuid.New(), task: uuid.New(), blocked: uuid.New()}
	reg := mcp.NewRegistry()
	require.NoError(t, mcptools.Register(reg, mcptools.Deps{Manage: e.svc, HumanOnly: g}))
	authz := rbacauth.New(func(_ context.Context, tenant uuid.UUID, id uint) (user.User, error) {
		u, ok := e.users[id]
		if !ok || tenant != e.tenant {
			return nil, fmt.Errorf("unknown account")
		}
		return u, nil
	})
	engine, err := mcp.NewEngine(reg, authz)
	require.NoError(t, err)
	call := func(tool string, input map[string]any) mcp.Result {
		raw, err := json.Marshal(input)
		require.NoError(t, err)
		return engine.Call(e.principal(adminID, mcp.AllScopes()), tool, raw)
	}
	id := g.instance.String()
	for tool, input := range map[string]map[string]any{
		"ext_workflow_signal":      {"id": id, "name": "inbox"},
		"ext_workflow_terminate":   {"id": id},
		"ext_workflow_start_v2":    {"id": id},
		"ext_workflow_resolve":     {"id": id, "name": "call", "n": 0},
		"ext_workflow_retry":       {"id": id, "name": "call", "n": 0},
		"ext_workflow_resume":      {"id": id},
		"ext_workflow_decide_task": {"task_id": g.task.String(), "decision": map[string]any{"outcome": "approve"}},
	} {
		res := call(tool, input)
		require.False(t, res.OK, tool)
		assert.Equal(t, mcp.CodeApprovalRequired, res.Error.Code, tool)
		assert.Equal(t, mcp.RecoveryOpenApproval, res.Error.Recovery, tool)
		require.Len(t, res.Error.Issues, 1, tool)
		assert.Equal(t, mcptools.IssueHumanRequired, res.Error.Issues[0].Code, tool)
		assert.Contains(t, res.Error.Message, "https://app.test/runs/", tool)
	}

	// Reads stay; an instance that is not guarded reaches the service (no workflow engine here).
	res := call("ext_workflow_get", map[string]any{"id": id})
	if !res.OK {
		assert.NotEqual(t, mcp.CodeApprovalRequired, res.Error.Code)
	}
	res = call("ext_workflow_signal", map[string]any{"id": uuid.NewString(), "name": "inbox"})
	require.False(t, res.OK)
	assert.NotEqual(t, mcp.CodeApprovalRequired, res.Error.Code)

	// A guarded task the caller may not decide is refused by the guard itself, without the page: it never
	// reaches the service.
	res = call("ext_workflow_decide_task", map[string]any{"task_id": g.blocked.String(), "decision": map[string]any{"outcome": "approve"}})
	require.False(t, res.OK)
	assert.Equal(t, mcp.CodeForbidden, res.Error.Code)
	assert.NotContains(t, res.Error.Message, "https://")

	// Who may not operate instances is refused for that, and learns nothing about the instance.
	raw, err := json.Marshal(map[string]any{"id": id, "name": "inbox"})
	require.NoError(t, err)
	res = engine.Call(e.principal(readerID, mcp.AllScopes()), "ext_workflow_signal", raw)
	require.False(t, res.OK)
	assert.Equal(t, mcp.CodeForbidden, res.Error.Code)
	assert.NotContains(t, res.Error.Message, "https://app.test")
}
