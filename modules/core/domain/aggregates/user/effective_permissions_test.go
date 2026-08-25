package user_test

import (
	"testing"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/role"
	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/user"
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/permission"
	"github.com/iota-uz/iota-sdk/modules/core/domain/value_objects/internet"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEffectivePermissions_IncludeGroupRolePermissions(t *testing.T) {
	t.Parallel()

	groupPermission := permission.New(
		permission.WithID(uuid.New()),
		permission.WithName("Test.Group.Read"),
		permission.WithResource("TestGroup"),
		permission.WithAction(permission.ActionRead),
		permission.WithModifier(permission.ModifierAll),
	)
	email, err := internet.NewEmail("group-effective@example.com")
	require.NoError(t, err)
	entity := user.New(
		"Group", "Member", email, user.UILanguageEN,
		user.WithRoles([]role.Role{}),
		user.WithGroupPermissions([]permission.Permission{groupPermission}),
	)

	// Falsely green if this test supplies the permission as a direct user permission.
	assert.True(t, entity.Can(groupPermission))
	assert.Equal(t, []permission.Permission{groupPermission}, user.EffectivePermissions(entity))
}
