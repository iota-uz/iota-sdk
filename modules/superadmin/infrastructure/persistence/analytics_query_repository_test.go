package persistence_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/user"
	"github.com/iota-uz/iota-sdk/modules/core/domain/value_objects/internet"
	corePersistence "github.com/iota-uz/iota-sdk/modules/core/infrastructure/persistence"
	"github.com/iota-uz/iota-sdk/modules/superadmin/infrastructure/persistence"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/itf"
	"github.com/iota-uz/iota-sdk/pkg/repo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Helper to create empty SortBy for default sorting
var emptySortBy = persistence.SortBy{}

// Helper to create SortBy for specific fields
func sortBy(field string, ascending bool) persistence.SortBy {
	return persistence.SortBy{
		Fields: []repo.SortByField[string]{
			{Field: field, Ascending: ascending},
		},
	}
}

func TestPgAnalyticsQueryRepository_GetDashboardMetrics(t *testing.T) {
	t.Parallel()
	f := setupTest(t)

	repo := persistence.NewPgAnalyticsQueryRepository()

	t.Run("Happy_Path_With_Valid_Dates", func(t *testing.T) {
		startDate := time.Now().AddDate(0, 0, -7)
		endDate := time.Now()

		metrics, err := repo.GetDashboardMetrics(f.Ctx, startDate, endDate)
		require.NoError(t, err)
		require.NotNil(t, metrics)

		// Verify metrics structure is returned
		assert.GreaterOrEqual(t, metrics.TenantCount, 0)
		assert.GreaterOrEqual(t, metrics.UserCount, 0)
		assert.GreaterOrEqual(t, metrics.DAU, 0)
		assert.GreaterOrEqual(t, metrics.WAU, 0)
		assert.GreaterOrEqual(t, metrics.MAU, 0)
		assert.GreaterOrEqual(t, metrics.SessionCount, 0)
	})

	t.Run("One_Day_Range", func(t *testing.T) {
		startDate := time.Now().Truncate(24 * time.Hour)
		endDate := startDate.Add(24 * time.Hour)

		metrics, err := repo.GetDashboardMetrics(f.Ctx, startDate, endDate)
		require.NoError(t, err)
		require.NotNil(t, metrics)

		assert.GreaterOrEqual(t, metrics.TenantCount, 0)
	})

	t.Run("Long_Range_90_Days", func(t *testing.T) {
		startDate := time.Now().AddDate(0, 0, -90)
		endDate := time.Now()

		metrics, err := repo.GetDashboardMetrics(f.Ctx, startDate, endDate)
		require.NoError(t, err)
		require.NotNil(t, metrics)

		assert.GreaterOrEqual(t, metrics.TenantCount, 0)
	})

	t.Run("Context_Cancellation", func(t *testing.T) {
		ctx, cancel := context.WithCancel(f.Ctx)
		cancel() // Cancel immediately

		startDate := time.Now().AddDate(0, 0, -7)
		endDate := time.Now()

		metrics, err := repo.GetDashboardMetrics(ctx, startDate, endDate)
		require.Error(t, err)
		assert.Nil(t, metrics)
	})

	t.Run("With_Transaction_Context", func(t *testing.T) {
		txCtx := f.WithTx(f.Ctx)

		startDate := time.Now().AddDate(0, 0, -7)
		endDate := time.Now()

		metrics, err := repo.GetDashboardMetrics(txCtx, startDate, endDate)
		require.NoError(t, err)
		require.NotNil(t, metrics)

		assert.GreaterOrEqual(t, metrics.TenantCount, 0)
	})

	t.Run("Metrics_Should_Be_Non_Negative", func(t *testing.T) {
		startDate := time.Now().AddDate(0, 0, -7)
		endDate := time.Now()

		metrics, err := repo.GetDashboardMetrics(f.Ctx, startDate, endDate)
		require.NoError(t, err)
		require.NotNil(t, metrics)

		// All metrics should be non-negative
		assert.GreaterOrEqual(t, metrics.TenantCount, 0, "TenantCount should be non-negative")
		assert.GreaterOrEqual(t, metrics.UserCount, 0, "UserCount should be non-negative")
		assert.GreaterOrEqual(t, metrics.DAU, 0, "DAU should be non-negative")
		assert.GreaterOrEqual(t, metrics.WAU, 0, "WAU should be non-negative")
		assert.GreaterOrEqual(t, metrics.MAU, 0, "MAU should be non-negative")
		assert.GreaterOrEqual(t, metrics.SessionCount, 0, "SessionCount should be non-negative")
	})

	t.Run("Active_User_Hierarchy_DAU_LTE_WAU_LTE_MAU", func(t *testing.T) {
		startDate := time.Now().AddDate(0, 0, -30)
		endDate := time.Now()

		metrics, err := repo.GetDashboardMetrics(f.Ctx, startDate, endDate)
		require.NoError(t, err)
		require.NotNil(t, metrics)

		// DAU should be <= WAU <= MAU (active user hierarchy)
		assert.LessOrEqual(t, metrics.DAU, metrics.WAU, "DAU should be <= WAU")
		assert.LessOrEqual(t, metrics.WAU, metrics.MAU, "WAU should be <= MAU")
	})
}

