package services_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/modules/superadmin/domain"
	"github.com/iota-uz/iota-sdk/modules/superadmin/domain/entities"
	"github.com/iota-uz/iota-sdk/modules/superadmin/infrastructure/persistence"
	"github.com/iota-uz/iota-sdk/modules/superadmin/services"
	"github.com/iota-uz/iota-sdk/pkg/itf"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTenantService_ListTenants(t *testing.T) {
	t.Parallel()
	f := setupTest(t)

	repo := persistence.NewPgAnalyticsQueryRepository()
	service := services.NewTenantService(repo)

	t.Run("Happy_Path_Default_Limit", func(t *testing.T) {
		tenants, total, err := service.ListTenants(f.Ctx, 10, 0)
		require.NoError(t, err)
		assert.GreaterOrEqual(t, total, 0)
		assert.LessOrEqual(t, len(tenants), 10)
	})

	t.Run("Zero_Limit_Uses_Default_50", func(t *testing.T) {
		tenants, total, err := service.ListTenants(f.Ctx, 0, 0)
		require.NoError(t, err)
		assert.GreaterOrEqual(t, total, 0)
		assert.LessOrEqual(t, len(tenants), 50)
	})

	t.Run("Negative_Limit_Uses_Default_50", func(t *testing.T) {
		tenants, total, err := service.ListTenants(f.Ctx, -10, 0)
		require.NoError(t, err)
		assert.GreaterOrEqual(t, total, 0)
		assert.LessOrEqual(t, len(tenants), 50)
	})

	t.Run("Limit_Exceeds_Maximum_1000", func(t *testing.T) {
		tenants, total, err := service.ListTenants(f.Ctx, 2000, 0)
		require.Error(t, err)
		assert.Nil(t, tenants)
		assert.Equal(t, 0, total)
		assert.Contains(t, err.Error(), "limit cannot exceed 1000")
	})

	t.Run("Negative_Offset_Defaults_To_Zero", func(t *testing.T) {
		tenants, total, err := service.ListTenants(f.Ctx, 10, -5)
		require.NoError(t, err)
		assert.GreaterOrEqual(t, total, 0)
		assert.LessOrEqual(t, len(tenants), 10)
	})

	t.Run("Offset_Beyond_Total_Returns_Empty", func(t *testing.T) {
		tenants, total, err := service.ListTenants(f.Ctx, 10, 10000)
		require.NoError(t, err)
		assert.GreaterOrEqual(t, total, 0)
		assert.Empty(t, tenants)
	})

	t.Run("Small_Limit_Returns_Correct_Count", func(t *testing.T) {
		tenants, total, err := service.ListTenants(f.Ctx, 1, 0)
		require.NoError(t, err)
		assert.GreaterOrEqual(t, total, 0)
		assert.LessOrEqual(t, len(tenants), 1)
	})

	t.Run("Pagination_First_Page", func(t *testing.T) {
		tenants, total, err := service.ListTenants(f.Ctx, 5, 0)
		require.NoError(t, err)
		assert.GreaterOrEqual(t, total, 0)
		assert.LessOrEqual(t, len(tenants), 5)
	})

	t.Run("Pagination_Second_Page", func(t *testing.T) {
		tenants, total, err := service.ListTenants(f.Ctx, 5, 5)
		require.NoError(t, err)
		assert.GreaterOrEqual(t, total, 0)
		assert.LessOrEqual(t, len(tenants), 5)
	})

	t.Run("Context_Cancellation", func(t *testing.T) {
		ctx, cancel := context.WithCancel(f.Ctx)
		cancel()

		tenants, total, err := service.ListTenants(ctx, 10, 0)
		require.Error(t, err)
		assert.Nil(t, tenants)
		assert.Equal(t, 0, total)
	})

	t.Run("Tenant_Fields_Are_Populated", func(t *testing.T) {
		tenants, _, err := service.ListTenants(f.Ctx, 10, 0)
		require.NoError(t, err)

		if len(tenants) > 0 {
			tenant := tenants[0]
			assert.NotEqual(t, uuid.Nil, tenant.ID, "Tenant ID should not be nil")
			assert.NotEmpty(t, tenant.Name, "Tenant name should not be empty")
			assert.NotEmpty(t, tenant.Domain, "Tenant domain should not be empty")
			assert.GreaterOrEqual(t, tenant.UserCount, 0, "User count should be non-negative")
			assert.False(t, tenant.CreatedAt.IsZero(), "CreatedAt should not be zero")
			assert.False(t, tenant.UpdatedAt.IsZero(), "UpdatedAt should not be zero")
		}
	})
}

