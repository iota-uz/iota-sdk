package automations_test

import (
	"context"
	"embed"
	"encoding/json"
	stderrors "errors"
	"sync"
	"testing"
	"time"

	"github.com/google/jsonschema-go/jsonschema"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/iota-uz/iota-sdk/modules"
	"github.com/iota-uz/iota-sdk/modules/automations"
	"github.com/iota-uz/iota-sdk/modules/automations/permissions"
	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/role"
	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/user"
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/permission"
	"github.com/iota-uz/iota-sdk/modules/core/domain/value_objects/internet"
	"github.com/iota-uz/iota-sdk/pkg/composition"
	"github.com/iota-uz/iota-sdk/pkg/config"
	"github.com/iota-uz/iota-sdk/pkg/config/providers/static"
	"github.com/iota-uz/iota-sdk/pkg/config/stdconfig/extruntimeconfig"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/wiring"
	"github.com/iota-uz/iota-sdk/pkg/itf"
	"github.com/iota-uz/iota-sdk/pkg/mcp"
	"github.com/iota-uz/iota-sdk/pkg/mcp/changes"
	"github.com/iota-uz/iota-sdk/pkg/mcp/mcpwiring"
	"github.com/iota-uz/iota-sdk/pkg/mcp/rbacauth"
)

// flakyAuditor refuses the first fail confirmations and records the rest.
type flakyAuditor struct {
	mu        sync.Mutex
	fail      int
	refused   int
	confirmed []mcp.AuditEntry
}

func (a *flakyAuditor) Record(context.Context, mcp.AuditEntry) {}

func (a *flakyAuditor) RecordConfirmed(_ context.Context, e mcp.AuditEntry) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.refused < a.fail {
		a.refused++
		return stderrors.New("audit store unavailable")
	}
	a.confirmed = append(a.confirmed, e)
	return nil
}

func (a *flakyAuditor) tools() (refused int, tools []string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	for _, e := range a.confirmed {
		tools = append(tools, e.Tool)
	}
	return a.refused, tools
}

const mcpAgentID = 7

// strayTools contributes bare []mcp.Tool for a purpose of its own, with a name the automation tools use:
// only mcpwiring.ContributeTools reaches the application's registry, so the start does not fail.
type strayTools struct{}

func (strayTools) Descriptor() composition.Descriptor {
	return composition.Descriptor{Name: "stray_tools"}
}
func (strayTools) LocaleFS() []*embed.FS { return nil }

func (strayTools) Build(b *composition.Builder) error {
	composition.Contribute(b, func(*composition.Container) ([]mcp.Tool, error) {
		return []mcp.Tool{strayTool{}}, nil
	})
	return nil
}

type strayTool struct{}

func (strayTool) Spec() mcp.Spec {
	return mcp.Spec{Name: "automation_list", Input: &jsonschema.Schema{Type: "object"}}
}

func (strayTool) Call(context.Context, json.RawMessage) (any, error) { return nil, nil }

func mcpAgent(tenant uuid.UUID) user.User {
	email, err := internet.NewEmail("agent@example.test")
	if err != nil {
		panic(err)
	}
	r := role.New("agent", role.WithID(mcpAgentID), role.WithTenantID(tenant),
		role.WithPermissions([]permission.Permission{permissions.AutomationsRead, permissions.AutomationsAuthor}))
	return user.New("", "", email, "", user.WithID(mcpAgentID), user.WithTenantID(tenant), user.WithRoles([]role.Role{r}))
}

