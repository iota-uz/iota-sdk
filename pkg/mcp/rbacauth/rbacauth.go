// Package rbacauth authorizes MCP principals against the SDK account model:
// the effective access is the grant's scopes intersected with the account's
// current RBAC permissions, re-read on every call.
package rbacauth

import (
	"context"
	"slices"
	"strings"

	"github.com/google/uuid"

	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/user"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/mcp"
)

// UserLoader reads the account fresh from storage; it must not serve cached
// state, otherwise revoked permissions would survive in warm sessions.
type UserLoader func(ctx context.Context, tenantID uuid.UUID, userID uint) (user.User, error)

type Authorizer struct {
	load UserLoader
}

func New(load UserLoader) *Authorizer {
	return &Authorizer{load: load}
}

func (a *Authorizer) Authorize(ctx context.Context, p mcp.Principal, policy mcp.Policy) (context.Context, error) {
	if p.UserID == 0 {
		return nil, mcp.Fail(mcp.CodeForbidden, mcp.RecoveryRequestAccess, "service subjects need an application authorizer")
	}
	if missing := p.Scopes.Missing(policy.Scopes); len(missing) > 0 {
		return nil, mcp.Fail(mcp.CodeForbidden, mcp.RecoveryReauthenticate,
			"the grant lacks scope "+strings.Join(missing, ", "))
	}
	ctx = composables.WithTenantID(ctx, p.TenantID)
	u, err := a.load(ctx, p.TenantID, p.UserID)
	if err != nil || u == nil || u.TenantID() != p.TenantID || u.IsBlocked() {
		return nil, mcp.Fail(mcp.CodeForbidden, mcp.RecoveryReauthenticate, "the account is not available")
	}
	granted := composables.EffectivePermissionNames(u)
	for _, perm := range policy.Permissions {
		if !slices.Contains(granted, perm) {
			return nil, mcp.Fail(mcp.CodeForbidden, mcp.RecoveryRequestAccess, "missing permission "+perm)
		}
	}
	return composables.WithUser(ctx, u), nil
}
