package mcp_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/iota-uz/iota-sdk/pkg/mcp"
)

type entries struct{ got []mcp.AuditEntry }

func (e *entries) Record(_ context.Context, a mcp.AuditEntry) { e.got = append(e.got, a) }

// A handler of an audited engine that calls a second engine without an auditor: the inner call's
// mcp.Audit records nothing, in no auditor of the outer engine.
func TestAuditScopeIsNotInheritedByAnEngineWithoutAuditor(t *testing.T) {
	allow := mcp.AuthorizerFunc(func(ctx context.Context, _ mcp.Principal, _ mcp.Policy) (context.Context, error) { return ctx, nil })
	type none struct{}
	innerSeen := false
	inner := mcp.NewRegistry()
	require.NoError(t, inner.Register(mcp.MustNew(mcp.Spec{Name: "inner"}, func(ctx context.Context, _ none) (none, error) {
		_, innerSeen = mcp.AuditorFrom(ctx)
		mcp.Audit(ctx, "inner:event", mcp.CodeApprovalRequired, "")
		return none{}, nil
	})))
	innerEngine, err := mcp.NewEngine(inner, allow)
	require.NoError(t, err)

	outerLog := &entries{}
	outer := mcp.NewRegistry()
	require.NoError(t, outer.Register(mcp.MustNew(mcp.Spec{Name: "outer"}, func(ctx context.Context, _ none) (none, error) {
		res := innerEngine.Call(ctx, "inner", json.RawMessage(`{}`))
		require.True(t, res.OK, "%+v", res.Error)
		return none{}, nil
	})))
	outerEngine, err := mcp.NewEngine(outer, allow, mcp.WithAuditor(outerLog))
	require.NoError(t, err)

	ctx := mcp.WithPrincipal(context.Background(), mcp.Principal{TenantID: uuid.New(), UserID: 1, Scopes: mcp.AllScopes()})
	res := outerEngine.Call(ctx, "outer", json.RawMessage(`{}`))
	require.True(t, res.OK, "%+v", res.Error)
	assert.False(t, innerSeen, "the inner engine has no auditor")
	for _, e := range outerLog.got {
		assert.NotEqual(t, "inner:event", e.Tool, "the inner event does not reach the outer auditor")
	}
}