func TestTenantService_FilterByDateRange(t *testing.T) {
	t.Parallel()
	f := setupTest(t)

	repo := persistence.NewPgAnalyticsQueryRepository()
	service := services.NewTenantService(repo)

	now := time.Now()

	t.Run("Happy_Path_Last_30_Days", func(t *testing.T) {
		startDate := now.AddDate(0, 0, -30)
		endDate := now

		tenants, total, err := service.FilterByDateRange(f.Ctx, startDate, endDate, 10, 0, domain.TenantSortBy{})
		require.NoError(t, err)
		assert.GreaterOrEqual(t, total, 0)
		assert.LessOrEqual(t, len(tenants), 10)
	})

	t.Run("Zero_Start_Date_Uses_Default", func(t *testing.T) {
		endDate := now

		_, total, err := service.FilterByDateRange(f.Ctx, time.Time{}, endDate, 10, 0, domain.TenantSortBy{})
		require.NoError(t, err)
		assert.GreaterOrEqual(t, total, 0)
	})

	t.Run("Zero_End_Date_Uses_Now", func(t *testing.T) {
		startDate := now.AddDate(0, 0, -30)

		_, total, err := service.FilterByDateRange(f.Ctx, startDate, time.Time{}, 10, 0, domain.TenantSortBy{})
		require.NoError(t, err)
		assert.GreaterOrEqual(t, total, 0)
	})

	t.Run("Both_Dates_Zero_Returns_All", func(t *testing.T) {
		tenants, total, err := service.FilterByDateRange(f.Ctx, time.Time{}, time.Time{}, 10, 0, domain.TenantSortBy{})
		require.NoError(t, err)
		assert.GreaterOrEqual(t, total, 0)
		assert.LessOrEqual(t, len(tenants), 10)
	})

	t.Run("Invalid_Date_Range_Start_After_End", func(t *testing.T) {
		startDate := now
		endDate := now.AddDate(0, 0, -30)

		tenants, total, err := service.FilterByDateRange(f.Ctx, startDate, endDate, 10, 0, domain.TenantSortBy{})
		require.Error(t, err)
		assert.Nil(t, tenants)
		assert.Equal(t, 0, total)
		assert.Contains(t, err.Error(), "start date cannot be after end date")
	})

	t.Run("Same_Day_Range", func(t *testing.T) {
		startDate := now.Truncate(24 * time.Hour)
		endDate := startDate.Add(24 * time.Hour)

		_, total, err := service.FilterByDateRange(f.Ctx, startDate, endDate, 10, 0, domain.TenantSortBy{})
		require.NoError(t, err)
		assert.GreaterOrEqual(t, total, 0)
	})

	t.Run("One_Year_Range", func(t *testing.T) {
		startDate := now.AddDate(-1, 0, 0)
		endDate := now

		_, total, err := service.FilterByDateRange(f.Ctx, startDate, endDate, 10, 0, domain.TenantSortBy{})
		require.NoError(t, err)
		assert.GreaterOrEqual(t, total, 0)
	})

	t.Run("Limit_Validation_Zero_Uses_Default", func(t *testing.T) {
		startDate := now.AddDate(0, 0, -30)
		endDate := now

		tenants, total, err := service.FilterByDateRange(f.Ctx, startDate, endDate, 0, 0, domain.TenantSortBy{})
		require.NoError(t, err)
		assert.GreaterOrEqual(t, total, 0)
		assert.LessOrEqual(t, len(tenants), 50)
	})

	t.Run("Limit_Validation_Exceeds_Maximum", func(t *testing.T) {
		startDate := now.AddDate(0, 0, -30)
		endDate := now

		tenants, total, err := service.FilterByDateRange(f.Ctx, startDate, endDate, 2000, 0, domain.TenantSortBy{})
		require.Error(t, err)
		assert.Nil(t, tenants)
		assert.Equal(t, 0, total)
		assert.Contains(t, err.Error(), "limit cannot exceed 1000")
	})

	t.Run("Negative_Offset_Defaults_To_Zero", func(t *testing.T) {
		startDate := now.AddDate(0, 0, -30)
		endDate := now

		_, total, err := service.FilterByDateRange(f.Ctx, startDate, endDate, 10, -5, domain.TenantSortBy{})
		require.NoError(t, err)
		assert.GreaterOrEqual(t, total, 0)
	})

	t.Run("Pagination_Works_With_Date_Filter", func(t *testing.T) {
		startDate := now.AddDate(0, 0, -30)
		endDate := now

		// Get first page
		_, total, err := service.FilterByDateRange(f.Ctx, startDate, endDate, 5, 0, domain.TenantSortBy{})
		require.NoError(t, err)
		assert.GreaterOrEqual(t, total, 0)

		// Get second page
		_, total2, err := service.FilterByDateRange(f.Ctx, startDate, endDate, 5, 5, domain.TenantSortBy{})
		require.NoError(t, err)
		assert.Equal(t, total, total2, "Total count should be same across pages")
	})

	t.Run("Future_Date_Range_Returns_Empty", func(t *testing.T) {
		startDate := now.AddDate(0, 0, 1)
		endDate := now.AddDate(0, 0, 7)

		tenants, total, err := service.FilterByDateRange(f.Ctx, startDate, endDate, 10, 0, domain.TenantSortBy{})
		require.NoError(t, err)
		assert.Equal(t, 0, total)
		assert.Empty(t, tenants)
	})

	t.Run("Context_Cancellation", func(t *testing.T) {
		ctx, cancel := context.WithCancel(f.Ctx)
		cancel()

		startDate := now.AddDate(0, 0, -30)
		endDate := now

		tenants, total, err := service.FilterByDateRange(ctx, startDate, endDate, 10, 0, domain.TenantSortBy{})
		require.Error(t, err)
		assert.Nil(t, tenants)
		assert.Equal(t, 0, total)
	})
}

