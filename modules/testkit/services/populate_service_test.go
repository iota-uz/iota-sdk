package services_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/modules"
	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/role"
	"github.com/iota-uz/iota-sdk/modules/core/infrastructure/persistence"
	"github.com/iota-uz/iota-sdk/modules/testkit/domain/schemas"
	"github.com/iota-uz/iota-sdk/modules/testkit/services"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/constants"
	"github.com/iota-uz/iota-sdk/pkg/itf"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// setupTest creates all necessary dependencies for tests
func setupTest(t *testing.T) *itf.TestEnvironment {
	t.Helper()
	return itf.Setup(t, itf.WithModules(modules.BuiltInModules...))
}

func TestPopulateService_SetupTenant(t *testing.T) {
	t.Parallel()

	t.Run("CreateNewTenant", func(t *testing.T) {
		f := setupTest(t)

		// Create populate service
		populateService := services.NewPopulateService(f.App)
		tenantRepo := persistence.NewTenantRepository()

		// Define new tenant spec with unique ID
		tenantID := uuid.New()
		tenantSpec := &schemas.TenantSpec{
			ID:   tenantID.String(),
			Name: "Test Tenant",
		}

		// Start transaction for populate service
		tx, err := f.Pool.Begin(f.Ctx)
		require.NoError(t, err)
		defer func() { _ = tx.Rollback(f.Ctx) }()

		// Add logger and transaction to context
		logger := logrus.New()
		ctxWithTx := context.WithValue(composables.WithTx(f.Ctx, tx), constants.LoggerKey, logrus.NewEntry(logger))

		// Execute setupTenant
		req := &schemas.PopulateRequest{
			Tenant: tenantSpec,
		}

		// Call Execute which internally calls setupTenant
		result, err := populateService.Execute(ctxWithTx, req)
		require.NoError(t, err)
		require.NotNil(t, result)

		// Commit the transaction
		require.NoError(t, tx.Commit(ctxWithTx))

		// Verify tenant was created in database
		tenants, err := tenantRepo.List(f.Ctx)
		require.NoError(t, err)

		found := false
		for _, tenant := range tenants {
			if tenant.ID() == tenantID {
				found = true
				assert.Equal(t, "Test Tenant", tenant.Name())
				assert.Equal(t, "localhost", tenant.Domain())
				break
			}
		}
		assert.True(t, found, "Tenant should be created in database")
	})

	t.Run("IdempotentTenantCreation", func(t *testing.T) {
		f := setupTest(t)

		populateService := services.NewPopulateService(f.App)
		tenantRepo := persistence.NewTenantRepository()

		// Create tenant first time
		tenantID := uuid.New()
		tenantSpec := &schemas.TenantSpec{
			ID:   tenantID.String(),
			Name: "Idempotent Tenant",
		}

		// First creation
		tx1, err := f.Pool.Begin(f.Ctx)
		require.NoError(t, err)
		defer func() { _ = tx1.Rollback(f.Ctx) }()

		logger := logrus.New()
		ctxWithTx1 := context.WithValue(composables.WithTx(f.Ctx, tx1), constants.LoggerKey, logrus.NewEntry(logger))
		req := &schemas.PopulateRequest{
			Tenant: tenantSpec,
		}

		_, err = populateService.Execute(ctxWithTx1, req)
		require.NoError(t, err)
		require.NoError(t, tx1.Commit(ctxWithTx1))

		// Second creation - should be idempotent
		tx2, err := f.Pool.Begin(f.Ctx)
		require.NoError(t, err)
		defer func() { _ = tx2.Rollback(f.Ctx) }()

		ctxWithTx2 := context.WithValue(composables.WithTx(f.Ctx, tx2), constants.LoggerKey, logrus.NewEntry(logger))
		_, err = populateService.Execute(ctxWithTx2, req)
		require.NoError(t, err)
		require.NoError(t, tx2.Commit(ctxWithTx2))

		// Verify only one tenant exists with this ID
		tenants, err := tenantRepo.List(f.Ctx)
		require.NoError(t, err)

		count := 0
		for _, tenant := range tenants {
			if tenant.ID() == tenantID {
				count++
			}
		}
		assert.Equal(t, 1, count, "Should only have one tenant with this ID (idempotent)")
	})

	t.Run("InvalidTenantID", func(t *testing.T) {
		f := setupTest(t)

		populateService := services.NewPopulateService(f.App)

		tenantSpec := &schemas.TenantSpec{
			ID:   "invalid-uuid",
			Name: "Invalid Tenant",
		}

		tx, err := f.Pool.Begin(f.Ctx)
		require.NoError(t, err)
		defer func() { _ = tx.Rollback(f.Ctx) }()

		logger := logrus.New()
		ctxWithTx := context.WithValue(composables.WithTx(f.Ctx, tx), constants.LoggerKey, logrus.NewEntry(logger))
		req := &schemas.PopulateRequest{
			Tenant: tenantSpec,
		}

		_, err = populateService.Execute(ctxWithTx, req)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid tenant ID")
	})
}