func TestPgAnalyticsQueryRepository_GetTenantCount(t *testing.T) {
	t.Parallel()
	f := setupTest(t)

	repo := persistence.NewPgAnalyticsQueryRepository()

	t.Run("Happy_Path", func(t *testing.T) {
		count, err := repo.GetTenantCount(f.Ctx)
		require.NoError(t, err)
		assert.GreaterOrEqual(t, count, 0, "Tenant count should be non-negative")
	})

	t.Run("With_Transaction_Context", func(t *testing.T) {
		txCtx := f.WithTx(f.Ctx)

		count, err := repo.GetTenantCount(txCtx)
		require.NoError(t, err)
		assert.GreaterOrEqual(t, count, 0)
	})

	t.Run("Context_Cancellation", func(t *testing.T) {
		ctx, cancel := context.WithCancel(f.Ctx)
		cancel() // Cancel immediately

		count, err := repo.GetTenantCount(ctx)
		require.Error(t, err)
		assert.Equal(t, 0, count)
	})

	t.Run("Multiple_Tenants_Created", func(t *testing.T) {
		// Get initial count
		initialCount, err := repo.GetTenantCount(f.Ctx)
		require.NoError(t, err)

		// Create additional tenants
		_, err = itf.CreateTestTenant(f.Ctx, f.Pool)
		require.NoError(t, err)

		_, err = itf.CreateTestTenant(f.Ctx, f.Pool)
		require.NoError(t, err)

		// Get new count
		newCount, err := repo.GetTenantCount(f.Ctx)
		require.NoError(t, err)

		// Count should have increased by 2
		assert.Equal(t, initialCount+2, newCount, "Count should increase by 2 after creating 2 tenants")
	})
}

func TestPgAnalyticsQueryRepository_GetUserCount(t *testing.T) {
	t.Parallel()
	f := setupTest(t)

	repo := persistence.NewPgAnalyticsQueryRepository()

	t.Run("Happy_Path", func(t *testing.T) {
		count, err := repo.GetUserCount(f.Ctx)
		require.NoError(t, err)
		assert.GreaterOrEqual(t, count, 0, "User count should be non-negative")
	})

	t.Run("With_Transaction_Context", func(t *testing.T) {
		txCtx := f.WithTx(f.Ctx)

		count, err := repo.GetUserCount(txCtx)
		require.NoError(t, err)
		assert.GreaterOrEqual(t, count, 0)
	})

	t.Run("Context_Cancellation", func(t *testing.T) {
		ctx, cancel := context.WithCancel(f.Ctx)
		cancel() // Cancel immediately

		count, err := repo.GetUserCount(ctx)
		require.Error(t, err)
		assert.Equal(t, 0, count)
	})

	t.Run("Count_Increases_After_User_Creation", func(t *testing.T) {
		uploadRepo := corePersistence.NewUploadRepository()
		userRepo := corePersistence.NewUserRepository(uploadRepo)

		tenantID, err := composables.UseTenantID(f.Ctx)
		require.NoError(t, err)

		// Get initial count
		initialCount, err := repo.GetUserCount(f.Ctx)
		require.NoError(t, err)

		// Create a new user
		email, err := internet.NewEmail("newuser@test.com")
		require.NoError(t, err)

		newUser := user.New("New", "User", email, user.UILanguageEN, user.WithTenantID(tenantID))
		_, err = userRepo.Create(f.Ctx, newUser)
		require.NoError(t, err)

		// Get new count
		newCount, err := repo.GetUserCount(f.Ctx)
		require.NoError(t, err)

		// Count should have increased by 1
		assert.Equal(t, initialCount+1, newCount, "Count should increase by 1 after creating 1 user")
	})
}

func TestPgAnalyticsQueryRepository_GetActiveUsersCount(t *testing.T) {
	t.Parallel()
	f := setupTest(t)

	repo := persistence.NewPgAnalyticsQueryRepository()

	t.Run("Happy_Path_Last_24_Hours", func(t *testing.T) {
		since := time.Now().AddDate(0, 0, -1)

		count, err := repo.GetActiveUsersCount(f.Ctx, since)
		require.NoError(t, err)
		assert.GreaterOrEqual(t, count, 0, "Active user count should be non-negative")
	})

	t.Run("Last_7_Days", func(t *testing.T) {
		since := time.Now().AddDate(0, 0, -7)

		count, err := repo.GetActiveUsersCount(f.Ctx, since)
		require.NoError(t, err)
		assert.GreaterOrEqual(t, count, 0)
	})

	t.Run("Last_30_Days", func(t *testing.T) {
		since := time.Now().AddDate(0, 0, -30)

		count, err := repo.GetActiveUsersCount(f.Ctx, since)
		require.NoError(t, err)
		assert.GreaterOrEqual(t, count, 0)
	})

	t.Run("Future_Date_Returns_Zero", func(t *testing.T) {
		since := time.Now().AddDate(0, 0, 1)

		count, err := repo.GetActiveUsersCount(f.Ctx, since)
		require.NoError(t, err)
		assert.Equal(t, 0, count, "Future date should return 0 active users")
	})

	t.Run("With_Transaction_Context", func(t *testing.T) {
		txCtx := f.WithTx(f.Ctx)
		since := time.Now().AddDate(0, 0, -7)

		count, err := repo.GetActiveUsersCount(txCtx, since)
		require.NoError(t, err)
		assert.GreaterOrEqual(t, count, 0)
	})

	t.Run("Context_Cancellation", func(t *testing.T) {
		ctx, cancel := context.WithCancel(f.Ctx)
		cancel() // Cancel immediately

		since := time.Now().AddDate(0, 0, -7)

		count, err := repo.GetActiveUsersCount(ctx, since)
		require.Error(t, err)
		assert.Equal(t, 0, count)
	})

	t.Run("Count_Hierarchy_DAU_LTE_WAU_LTE_MAU", func(t *testing.T) {
		now := time.Now()
		dayAgo := now.AddDate(0, 0, -1)
		weekAgo := now.AddDate(0, 0, -7)
		monthAgo := now.AddDate(0, 0, -30)

		dau, err := repo.GetActiveUsersCount(f.Ctx, dayAgo)
		require.NoError(t, err)

		wau, err := repo.GetActiveUsersCount(f.Ctx, weekAgo)
		require.NoError(t, err)

		mau, err := repo.GetActiveUsersCount(f.Ctx, monthAgo)
		require.NoError(t, err)

		// DAU should be <= WAU <= MAU
		assert.LessOrEqual(t, dau, wau, "DAU should be <= WAU")
		assert.LessOrEqual(t, wau, mau, "WAU should be <= MAU")
	})
}

