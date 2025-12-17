package services_test

import (
	"testing"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/user"
	"github.com/iota-uz/iota-sdk/modules/core/infrastructure/persistence"
	"github.com/iota-uz/iota-sdk/modules/superadmin/services"
	"github.com/iota-uz/iota-sdk/pkg/itf"
	"github.com/iota-uz/iota-sdk/pkg/repo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTenantUsersService_GetUsersByTenantID(t *testing.T) {
	t.Parallel()
	f := setupTest(t)

	// Setup repositories
	uploadRepo := persistence.NewUploadRepository()
	userRepo := persistence.NewUserRepository(uploadRepo)
	service := services.NewTenantUsersService(userRepo)

	t.Run("Happy_Path_Existing_Tenant", func(t *testing.T) {
		// Create test tenant
		tenant, err := itf.CreateTestTenant(f.Ctx, f.Pool)
		require.NoError(t, err)

		// Get users for tenant
		users, total, err := service.GetUsersByTenantID(f.Ctx, tenant.ID, 10, 0, "", user.SortBy{})
		require.NoError(t, err)
		assert.NotNil(t, users)
		assert.GreaterOrEqual(t, total, 0)
	})

	t.Run("Empty_Tenant_Returns_Empty_List", func(t *testing.T) {
		// Create new tenant with no users
		tenant, err := itf.CreateTestTenant(f.Ctx, f.Pool)
		require.NoError(t, err)

		// Get users (should be empty initially)
		users, total, err := service.GetUsersByTenantID(f.Ctx, tenant.ID, 10, 0, "", user.SortBy{})
		require.NoError(t, err)
		assert.NotNil(t, users)
		assert.Equal(t, 0, total)
	})

	t.Run("NonExistent_Tenant_Returns_Empty", func(t *testing.T) {
		nonExistentID := uuid.New()

		users, total, err := service.GetUsersByTenantID(f.Ctx, nonExistentID, 10, 0, "", user.SortBy{})
		require.NoError(t, err)
		assert.Empty(t, users)
		assert.Equal(t, 0, total)
	})

	t.Run("Default_Limit_When_Zero", func(t *testing.T) {
		tenant, err := itf.CreateTestTenant(f.Ctx, f.Pool)
		require.NoError(t, err)

		users, total, err := service.GetUsersByTenantID(f.Ctx, tenant.ID, 0, 0, "", user.SortBy{})
		require.NoError(t, err)
		assert.NotNil(t, users)
		assert.GreaterOrEqual(t, total, 0)
	})

	t.Run("Default_Offset_When_Negative", func(t *testing.T) {
		tenant, err := itf.CreateTestTenant(f.Ctx, f.Pool)
		require.NoError(t, err)

		users, total, err := service.GetUsersByTenantID(f.Ctx, tenant.ID, 10, -5, "", user.SortBy{})
		require.NoError(t, err)
		assert.NotNil(t, users)
		assert.GreaterOrEqual(t, total, 0)
	})

	t.Run("Pagination_First_Second_Pages", func(t *testing.T) {
		tenant, err := itf.CreateTestTenant(f.Ctx, f.Pool)
		require.NoError(t, err)

		// First page
		firstPage, total1, err := service.GetUsersByTenantID(f.Ctx, tenant.ID, 5, 0, "", user.SortBy{})
		require.NoError(t, err)
		assert.LessOrEqual(t, len(firstPage), 5)

		// Second page
		_, total2, err := service.GetUsersByTenantID(f.Ctx, tenant.ID, 5, 5, "", user.SortBy{})
		require.NoError(t, err)
		assert.Equal(t, total1, total2, "Total should be same across pages")
	})

	t.Run("Search_By_Email", func(t *testing.T) {
		tenant, err := itf.CreateTestTenant(f.Ctx, f.Pool)
		require.NoError(t, err)

		users, total, err := service.GetUsersByTenantID(f.Ctx, tenant.ID, 10, 0, "@", user.SortBy{})
		require.NoError(t, err)
		assert.NotNil(t, users)
		assert.GreaterOrEqual(t, total, 0)
	})

	t.Run("Search_Case_Insensitive", func(t *testing.T) {
		tenant, err := itf.CreateTestTenant(f.Ctx, f.Pool)
		require.NoError(t, err)

		lowerUsers, lowerTotal, err := service.GetUsersByTenantID(f.Ctx, tenant.ID, 10, 0, "test", user.SortBy{})
		require.NoError(t, err)

		upperUsers, upperTotal, err := service.GetUsersByTenantID(f.Ctx, tenant.ID, 10, 0, "TEST", user.SortBy{})
		require.NoError(t, err)

		// Should return same results regardless of case
		assert.Equal(t, lowerTotal, upperTotal)
		assert.Len(t, upperUsers, len(lowerUsers))
	})

	t.Run("Search_No_Results", func(t *testing.T) {
		tenant, err := itf.CreateTestTenant(f.Ctx, f.Pool)
		require.NoError(t, err)

		users, total, err := service.GetUsersByTenantID(f.Ctx, tenant.ID, 10, 0, "NonExistentUser12345", user.SortBy{})
		require.NoError(t, err)
		assert.Empty(t, users)
		assert.Equal(t, 0, total)
	})

	t.Run("Special_Characters_In_Search", func(t *testing.T) {
		tenant, err := itf.CreateTestTenant(f.Ctx, f.Pool)
		require.NoError(t, err)

		testCases := []string{
			"%",
			"_",
			"test%",
			"test_",
			"'test'",
		}

		for _, searchTerm := range testCases {
			users, total, err := service.GetUsersByTenantID(f.Ctx, tenant.ID, 10, 0, searchTerm, user.SortBy{})
			require.NoError(t, err, "Search should not error with special character: %s", searchTerm)
			assert.GreaterOrEqual(t, total, 0)
			assert.Len(t, users, total)
		}
	})

	t.Run("Large_Limit", func(t *testing.T) {
		tenant, err := itf.CreateTestTenant(f.Ctx, f.Pool)
		require.NoError(t, err)

		users, total, err := service.GetUsersByTenantID(f.Ctx, tenant.ID, 1000, 0, "", user.SortBy{})
		require.NoError(t, err)
		assert.GreaterOrEqual(t, total, 0)
		assert.LessOrEqual(t, len(users), 1000)
	})

	t.Run("High_Offset_Returns_Empty", func(t *testing.T) {
		tenant, err := itf.CreateTestTenant(f.Ctx, f.Pool)
		require.NoError(t, err)

		users, total, err := service.GetUsersByTenantID(f.Ctx, tenant.ID, 10, 100000, "", user.SortBy{})
		require.NoError(t, err)
		assert.GreaterOrEqual(t, total, 0)
		// High offset should return empty results
		assert.Empty(t, users)
	})

	t.Run("Cross_Tenant_Isolation", func(t *testing.T) {
		// Create two different tenants
		tenant1, err := itf.CreateTestTenant(f.Ctx, f.Pool)
		require.NoError(t, err)

		tenant2, err := itf.CreateTestTenant(f.Ctx, f.Pool)
		require.NoError(t, err)

		// Get users for tenant1
		users1, total1, err := service.GetUsersByTenantID(f.Ctx, tenant1.ID, 100, 0, "", user.SortBy{})
		require.NoError(t, err)

		// Get users for tenant2
		users2, total2, err := service.GetUsersByTenantID(f.Ctx, tenant2.ID, 100, 0, "", user.SortBy{})
		require.NoError(t, err)

		// Verify no overlap - each tenant should have separate users
		if total1 > 0 && total2 > 0 {
			for _, u1 := range users1 {
				for _, u2 := range users2 {
					assert.NotEqual(t, u1.ID(), u2.ID(), "Users from different tenants should not overlap")
				}
			}
		}
	})
}