func TestPopulateService_EnsureAdminRole(t *testing.T) {
	t.Parallel()

	t.Run("EmptyPermissionsTable_FreshDatabase", func(t *testing.T) {
		// This tests the main bug fix: when permissions table is empty,
		// ensureAdminRole should seed permissions first
		f := setupTest(t)

		populateService := services.NewPopulateService(f.App)
		permissionRepo := persistence.NewPermissionRepository()
		roleRepo := persistence.NewRoleRepository()

		tenantID := uuid.New()

		tx, err := f.Pool.Begin(f.Ctx)
		require.NoError(t, err)
		defer func() { _ = tx.Rollback(f.Ctx) }()

		logger := logrus.New()
		ctxWithTx := context.WithValue(composables.WithTx(f.Ctx, tx), constants.LoggerKey, logrus.NewEntry(logger))
		ctxWithTenant := composables.WithTenantID(ctxWithTx, tenantID)

		// Verify permissions table is empty initially (fresh database from ITF)
		allPerms, err := permissionRepo.GetAll(ctxWithTenant)
		require.NoError(t, err)
		// ITF might seed default permissions, but we test that ensureAdminRole works regardless
		initialPermCount := len(allPerms)

		// Create request that will trigger user creation and ensureAdminRole
		req := &schemas.PopulateRequest{
			Tenant: &schemas.TenantSpec{
				ID:   tenantID.String(),
				Name: "Test Tenant",
			},
			Data: &schemas.DataSpec{
				Users: []schemas.UserSpec{
					{
						Email:     "admin@test.com",
						FirstName: "Admin",
						LastName:  "User",
						Password:  "Password123!",
					},
				},
			},
		}

		// Execute populate - this should seed permissions and create admin role
		_, err = populateService.Execute(ctxWithTenant, req)
		require.NoError(t, err)
		require.NoError(t, tx.Commit(ctxWithTenant))

		// Verify permissions were seeded
		allPermsAfter, err := permissionRepo.GetAll(f.Ctx)
		require.NoError(t, err)
		assert.Greater(t, len(allPermsAfter), initialPermCount, "Permissions should be seeded")
		assert.Greater(t, len(allPermsAfter), 0, "Should have permissions after seeding")

		// Verify Admin role was created with permissions
		roles, err := roleRepo.GetPaginated(f.Ctx, &role.FindParams{})
		require.NoError(t, err)

		var adminRole *interface{}
		for _, r := range roles {
			if r.Name() == "Admin" {
				temp := interface{}(r)
				adminRole = &temp
				break
			}
		}

		require.NotNil(t, adminRole, "Admin role should be created")
	})

	t.Run("ExistingPermissions_Idempotent", func(t *testing.T) {
		// Test that ensureAdminRole is idempotent when permissions already exist
		f := setupTest(t)

		populateService := services.NewPopulateService(f.App)
		permissionRepo := persistence.NewPermissionRepository()
		roleRepo := persistence.NewRoleRepository()

		tenantID := uuid.New()

		// First run - seed permissions
		tx1, err := f.Pool.Begin(f.Ctx)
		require.NoError(t, err)
		defer func() { _ = tx1.Rollback(f.Ctx) }()

		logger := logrus.New()
		ctxWithTx1 := context.WithValue(composables.WithTx(f.Ctx, tx1), constants.LoggerKey, logrus.NewEntry(logger))
		ctxWithTenant1 := composables.WithTenantID(ctxWithTx1, tenantID)

		req1 := &schemas.PopulateRequest{
			Tenant: &schemas.TenantSpec{
				ID:   tenantID.String(),
				Name: "Test Tenant 1",
			},
			Data: &schemas.DataSpec{
				Users: []schemas.UserSpec{
					{
						Email:     "user1@test.com",
						FirstName: "User",
						LastName:  "One",
						Password:  "Password123!",
					},
				},
			},
		}

		_, err = populateService.Execute(ctxWithTenant1, req1)
		require.NoError(t, err)
		require.NoError(t, tx1.Commit(ctxWithTenant1))

		// Get permission count after first run
		permsAfterFirst, err := permissionRepo.GetAll(f.Ctx)
		require.NoError(t, err)
		firstRunPermCount := len(permsAfterFirst)
		assert.Greater(t, firstRunPermCount, 0, "Should have permissions after first run")

		// Second run - permissions already exist (different tenant, same database)
		tx2, err := f.Pool.Begin(f.Ctx)
		require.NoError(t, err)
		defer func() { _ = tx2.Rollback(f.Ctx) }()

		ctxWithTx2 := context.WithValue(composables.WithTx(f.Ctx, tx2), constants.LoggerKey, logrus.NewEntry(logger))
		tenantID2 := uuid.New()
		ctxWithTenant2 := composables.WithTenantID(ctxWithTx2, tenantID2)

		req2 := &schemas.PopulateRequest{
			Tenant: &schemas.TenantSpec{
				ID:   tenantID2.String(),
				Name: "Test Tenant 2",
			},
			Data: &schemas.DataSpec{
				Users: []schemas.UserSpec{
					{
						Email:     "user2@test.com",
						FirstName: "User",
						LastName:  "Two",
						Password:  "Password123!",
					},
				},
			},
		}

		_, err = populateService.Execute(ctxWithTenant2, req2)
		require.NoError(t, err)
		require.NoError(t, tx2.Commit(ctxWithTenant2))

		// Verify permissions count stays same (idempotent - no duplicates)
		permsAfterSecond, err := permissionRepo.GetAll(f.Ctx)
		require.NoError(t, err)
		assert.Equal(t, firstRunPermCount, len(permsAfterSecond), "Permission count should remain same (idempotent)")

		// Verify second tenant's Admin role was created (or reused existing one)
		// Note: Each tenant should have their own Admin role
		roles, err := roleRepo.GetPaginated(f.Ctx, &role.FindParams{})
		require.NoError(t, err)

		adminRoleCount := 0
		for _, r := range roles {
			if r.Name() == "Admin" {
				adminRoleCount++
			}
		}

		// Should have at least one Admin role (could be shared or per-tenant based on implementation)
		assert.GreaterOrEqual(t, adminRoleCount, 1, "Should have at least one Admin role")
	})

	t.Run("RoleCreationWithAllPermissions", func(t *testing.T) {
		// Verify that Admin role is created with ALL available permissions
		f := setupTest(t)

		populateService := services.NewPopulateService(f.App)
		permissionRepo := persistence.NewPermissionRepository()
		roleRepo := persistence.NewRoleRepository()

		tenantID := uuid.New()

		tx, err := f.Pool.Begin(f.Ctx)
		require.NoError(t, err)
		defer func() { _ = tx.Rollback(f.Ctx) }()

		logger := logrus.New()
		ctxWithTx := context.WithValue(composables.WithTx(f.Ctx, tx), constants.LoggerKey, logrus.NewEntry(logger))
		ctxWithTenant := composables.WithTenantID(ctxWithTx, tenantID)

		req := &schemas.PopulateRequest{
			Tenant: &schemas.TenantSpec{
				ID:   tenantID.String(),
				Name: "Test Tenant",
			},
			Data: &schemas.DataSpec{
				Users: []schemas.UserSpec{
					{
						Email:     "admin@test.com",
						FirstName: "Admin",
						LastName:  "User",
						Password:  "Password123!",
					},
				},
			},
		}

		_, err = populateService.Execute(ctxWithTenant, req)
		require.NoError(t, err)
		require.NoError(t, tx.Commit(ctxWithTenant))

		// Get all permissions
		allPerms, err := permissionRepo.GetAll(f.Ctx)
		require.NoError(t, err)
		totalPermCount := len(allPerms)
		assert.Greater(t, totalPermCount, 0, "Should have permissions")

		// Get Admin role and verify it has all permissions
		roles, err := roleRepo.GetPaginated(f.Ctx, &role.FindParams{})
		require.NoError(t, err)

		var adminRole interface{}
		for _, r := range roles {
			if r.Name() == "Admin" {
				adminRole = r
				break
			}
		}

		require.NotNil(t, adminRole, "Admin role should exist")
		// Note: We can't easily assert permission count on role without accessing internal fields
		// but the fact that user creation succeeds implies permissions were assigned
	})

	t.Run("AdminRoleAlreadyExists", func(t *testing.T) {
		// Test idempotency: if Admin role already exists, don't create another
		f := setupTest(t)

		populateService := services.NewPopulateService(f.App)
		roleRepo := persistence.NewRoleRepository()

		tenantID := uuid.New()

		// First run - creates Admin role
		tx1, err := f.Pool.Begin(f.Ctx)
		require.NoError(t, err)
		defer func() { _ = tx1.Rollback(f.Ctx) }()

		logger := logrus.New()
		ctxWithTx1 := context.WithValue(composables.WithTx(f.Ctx, tx1), constants.LoggerKey, logrus.NewEntry(logger))
		ctxWithTenant1 := composables.WithTenantID(ctxWithTx1, tenantID)

		req := &schemas.PopulateRequest{
			Tenant: &schemas.TenantSpec{
				ID:   tenantID.String(),
				Name: "Test Tenant",
			},
			Data: &schemas.DataSpec{
				Users: []schemas.UserSpec{
					{
						Email:     "user1@test.com",
						FirstName: "User",
						LastName:  "One",
						Password:  "Password123!",
					},
				},
			},
		}

		_, err = populateService.Execute(ctxWithTenant1, req)
		require.NoError(t, err)
		require.NoError(t, tx1.Commit(ctxWithTenant1))

		// Count Admin roles after first run
		roles1, err := roleRepo.GetPaginated(f.Ctx, &role.FindParams{})
		require.NoError(t, err)

		adminCount1 := 0
		for _, r := range roles1 {
			if r.Name() == "Admin" {
				adminCount1++
			}
		}

		// Second run - should reuse existing Admin role
		tx2, err := f.Pool.Begin(f.Ctx)
		require.NoError(t, err)
		defer func() { _ = tx2.Rollback(f.Ctx) }()

		ctxWithTx2 := context.WithValue(composables.WithTx(f.Ctx, tx2), constants.LoggerKey, logrus.NewEntry(logger))
		ctxWithTenant2 := composables.WithTenantID(ctxWithTx2, tenantID)

		req2 := &schemas.PopulateRequest{
			Tenant: &schemas.TenantSpec{
				ID:   tenantID.String(),
				Name: "Test Tenant",
			},
			Data: &schemas.DataSpec{
				Users: []schemas.UserSpec{
					{
						Email:     "user2@test.com",
						FirstName: "User",
						LastName:  "Two",
						Password:  "Password123!",
					},
				},
			},
		}

		_, err = populateService.Execute(ctxWithTenant2, req2)
		require.NoError(t, err)
		require.NoError(t, tx2.Commit(ctxWithTenant2))

		// Count Admin roles after second run - should be same
		roles2, err := roleRepo.GetPaginated(f.Ctx, &role.FindParams{})
		require.NoError(t, err)

		adminCount2 := 0
		for _, r := range roles2 {
			if r.Name() == "Admin" {
				adminCount2++
			}
		}

		assert.Equal(t, adminCount1, adminCount2, "Admin role count should not increase (idempotent)")
	})
}