func TestPgAnalyticsQueryRepository_ListTenants(t *testing.T) {
	t.Parallel()
	f := setupTest(t)

	repo := persistence.NewPgAnalyticsQueryRepository()

	t.Run("Happy_Path_With_Pagination", func(t *testing.T) {
		tenants, total, err := repo.ListTenants(f.Ctx, 10, 0, emptySortBy)
		require.NoError(t, err)
		assert.NotNil(t, tenants)
		assert.GreaterOrEqual(t, total, 0)

		// Verify tenant info structure
		for _, tenant := range tenants {
			assert.NotEqual(t, uuid.Nil, tenant.ID)
			assert.NotEmpty(t, tenant.Name)
			assert.GreaterOrEqual(t, tenant.UserCount, 0)
			assert.False(t, tenant.CreatedAt.IsZero())
			assert.False(t, tenant.UpdatedAt.IsZero())
		}
	})

	t.Run("First_Page_Limit_5", func(t *testing.T) {
		tenants, total, err := repo.ListTenants(f.Ctx, 5, 0, emptySortBy)
		require.NoError(t, err)
		assert.LessOrEqual(t, len(tenants), 5)
		assert.GreaterOrEqual(t, total, len(tenants))
	})

	t.Run("Second_Page_Offset_5", func(t *testing.T) {
		_, total, err := repo.ListTenants(f.Ctx, 5, 5, emptySortBy)
		require.NoError(t, err)
		// Result can be empty if there are fewer than 6 tenants total
		assert.GreaterOrEqual(t, total, 0)
	})

	t.Run("Large_Limit", func(t *testing.T) {
		tenants, total, err := repo.ListTenants(f.Ctx, 1000, 0, emptySortBy)
		require.NoError(t, err)
		assert.NotNil(t, tenants)
		assert.Equal(t, len(tenants), total, "With large limit, should return all tenants")
	})

	t.Run("Zero_Limit_Returns_Empty", func(t *testing.T) {
		tenants, total, err := repo.ListTenants(f.Ctx, 0, 0, emptySortBy)
		require.NoError(t, err)
		assert.Empty(t, tenants)
		assert.GreaterOrEqual(t, total, 0)
	})

	t.Run("Negative_Limit_Returns_Error", func(t *testing.T) {
		tenants, total, err := repo.ListTenants(f.Ctx, -1, 0, emptySortBy)
		require.Error(t, err)
		assert.Nil(t, tenants)
		assert.Equal(t, 0, total)
		assert.Contains(t, err.Error(), "limit cannot be negative")
	})

	t.Run("High_Offset_Returns_Empty", func(t *testing.T) {
		tenants, total, err := repo.ListTenants(f.Ctx, 10, 1000000, emptySortBy)
		require.NoError(t, err)
		assert.Empty(t, tenants)
		assert.GreaterOrEqual(t, total, 0)
	})

	t.Run("With_Transaction_Context", func(t *testing.T) {
		txCtx := f.WithTx(f.Ctx)

		tenants, total, err := repo.ListTenants(txCtx, 10, 0, emptySortBy)
		require.NoError(t, err)
		assert.NotNil(t, tenants)
		assert.GreaterOrEqual(t, total, 0)
	})

	t.Run("Context_Cancellation", func(t *testing.T) {
		ctx, cancel := context.WithCancel(f.Ctx)
		cancel() // Cancel immediately

		tenants, total, err := repo.ListTenants(ctx, 10, 0, emptySortBy)
		require.Error(t, err)
		assert.Nil(t, tenants)
		assert.Equal(t, 0, total)
	})

	t.Run("Tenants_Sorted_By_CreatedAt_Descending", func(t *testing.T) {
		tenants, _, err := repo.ListTenants(f.Ctx, 10, 0, emptySortBy)
		require.NoError(t, err)

		if len(tenants) > 1 {
			for i := 0; i < len(tenants)-1; i++ {
				assert.True(t,
					tenants[i].CreatedAt.After(tenants[i+1].CreatedAt) ||
						tenants[i].CreatedAt.Equal(tenants[i+1].CreatedAt),
					"Tenants should be sorted by CreatedAt descending",
				)
			}
		}
	})

	t.Run("User_Count_Reflects_Actual_Users", func(t *testing.T) {
		uploadRepo := corePersistence.NewUploadRepository()
		userRepo := corePersistence.NewUserRepository(uploadRepo)

		// Create a new tenant
		newTenant, err := itf.CreateTestTenant(f.Ctx, f.Pool)
		require.NoError(t, err)

		// Create users for the tenant
		tenantCtx := composables.WithTenantID(f.Ctx, newTenant.ID)

		email1, err := internet.NewEmail("user1@tenant.com")
		require.NoError(t, err)
		user1 := user.New("User", "One", email1, user.UILanguageEN, user.WithTenantID(newTenant.ID))
		_, err = userRepo.Create(tenantCtx, user1)
		require.NoError(t, err)

		email2, err := internet.NewEmail("user2@tenant.com")
		require.NoError(t, err)
		user2 := user.New("User", "Two", email2, user.UILanguageEN, user.WithTenantID(newTenant.ID))
		_, err = userRepo.Create(tenantCtx, user2)
		require.NoError(t, err)

		// List tenants and find the new one
		tenants, _, err := repo.ListTenants(f.Ctx, 100, 0, emptySortBy)
		require.NoError(t, err)

		var foundTenant *struct {
			ID        uuid.UUID
			UserCount int
		}
		for _, tenant := range tenants {
			if tenant.ID == newTenant.ID {
				foundTenant = &struct {
					ID        uuid.UUID
					UserCount int
				}{
					ID:        tenant.ID,
					UserCount: tenant.UserCount,
				}
				break
			}
		}

		require.NotNil(t, foundTenant, "Created tenant should be found in list")
		assert.Equal(t, 2, foundTenant.UserCount, "Tenant should have 2 users")
	})
}