func TestTenantService_GetTenantDetails(t *testing.T) {
	t.Parallel()
	f := setupTest(t)

	repo := persistence.NewPgAnalyticsQueryRepository()
	service := services.NewTenantService(repo)

	t.Run("Get_Test_Tenant_Details", func(t *testing.T) {
		// Get the test tenant ID from ITF context
		tenantID := f.TenantID()

		tenant, err := service.GetTenantDetails(f.Ctx, tenantID)
		require.NoError(t, err)
		require.NotNil(t, tenant)

		assert.Equal(t, tenantID, tenant.ID)
		assert.NotEmpty(t, tenant.Name)
		assert.NotEmpty(t, tenant.Domain)
		assert.GreaterOrEqual(t, tenant.UserCount, 0, "User count should be non-negative")
		assert.False(t, tenant.CreatedAt.IsZero())
		assert.False(t, tenant.UpdatedAt.IsZero())
	})

	t.Run("Nil_Tenant_ID", func(t *testing.T) {
		tenant, err := service.GetTenantDetails(f.Ctx, uuid.Nil)
		require.Error(t, err)
		assert.Nil(t, tenant)
		assert.Contains(t, err.Error(), "tenant ID cannot be nil")
	})

	t.Run("Non_Existent_Tenant_ID", func(t *testing.T) {
		nonExistentID := uuid.New()

		tenant, err := service.GetTenantDetails(f.Ctx, nonExistentID)
		require.Error(t, err)
		assert.Nil(t, tenant)
		assert.Contains(t, err.Error(), "tenant not found")
	})

	t.Run("Context_Cancellation", func(t *testing.T) {
		ctx, cancel := context.WithCancel(f.Ctx)
		cancel()

		tenantID := f.TenantID()
		tenant, err := service.GetTenantDetails(ctx, tenantID)
		require.Error(t, err)
		assert.Nil(t, tenant)
	})
}