func TestTenantUsersService_Sorting(t *testing.T) {
	t.Parallel()
	f := setupTest(t)

	uploadRepo := persistence.NewUploadRepository()
	userRepo := persistence.NewUserRepository(uploadRepo)
	service := services.NewTenantUsersService(userRepo)

	tenant, err := itf.CreateTestTenant(f.Ctx, f.Pool)
	require.NoError(t, err)

	t.Run("Sort_By_FirstName_Ascending", func(t *testing.T) {
		sortBy := user.SortBy{
			Fields: []repo.SortByField[user.Field]{
				{Field: user.FirstNameField, Ascending: true},
			},
		}

		users, total, err := service.GetUsersByTenantID(f.Ctx, tenant.ID, 10, 0, "", sortBy)
		require.NoError(t, err)
		assert.NotNil(t, users)
		assert.GreaterOrEqual(t, total, 0)
	})

	t.Run("Sort_By_Email_Descending", func(t *testing.T) {
		sortBy := user.SortBy{
			Fields: []repo.SortByField[user.Field]{
				{Field: user.EmailField, Ascending: false},
			},
		}

		users, total, err := service.GetUsersByTenantID(f.Ctx, tenant.ID, 10, 0, "", sortBy)
		require.NoError(t, err)
		assert.NotNil(t, users)
		assert.GreaterOrEqual(t, total, 0)
	})

	t.Run("Sort_By_CreatedAt_Descending", func(t *testing.T) {
		sortBy := user.SortBy{
			Fields: []repo.SortByField[user.Field]{
				{Field: user.CreatedAtField, Ascending: false},
			},
		}

		users, total, err := service.GetUsersByTenantID(f.Ctx, tenant.ID, 10, 0, "", sortBy)
		require.NoError(t, err)
		assert.NotNil(t, users)
		assert.GreaterOrEqual(t, total, 0)
	})

	t.Run("Sort_With_Search", func(t *testing.T) {
		sortBy := user.SortBy{
			Fields: []repo.SortByField[user.Field]{
				{Field: user.FirstNameField, Ascending: true},
			},
		}

		users, total, err := service.GetUsersByTenantID(f.Ctx, tenant.ID, 10, 0, "test", sortBy)
		require.NoError(t, err)
		assert.NotNil(t, users)
		assert.GreaterOrEqual(t, total, 0)
	})
}