func TestPgAnalyticsQueryRepository_ListTenants_Sorting(t *testing.T) {
	t.Parallel()
	f := setupTest(t)

	repo := persistence.NewPgAnalyticsQueryRepository()

	t.Run("Sort_By_Created_At_Asc", func(t *testing.T) {
		tenants, _, err := repo.ListTenants(f.Ctx, 10, 0, sortBy("created_at", true))
		require.NoError(t, err)

		// Verify ascending order
		if len(tenants) > 1 {
			for i := 0; i < len(tenants)-1; i++ {
				assert.True(t,
					tenants[i].CreatedAt.Before(tenants[i+1].CreatedAt) ||
						tenants[i].CreatedAt.Equal(tenants[i+1].CreatedAt),
					"Tenants should be sorted by CreatedAt ascending",
				)
			}
		}
	})

	t.Run("Sort_By_Created_At_Desc", func(t *testing.T) {
		tenants, _, err := repo.ListTenants(f.Ctx, 10, 0, sortBy("created_at", false))
		require.NoError(t, err)

		// Verify descending order
		if len(tenants) > 1 {
			for i := 0; i < len(tenants)-1; i++ {
				assert.True(t,
					tenants[i].CreatedAt.After(tenants[i+1].CreatedAt) ||
						tenants[i].CreatedAt.Equal(tenants[i+1].CreatedAt),
					"Tenants should be sorted by CreatedAt descending",
				)
			}
		}
	})

	t.Run("Invalid_Sort_Field_Uses_Default", func(t *testing.T) {
		tenants, _, err := repo.ListTenants(f.Ctx, 10, 0, sortBy("invalid_field", true))
		require.NoError(t, err)

		// Should default to created_at DESC
		if len(tenants) > 1 {
			for i := 0; i < len(tenants)-1; i++ {
				assert.True(t,
					tenants[i].CreatedAt.After(tenants[i+1].CreatedAt) ||
						tenants[i].CreatedAt.Equal(tenants[i+1].CreatedAt),
					"Invalid sort field should default to CreatedAt descending",
				)
			}
		}
	})

	t.Run("Invalid_Sort_Order_Uses_Default", func(t *testing.T) {
		tenants, _, err := repo.ListTenants(f.Ctx, 10, 0, sortBy("created_at", false))
		require.NoError(t, err)

		// Should default to DESC
		if len(tenants) > 1 {
			for i := 0; i < len(tenants)-1; i++ {
				assert.True(t,
					tenants[i].CreatedAt.After(tenants[i+1].CreatedAt) ||
						tenants[i].CreatedAt.Equal(tenants[i+1].CreatedAt),
					"Invalid sort order should default to descending",
				)
			}
		}
	})

	t.Run("Empty_Sort_Params_Uses_Default", func(t *testing.T) {
		tenants, _, err := repo.ListTenants(f.Ctx, 10, 0, emptySortBy)
		require.NoError(t, err)

		// Should default to created_at DESC
		if len(tenants) > 1 {
			for i := 0; i < len(tenants)-1; i++ {
				assert.True(t,
					tenants[i].CreatedAt.After(tenants[i+1].CreatedAt) ||
						tenants[i].CreatedAt.Equal(tenants[i+1].CreatedAt),
					"Empty sort params should default to CreatedAt descending",
				)
			}
		}
	})

	t.Run("SQL_Injection_Protection", func(t *testing.T) {
		// Should not cause SQL errors or execute malicious code
		tenants, _, err := repo.ListTenants(f.Ctx, 10, 0, sortBy("name; DROP TABLE tenants;", true))
		require.NoError(t, err)
		assert.NotNil(t, tenants)
	})
}

