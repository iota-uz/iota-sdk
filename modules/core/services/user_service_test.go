package services_test

import (
	"testing"

	"github.com/iota-uz/iota-sdk/modules"
	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/user"
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/permission"
	"github.com/iota-uz/iota-sdk/modules/core/domain/value_objects/internet"
	"github.com/iota-uz/iota-sdk/modules/core/infrastructure/persistence"
	"github.com/iota-uz/iota-sdk/modules/core/permissions"
	"github.com/iota-uz/iota-sdk/modules/core/services"
	"github.com/iota-uz/iota-sdk/modules/core/validators"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/eventbus"
	"github.com/iota-uz/iota-sdk/pkg/itf"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// setupTestWithPermissions creates test environment with specified permissions
func setupTestWithPermissions(t *testing.T, permissions ...*permission.Permission) *itf.TestEnvironment {
	t.Helper()

	user := itf.User(permissions...)
	return itf.Setup(t,
		itf.WithModules(modules.BuiltInModules...),
		itf.WithUser(user),
	)
}

func TestUserService_CanUserBeDeleted(t *testing.T) {
	t.Parallel()
	f := setupTest(t)

	// Create required dependencies
	uploadRepository := persistence.NewUploadRepository()
	userRepository := persistence.NewUserRepository(uploadRepository)
	userValidator := validators.NewUserValidator(userRepository)
	eventBus := eventbus.NewEventPublisher(logrus.New())
	userService := services.NewUserService(userRepository, userValidator, eventBus)

	tenant, err := composables.UseTenantID(f.Ctx)
	require.NoError(t, err)

	t.Run("System_User_Cannot_Be_Deleted", func(t *testing.T) {
		// Create system user
		email, err := internet.NewEmail("system@test.com")
		require.NoError(t, err)
		systemUser := user.New("System", "User", email, user.UILanguageEN,
			user.WithType(user.TypeSystem),
			user.WithTenantID(tenant))

		createdUser, err := userRepository.Create(f.Ctx, systemUser)
		require.NoError(t, err)

		// Test CanUserBeDeleted
		canDelete, err := userService.CanUserBeDeleted(f.Ctx, createdUser.ID())
		require.NoError(t, err)
		assert.False(t, canDelete, "System user should not be deletable")
	})

	t.Run("Last_User_In_Tenant_Cannot_Be_Deleted", func(t *testing.T) {
		// Create new tenant with single user
		secondTenant, err := itf.CreateTestTenant(f.Ctx, f.Pool)
		require.NoError(t, err)

		email, err := internet.NewEmail("lastuser@test.com")
		require.NoError(t, err)
		regularUser := user.New("Last", "User", email, user.UILanguageEN,
			user.WithType(user.TypeUser),
			user.WithTenantID(secondTenant.ID))

		// Switch context to second tenant
		secondTenantCtx := composables.WithTenantID(f.Ctx, secondTenant.ID)
		createdUser, err := userRepository.Create(secondTenantCtx, regularUser)
		require.NoError(t, err)

		// Test CanUserBeDeleted
		canDelete, err := userService.CanUserBeDeleted(secondTenantCtx, createdUser.ID())
		require.NoError(t, err)
		assert.False(t, canDelete, "Last user in tenant should not be deletable")
	})

	t.Run("Non_Last_User_In_Tenant_Can_Be_Deleted", func(t *testing.T) {
		// Create multiple users in tenant
		email1, err := internet.NewEmail("user1@test.com")
		require.NoError(t, err)
		user1 := user.New("User", "One", email1, user.UILanguageEN,
			user.WithType(user.TypeUser),
			user.WithTenantID(tenant))

		email2, err := internet.NewEmail("user2@test.com")
		require.NoError(t, err)
		user2 := user.New("User", "Two", email2, user.UILanguageEN,
			user.WithType(user.TypeUser),
			user.WithTenantID(tenant))

		createdUser1, err := userRepository.Create(f.Ctx, user1)
		require.NoError(t, err)
		_, err = userRepository.Create(f.Ctx, user2)
		require.NoError(t, err)

		// Test CanUserBeDeleted - should be true since there are multiple users
		canDelete, err := userService.CanUserBeDeleted(f.Ctx, createdUser1.ID())
		require.NoError(t, err)
		assert.True(t, canDelete, "Non-last user in tenant should be deletable")
	})

	t.Run("NonExistent_User", func(t *testing.T) {
		nonExistentID := uint(99999)

		canDelete, err := userService.CanUserBeDeleted(f.Ctx, nonExistentID)
		require.Error(t, err)
		assert.False(t, canDelete, "Non-existent user should return false with error")
		assert.ErrorIs(t, err, persistence.ErrUserNotFound)
	})
}

func TestUserService_Delete_SelfDeletionPrevention(t *testing.T) {
	t.Parallel()
	f := setupTestWithPermissions(t, permissions.UserDelete)

	// Create required dependencies
	uploadRepository := persistence.NewUploadRepository()
	userRepository := persistence.NewUserRepository(uploadRepository)
	userValidator := validators.NewUserValidator(userRepository)
	eventBus := eventbus.NewEventPublisher(logrus.New())
	userService := services.NewUserService(userRepository, userValidator, eventBus)

	tenant, err := composables.UseTenantID(f.Ctx)
	require.NoError(t, err)

	t.Run("Delete_Last_User_In_Tenant_Should_Fail", func(t *testing.T) {
		// Create new tenant with single user
		isolatedTenant, err := itf.CreateTestTenant(f.Ctx, f.Pool)
		require.NoError(t, err)

		email, err := internet.NewEmail("lonely@test.com")
		require.NoError(t, err)
		lonelyUser := user.New("Lonely", "User", email, user.UILanguageEN,
			user.WithType(user.TypeUser),
			user.WithTenantID(isolatedTenant.ID))

		// Switch context to isolated tenant
		isolatedTenantCtx := composables.WithTenantID(f.Ctx, isolatedTenant.ID)
		createdUser, err := userRepository.Create(isolatedTenantCtx, lonelyUser)
		require.NoError(t, err)

		// Attempt to delete the last user
		_, err = userService.Delete(isolatedTenantCtx, createdUser.ID())
		require.Error(t, err)
		assert.Contains(t, err.Error(), "cannot delete the last user in tenant")
	})

	t.Run("Delete_Non_Last_User_Should_Succeed", func(t *testing.T) {
		// Create multiple users in tenant
		email1, err := internet.NewEmail("deletable1@test.com")
		require.NoError(t, err)
		deletableUser := user.New("Deletable", "User", email1, user.UILanguageEN,
			user.WithType(user.TypeUser),
			user.WithTenantID(tenant))

		email2, err := internet.NewEmail("keeper@test.com")
		require.NoError(t, err)
		keeperUser := user.New("Keeper", "User", email2, user.UILanguageEN,
			user.WithType(user.TypeUser),
			user.WithTenantID(tenant))

		createdDeletableUser, err := userRepository.Create(f.Ctx, deletableUser)
		require.NoError(t, err)
		_, err = userRepository.Create(f.Ctx, keeperUser)
		require.NoError(t, err)

		// Delete one user (should succeed)
		deletedUser, err := userService.Delete(f.Ctx, createdDeletableUser.ID())
		require.NoError(t, err)
		require.NotNil(t, deletedUser)
		assert.Equal(t, createdDeletableUser.ID(), deletedUser.ID())

		// Verify user is actually deleted
		_, err = userRepository.GetByID(f.Ctx, createdDeletableUser.ID())
		require.Error(t, err)
		assert.ErrorIs(t, err, persistence.ErrUserNotFound)
	})

	t.Run("System_User_Deletion_Protection_Still_Works", func(t *testing.T) {
		// Create system user
		email, err := internet.NewEmail("systemuser@test.com")
		require.NoError(t, err)
		systemUser := user.New("System", "Admin", email, user.UILanguageEN,
			user.WithType(user.TypeSystem),
			user.WithTenantID(tenant))

		createdSystemUser, err := userRepository.Create(f.Ctx, systemUser)
		require.NoError(t, err)

		// Attempt to delete system user
		_, err = userService.Delete(f.Ctx, createdSystemUser.ID())
		require.Error(t, err)
		assert.Equal(t, composables.ErrForbidden, err, "System user deletion should return ErrForbidden")
	})
}