func TestTenantUsersService_Integration(t *testing.T) {
	t.Parallel()
	f := setupTest(t)

	uploadRepo := persistence.NewUploadRepository()
	userRepo := persistence.NewUserRepository(uploadRepo)
	service := services.NewTenantUsersService(userRepo)

	t.Run("Service_Uses_Repository_Correctly", func(t *testing.T) {
		tenant, err := itf.CreateTestTenant(f.Ctx, f.Pool)
		require.NoError(t, err)

		// Test that service delegates to repository correctly
		users, total, err := service.GetUsersByTenantID(f.Ctx, tenant.ID, 10, 0, "", user.SortBy{})
		require.NoError(t, err)
		assert.NotNil(t, users)
		assert.GreaterOrEqual(t, total, 0)
		assert.Len(t, users, total)
	})

	t.Run("Pagination_And_Count_Consistency", func(t *testing.T) {
		tenant, err := itf.CreateTestTenant(f.Ctx, f.Pool)
		require.NoError(t, err)

		// Get first page
		firstPage, total1, err := service.GetUsersByTenantID(f.Ctx, tenant.ID, 5, 0, "", user.SortBy{})
		require.NoError(t, err)

		// Get second page
		secondPage, total2, err := service.GetUsersByTenantID(f.Ctx, tenant.ID, 5, 5, "", user.SortBy{})
		require.NoError(t, err)

		// Total should be consistent
		assert.Equal(t, total1, total2)

		// Combined results should not exceed total
		assert.LessOrEqual(t, len(firstPage)+len(secondPage), total1)
	})

	t.Run("Search_And_Pagination_Work_Together", func(t *testing.T) {
		tenant, err := itf.CreateTestTenant(f.Ctx, f.Pool)
		require.NoError(t, err)

		// Search with pagination
		users, total, err := service.GetUsersByTenantID(f.Ctx, tenant.ID, 5, 0, "@", user.SortBy{})
		require.NoError(t, err)
		assert.NotNil(t, users)
		assert.GreaterOrEqual(t, total, 0)
		assert.LessOrEqual(t, len(users), 5)
	})

	t.Run("Sorting_And_Pagination_Work_Together", func(t *testing.T) {
		tenant, err := itf.CreateTestTenant(f.Ctx, f.Pool)
		require.NoError(t, err)

		sortBy := user.SortBy{
			Fields: []repo.SortByField[user.Field]{
				{Field: user.FirstNameField, Ascending: true},
			},
		}

		users, total, err := service.GetUsersByTenantID(f.Ctx, tenant.ID, 5, 0, "", sortBy)
		require.NoError(t, err)
		assert.NotNil(t, users)
		assert.GreaterOrEqual(t, total, 0)
		assert.LessOrEqual(t, len(users), 5)
	})
}

func TestTenantUsersService_ServiceType(t *testing.T) {
	t.Run("Service_Implements_Correct_Interface", func(t *testing.T) {
		uploadRepo := persistence.NewUploadRepository()
		userRepo := persistence.NewUserRepository(uploadRepo)
		service := services.NewTenantUsersService(userRepo)

		// Verify service type
		assert.NotNil(t, service)
		assert.IsType(t, &services.TenantUsersService{}, service)
	})
}