func TestPgAnalyticsQueryRepository_FilterTenantsByDateRange(t *testing.T) {
	t.Parallel()
	f := setupTest(t)

	repo := persistence.NewPgAnalyticsQueryRepository()

	t.Run("Happy_Path_Last_7_Days", func(t *testing.T) {
		startDate := time.Now().AddDate(0, 0, -7)
		endDate := time.Now()

		tenants, total, err := repo.FilterTenantsByDateRange(f.Ctx, startDate, endDate, 10, 0, emptySortBy)
		require.NoError(t, err)
		assert.NotNil(t, tenants)
		assert.GreaterOrEqual(t, total, 0)

		// Verify all tenants are within date range
		for _, tenant := range tenants {
			assert.True(t,
				(tenant.CreatedAt.After(startDate) || tenant.CreatedAt.Equal(startDate)) &&
					(tenant.CreatedAt.Before(endDate) || tenant.CreatedAt.Equal(endDate)),
				"Tenant should be within date range",
			)
		}
	})

	t.Run("Last_30_Days", func(t *testing.T) {
		startDate := time.Now().AddDate(0, 0, -30)
		endDate := time.Now()

		tenants, total, err := repo.FilterTenantsByDateRange(f.Ctx, startDate, endDate, 10, 0, emptySortBy)
		require.NoError(t, err)
		assert.NotNil(t, tenants)
		assert.GreaterOrEqual(t, total, 0)
	})

	t.Run("One_Day_Range", func(t *testing.T) {
		startDate := time.Now().Truncate(24 * time.Hour)
		endDate := startDate.Add(24 * time.Hour)

		tenants, total, err := repo.FilterTenantsByDateRange(f.Ctx, startDate, endDate, 10, 0, emptySortBy)
		require.NoError(t, err)
		assert.NotNil(t, tenants)
		assert.GreaterOrEqual(t, total, 0)
	})

	t.Run("Future_Date_Range_Returns_Empty", func(t *testing.T) {
		startDate := time.Now().AddDate(0, 0, 1)
		endDate := time.Now().AddDate(0, 0, 7)

		tenants, total, err := repo.FilterTenantsByDateRange(f.Ctx, startDate, endDate, 10, 0, emptySortBy)
		require.NoError(t, err)
		assert.Empty(t, tenants)
		assert.Equal(t, 0, total)
	})

	t.Run("With_Pagination", func(t *testing.T) {
		startDate := time.Now().AddDate(0, 0, -90)
		endDate := time.Now()

		// First page
		firstPage, total, err := repo.FilterTenantsByDateRange(f.Ctx, startDate, endDate, 5, 0, emptySortBy)
		require.NoError(t, err)
		assert.LessOrEqual(t, len(firstPage), 5)

		// Second page
		secondPage, total2, err := repo.FilterTenantsByDateRange(f.Ctx, startDate, endDate, 5, 5, emptySortBy)
		require.NoError(t, err)
		assert.Equal(t, total, total2, "Total should be same across pages")

		// Verify no overlap
		if len(firstPage) > 0 && len(secondPage) > 0 {
			for _, t1 := range firstPage {
				for _, t2 := range secondPage {
					assert.NotEqual(t, t1.ID, t2.ID, "Pages should not have overlapping tenants")
				}
			}
		}
	})

	t.Run("Zero_Limit_Returns_Empty", func(t *testing.T) {
		startDate := time.Now().AddDate(0, 0, -7)
		endDate := time.Now()

		tenants, total, err := repo.FilterTenantsByDateRange(f.Ctx, startDate, endDate, 0, 0, emptySortBy)
		require.NoError(t, err)
		assert.Empty(t, tenants)
		assert.GreaterOrEqual(t, total, 0)
	})

	t.Run("Negative_Limit_Returns_Error", func(t *testing.T) {
		startDate := time.Now().AddDate(0, 0, -7)
		endDate := time.Now()

		tenants, total, err := repo.FilterTenantsByDateRange(f.Ctx, startDate, endDate, -5, 0, emptySortBy)
		require.Error(t, err)
		assert.Nil(t, tenants)
		assert.Equal(t, 0, total)
		assert.Contains(t, err.Error(), "limit cannot be negative")
	})

	t.Run("With_Transaction_Context", func(t *testing.T) {
		txCtx := f.WithTx(f.Ctx)
		startDate := time.Now().AddDate(0, 0, -7)
		endDate := time.Now()

		tenants, total, err := repo.FilterTenantsByDateRange(txCtx, startDate, endDate, 10, 0, emptySortBy)
		require.NoError(t, err)
		assert.NotNil(t, tenants)
		assert.GreaterOrEqual(t, total, 0)
	})

	t.Run("Context_Cancellation", func(t *testing.T) {
		ctx, cancel := context.WithCancel(f.Ctx)
		cancel() // Cancel immediately

		startDate := time.Now().AddDate(0, 0, -7)
		endDate := time.Now()

		tenants, total, err := repo.FilterTenantsByDateRange(ctx, startDate, endDate, 10, 0, emptySortBy)
		require.Error(t, err)
		assert.Nil(t, tenants)
		assert.Equal(t, 0, total)
	})

	t.Run("Tenants_Sorted_By_CreatedAt_Descending", func(t *testing.T) {
		startDate := time.Now().AddDate(0, 0, -90)
		endDate := time.Now()

		tenants, _, err := repo.FilterTenantsByDateRange(f.Ctx, startDate, endDate, 10, 0, emptySortBy)
		require.NoError(t, err)

		if len(tenants) > 1 {
			for i := 0; i < len(tenants)-1; i++ {
				assert.True(t,
					tenants[i].CreatedAt.After(tenants[i+1].CreatedAt) ||
						tenants[i].CreatedAt.Equal(tenants[i+1].CreatedAt),
					"Tenants should be sorted by CreatedAt descending",
				)
			}
		}
	})
}

