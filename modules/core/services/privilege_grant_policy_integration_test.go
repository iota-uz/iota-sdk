package services_test

import (
	"testing"

	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/group"
	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/role"
	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/user"
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/permission"
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/session"
	"github.com/iota-uz/iota-sdk/modules/core/domain/value_objects/internet"
	"github.com/iota-uz/iota-sdk/modules/core/infrastructure/persistence"
	"github.com/iota-uz/iota-sdk/modules/core/permissions"
	"github.com/iota-uz/iota-sdk/modules/core/services"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/itf"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPrivilegeGrantPolicy_ClosesAdministrativeEscalationPaths(t *testing.T) {
	f := setupTest(t)
	tenantID := f.TenantID()

	managementPermissions := []permission.Permission{
		permissions.RoleCreate,
		permissions.RoleUpdate,
		permissions.RoleDelete,
		permissions.UserCreate,
		permissions.UserUpdate,
		permissions.UserDelete,
		permissions.UserUpdateBlockStatus,
		permissions.GroupCreate,
		permissions.GroupUpdate,
		permissions.GroupDelete,
		permissions.SessionDelete,
	}
	strongPermission := permissions.DepartmentDelete
	permissionRepository := persistence.NewPermissionRepository()
	for _, candidate := range append(managementPermissions, strongPermission) {
		require.NoError(t, permissionRepository.Save(f.Ctx, candidate))
	}

	userRepository := persistence.NewUserRepository(persistence.NewUploadRepository())
	actorEmail, err := internet.NewEmail("limited-admin@example.com")
	require.NoError(t, err)
	actor, err := userRepository.Create(f.Ctx, user.New(
		"Limited", "Admin", actorEmail, user.UILanguageEN,
		user.WithTenantID(tenantID),
		user.WithPermissions(managementPermissions),
	))
	require.NoError(t, err)
	ctx := composables.WithUser(f.Ctx, actor)

	roleRepository := persistence.NewRoleRepository()
	strongRole, err := roleRepository.Create(ctx, role.New(
		"Strong role",
		role.WithTenantID(tenantID),
		role.WithPermissions([]permission.Permission{strongPermission}),
	))
	require.NoError(t, err)

	roleService := itf.GetService[services.RoleService](f)
	userService := itf.GetService[services.UserService](f)
	groupService := itf.GetService[services.GroupService](f)
	sessionService := itf.GetService[services.SessionService](f)
	require.NotNil(t, roleService)
	require.NotNil(t, userService)
	require.NotNil(t, groupService)
	require.NotNil(t, sessionService)

	t.Run("role grant ceiling", func(t *testing.T) {
		// Falsely green if the role is rejected only by form filtering.
		_, err := roleService.Create(ctx, role.New(
			"Forged elevated role",
			role.WithPermissions([]permission.Permission{strongPermission}),
		))
		require.Error(t, err)
		require.ErrorIs(t, err, composables.ErrForbidden)
	})

	t.Run("stronger role cannot be weakened or deleted", func(t *testing.T) {
		// Falsely green if only newly selected permissions are checked and the current role is ignored.
		updateErr := roleService.Update(ctx, strongRole.SetPermissions(nil))
		require.Error(t, updateErr)
		require.ErrorIs(t, updateErr, composables.ErrForbidden)
		deleteErr := roleService.Delete(ctx, strongRole.ID())
		require.Error(t, deleteErr)
		require.ErrorIs(t, deleteErr, composables.ErrForbidden)
		reloaded, reloadErr := roleRepository.GetByID(ctx, strongRole.ID())
		require.NoError(t, reloadErr)
		assert.Len(t, reloaded.Permissions(), 1)
	})

	t.Run("second account escalation", func(t *testing.T) {
		email, emailErr := internet.NewEmail("second-account@example.com")
		require.NoError(t, emailErr)
		// Falsely green if only direct permissions, rather than assigned roles, are checked.
		_, err := userService.Create(ctx, user.New(
			"Second", "Account", email, user.UILanguageEN,
			user.WithTenantID(tenantID),
			user.WithRoles([]role.Role{strongRole}),
		))
		require.Error(t, err)
		require.ErrorIs(t, err, composables.ErrForbidden)
	})

	t.Run("group indirection escalation", func(t *testing.T) {
		// Falsely green if group roles are omitted from the desired post-state.
		_, err := groupService.Create(ctx, group.New(
			"Forged elevated group",
			group.WithRoles([]role.Role{strongRole}),
		))
		require.Error(t, err)
		require.ErrorIs(t, err, composables.ErrForbidden)
	})

	groupRepository := persistence.NewGroupRepository(userRepository, roleRepository)
	strongGroup, err := groupRepository.Save(ctx, group.New(
		"Existing strong group",
		group.WithTenantID(tenantID),
		group.WithRoles([]role.Role{strongRole}),
	))
	require.NoError(t, err)
	weakGroup, err := groupRepository.Save(ctx, group.New(
		"Existing weak group",
		group.WithTenantID(tenantID),
	))
	require.NoError(t, err)

	t.Run("existing group paths cannot bypass the ceiling", func(t *testing.T) {
		// Falsely green if only Group.Create is protected while update, delete, membership, or AssignRole bypass the policy.
		_, updateErr := groupService.Update(ctx, strongGroup.SetRoles(nil))
		require.Error(t, updateErr)
		require.ErrorIs(t, updateErr, composables.ErrForbidden)
		deleteErr := groupService.Delete(ctx, strongGroup.ID())
		require.Error(t, deleteErr)
		require.ErrorIs(t, deleteErr, composables.ErrForbidden)
		_, assignErr := groupService.AssignRole(ctx, weakGroup.ID(), strongRole)
		require.Error(t, assignErr)
		require.ErrorIs(t, assignErr, composables.ErrForbidden)
	})

	targetEmail, err := internet.NewEmail("strong-target@example.com")
	require.NoError(t, err)
	target, err := userRepository.Create(ctx, user.New(
		"Strong", "Target", targetEmail, user.UILanguageEN,
		user.WithTenantID(tenantID),
		user.WithPermissions([]permission.Permission{strongPermission}),
	))
	require.NoError(t, err)

	weakEmail, err := internet.NewEmail("weak-membership-target@example.com")
	require.NoError(t, err)
	weakTarget, err := userRepository.Create(ctx, user.New(
		"Weak", "Membership Target", weakEmail, user.UILanguageEN,
		user.WithTenantID(tenantID),
	))
	require.NoError(t, err)

	t.Run("membership in a privileged group is denied", func(t *testing.T) {
		// Falsely green if user updates are protected but GroupService.AddUser can still grant the group's roles.
		_, addErr := groupService.AddUser(ctx, strongGroup.ID(), weakTarget)
		require.Error(t, addErr)
		require.ErrorIs(t, addErr, composables.ErrForbidden)
		reloaded, reloadErr := userRepository.GetByID(ctx, weakTarget.ID())
		require.NoError(t, reloadErr)
		assert.Empty(t, reloaded.GroupIDs())
	})

	t.Run("credential takeover", func(t *testing.T) {
		changedEmail, emailErr := internet.NewEmail("taken-over@example.com")
		require.NoError(t, emailErr)
		// Falsely green if only role changes are checked and credential fields bypass target dominance.
		_, err := userService.Update(ctx, target.SetEmail(changedEmail))
		require.Error(t, err)
		require.ErrorIs(t, err, composables.ErrForbidden)
		reloaded, reloadErr := userRepository.GetByID(ctx, target.ID())
		require.NoError(t, reloadErr)
		assert.Equal(t, targetEmail.Value(), reloaded.Email().Value())
	})

	t.Run("delete and block stronger target", func(t *testing.T) {
		// Falsely green if destructive side effects happen before target dominance is evaluated.
		_, deleteErr := userService.Delete(ctx, target.ID())
		require.Error(t, deleteErr)
		require.ErrorIs(t, deleteErr, composables.ErrForbidden)
		_, blockErr := userService.BlockUser(ctx, target.ID(), "security test")
		require.Error(t, blockErr)
		require.ErrorIs(t, blockErr, composables.ErrForbidden)
		reloaded, reloadErr := userRepository.GetByID(ctx, target.ID())
		require.NoError(t, reloadErr)
		assert.False(t, reloaded.IsBlocked())
	})

	t.Run("session revocation for stronger target", func(t *testing.T) {
		const token = "strong-target-session"
		sessionRepository := persistence.NewSessionRepository()
		require.NoError(t, sessionRepository.Create(ctx, session.New(token, target.ID(), tenantID, "127.0.0.1", "test")))
		// Falsely green if the service trusts the submitted token without authorizing its owner.
		err := sessionService.TerminateUserSession(ctx, target.ID(), token)
		require.Error(t, err)
		require.ErrorIs(t, err, composables.ErrForbidden)
		_, reloadErr := sessionRepository.GetByToken(ctx, token)
		require.NoError(t, reloadErr)
	})

	t.Run("generic self update", func(t *testing.T) {
		// Falsely green if the generic route silently delegates to UpdateSelf.
		_, err := userService.Update(ctx, actor.SetName("Changed", actor.LastName(), actor.MiddleName()))
		require.Error(t, err)
		require.ErrorIs(t, err, composables.ErrForbidden)
	})

	t.Run("system role assignment requires the explicit permission", func(t *testing.T) {
		require.NoError(t, permissionRepository.Save(ctx, permissions.RoleAssignSystem))
		systemRole, createErr := roleRepository.Create(ctx, role.New(
			"System assignment test",
			role.WithType(role.TypeSystem),
			role.WithTenantID(tenantID),
		))
		require.NoError(t, createErr)
		withoutPermissionEmail, emailErr := internet.NewEmail("system-role-denied@example.com")
		require.NoError(t, emailErr)
		// Falsely green if system roles are treated like empty user roles and the explicit assignment gate is skipped.
		_, deniedErr := userService.Create(ctx, user.New(
			"System Role", "Denied", withoutPermissionEmail, user.UILanguageEN,
			user.WithTenantID(tenantID),
			user.WithRoles([]role.Role{systemRole}),
		))
		require.Error(t, deniedErr)
		require.ErrorIs(t, deniedErr, composables.ErrForbidden)

		reloadedActor, reloadErr := userRepository.GetByID(ctx, actor.ID())
		require.NoError(t, reloadErr)
		require.NoError(t, userRepository.Update(ctx, reloadedActor.AddPermission(permissions.RoleAssignSystem)))
		withPermissionEmail, emailErr := internet.NewEmail("system-role-allowed@example.com")
		require.NoError(t, emailErr)
		created, createErr := userService.Create(ctx, user.New(
			"System Role", "Allowed", withPermissionEmail, user.UILanguageEN,
			user.WithTenantID(tenantID),
			user.WithRoles([]role.Role{systemRole}),
		))
		require.NoError(t, createErr)
		assert.Len(t, created.Roles(), 1)
	})
}
