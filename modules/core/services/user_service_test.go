package services_test

import (
	"context"
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

// userCommittedCtx returns a context without the test transaction, so data saved
// with this context will be committed immediately and visible to InTx operations.
func userCommittedCtx(fixtures *itf.TestEnvironment) context.Context {
	ctx := context.Background()
	ctx = composables.WithPool(ctx, fixtures.Pool)
	ctx = composables.WithTenantID(ctx, fixtures.TenantID())
	ctx = composables.WithParams(ctx, itf.DefaultParams())
	ctx = composables.WithSession(ctx, itf.MockSession())
	ctx = composables.WithUser(ctx, fixtures.User)
	return ctx
}

// setupTestWithPermissions creates test environment with specified permissions
func setupTestWithPermissions(t *testing.T, permissions ...permission.Permission) *itf.TestEnvironment {
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
		// Use committed context so data is visible to InTx operations
		ctx := userCommittedCtx(f)

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

		createdDeletableUser, err := userRepository.Create(ctx, deletableUser)
		require.NoError(t, err)
		_, err = userRepository.Create(ctx, keeperUser)
		require.NoError(t, err)

		// Delete one user (should succeed)
		deletedUser, err := userService.Delete(ctx, createdDeletableUser.ID())
		require.NoError(t, err)
		require.NotNil(t, deletedUser)
		assert.Equal(t, createdDeletableUser.ID(), deletedUser.ID())

		// Verify user is actually deleted
		_, err = userRepository.GetByID(ctx, createdDeletableUser.ID())
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

func TestUserService_Update_SelfUpdatePermission(t *testing.T) {
	t.Parallel()

	t.Run("Any_Authenticated_User_Can_Update_Self_Via_UpdateSelf", func(t *testing.T) {
		// Setup test environment without any special permissions
		// Self-updates are allowed for all authenticated users via UpdateSelf method
		f := setupTestWithPermissions(t)

		uploadRepository := persistence.NewUploadRepository()
		userRepository := persistence.NewUserRepository(uploadRepository)
		userValidator := validators.NewUserValidator(userRepository)
		eventBus := eventbus.NewEventPublisher(logrus.New())
		userService := services.NewUserService(userRepository, userValidator, eventBus)

		tenant, err := composables.UseTenantID(f.Ctx)
		require.NoError(t, err)

		// Create a user
		email, err := internet.NewEmail("selfupdate@test.com")
		require.NoError(t, err)
		testUser := user.New("Self", "Update", email, user.UILanguageEN,
			user.WithType(user.TypeUser),
			user.WithTenantID(tenant))

		createdUser, err := userRepository.Create(f.Ctx, testUser)
		require.NoError(t, err)

		// Set the created user as the current user in context
		ctx := composables.WithUser(f.Ctx, createdUser)

		// Update the user's own information via UpdateSelf
		updatedUser := createdUser.SetName("NewFirst", "NewLast", createdUser.MiddleName())

		// Should succeed without any special permission when using UpdateSelf
		result, err := userService.UpdateSelf(ctx, updatedUser)
		require.NoError(t, err)
		assert.Equal(t, "NewFirst", result.FirstName())
		assert.Equal(t, "NewLast", result.LastName())
	})

	t.Run("User_Without_Admin_Permission_Cannot_Update_Others", func(t *testing.T) {
		// Setup test environment without UserUpdate permission
		f := setupTestWithPermissions(t)

		uploadRepository := persistence.NewUploadRepository()
		userRepository := persistence.NewUserRepository(uploadRepository)
		userValidator := validators.NewUserValidator(userRepository)
		eventBus := eventbus.NewEventPublisher(logrus.New())
		userService := services.NewUserService(userRepository, userValidator, eventBus)

		tenant, err := composables.UseTenantID(f.Ctx)
		require.NoError(t, err)

		// Create two users
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
		createdUser2, err := userRepository.Create(f.Ctx, user2)
		require.NoError(t, err)

		// Set user1 as current user in context
		ctx := composables.WithUser(f.Ctx, createdUser1)

		// Try to update user2's information (should fail)
		updatedUser2 := createdUser2.SetName("Hacked", createdUser2.LastName(), createdUser2.MiddleName())

		_, err = userService.Update(ctx, updatedUser2)
		require.Error(t, err)
		assert.Equal(t, composables.ErrForbidden, err)
	})

	t.Run("User_With_Update_Can_Update_Others", func(t *testing.T) {
		// Setup test environment with UserUpdate permission (admin)
		f := setupTestWithPermissions(t, permissions.UserUpdate)

		uploadRepository := persistence.NewUploadRepository()
		userRepository := persistence.NewUserRepository(uploadRepository)
		userValidator := validators.NewUserValidator(userRepository)
		eventBus := eventbus.NewEventPublisher(logrus.New())
		userService := services.NewUserService(userRepository, userValidator, eventBus)

		tenant, err := composables.UseTenantID(f.Ctx)
		require.NoError(t, err)

		// Create two users
		email1, err := internet.NewEmail("admin@test.com")
		require.NoError(t, err)
		adminUser := user.New("Admin", "User", email1, user.UILanguageEN,
			user.WithType(user.TypeUser),
			user.WithTenantID(tenant))

		email2, err := internet.NewEmail("targetuser@test.com")
		require.NoError(t, err)
		targetUser := user.New("Target", "User", email2, user.UILanguageEN,
			user.WithType(user.TypeUser),
			user.WithTenantID(tenant))

		createdAdmin, err := userRepository.Create(f.Ctx, adminUser)
		require.NoError(t, err)
		createdTarget, err := userRepository.Create(f.Ctx, targetUser)
		require.NoError(t, err)

		// Set admin as current user in context
		ctx := composables.WithUser(f.Ctx, createdAdmin)

		// Admin updates target user's information (should succeed)
		updatedTarget := createdTarget.SetName("Modified", "ByAdmin", createdTarget.MiddleName())

		result, err := userService.Update(ctx, updatedTarget)
		require.NoError(t, err)
		assert.Equal(t, "Modified", result.FirstName())
		assert.Equal(t, "ByAdmin", result.LastName())
	})

	t.Run("User_With_Update_Can_Also_Update_Self", func(t *testing.T) {
		// Setup test environment with UserUpdate permission (admin)
		f := setupTestWithPermissions(t, permissions.UserUpdate)

		uploadRepository := persistence.NewUploadRepository()
		userRepository := persistence.NewUserRepository(uploadRepository)
		userValidator := validators.NewUserValidator(userRepository)
		eventBus := eventbus.NewEventPublisher(logrus.New())
		userService := services.NewUserService(userRepository, userValidator, eventBus)

		tenant, err := composables.UseTenantID(f.Ctx)
		require.NoError(t, err)

		// Create admin user
		email, err := internet.NewEmail("selfadmin@test.com")
		require.NoError(t, err)
		adminUser := user.New("Admin", "Self", email, user.UILanguageEN,
			user.WithType(user.TypeUser),
			user.WithTenantID(tenant))

		createdAdmin, err := userRepository.Create(f.Ctx, adminUser)
		require.NoError(t, err)

		// Set admin as current user in context
		ctx := composables.WithUser(f.Ctx, createdAdmin)

		// Admin updates their own information (should succeed with UserUpdate permission)
		updatedAdmin := createdAdmin.SetName("SelfModified", createdAdmin.LastName(), createdAdmin.MiddleName())

		result, err := userService.Update(ctx, updatedAdmin)
		require.NoError(t, err)
		assert.Equal(t, "SelfModified", result.FirstName())
	})

	t.Run("User_With_Only_Read_Permission_Can_Still_Update_Self_Via_UpdateSelf", func(t *testing.T) {
		// Setup test environment with only UserRead permission (no UserUpdate)
		f := setupTestWithPermissions(t, permissions.UserRead)

		uploadRepository := persistence.NewUploadRepository()
		userRepository := persistence.NewUserRepository(uploadRepository)
		userValidator := validators.NewUserValidator(userRepository)
		eventBus := eventbus.NewEventPublisher(logrus.New())
		userService := services.NewUserService(userRepository, userValidator, eventBus)

		tenant, err := composables.UseTenantID(f.Ctx)
		require.NoError(t, err)

		// Create user
		email, err := internet.NewEmail("readonly@test.com")
		require.NoError(t, err)
		testUser := user.New("Read", "Only", email, user.UILanguageEN,
			user.WithType(user.TypeUser),
			user.WithTenantID(tenant))

		createdUser, err := userRepository.Create(f.Ctx, testUser)
		require.NoError(t, err)

		// Set user as current user in context
		ctx := composables.WithUser(f.Ctx, createdUser)

		// Update own information via UpdateSelf - should succeed even without UserUpdate permission
		updatedUser := createdUser.SetName("Updated", "Successfully", createdUser.MiddleName())

		result, err := userService.UpdateSelf(ctx, updatedUser)
		require.NoError(t, err)
		assert.Equal(t, "Updated", result.FirstName())
		assert.Equal(t, "Successfully", result.LastName())
	})
}

func TestUserService_UpdateSelf_SecurityValidation(t *testing.T) {
	t.Parallel()

	t.Run("UpdateSelf_Prevents_Cross_User_Updates", func(t *testing.T) {
		f := setupTestWithPermissions(t)

		uploadRepository := persistence.NewUploadRepository()
		userRepository := persistence.NewUserRepository(uploadRepository)
		userValidator := validators.NewUserValidator(userRepository)
		eventBus := eventbus.NewEventPublisher(logrus.New())
		userService := services.NewUserService(userRepository, userValidator, eventBus)

		tenant, err := composables.UseTenantID(f.Ctx)
		require.NoError(t, err)

		// Create two users
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
		createdUser2, err := userRepository.Create(f.Ctx, user2)
		require.NoError(t, err)

		// Set user1 as current user in context
		ctx := composables.WithUser(f.Ctx, createdUser1)

		// Try to update user2's information using UpdateSelf (should fail)
		updatedUser2 := createdUser2.SetName("Hacked", "Name", createdUser2.MiddleName())

		_, err = userService.UpdateSelf(ctx, updatedUser2)
		require.Error(t, err)
		assert.Equal(t, composables.ErrForbidden, err)
	})

	t.Run("UpdateSelf_Preserves_Roles_And_Permissions", func(t *testing.T) {
		f := setupTestWithPermissions(t, permissions.UserRead)

		uploadRepository := persistence.NewUploadRepository()
		userRepository := persistence.NewUserRepository(uploadRepository)
		userValidator := validators.NewUserValidator(userRepository)
		eventBus := eventbus.NewEventPublisher(logrus.New())
		userService := services.NewUserService(userRepository, userValidator, eventBus)

		tenant, err := composables.UseTenantID(f.Ctx)
		require.NoError(t, err)

		// Create a user with specific permissions
		email, err := internet.NewEmail("testuser@test.com")
		require.NoError(t, err)
		testUser := user.New("Test", "User", email, user.UILanguageEN,
			user.WithType(user.TypeUser),
			user.WithTenantID(tenant),
			user.WithPermissions([]permission.Permission{permissions.UserRead}))

		createdUser, err := userRepository.Create(f.Ctx, testUser)
		require.NoError(t, err)

		// Set user as current user in context
		ctx := composables.WithUser(f.Ctx, createdUser)

		// Try to escalate privileges by modifying permissions
		// Create an admin permission to attempt privilege escalation
		adminPermissions := []permission.Permission{permissions.UserUpdate, permissions.UserDelete}
		maliciousUpdate := createdUser.
			SetName("Hacker", "User", createdUser.MiddleName()).
			SetPermissions(adminPermissions)

		// UpdateSelf should preserve original permissions
		result, err := userService.UpdateSelf(ctx, maliciousUpdate)
		require.NoError(t, err)

		// Name should be updated
		assert.Equal(t, "Hacker", result.FirstName())
		assert.Equal(t, "User", result.LastName())

		// But permissions should remain original (UserRead only)
		assert.Equal(t, 1, len(result.Permissions()))
		assert.Equal(t, permissions.UserRead.ID(), result.Permissions()[0].ID())
	})
}