func TestPgAnalyticsQueryRepository_GetTenantDetails(t *testing.T) {
	t.Parallel()
	f := setupTest(t)

	repo := persistence.NewPgAnalyticsQueryRepository()

	t.Run("Happy_Path_Existing_Tenant", func(t *testing.T) {
		// Create a test tenant
		newTenant, err := itf.CreateTestTenant(f.Ctx, f.Pool)
		require.NoError(t, err)

		// Get tenant details
		details, err := repo.GetTenantDetails(f.Ctx, newTenant.ID)
		require.NoError(t, err)
		require.NotNil(t, details)

		assert.Equal(t, newTenant.ID, details.ID)
		assert.Equal(t, newTenant.Name, details.Name)
		assert.GreaterOrEqual(t, details.UserCount, 0)
		assert.False(t, details.CreatedAt.IsZero())
		assert.False(t, details.UpdatedAt.IsZero())
	})

	t.Run("NonExistent_Tenant_Returns_Error", func(t *testing.T) {
		nonExistentID := uuid.New()

		details, err := repo.GetTenantDetails(f.Ctx, nonExistentID)
		require.Error(t, err)
		assert.Nil(t, details)
		assert.Contains(t, err.Error(), "tenant not found")
	})

	t.Run("Nil_UUID_Returns_Error", func(t *testing.T) {
		details, err := repo.GetTenantDetails(f.Ctx, uuid.Nil)
		require.Error(t, err)
		assert.Nil(t, details)
	})

	t.Run("Tenant_With_Users_Shows_Correct_Count", func(t *testing.T) {
		uploadRepo := corePersistence.NewUploadRepository()
		userRepo := corePersistence.NewUserRepository(uploadRepo)

		// Create a new tenant
		newTenant, err := itf.CreateTestTenant(f.Ctx, f.Pool)
		require.NoError(t, err)

		// Create users for the tenant
		tenantCtx := composables.WithTenantID(f.Ctx, newTenant.ID)

		email1, err := internet.NewEmail("user1@details.com")
		require.NoError(t, err)
		user1 := user.New("User", "One", email1, user.UILanguageEN, user.WithTenantID(newTenant.ID))
		_, err = userRepo.Create(tenantCtx, user1)
		require.NoError(t, err)

		email2, err := internet.NewEmail("user2@details.com")
		require.NoError(t, err)
		user2 := user.New("User", "Two", email2, user.UILanguageEN, user.WithTenantID(newTenant.ID))
		_, err = userRepo.Create(tenantCtx, user2)
		require.NoError(t, err)

		email3, err := internet.NewEmail("user3@details.com")
		require.NoError(t, err)
		user3 := user.New("User", "Three", email3, user.UILanguageEN, user.WithTenantID(newTenant.ID))
		_, err = userRepo.Create(tenantCtx, user3)
		require.NoError(t, err)

		// Get tenant details
		details, err := repo.GetTenantDetails(f.Ctx, newTenant.ID)
		require.NoError(t, err)
		require.NotNil(t, details)

		assert.Equal(t, 3, details.UserCount, "Tenant should have 3 users")
	})

	t.Run("Tenant_Without_Users_Shows_Zero_Count", func(t *testing.T) {
		// Create a new tenant without users
		newTenant, err := itf.CreateTestTenant(f.Ctx, f.Pool)
		require.NoError(t, err)

		// Get tenant details
		details, err := repo.GetTenantDetails(f.Ctx, newTenant.ID)
		require.NoError(t, err)
		require.NotNil(t, details)

		assert.Equal(t, 0, details.UserCount, "Tenant without users should have count 0")
	})

	t.Run("With_Transaction_Context", func(t *testing.T) {
		txCtx := f.WithTx(f.Ctx)

		// Create a test tenant
		newTenant, err := itf.CreateTestTenant(f.Ctx, f.Pool)
		require.NoError(t, err)

		details, err := repo.GetTenantDetails(txCtx, newTenant.ID)
		require.NoError(t, err)
		require.NotNil(t, details)

		assert.Equal(t, newTenant.ID, details.ID)
	})

	t.Run("Context_Cancellation", func(t *testing.T) {
		ctx, cancel := context.WithCancel(f.Ctx)
		cancel() // Cancel immediately

		tenantID := uuid.New()

		details, err := repo.GetTenantDetails(ctx, tenantID)
		require.Error(t, err)
		assert.Nil(t, details)
	})
}

// Table-driven test for various date ranges
func TestPgAnalyticsQueryRepository_DateRanges(t *testing.T) {
	t.Parallel()
	f := setupTest(t)

	repo := persistence.NewPgAnalyticsQueryRepository()

	now := time.Now()
	testCases := []struct {
		name      string
		startDate time.Time
		endDate   time.Time
	}{
		{
			name:      "Last_7_Days",
			startDate: now.AddDate(0, 0, -7),
			endDate:   now,
		},
		{
			name:      "Last_30_Days",
			startDate: now.AddDate(0, 0, -30),
			endDate:   now,
		},
		{
			name:      "Last_90_Days",
			startDate: now.AddDate(0, 0, -90),
			endDate:   now,
		},
		{
			name:      "Last_Year",
			startDate: now.AddDate(-1, 0, 0),
			endDate:   now,
		},
		{
			name:      "Same_Day",
			startDate: now.Truncate(24 * time.Hour),
			endDate:   now.Truncate(24 * time.Hour).Add(23*time.Hour + 59*time.Minute),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			metrics, err := repo.GetDashboardMetrics(f.Ctx, tc.startDate, tc.endDate)
			require.NoError(t, err)
			require.NotNil(t, metrics)

			tenants, total, err := repo.FilterTenantsByDateRange(f.Ctx, tc.startDate, tc.endDate, 10, 0, emptySortBy)
			require.NoError(t, err)
			assert.NotNil(t, tenants)
			assert.GreaterOrEqual(t, total, 0)
		})
	}
}

// Benchmark tests
func BenchmarkPgAnalyticsQueryRepository_GetDashboardMetrics(b *testing.B) {
	f := setupTest(b)
	repo := persistence.NewPgAnalyticsQueryRepository()

	startDate := time.Now().AddDate(0, 0, -30)
	endDate := time.Now()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := repo.GetDashboardMetrics(f.Ctx, startDate, endDate)
		if err != nil {
			b.Fatalf("unexpected error: %v", err)
		}
	}
}

func BenchmarkPgAnalyticsQueryRepository_GetTenantCount(b *testing.B) {
	f := setupTest(b)
	repo := persistence.NewPgAnalyticsQueryRepository()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := repo.GetTenantCount(f.Ctx)
		if err != nil {
			b.Fatalf("unexpected error: %v", err)
		}
	}
}

func BenchmarkPgAnalyticsQueryRepository_ListTenants(b *testing.B) {
	f := setupTest(b)
	repo := persistence.NewPgAnalyticsQueryRepository()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _, err := repo.ListTenants(f.Ctx, 10, 0, emptySortBy)
		if err != nil {
			b.Fatalf("unexpected error: %v", err)
		}
	}
}

