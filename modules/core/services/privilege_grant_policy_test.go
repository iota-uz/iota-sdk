package services_test

import (
	"testing"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/group"
	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/role"
	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/user"
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/permission"
	"github.com/iota-uz/iota-sdk/modules/core/domain/value_objects/internet"
	"github.com/iota-uz/iota-sdk/modules/core/permissions"
	"github.com/iota-uz/iota-sdk/modules/core/services"
	"github.com/stretchr/testify/assert"
)

func TestPrivilegeGrantPolicy_DominatesModifiers(t *testing.T) {
	t.Parallel()

	all := permission.New(
		permission.WithID(uuid.New()),
		permission.WithName("Test.Read.All"),
		permission.WithResource("Test"),
		permission.WithAction(permission.ActionRead),
		permission.WithModifier(permission.ModifierAll),
	)
	own := permission.New(
		permission.WithID(uuid.New()),
		permission.WithName("Test.Read.Own"),
		permission.WithResource("Test"),
		permission.WithAction(permission.ActionRead),
		permission.WithModifier(permission.ModifierOwn),
	)
	updateOwn := permission.New(
		permission.WithID(uuid.New()),
		permission.WithName("Test.Update.Own"),
		permission.WithResource("Test"),
		permission.WithAction(permission.ActionUpdate),
		permission.WithModifier(permission.ModifierOwn),
	)

	// Falsely green if dominance compares only resource or only permission IDs.
	assert.True(t, services.Dominates([]permission.Permission{all}, []permission.Permission{own}))
	assert.False(t, services.Dominates([]permission.Permission{own}, []permission.Permission{all}))
	assert.False(t, services.Dominates([]permission.Permission{all}, []permission.Permission{updateOwn}))
}

func TestPrivilegeGrantPolicy_CanManageGroupProjectionMatchesEntity(t *testing.T) {
	t.Parallel()

	tenantID := uuid.New()
	email, err := internet.NewEmail("group-projection@example.com")
	assert.NoError(t, err)
	actor := user.New("Group", "Projection", email, user.UILanguageEN,
		user.WithTenantID(tenantID),
		user.WithPermissions([]permission.Permission{permissions.GroupRead, permissions.GroupUpdate}),
	)
	weak := []permission.Permission{permissions.GroupRead}
	strong := []permission.Permission{permissions.DepartmentDelete}
	policy := &services.PrivilegeGrantPolicy{}

	cases := []struct {
		name        string
		tenantID    uuid.UUID
		groupType   group.Type
		permissions []permission.Permission
		want        bool
	}{
		{name: "dominated user group", tenantID: tenantID, groupType: group.TypeUser, permissions: weak, want: true},
		{name: "group without roles", tenantID: tenantID, groupType: group.TypeUser, want: true},
		{name: "stronger group", tenantID: tenantID, groupType: group.TypeUser, permissions: strong},
		{name: "system group", tenantID: tenantID, groupType: group.TypeSystem, permissions: weak},
		{name: "foreign tenant", tenantID: uuid.New(), groupType: group.TypeUser, permissions: weak},
	}
	for _, tc := range cases {
		entity := group.New(tc.name,
			group.WithTenantID(tc.tenantID),
			group.WithType(tc.groupType),
			group.WithRoles([]role.Role{role.New(tc.name, role.WithTenantID(tc.tenantID), role.WithPermissions(tc.permissions))}),
		)
		// Falsely green if the list projection and the entity check drift apart on any rule.
		assert.Equal(t, tc.want, policy.CanManageGroupProjection(actor, tc.tenantID, tc.groupType, tc.permissions), tc.name)
		assert.Equal(t, tc.want, policy.CanManageGroup(actor, entity), tc.name)
	}
	assert.False(t, policy.CanManageGroupProjection(nil, tenantID, group.TypeUser, weak))
}
