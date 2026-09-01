package oidc

import (
	"testing"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/role"
	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/user"
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/permission"
	"github.com/iota-uz/iota-sdk/modules/core/domain/value_objects/internet"
	"github.com/stretchr/testify/require"
)

func TestPermissionClaimsReturnsOnlyRequestedEffectivePermissions(t *testing.T) {
	t.Parallel()

	direct := testPermission("Ali.Config")
	viaRole := testPermission("Ali.Eval")
	viaGroup := testPermission("Ali.Logs")
	email, err := internet.NewEmail("oidc-permissions@example.com")
	require.NoError(t, err)
	entity := user.New(
		"OIDC",
		"User",
		email,
		user.UILanguageEN,
		user.WithPermissions([]permission.Permission{direct}),
		user.WithRoles([]role.Role{role.New("evaluator", role.WithPermissions([]permission.Permission{viaRole}))}),
		user.WithGroupPermissions([]permission.Permission{viaGroup}),
	)

	claims, requested := permissionClaims(entity, []string{
		"openid",
		"permission:Ali.Logs",
		"permission:Ali.Config",
		"permission:Ali.Missing",
	})

	require.True(t, requested)
	require.Equal(t, []string{"Ali.Config", "Ali.Logs"}, claims)
	// Falsely green if the helper returns every effective grant or ignores
	// group-inherited permissions instead of intersecting requested scopes.
	require.NotContains(t, claims, viaRole.Name())
}

func TestPermissionClaimsDistinguishesUnrequestedFromDenied(t *testing.T) {
	t.Parallel()

	email, err := internet.NewEmail("oidc-no-permissions@example.com")
	require.NoError(t, err)
	entity := user.New("OIDC", "User", email, user.UILanguageEN)

	claims, requested := permissionClaims(entity, []string{"openid"})
	require.False(t, requested)
	require.Nil(t, claims)

	claims, requested = permissionClaims(entity, []string{"openid", "permission:Ali.Config"})
	require.True(t, requested)
	require.Empty(t, claims)
	// Falsely green if an empty permission claim is omitted even when the client
	// explicitly requested a permission scope the user does not have.
}

func testPermission(name string) permission.Permission {
	return permission.New(
		permission.WithID(uuid.New()),
		permission.WithName(name),
		permission.WithResource(permission.Resource(name)),
		permission.WithAction(permission.ActionRead),
		permission.WithModifier(permission.ModifierAll),
	)
}