func TestPgAnalyticsQueryRepository_SearchTenants(t *testing.T) {
	t.Parallel()
	f := setupTest(t)

	repo := persistence.NewPgAnalyticsQueryRepository()

	t.Run("Happy_Path_Search_By_Name", func(t *testing.T) {
		tenants, total, err := repo.SearchTenants(f.Ctx, "Test", 10, 0, emptySortBy)
		require.NoError(t, err)
		assert.NotNil(t, tenants)
		assert.GreaterOrEqual(t, total, 0)

		// Verify all returned tenants match search
		for _, tenant := range tenants {
			matchesName := contains(tenant.Name, "Test")
			matchesDomain := contains(tenant.Domain, "Test")
			assert.True(t, matchesName || matchesDomain, "Tenant should match search criteria")
		}
	})

	t.Run("Case_Insensitive_Search", func(t *testing.T) {
		// Search with lowercase
		lowerResults, lowerTotal, err := repo.SearchTenants(f.Ctx, "test", 10, 0, emptySortBy)
		require.NoError(t, err)

		// Search with uppercase
		upperResults, upperTotal, err := repo.SearchTenants(f.Ctx, "TEST", 10, 0, emptySortBy)
		require.NoError(t, err)

		// Search with mixed case
		mixedResults, mixedTotal, err := repo.SearchTenants(f.Ctx, "TeSt", 10, 0, emptySortBy)
		require.NoError(t, err)

		// All should return same results
		assert.Equal(t, lowerTotal, upperTotal, "Case should not affect search")
		assert.Equal(t, lowerTotal, mixedTotal, "Case should not affect search")
		assert.Equal(t, len(lowerResults), len(upperResults))
		assert.Equal(t, len(lowerResults), len(mixedResults))
	})

	t.Run("Search_By_Domain", func(t *testing.T) {
		// Search for common domain pattern
		tenants, total, err := repo.SearchTenants(f.Ctx, ".com", 10, 0, emptySortBy)
		require.NoError(t, err)
		assert.NotNil(t, tenants)
		assert.GreaterOrEqual(t, total, 0)

		// Verify results contain domain match
		for _, tenant := range tenants {
			assert.True(t, contains(tenant.Domain, ".com"), "Domain should contain .com")
		}
	})

	t.Run("Partial_Match", func(t *testing.T) {
		tenants, total, err := repo.SearchTenants(f.Ctx, "Ten", 10, 0, emptySortBy)
		require.NoError(t, err)
		assert.NotNil(t, tenants)
		assert.GreaterOrEqual(t, total, 0)

		// Verify partial matches work
		for _, tenant := range tenants {
			matchesName := contains(tenant.Name, "Ten")
			matchesDomain := contains(tenant.Domain, "Ten")
			assert.True(t, matchesName || matchesDomain)
		}
	})

	t.Run("Empty_Search_Returns_All", func(t *testing.T) {
		// Empty search should be handled by service layer
		// But repository should handle it gracefully
		tenants, total, err := repo.SearchTenants(f.Ctx, "", 10, 0, emptySortBy)
		require.NoError(t, err)
		assert.NotNil(t, tenants)
		assert.GreaterOrEqual(t, total, 0)
	})

	t.Run("No_Results_Found", func(t *testing.T) {
		tenants, total, err := repo.SearchTenants(f.Ctx, "NonExistentTenantName12345", 10, 0, emptySortBy)
		require.NoError(t, err)
		assert.Empty(t, tenants)
		assert.Equal(t, 0, total)
	})

	t.Run("Search_With_Pagination", func(t *testing.T) {
		// First page
		firstPage, total, err := repo.SearchTenants(f.Ctx, "Test", 5, 0, emptySortBy)
		require.NoError(t, err)
		assert.LessOrEqual(t, len(firstPage), 5)

		// Second page
		secondPage, total2, err := repo.SearchTenants(f.Ctx, "Test", 5, 5, emptySortBy)
		require.NoError(t, err)
		assert.Equal(t, total, total2, "Total should be same across pages")

		// Verify no overlap
		if len(firstPage) > 0 && len(secondPage) > 0 {
			for _, t1 := range firstPage {
				for _, t2 := range secondPage {
					assert.NotEqual(t, t1.ID, t2.ID, "Pages should not overlap")
				}
			}
		}
	})

	t.Run("Special_Characters_In_Search", func(t *testing.T) {
		// Test with special characters that might break SQL
		// We're testing that these don't cause SQL errors, not that they match anything
		testCases := []string{
			"%",
			"_",
			"test%",
			"test_",
			"'test'",
			"test\\",
		}

		for _, searchTerm := range testCases {
			tenants, total, err := repo.SearchTenants(f.Ctx, searchTerm, 10, 0, emptySortBy)
			require.NoError(t, err, "Search with special character should not error: %s", searchTerm)
			// tenants can be nil (empty slice) if no matches
			assert.GreaterOrEqual(t, total, 0)
			// Verify result is consistent (length matches what we got)
			assert.Len(t, tenants, total)
		}
	})

	t.Run("Zero_Limit_Returns_Empty", func(t *testing.T) {
		tenants, total, err := repo.SearchTenants(f.Ctx, "Test", 0, 0, emptySortBy)
		require.NoError(t, err)
		assert.Empty(t, tenants)
		assert.GreaterOrEqual(t, total, 0)
	})

	t.Run("Negative_Limit_Returns_Error", func(t *testing.T) {
		tenants, total, err := repo.SearchTenants(f.Ctx, "Test", -10, 0, emptySortBy)
		require.Error(t, err)
		assert.Nil(t, tenants)
		assert.Equal(t, 0, total)
		assert.Contains(t, err.Error(), "limit cannot be negative")
	})

	t.Run("High_Offset_Returns_Empty", func(t *testing.T) {
		tenants, total, err := repo.SearchTenants(f.Ctx, "Test", 10, 1000000, emptySortBy)
		require.NoError(t, err)
		assert.Empty(t, tenants)
		assert.GreaterOrEqual(t, total, 0)
	})

	t.Run("With_Transaction_Context", func(t *testing.T) {
		txCtx := f.WithTx(f.Ctx)

		tenants, total, err := repo.SearchTenants(txCtx, "Test", 10, 0, emptySortBy)
		require.NoError(t, err)
		assert.NotNil(t, tenants)
		assert.GreaterOrEqual(t, total, 0)
	})

	t.Run("Context_Cancellation", func(t *testing.T) {
		ctx, cancel := context.WithCancel(f.Ctx)
		cancel() // Cancel immediately

		tenants, total, err := repo.SearchTenants(ctx, "Test", 10, 0, emptySortBy)
		require.Error(t, err)
		assert.Nil(t, tenants)
		assert.Equal(t, 0, total)
	})

	t.Run("Results_Sorted_By_CreatedAt_Descending", func(t *testing.T) {
		tenants, _, err := repo.SearchTenants(f.Ctx, "Test", 10, 0, emptySortBy)
		require.NoError(t, err)

		if len(tenants) > 1 {
			for i := 0; i < len(tenants)-1; i++ {
				assert.True(t,
					tenants[i].CreatedAt.After(tenants[i+1].CreatedAt) ||
						tenants[i].CreatedAt.Equal(tenants[i+1].CreatedAt),
					"Search results should be sorted by CreatedAt descending",
				)
			}
		}
	})

	t.Run("User_Count_Included_In_Search_Results", func(t *testing.T) {
		tenants, _, err := repo.SearchTenants(f.Ctx, "Test", 10, 0, emptySortBy)
		require.NoError(t, err)

		for _, tenant := range tenants {
			assert.GreaterOrEqual(t, tenant.UserCount, 0, "User count should be non-negative")
			assert.NotEqual(t, uuid.Nil, tenant.ID)
			assert.NotEmpty(t, tenant.Name)
			assert.False(t, tenant.CreatedAt.IsZero())
			assert.False(t, tenant.UpdatedAt.IsZero())
		}
	})

	t.Run("Search_With_Multiple_Matching_Tenants", func(t *testing.T) {
		// Create multiple test tenants
		_, err := itf.CreateTestTenant(f.Ctx, f.Pool)
		require.NoError(t, err)

		_, err = itf.CreateTestTenant(f.Ctx, f.Pool)
		require.NoError(t, err)

		tenants, total, err := repo.SearchTenants(f.Ctx, "Test", 10, 0, emptySortBy)
		require.NoError(t, err)
		assert.NotNil(t, tenants)
		assert.GreaterOrEqual(t, total, 2, "Should have at least 2 test tenants")
	})
}

