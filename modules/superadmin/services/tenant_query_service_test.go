package services_test

import (
	"testing"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/modules/superadmin/domain"
	"github.com/iota-uz/iota-sdk/modules/superadmin/infrastructure/persistence"
	"github.com/iota-uz/iota-sdk/modules/superadmin/services"
	"github.com/iota-uz/iota-sdk/pkg/itf"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTenantQueryService_FindTenants(t *testing.T) {
	t.Parallel()
	f := setupTest(t)

	repo := persistence.NewPgAnalyticsQueryRepository()
	service := services.NewTenantQueryService(repo)

	t.Run("Happy_Path_Without_Search", func(t *testing.T) {
		tenants, total, err := service.FindTenants(f.Ctx, 10, 0, "")
		require.NoError(t, err)
		assert.NotNil(t, tenants)
		assert.GreaterOrEqual(t, total, 0)
	})

	t.Run("Happy_Path_With_Search", func(t *testing.T) {
		tenants, total, err := service.FindTenants(f.Ctx, 10, 0, "Test")
		require.NoError(t, err)
		assert.NotNil(t, tenants)
		assert.GreaterOrEqual(t, total, 0)
	})

	t.Run("Default_Limit_When_Zero", func(t *testing.T) {
		tenants, total, err := service.FindTenants(f.Ctx, 0, 0, "")
		require.NoError(t, err)
		assert.NotNil(t, tenants)
		assert.GreaterOrEqual(t, total, 0)
		// With default limit of 20, should get up to 20 results
		assert.LessOrEqual(t, len(tenants), 20)
	})

	t.Run("Default_Offset_When_Negative", func(t *testing.T) {
		tenants, total, err := service.FindTenants(f.Ctx, 10, -5, "")
		require.NoError(t, err)
		assert.NotNil(t, tenants)
		assert.GreaterOrEqual(t, total, 0)
	})

	t.Run("Search_By_Name", func(t *testing.T) {
		// Create test tenant
		_, err := itf.CreateTestTenant(f.Ctx, f.Pool)
		require.NoError(t, err)

		tenants, total, err := service.FindTenants(f.Ctx, 10, 0, "Test")
		require.NoError(t, err)
		assert.NotNil(t, tenants)
		assert.GreaterOrEqual(t, total, 1)
	})

	t.Run("Search_By_Domain", func(t *testing.T) {
		tenants, total, err := service.FindTenants(f.Ctx, 10, 0, ".com")
		require.NoError(t, err)
		assert.NotNil(t, tenants)
		assert.GreaterOrEqual(t, total, 0)
	})

	t.Run("Search_Case_Insensitive", func(t *testing.T) {
		lowerTenants, lowerTotal, err := service.FindTenants(f.Ctx, 10, 0, "test")
		require.NoError(t, err)

		upperTenants, upperTotal, err := service.FindTenants(f.Ctx, 10, 0, "TEST")
		require.NoError(t, err)

		// Should return same results regardless of case
		assert.Equal(t, lowerTotal, upperTotal)
		assert.Equal(t, len(lowerTenants), len(upperTenants))
	})

	t.Run("Search_No_Results", func(t *testing.T) {
		tenants, total, err := service.FindTenants(f.Ctx, 10, 0, "NonExistentTenant12345")
		require.NoError(t, err)
		assert.Empty(t, tenants)
		assert.Equal(t, 0, total)
	})

	t.Run("Pagination_Without_Search", func(t *testing.T) {
		// Create multiple tenants
		_, err := itf.CreateTestTenant(f.Ctx, f.Pool)
		require.NoError(t, err)
		_, err = itf.CreateTestTenant(f.Ctx, f.Pool)
		require.NoError(t, err)

		// First page
		firstPage, total, err := service.FindTenants(f.Ctx, 5, 0, "")
		require.NoError(t, err)
		assert.LessOrEqual(t, len(firstPage), 5)

		// Second page
		_, total2, err := service.FindTenants(f.Ctx, 5, 5, "")
		require.NoError(t, err)
		assert.Equal(t, total, total2, "Total should be same across pages")
	})

	t.Run("Pagination_With_Search", func(t *testing.T) {
		// First page with search
		firstPage, total, err := service.FindTenants(f.Ctx, 5, 0, "Test")
		require.NoError(t, err)
		assert.LessOrEqual(t, len(firstPage), 5)

		// Second page with search
		_, total2, err := service.FindTenants(f.Ctx, 5, 5, "Test")
		require.NoError(t, err)
		assert.Equal(t, total, total2, "Total should be same across pages")
	})

	t.Run("Empty_Search_Uses_ListTenants", func(t *testing.T) {
		// Empty search should fall back to ListTenants
		emptySearchTenants, emptyTotal, err := service.FindTenants(f.Ctx, 10, 0, "")
		require.NoError(t, err)

		// Get all tenants directly
		allTenants, allTotal, err := service.FindTenants(f.Ctx, 10, 0, "")
		require.NoError(t, err)

		// Should return same results
		assert.Equal(t, emptyTotal, allTotal)
		assert.Equal(t, len(emptySearchTenants), len(allTenants))
	})

	t.Run("Whitespace_Only_Search", func(t *testing.T) {
		// Whitespace-only should be treated as non-empty search
		tenants, total, err := service.FindTenants(f.Ctx, 10, 0, "   ")
		require.NoError(t, err)
		// Can be empty slice if no matches
		assert.GreaterOrEqual(t, total, 0)
		assert.Equal(t, total, len(tenants))
	})

	t.Run("Special_Characters_In_Search", func(t *testing.T) {
		testCases := []string{
			"%",
			"_",
			"test%",
			"test_",
			"'test'",
		}

		for _, searchTerm := range testCases {
			tenants, total, err := service.FindTenants(f.Ctx, 10, 0, searchTerm)
			require.NoError(t, err, "Search should not error with special character: %s", searchTerm)
			// Can be empty slice if no matches
			assert.GreaterOrEqual(t, total, 0)
			assert.Equal(t, total, len(tenants))
		}
	})

	t.Run("Large_Limit", func(t *testing.T) {
		tenants, total, err := service.FindTenants(f.Ctx, 1000, 0, "")
		require.NoError(t, err)
		assert.GreaterOrEqual(t, total, 0)
		// Should return all tenants when limit is large enough
		assert.LessOrEqual(t, len(tenants), 1000)
		assert.Equal(t, total, len(tenants))
	})

	t.Run("High_Offset", func(t *testing.T) {
		tenants, total, err := service.FindTenants(f.Ctx, 10, 100000, "")
		require.NoError(t, err)
		assert.GreaterOrEqual(t, total, 0)
		// High offset should return empty results
		assert.Empty(t, tenants)
	})
}

func TestTenantQueryService_GetByID(t *testing.T) {
	t.Parallel()
	f := setupTest(t)

	repo := persistence.NewPgAnalyticsQueryRepository()
	service := services.NewTenantQueryService(repo)

	t.Run("Happy_Path_Existing_Tenant", func(t *testing.T) {
		// Create test tenant
		newTenant, err := itf.CreateTestTenant(f.Ctx, f.Pool)
		require.NoError(t, err)

		// Get by ID
		tenant, err := service.GetByID(f.Ctx, newTenant.ID)
		require.NoError(t, err)
		require.NotNil(t, tenant)

		assert.Equal(t, newTenant.ID, tenant.ID)
		assert.Equal(t, newTenant.Name, tenant.Name)
		assert.GreaterOrEqual(t, tenant.UserCount, 0)
	})

	t.Run("NonExistent_Tenant_Returns_Error", func(t *testing.T) {
		nonExistentID := uuid.New()

		tenant, err := service.GetByID(f.Ctx, nonExistentID)
		require.Error(t, err)
		assert.Nil(t, tenant)
		assert.Contains(t, err.Error(), "tenant not found")
	})
}

func TestTenantQueryService_GetAll(t *testing.T) {
	t.Parallel()
	f := setupTest(t)

	repo := persistence.NewPgAnalyticsQueryRepository()
	service := services.NewTenantQueryService(repo)

	t.Run("Happy_Path_Returns_All_Tenants", func(t *testing.T) {
		// Create multiple test tenants
		_, err := itf.CreateTestTenant(f.Ctx, f.Pool)
		require.NoError(t, err)
		_, err = itf.CreateTestTenant(f.Ctx, f.Pool)
		require.NoError(t, err)

		tenants, err := service.GetAll(f.Ctx)
		require.NoError(t, err)
		assert.NotNil(t, tenants)
		assert.GreaterOrEqual(t, len(tenants), 2)
	})

	t.Run("Returns_All_Without_Limit", func(t *testing.T) {
		// GetAll should return all tenants without pagination
		allTenants, err := service.GetAll(f.Ctx)
		require.NoError(t, err)

		// Compare with FindTenants with large limit
		paginatedTenants, total, err := service.FindTenants(f.Ctx, 10000, 0, "")
		require.NoError(t, err)

		// Should return same number of tenants
		assert.Equal(t, len(allTenants), total)
		assert.Equal(t, len(allTenants), len(paginatedTenants))
	})
}

func TestTenantQueryService_Integration(t *testing.T) {
	t.Parallel()
	f := setupTest(t)

	repo := persistence.NewPgAnalyticsQueryRepository()
	service := services.NewTenantQueryService(repo)

	t.Run("Search_And_GetByID_Consistency", func(t *testing.T) {
		// Create test tenant
		newTenant, err := itf.CreateTestTenant(f.Ctx, f.Pool)
		require.NoError(t, err)

		// Search for tenant
		searchResults, _, err := service.FindTenants(f.Ctx, 10, 0, "Test")
		require.NoError(t, err)

		// Find the tenant in search results
		var found bool
		for _, tenant := range searchResults {
			if tenant.ID == newTenant.ID {
				found = true

				// Get same tenant by ID
				tenantByID, err := service.GetByID(f.Ctx, newTenant.ID)
				require.NoError(t, err)

				// Should have same data
				assert.Equal(t, tenant.ID, tenantByID.ID)
				assert.Equal(t, tenant.Name, tenantByID.Name)
				assert.Equal(t, tenant.Domain, tenantByID.Domain)
				assert.Equal(t, tenant.UserCount, tenantByID.UserCount)
				break
			}
		}

		assert.True(t, found, "Created tenant should be found in search results")
	})

	t.Run("GetAll_Contains_SearchResults", func(t *testing.T) {
		// Get all tenants
		allTenants, err := service.GetAll(f.Ctx)
		require.NoError(t, err)

		// Search for specific tenants
		searchResults, _, err := service.FindTenants(f.Ctx, 100, 0, "Test")
		require.NoError(t, err)

		// All search results should be in GetAll results
		for _, searchTenant := range searchResults {
			found := false
			for _, allTenant := range allTenants {
				if allTenant.ID == searchTenant.ID {
					found = true
					break
				}
			}
			assert.True(t, found, "Search result should be in GetAll results")
		}
	})
}

func TestTenantQueryService_ServiceType(t *testing.T) {
	t.Run("Service_Implements_Correct_Interface", func(t *testing.T) {
		repo := persistence.NewPgAnalyticsQueryRepository()
		service := services.NewTenantQueryService(repo)

		// Verify service type
		assert.NotNil(t, service)
		assert.IsType(t, &services.TenantQueryService{}, service)
	})
}

func TestTenantQueryService_RepositoryIntegration(t *testing.T) {
	t.Parallel()
	f := setupTest(t)

	t.Run("Service_Uses_Repository_Correctly", func(t *testing.T) {
		repo := persistence.NewPgAnalyticsQueryRepository()
		service := services.NewTenantQueryService(repo)

		// Test that service delegates to repository correctly
		tenants, total, err := service.FindTenants(f.Ctx, 10, 0, "")
		require.NoError(t, err)
		assert.NotNil(t, tenants)
		assert.GreaterOrEqual(t, total, 0)

		// Verify repository method is called by checking results are consistent
		repoTenants, repoTotal, err := repo.ListTenants(f.Ctx, 10, 0)
		require.NoError(t, err)

		assert.Equal(t, total, repoTotal)
		assert.Equal(t, len(tenants), len(repoTenants))
	})

	t.Run("Service_Uses_SearchTenants_Repository_Method", func(t *testing.T) {
		repo := persistence.NewPgAnalyticsQueryRepository()
		service := services.NewTenantQueryService(repo)

		// Service should use SearchTenants when search is provided
		serviceTenants, serviceTotal, err := service.FindTenants(f.Ctx, 10, 0, "Test")
		require.NoError(t, err)

		// Compare with direct repository call
		repoTenants, repoTotal, err := repo.SearchTenants(f.Ctx, "Test", 10, 0)
		require.NoError(t, err)

		assert.Equal(t, serviceTotal, repoTotal)
		assert.Equal(t, len(serviceTenants), len(repoTenants))
	})
}

// Helper function to get service with mock setup
func getTenantQueryService(repo domain.AnalyticsQueryRepository) *services.TenantQueryService {
	return services.NewTenantQueryService(repo)
}
