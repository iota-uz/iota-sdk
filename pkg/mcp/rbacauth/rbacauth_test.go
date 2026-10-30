package rbacauth_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/role"
	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/user"
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/permission"
	"github.com/iota-uz/iota-sdk/modules/core/domain/value_objects/internet"
	"github.com/iota-uz/iota-sdk/modules/core/permissions"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/mcp"
	"github.com/iota-uz/iota-sdk/pkg/mcp/rbacauth"
)

func account(t *testing.T, tenant uuid.UUID, perms ...permission.Permission) user.User {
	t.Helper()
	email, err := internet.NewEmail("ai@example.com")
	require.NoError(t, err)
	r := role.New("staff", role.WithID(1), role.WithPermissions(perms), role.WithTenantID(tenant))
	return user.New("", "", email, "", user.WithID(1), user.WithTenantID(tenant), user.WithRoles([]role.Role{r}))
}

func TestAuthorizer_Authorize(t *testing.T) {
	tenant := uuid.New()
	reader := account(t, tenant, permissions.UserRead)
	blocked := reader.Block("left", 1, tenant)
	policy := mcp.Policy{Scopes: []string{"mcp:read"}, Permissions: []string{permissions.UserRead.Name()}}

	cases := []struct {
		name      string
		principal mcp.Principal
		account   user.User
		loadErr   error
		policy    mcp.Policy
		code      mcp.Code
		recovery  mcp.Recovery
	}{
		{name: "scope and permission granted", principal: mcp.Principal{TenantID: tenant, UserID: 1, Scopes: mcp.GrantedScopes("mcp:read")},
			account: reader, policy: policy},
		{name: "first-party session limited by RBAC only", principal: mcp.Principal{TenantID: tenant, UserID: 1, Scopes: mcp.AllScopes()},
			account: reader, policy: policy},
		{name: "scope missing although RBAC allows", principal: mcp.Principal{TenantID: tenant, UserID: 1, Scopes: mcp.GrantedScopes("mcp:other")},
			account: reader, policy: policy, code: mcp.CodeForbidden, recovery: mcp.RecoveryReauthenticate},
		{name: "permission revoked although scope allows", principal: mcp.Principal{TenantID: tenant, UserID: 1, Scopes: mcp.GrantedScopes("mcp:read")},
			account: account(t, tenant), policy: policy, code: mcp.CodeForbidden, recovery: mcp.RecoveryRequestAccess},
		{name: "blocked account", principal: mcp.Principal{TenantID: tenant, UserID: 1, Scopes: mcp.AllScopes()},
			account: blocked, policy: policy, code: mcp.CodeForbidden, recovery: mcp.RecoveryReauthenticate},
		{name: "account of another tenant", principal: mcp.Principal{TenantID: uuid.New(), UserID: 1, Scopes: mcp.AllScopes()},
			account: reader, policy: policy, code: mcp.CodeForbidden, recovery: mcp.RecoveryReauthenticate},
		{name: "account gone", principal: mcp.Principal{TenantID: tenant, UserID: 1, Scopes: mcp.AllScopes()},
			loadErr: errors.New("not found"), policy: policy, code: mcp.CodeForbidden, recovery: mcp.RecoveryReauthenticate},
		{name: "service subject needs an application authorizer", principal: mcp.Principal{TenantID: tenant, Service: "sync", Scopes: mcp.AllScopes()},
			account: reader, policy: mcp.Policy{}, code: mcp.CodeForbidden, recovery: mcp.RecoveryRequestAccess},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			authz := rbacauth.New(func(_ context.Context, _ uuid.UUID, _ uint) (user.User, error) {
				return tc.account, tc.loadErr
			})
			ctx, err := authz.Authorize(context.Background(), tc.principal, tc.policy)
			if tc.code == "" {
				require.NoError(t, err)
				u, uerr := composables.UseUser(ctx)
				require.NoError(t, uerr)
				assert.Equal(t, tc.account, u)
				tid, terr := composables.UseTenantID(ctx)
				require.NoError(t, terr)
				assert.Equal(t, tenant, tid)
				return
			}
			var f *mcp.Failure
			require.ErrorAs(t, err, &f)
			assert.Equal(t, tc.code, f.Code)
			assert.Equal(t, tc.recovery, f.Recovery)
		})
	}
}