func TestTenantService_PrepareExcelExport(t *testing.T) {
	t.Parallel()
	f := setupTest(t)

	repo := persistence.NewPgAnalyticsQueryRepository()
	service := services.NewTenantService(repo)

	t.Run("Export_Empty_Tenants", func(t *testing.T) {
		data, err := service.PrepareExcelExport(f.Ctx, []*entities.TenantInfo{})
		require.NoError(t, err)
		require.NotNil(t, data)

		assert.Len(t, data.Headers, 6)
		assert.Empty(t, data.Rows)
		assert.Contains(t, data.Headers, "ID")
		assert.Contains(t, data.Headers, "Name")
		assert.Contains(t, data.Headers, "Domain")
		assert.Contains(t, data.Headers, "User Count")
		assert.Contains(t, data.Headers, "Created At")
		assert.Contains(t, data.Headers, "Updated At")
	})

	t.Run("Export_Single_Tenant", func(t *testing.T) {
		tenant := &entities.TenantInfo{
			ID:        uuid.New(),
			Name:      "Test Tenant",
			Domain:    "test.example.com",
			UserCount: 5,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}

		data, err := service.PrepareExcelExport(f.Ctx, []*entities.TenantInfo{tenant})
		require.NoError(t, err)
		require.NotNil(t, data)

		assert.Len(t, data.Headers, 6)
		assert.Len(t, data.Rows, 1)

		row := data.Rows[0]
		assert.Len(t, row, 6)
		assert.Equal(t, tenant.ID.String(), row[0])
		assert.Equal(t, tenant.Name, row[1])
		assert.Equal(t, tenant.Domain, row[2])
		assert.Equal(t, tenant.UserCount, row[3])
	})

	t.Run("Export_Multiple_Tenants", func(t *testing.T) {
		tenants := []*entities.TenantInfo{
			{
				ID:        uuid.New(),
				Name:      "Tenant 1",
				Domain:    "tenant1.com",
				UserCount: 10,
				CreatedAt: time.Now(),
				UpdatedAt: time.Now(),
			},
			{
				ID:        uuid.New(),
				Name:      "Tenant 2",
				Domain:    "tenant2.com",
				UserCount: 20,
				CreatedAt: time.Now(),
				UpdatedAt: time.Now(),
			},
			{
				ID:        uuid.New(),
				Name:      "Tenant 3",
				Domain:    "tenant3.com",
				UserCount: 30,
				CreatedAt: time.Now(),
				UpdatedAt: time.Now(),
			},
		}

		data, err := service.PrepareExcelExport(f.Ctx, tenants)
		require.NoError(t, err)
		require.NotNil(t, data)

		assert.Len(t, data.Headers, 6)
		assert.Len(t, data.Rows, 3)

		// Verify each row has correct number of columns
		for i, row := range data.Rows {
			assert.Len(t, row, 6, "Row %d should have 6 columns", i)
		}
	})

	t.Run("Export_With_Real_Tenants", func(t *testing.T) {
		tenants, _, err := service.ListTenants(f.Ctx, 10, 0)
		require.NoError(t, err)

		if len(tenants) > 0 {
			data, err := service.PrepareExcelExport(f.Ctx, tenants)
			require.NoError(t, err)
			require.NotNil(t, data)

			assert.Len(t, data.Headers, 6)
			assert.Len(t, data.Rows, len(tenants))

			// Verify first row matches first tenant
			if len(data.Rows) > 0 {
				row := data.Rows[0]
				tenant := tenants[0]
				assert.Equal(t, tenant.ID.String(), row[0])
				assert.Equal(t, tenant.Name, row[1])
				assert.Equal(t, tenant.Domain, row[2])
				assert.Equal(t, tenant.UserCount, row[3])
			}
		}
	})

	t.Run("Export_Handles_Zero_User_Count", func(t *testing.T) {
		tenant := &entities.TenantInfo{
			ID:        uuid.New(),
			Name:      "Empty Tenant",
			Domain:    "empty.com",
			UserCount: 0,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}

		data, err := service.PrepareExcelExport(f.Ctx, []*entities.TenantInfo{tenant})
		require.NoError(t, err)
		require.NotNil(t, data)

		assert.Len(t, data.Rows, 1)
		assert.Equal(t, 0, data.Rows[0][3])
	})

	t.Run("Nil_Context_Does_Not_Panic", func(t *testing.T) {
		tenant := &entities.TenantInfo{
			ID:        uuid.New(),
			Name:      "Test",
			Domain:    "test.com",
			UserCount: 1,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}

		data, err := service.PrepareExcelExport(context.TODO(), []*entities.TenantInfo{tenant})
		require.NoError(t, err)
		require.NotNil(t, data)
	})
}