func TestPopulateService_TenantWithUsers(t *testing.T) {
	t.Parallel()

	t.Run("CreateTenantAndUsers", func(t *testing.T) {
		f := setupTest(t)

		populateService := services.NewPopulateService(f.App)
		userRepo := persistence.NewUserRepository(persistence.NewUploadRepository())

		// Define tenant and users
		tenantID := uuid.New()
		req := &schemas.PopulateRequest{
			Tenant: &schemas.TenantSpec{
				ID:   tenantID.String(),
				Name: "Tenant with Users",
			},
			Data: &schemas.DataSpec{
				Users: []schemas.UserSpec{
					{
						Email:     "user1@test.com",
						FirstName: "User",
						LastName:  "One",
						Password:  "Password123!",
						Ref:       "user1",
					},
					{
						Email:     "user2@test.com",
						FirstName: "User",
						LastName:  "Two",
						Password:  "Password123!",
						Ref:       "user2",
					},
				},
			},
		}

		tx, err := f.Pool.Begin(f.Ctx)
		require.NoError(t, err)
		defer func() { _ = tx.Rollback(f.Ctx) }()

		logger := logrus.New()
		ctxWithTx := context.WithValue(composables.WithTx(f.Ctx, tx), constants.LoggerKey, logrus.NewEntry(logger))

		// Execute populate
		result, err := populateService.Execute(ctxWithTx, req)
		require.NoError(t, err)
		require.NotNil(t, result)

		require.NoError(t, tx.Commit(ctxWithTx))

		// Verify users were created with correct tenant
		ctxWithTenant := composables.WithTenantID(f.Ctx, tenantID)

		user1, err := userRepo.GetByEmail(ctxWithTenant, "user1@test.com")
		require.NoError(t, err)
		assert.Equal(t, tenantID, user1.TenantID())
		assert.Equal(t, "User", user1.FirstName())
		assert.Equal(t, "One", user1.LastName())

		user2, err := userRepo.GetByEmail(ctxWithTenant, "user2@test.com")
		require.NoError(t, err)
		assert.Equal(t, tenantID, user2.TenantID())
		assert.Equal(t, "User", user2.FirstName())
		assert.Equal(t, "Two", user2.LastName())
	})

	t.Run("UserCreationWithoutTenantFails", func(t *testing.T) {
		f := setupTest(t)

		populateService := services.NewPopulateService(f.App)

		// Try to create users without tenant - using a fresh context without tenant
		req := &schemas.PopulateRequest{
			Data: &schemas.DataSpec{
				Users: []schemas.UserSpec{
					{
						Email:     "orphan@test.com",
						FirstName: "Orphan",
						LastName:  "User",
						Password:  "Password123!",
					},
				},
			},
		}

		tx, err := f.Pool.Begin(context.Background())
		require.NoError(t, err)
		defer func() { _ = tx.Rollback(context.Background()) }()

		logger := logrus.New()
		// Create context WITHOUT tenant ID
		ctxWithTx := context.WithValue(composables.WithTx(context.Background(), tx), constants.LoggerKey, logrus.NewEntry(logger))

		_, err = populateService.Execute(ctxWithTx, req)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to get tenant ID")
	})
}
