package persistence_test

import (
	"testing"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/group"
	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/role"
	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/user"
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/permission"
	"github.com/iota-uz/iota-sdk/modules/core/domain/value_objects/internet"
	"github.com/iota-uz/iota-sdk/modules/core/infrastructure/persistence"
	"github.com/iota-uz/iota-sdk/modules/core/permissions"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/itf"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPgUserRepository_EffectivePermissionsIncludeGroupRoles(t *testing.T) {
	f := setupTest(t)
	permissionRepository := persistence.NewPermissionRepository()
	require.NoError(t, permissionRepository.Save(f.Ctx, permissions.DepartmentDelete))

	roleRepository := persistence.NewRoleRepository()
	groupRole, err := roleRepository.Create(f.Ctx, role.New(
		"Group-only permission",
		role.WithTenantID(f.TenantID()),
		role.WithPermissions([]permission.Permission{permissions.DepartmentDelete}),
	))
	require.NoError(t, err)

	userRepository := persistence.NewUserRepository(persistence.NewUploadRepository())
	email, err := internet.NewEmail("group-member@example.com")
	require.NoError(t, err)
	member, err := userRepository.Create(f.Ctx, user.New(
		"Group", "Member", email, user.UILanguageEN,
		user.WithTenantID(f.TenantID()),
	))
	require.NoError(t, err)

	groupRepository := persistence.NewGroupRepository(userRepository, roleRepository)
	_, err = groupRepository.Save(f.Ctx, group.New(
		"Permission group",
		group.WithTenantID(f.TenantID()),
		group.WithRoles([]role.Role{groupRole}),
		group.WithUsers([]user.User{member}),
	))
	require.NoError(t, err)

	reloaded, err := userRepository.GetByID(f.Ctx, member.ID())
	require.NoError(t, err)
	// Falsely green if the permission is also attached directly or through user_roles.
	assert.Empty(t, reloaded.Permissions())
	assert.Empty(t, reloaded.Roles())
	assert.True(t, reloaded.Can(permissions.DepartmentDelete))
}

func TestRepositories_RejectCrossTenantAdministrativeRelationsAtomically(t *testing.T) {
	f := setupTest(t)
	secondTenant, err := itf.CreateTestTenant(f.Ctx, f.Pool)
	require.NoError(t, err)
	secondTenantCtx := composables.WithTenantID(f.Ctx, secondTenant.ID)

	permissionRepository := persistence.NewPermissionRepository()
	require.NoError(t, permissionRepository.Save(f.Ctx, permissions.DepartmentDelete))
	roleRepository := persistence.NewRoleRepository()
	foreignRole, err := roleRepository.Create(secondTenantCtx, role.New(
		"Foreign role",
		role.WithTenantID(secondTenant.ID),
		role.WithPermissions([]permission.Permission{permissions.DepartmentDelete}),
	))
	require.NoError(t, err)

	userRepository := persistence.NewUserRepository(persistence.NewUploadRepository())
	groupRepository := persistence.NewGroupRepository(userRepository, roleRepository)
	forgedGroup := group.New(
		"Forged group",
		group.WithTenantID(f.TenantID()),
		group.WithRoles([]role.Role{foreignRole}),
	)
	// Falsely green if the group row is inserted before an invalid relation is skipped.
	_, err = groupRepository.Save(f.Ctx, forgedGroup)
	require.Error(t, err)
	_, lookupErr := groupRepository.GetByID(f.Ctx, forgedGroup.ID())
	require.Error(t, lookupErr)

	foreignGroup, err := groupRepository.Save(secondTenantCtx, group.New(
		"Foreign group",
		group.WithTenantID(secondTenant.ID),
	))
	require.NoError(t, err)
	_, err = groupRepository.GetByID(f.Ctx, foreignGroup.ID())
	require.Error(t, err)

	email, err := internet.NewEmail("forged-membership@example.com")
	require.NoError(t, err)
	forgedUser := user.New(
		"Forged", "Membership", email, user.UILanguageEN,
		user.WithTenantID(f.TenantID()),
		user.WithGroupIDs([]uuid.UUID{foreignGroup.ID()}),
	)
	// Falsely green if the user is created while only the foreign membership is dropped.
	_, err = userRepository.Create(f.Ctx, forgedUser)
	require.Error(t, err)
	exists, existsErr := userRepository.EmailExists(f.Ctx, email.Value())
	require.NoError(t, existsErr)
	assert.False(t, exists)
}
