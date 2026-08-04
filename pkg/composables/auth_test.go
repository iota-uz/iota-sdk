package composables

import (
	"testing"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/role"
	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/user"
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/permission"
	"github.com/iota-uz/iota-sdk/modules/core/domain/value_objects/internet"
	"github.com/stretchr/testify/require"
)

func TestEffectivePermissionNames_IncludesRolePermissions(t *testing.T) {
	tenantID := uuid.New()
	direct := permission.New(
		permission.WithName("Client.Read"),
		permission.WithResource(permission.Resource("Client")),
		permission.WithAction(permission.ActionRead),
	)
	rolePerm := permission.New(
		permission.WithName("Portfolio.Read"),
		permission.WithResource(permission.Resource("Portfolio")),
		permission.WithAction(permission.ActionRead),
	)
	adminRole := role.New("Admin", role.WithTenantID(tenantID), role.WithPermissions([]permission.Permission{rolePerm}))
	u := user.New(
		"Test",
		"User",
		internet.MustParseEmail("test@example.com"),
		user.UILanguageRU,
		user.WithTenantID(tenantID),
		user.WithPermissions([]permission.Permission{direct}),
		user.WithRoles([]role.Role{adminRole}),
	)

	require.Equal(t, []string{"Client.Read", "Portfolio.Read"}, EffectivePermissionNames(u))
	require.Equal(t, []string{"Admin"}, RoleNames(u))
}
