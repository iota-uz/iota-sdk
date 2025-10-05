package services_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/modules"
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
		defer tx.Rollback(f.Ctx)

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
		defer tx1.Rollback(f.Ctx)

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
		defer tx2.Rollback(f.Ctx)

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
		defer tx.Rollback(f.Ctx)

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
		defer tx.Rollback(f.Ctx)

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
		defer tx.Rollback(context.Background())

		logger := logrus.New()
		// Create context WITHOUT tenant ID
		ctxWithTx := context.WithValue(composables.WithTx(context.Background(), tx), constants.LoggerKey, logrus.NewEntry(logger))

		_, err = populateService.Execute(ctxWithTx, req)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to get tenant ID")
	})
}