// The application's MCP composition with the automations module: the automation tools and the changes
// tools are in the application's registry, kinds of entity "automation" apply through apply_changes
// with links under the configured origin, the engine's run links are the editor's run pages, and an
// audit event the auditor did not confirm is delivered later by the dispatch loop.
func TestMCPCompositionWiresAutomationOperations(t *testing.T) {
	t.Parallel()
	src, err := config.Build(static.New(map[string]any{"http.origin": "https://erp.test"}))
	require.NoError(t, err)
	registry := mcp.NewRegistry()
	audit := &flakyAuditor{fail: 1}
	authz := rbacauth.New(func(_ context.Context, tenant uuid.UUID, id uint) (user.User, error) {
		if id != mcpAgentID {
			return nil, stderrors.New("unknown account")
		}
		return mcpAgent(tenant), nil
	})
	components := append(modules.Components(),
		wiring.NewComponent(wiring.Options{Natives: modules.Natives(), Section: &extruntimeconfig.Config{}}),
		strayTools{},
		mcpwiring.NewComponent(mcpwiring.Options{
			Registry: registry, Authorizer: authz, Auditor: audit,
			ApprovalURL:      func(id string) string { return "https://erp.test/mcp/approvals/" + id },
			DispatchInterval: 50 * time.Millisecond,
		}),
	)
	h := itf.NewHarness(t, itf.HarnessConfig{
		Name: "automations_mcp_composition", Components: components, Source: src,
		Database:  itf.DatabaseConfig{Provisioning: itf.ProvisioningPerTestDatabase},
		Isolation: itf.IsolationConfig{Mode: itf.IsolationCommitted},
	})
	s := h.Scope(t)

	for _, name := range []string{
		"automation_catalog", "automation_list", "automation_get", "automation_runs", "automation_run_get",
		"automation_validate", "automation_dry_run", changes.ToolPrepare, changes.ToolApply, changes.ToolOperationGet,
	} {
		_, ok := registry.Spec(name)
		assert.True(t, ok, "%s is registered", name)
	}
	holder, err := composition.Resolve[*mcpwiring.Changes](s.Container)
	require.NoError(t, err)
	_, ok := holder.Service()
	require.True(t, ok, "the changes service exists once the application started")

	links, err := automations.EngineLinks(s.Container)
	require.NoError(t, err)
	require.NotNil(t, links, "the engine runs with links")
	id, run := uuid.New(), uuid.New()
	assert.Equal(t, "https://erp.test/automations/"+id.String()+"/runs/"+run.String(), links.RunURL(s.Tenant.ID, id, run))

	eng, err := mcp.NewEngine(registry, authz, mcp.WithAuditor(audit))
	require.NoError(t, err)
	ctx := mcp.WithPrincipal(context.Background(), mcp.Principal{TenantID: s.Tenant.ID, UserID: mcpAgentID, ClientID: "agent", Scopes: mcp.AllScopes()})
	call := func(tool string, input any, into any) {
		t.Helper()
		raw, err := json.Marshal(input)
		require.NoError(t, err)
		res := eng.Call(ctx, tool, raw)
		require.True(t, res.OK, "%s: %+v", tool, res.Error)
		require.NoError(t, json.Unmarshal(res.Data, into))
	}
	g := map[string]any{"name": "Agent", "triggers": []any{map[string]any{"id": "go", "event": "automation.manual.v1"}},
		"nodes": []any{map[string]any{"id": "fin", "type": "end", "params": map[string]any{}}},
		"edges": []any{map[string]any{"from": map[string]any{"node": "go", "outcome": "fired"}, "to": "fin"}}}

	type prepared struct {
		Status   string                 `json:"status"`
		PlanID   string                 `json:"plan_id"`
		PlanHash string                 `json:"plan_hash"`
		Errors   []changes.IndexedIssue `json:"errors"`
	}
	var refused prepared
	call(changes.ToolPrepare, map[string]any{"changes": []any{map[string]any{"entity": "automation", "op": "create",
		"fields": map[string]any{"graph": g, "enabled": true}}}}, &refused)
	require.NotEqual(t, changes.StatusReady, refused.Status)
	require.NotEmpty(t, refused.Errors)
	assert.Contains(t, refused.Errors[0].Message, "https://erp.test/automations", "HUMAN_REQUIRED names the page under the origin")

	var p prepared
	call(changes.ToolPrepare, map[string]any{"changes": []any{map[string]any{"entity": "automation", "op": "create",
		"fields": map[string]any{"graph": g}}}}, &p)
	require.Equal(t, changes.StatusReady, p.Status, "%+v", p.Errors)
	var o struct {
		Status  string          `json:"status"`
		Receipt changes.Receipt `json:"receipt"`
	}
	call(changes.ToolApply, map[string]any{"plan_id": p.PlanID, "plan_hash": p.PlanHash, "idempotency_key": "create-1"}, &o)
	require.Equal(t, changes.OperationSucceeded, o.Status)
	require.Len(t, o.Receipt.Items, 1)
	var saved struct {
		ID        string `json:"id"`
		EditorURL string `json:"editor_url"`
	}
	require.NoError(t, json.Unmarshal(o.Receipt.Items[0].Result, &saved))
	assert.Equal(t, "https://erp.test/automations/"+saved.ID+"?version=1", saved.EditorURL)

	// The apply's own delivery was refused; the dispatch loop delivers the event after its backoff.
	require.Eventually(t, func() bool {
		_, tools := audit.tools()
		for _, tool := range tools {
			if tool == "changes:automation.create" {
				return true
			}
		}
		return false
	}, 15*time.Second, 50*time.Millisecond, "the dispatch loop delivers the deferred audit event")
	refusedCount, _ := audit.tools()
	assert.Equal(t, 1, refusedCount)
}