func TestTenantService_GetTenantsSummary(t *testing.T) {
	t.Parallel()
	f := setupTest(t)

	repo := persistence.NewPgAnalyticsQueryRepository()
	service := services.NewTenantService(repo)

	t.Run("Get_Summary_With_Real_Data", func(t *testing.T) {
		summary, err := service.GetTenantsSummary(f.Ctx)
		require.NoError(t, err)
		assert.NotEmpty(t, summary)

		// Should contain key metrics
		assert.Contains(t, summary, "Total Tenants:")
		assert.Contains(t, summary, "Total Users:")
		assert.Contains(t, summary, "Average Users per Tenant:")
	})

	t.Run("Context_Cancellation", func(t *testing.T) {
		ctx, cancel := context.WithCancel(f.Ctx)
		cancel()

		summary, err := service.GetTenantsSummary(ctx)
		require.Error(t, err)
		assert.Empty(t, summary)
	})
}

func TestTenantService_Integration(t *testing.T) {
	t.Parallel()
	f := setupTest(t)

	repo := persistence.NewPgAnalyticsQueryRepository()
	service := services.NewTenantService(repo)

	t.Run("List_Then_Get_Details", func(t *testing.T) {
		// List tenants
		tenants, total, err := service.ListTenants(f.Ctx, 10, 0)
		require.NoError(t, err)
		assert.GreaterOrEqual(t, total, 1)

		if len(tenants) > 0 {
			// Get details of first tenant
			tenant, err := service.GetTenantDetails(f.Ctx, tenants[0].ID)
			require.NoError(t, err)
			require.NotNil(t, tenant)

			// Details should match list item
			assert.Equal(t, tenants[0].ID, tenant.ID)
			assert.Equal(t, tenants[0].Name, tenant.Name)
			assert.Equal(t, tenants[0].Domain, tenant.Domain)
			assert.Equal(t, tenants[0].UserCount, tenant.UserCount)
		}
	})

	t.Run("Filter_Then_Export", func(t *testing.T) {
		now := time.Now()
		startDate := now.AddDate(0, 0, -30)
		endDate := now

		// Filter tenants
		tenants, total, err := service.FilterByDateRange(f.Ctx, startDate, endDate, 10, 0, domain.TenantSortBy{})
		require.NoError(t, err)
		assert.GreaterOrEqual(t, total, 0)

		// Export filtered tenants
		data, err := service.PrepareExcelExport(f.Ctx, tenants)
		require.NoError(t, err)
		require.NotNil(t, data)
		assert.Len(t, data.Rows, len(tenants))
	})

	t.Run("Created_Test_Tenant_Is_In_List", func(t *testing.T) {
		// The ITF framework creates a test tenant
		testTenantID := f.TenantID()

		// It should appear in the list
		tenants, _, err := service.ListTenants(f.Ctx, 1000, 0)
		require.NoError(t, err)

		found := false
		for _, tenant := range tenants {
			if tenant.ID == testTenantID {
				found = true
				assert.GreaterOrEqual(t, tenant.UserCount, 0, "User count should be non-negative")
				break
			}
		}
		assert.True(t, found, "Test tenant should be in the list")
	})
}