// contains checks if a string contains a substring (case-insensitive)
func contains(s, substr string) bool {
	return strings.Contains(strings.ToLower(s), strings.ToLower(substr))
}

// TestPgAnalyticsQueryRepository_LimitEdgeCases tests limit parameter validation across all methods
func TestPgAnalyticsQueryRepository_LimitEdgeCases(t *testing.T) {
	t.Parallel()
	f := setupTest(t)

	repo := persistence.NewPgAnalyticsQueryRepository()

	testCases := []struct {
		name          string
		limit         int
		expectError   bool
		expectEmpty   bool
		errorContains string
	}{
		{
			name:        "Zero_Limit",
			limit:       0,
			expectError: false,
			expectEmpty: true,
		},
		{
			name:          "Negative_Limit",
			limit:         -1,
			expectError:   true,
			expectEmpty:   true,
			errorContains: "limit cannot be negative",
		},
		{
			name:          "Large_Negative_Limit",
			limit:         -999999,
			expectError:   true,
			expectEmpty:   true,
			errorContains: "limit cannot be negative",
		},
		{
			name:        "Small_Positive_Limit",
			limit:       1,
			expectError: false,
			expectEmpty: false,
		},
		{
			name:        "Very_Large_Limit",
			limit:       999999,
			expectError: false,
			expectEmpty: false,
		},
	}

	for _, tc := range testCases {
		t.Run("ListTenants_"+tc.name, func(t *testing.T) {
			tenants, total, err := repo.ListTenants(f.Ctx, tc.limit, 0, emptySortBy)

			if tc.expectError {
				require.Error(t, err)
				assert.Nil(t, tenants)
				assert.Equal(t, 0, total)
				if tc.errorContains != "" {
					assert.Contains(t, err.Error(), tc.errorContains)
				}
			} else {
				require.NoError(t, err)
				assert.NotNil(t, tenants)
				assert.GreaterOrEqual(t, total, 0)

				if tc.expectEmpty {
					assert.Empty(t, tenants)
				}
			}
		})

		t.Run("SearchTenants_"+tc.name, func(t *testing.T) {
			tenants, total, err := repo.SearchTenants(f.Ctx, "Test", tc.limit, 0, emptySortBy)

			if tc.expectError {
				require.Error(t, err)
				assert.Nil(t, tenants)
				assert.Equal(t, 0, total)
				if tc.errorContains != "" {
					assert.Contains(t, err.Error(), tc.errorContains)
				}
			} else {
				require.NoError(t, err)
				assert.NotNil(t, tenants)
				assert.GreaterOrEqual(t, total, 0)

				if tc.expectEmpty {
					assert.Empty(t, tenants)
				}
			}
		})

		t.Run("FilterTenantsByDateRange_"+tc.name, func(t *testing.T) {
			startDate := time.Now().AddDate(0, 0, -7)
			endDate := time.Now()

			tenants, total, err := repo.FilterTenantsByDateRange(f.Ctx, startDate, endDate, tc.limit, 0, emptySortBy)

			if tc.expectError {
				require.Error(t, err)
				assert.Nil(t, tenants)
				assert.Equal(t, 0, total)
				if tc.errorContains != "" {
					assert.Contains(t, err.Error(), tc.errorContains)
				}
			} else {
				require.NoError(t, err)
				assert.NotNil(t, tenants)
				assert.GreaterOrEqual(t, total, 0)

				if tc.expectEmpty {
					assert.Empty(t, tenants)
				}
			}
		})
	}
}
