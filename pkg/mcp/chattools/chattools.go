// Package chattools exposes the shared MCP registry to the built-in chat. The
// chat only differs from the MCP gateway in how the trusted identity is
// established: here it comes from the authenticated chat session.
package chattools

import (
	"context"
	"encoding/json"

	"github.com/iota-uz/iota-sdk/pkg/bichat/agents"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/mcp"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
)

// Tools adapts every registered tool. Results reach the model as the same
// Result JSON an MCP client receives as structuredContent.
func Tools(engine *mcp.Engine) ([]agents.Tool, error) {
	const op serrors.Op = "chattools.Tools"
	specs := engine.Registry().Specs()
	out := make([]agents.Tool, 0, len(specs))
	for _, spec := range specs {
		raw, err := json.Marshal(spec.Input)
		if err != nil {
			return nil, serrors.Wrap(op, err)
		}
		var params map[string]any
		if err := json.Unmarshal(raw, &params); err != nil {
			return nil, serrors.Wrap(op, err)
		}
		out = append(out, &tool{engine: engine, name: spec.Name, description: spec.Description, params: params})
	}
	return out, nil
}

type tool struct {
	engine      *mcp.Engine
	name        string
	description string
	params      map[string]any
}

func (t *tool) Name() string               { return t.name }
func (t *tool) Description() string        { return t.description }
func (t *tool) Parameters() map[string]any { return t.params }

func (t *tool) Call(ctx context.Context, input string) (string, error) {
	res := t.engine.Call(SessionPrincipal(ctx), t.name, json.RawMessage(input))
	raw, err := json.Marshal(res)
	if err != nil {
		return "", err
	}
	return string(raw), nil
}

// SessionPrincipal binds the chat session's user and tenant as the caller.
// A first-party session holds no OAuth grant, so RBAC alone limits it.
func SessionPrincipal(ctx context.Context) context.Context {
	if _, ok := mcp.PrincipalFrom(ctx); ok {
		return ctx
	}
	tenantID, err := composables.UseTenantID(ctx)
	if err != nil {
		return ctx
	}
	u, err := composables.UseUser(ctx)
	if err != nil || u == nil {
		return ctx
	}
	return mcp.WithPrincipal(ctx, mcp.Principal{TenantID: tenantID, UserID: u.ID(), Scopes: mcp.AllScopes()})
}