// Table-driven test for limit validation
func TestTenantService_LimitValidation(t *testing.T) {
	t.Parallel()
	f := setupTest(t)

	repo := persistence.NewPgAnalyticsQueryRepository()
	service := services.NewTenantService(repo)

	testCases := []struct {
		name      string
		limit     int
		wantError bool
		errorMsg  string
	}{
		{name: "Valid_Limit_10", limit: 10, wantError: false},
		{name: "Valid_Limit_50", limit: 50, wantError: false},
		{name: "Valid_Limit_100", limit: 100, wantError: false},
		{name: "Valid_Limit_1000", limit: 1000, wantError: false},
		{name: "Invalid_Limit_1001", limit: 1001, wantError: true, errorMsg: "limit cannot exceed 1000"},
		{name: "Invalid_Limit_5000", limit: 5000, wantError: true, errorMsg: "limit cannot exceed 1000"},
		{name: "Zero_Limit_Uses_Default", limit: 0, wantError: false},
		{name: "Negative_Limit_Uses_Default", limit: -10, wantError: false},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			tenants, total, err := service.ListTenants(f.Ctx, tc.limit, 0)

			if tc.wantError {
				require.Error(t, err)
				assert.Nil(t, tenants)
				assert.Equal(t, 0, total)
				if tc.errorMsg != "" {
					assert.Contains(t, err.Error(), tc.errorMsg)
				}
			} else {
				require.NoError(t, err)
				assert.GreaterOrEqual(t, total, 0)
			}
		})
	}
}

// Benchmark tests
func BenchmarkTenantService_ListTenants(b *testing.B) {
	f := setupTest(b)
	repo := persistence.NewPgAnalyticsQueryRepository()
	service := services.NewTenantService(repo)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _, err := service.ListTenants(f.Ctx, 10, 0)
		if err != nil {
			b.Fatalf("unexpected error: %v", err)
		}
	}
}

func BenchmarkTenantService_PrepareExcelExport(b *testing.B) {
	f := setupTest(b)
	repo := persistence.NewPgAnalyticsQueryRepository()
	service := services.NewTenantService(repo)

	// Get some tenants first
	tenants, _, err := service.ListTenants(f.Ctx, 100, 0)
	if err != nil {
		b.Fatalf("failed to get tenants: %v", err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := service.PrepareExcelExport(f.Ctx, tenants)
		if err != nil {
			b.Fatalf("unexpected error: %v", err)
		}
	}
}

func TestTenantService_EdgeCases(t *testing.T) {
	t.Parallel()

	t.Run("Service_Creation", func(t *testing.T) {
		repo := persistence.NewPgAnalyticsQueryRepository()
		service := services.NewTenantService(repo)
		assert.NotNil(t, service, "Service should be created successfully")
	})
}

// Test for creating multiple test tenants
func TestTenantService_MultiTenant(t *testing.T) {
	t.Parallel()
	f := setupTest(t)

	repo := persistence.NewPgAnalyticsQueryRepository()
	service := services.NewTenantService(repo)

	// Create additional test tenant
	tenant2, err := itf.CreateTestTenant(f.Ctx, f.Pool)
	require.NoError(t, err)

	t.Run("Both_Tenants_Appear_In_List", func(t *testing.T) {
		tenants, total, err := service.ListTenants(f.Ctx, 1000, 0)
		require.NoError(t, err)
		assert.GreaterOrEqual(t, total, 2, "Should have at least 2 tenants")

		// Find both tenants
		tenant1Found := false
		tenant2Found := false

		for _, tenant := range tenants {
			if tenant.ID == f.TenantID() {
				tenant1Found = true
			}
			if tenant.ID == tenant2.ID {
				tenant2Found = true
			}
		}

		assert.True(t, tenant1Found, "First test tenant should be in list")
		assert.True(t, tenant2Found, "Second test tenant should be in list")
	})

	t.Run("Get_Details_Of_Second_Tenant", func(t *testing.T) {
		tenant, err := service.GetTenantDetails(f.Ctx, tenant2.ID)
		require.NoError(t, err)
		require.NotNil(t, tenant)
		assert.Equal(t, tenant2.ID, tenant.ID)
	})
}
