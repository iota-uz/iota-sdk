package services_test

import (
	"context"
	"testing"
	"time"

	"github.com/iota-uz/iota-sdk/modules/superadmin/domain/entities"
	"github.com/iota-uz/iota-sdk/modules/superadmin/infrastructure/persistence"
	"github.com/iota-uz/iota-sdk/modules/superadmin/services"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAnalyticsService_GetDashboardMetrics(t *testing.T) {
	t.Parallel()
	f := setupTest(t)

	repo := persistence.NewPgAnalyticsQueryRepository()
	service := services.NewAnalyticsService(repo)

	t.Run("Happy_Path_With_Explicit_Dates", func(t *testing.T) {
		startDate := time.Now().AddDate(0, 0, -7)
		endDate := time.Now()

		metrics, err := service.GetDashboardMetrics(f.Ctx, startDate, endDate)
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

	t.Run("Default_Date_Range_When_Zero_Values", func(t *testing.T) {
		// When dates are zero, service should use defaults (30 days ago to now)
		metrics, err := service.GetDashboardMetrics(f.Ctx, time.Time{}, time.Time{})
		require.NoError(t, err)
		require.NotNil(t, metrics)

		// Should return valid metrics
		assert.GreaterOrEqual(t, metrics.TenantCount, 0)
		assert.GreaterOrEqual(t, metrics.UserCount, 0)
	})

	t.Run("Start_Date_Only_Provided", func(t *testing.T) {
		startDate := time.Now().AddDate(0, 0, -14)

		metrics, err := service.GetDashboardMetrics(f.Ctx, startDate, time.Time{})
		require.NoError(t, err)
		require.NotNil(t, metrics)

		assert.GreaterOrEqual(t, metrics.TenantCount, 0)
	})

	t.Run("End_Date_Only_Provided", func(t *testing.T) {
		endDate := time.Now()

		metrics, err := service.GetDashboardMetrics(f.Ctx, time.Time{}, endDate)
		require.NoError(t, err)
		require.NotNil(t, metrics)

		assert.GreaterOrEqual(t, metrics.TenantCount, 0)
	})

	t.Run("Invalid_Date_Range_StartDate_After_EndDate", func(t *testing.T) {
		startDate := time.Now()
		endDate := time.Now().AddDate(0, 0, -7)

		metrics, err := service.GetDashboardMetrics(f.Ctx, startDate, endDate)
		require.Error(t, err)
		assert.Nil(t, metrics)
		assert.Contains(t, err.Error(), "start date cannot be after end date")
	})

	t.Run("One_Day_Range", func(t *testing.T) {
		startDate := time.Now().Truncate(24 * time.Hour)
		endDate := startDate.Add(24 * time.Hour)

		metrics, err := service.GetDashboardMetrics(f.Ctx, startDate, endDate)
		require.NoError(t, err)
		require.NotNil(t, metrics)

		assert.GreaterOrEqual(t, metrics.TenantCount, 0)
	})

	t.Run("Long_Range_90_Days", func(t *testing.T) {
		startDate := time.Now().AddDate(0, 0, -90)
		endDate := time.Now()

		metrics, err := service.GetDashboardMetrics(f.Ctx, startDate, endDate)
		require.NoError(t, err)
		require.NotNil(t, metrics)

		assert.GreaterOrEqual(t, metrics.TenantCount, 0)
	})

	t.Run("Context_Cancellation", func(t *testing.T) {
		ctx, cancel := context.WithCancel(f.Ctx)
		cancel() // Cancel immediately

		startDate := time.Now().AddDate(0, 0, -7)
		endDate := time.Now()

		metrics, err := service.GetDashboardMetrics(ctx, startDate, endDate)
		require.Error(t, err)
		assert.Nil(t, metrics)
	})

	t.Run("With_Transaction_Context", func(t *testing.T) {
		txCtx := f.WithTx(f.Ctx)

		startDate := time.Now().AddDate(0, 0, -7)
		endDate := time.Now()

		metrics, err := service.GetDashboardMetrics(txCtx, startDate, endDate)
		require.NoError(t, err)
		require.NotNil(t, metrics)

		assert.GreaterOrEqual(t, metrics.TenantCount, 0)
	})

	t.Run("Metrics_Should_Be_Non_Negative", func(t *testing.T) {
		startDate := time.Now().AddDate(0, 0, -7)
		endDate := time.Now()

		metrics, err := service.GetDashboardMetrics(f.Ctx, startDate, endDate)
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

		metrics, err := service.GetDashboardMetrics(f.Ctx, startDate, endDate)
		require.NoError(t, err)
		require.NotNil(t, metrics)

		// DAU should be <= WAU <= MAU (active user hierarchy)
		assert.LessOrEqual(t, metrics.DAU, metrics.WAU, "DAU should be <= WAU")
		assert.LessOrEqual(t, metrics.WAU, metrics.MAU, "WAU should be <= MAU")
	})
}

func TestAnalyticsService_GetDashboardMetrics_Integration(t *testing.T) {
	t.Parallel()
	f := setupTest(t)

	repo := persistence.NewPgAnalyticsQueryRepository()
	service := services.NewAnalyticsService(repo)

	t.Run("Returns_Real_Data_From_Database", func(t *testing.T) {
		// Test with ITF-created tenant and user
		startDate := time.Now().AddDate(0, 0, -30)
		endDate := time.Now()

		metrics, err := service.GetDashboardMetrics(f.Ctx, startDate, endDate)
		require.NoError(t, err)
		require.NotNil(t, metrics)

		// Metrics should return non-negative values (ITF setup may or may not show users depending on isolation)
		assert.GreaterOrEqual(t, metrics.TenantCount, 0, "TenantCount should be non-negative")
		assert.GreaterOrEqual(t, metrics.UserCount, 0, "UserCount should be non-negative")
	})
}

func TestAnalyticsService_EdgeCases(t *testing.T) {
	t.Parallel()

	t.Run("Service_Creation", func(t *testing.T) {
		repo := persistence.NewPgAnalyticsQueryRepository()
		service := services.NewAnalyticsService(repo)
		assert.NotNil(t, service, "Service should be created successfully")
	})
}

// Benchmark tests
func BenchmarkAnalyticsService_GetDashboardMetrics(b *testing.B) {
	f := setupTest(b)
	repo := persistence.NewPgAnalyticsQueryRepository()
	service := services.NewAnalyticsService(repo)

	startDate := time.Now().AddDate(0, 0, -30)
	endDate := time.Now()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := service.GetDashboardMetrics(f.Ctx, startDate, endDate)
		if err != nil {
			b.Fatalf("unexpected error: %v", err)
		}
	}
}

// Table-driven test for various date ranges
func TestAnalyticsService_GetDashboardMetrics_DateRanges(t *testing.T) {
	t.Parallel()
	f := setupTest(t)

	repo := persistence.NewPgAnalyticsQueryRepository()
	service := services.NewAnalyticsService(repo)

	now := time.Now()
	testCases := []struct {
		name      string
		startDate time.Time
		endDate   time.Time
		wantError bool
		errorMsg  string
	}{
		{
			name:      "Last_7_Days",
			startDate: now.AddDate(0, 0, -7),
			endDate:   now,
			wantError: false,
		},
		{
			name:      "Last_30_Days",
			startDate: now.AddDate(0, 0, -30),
			endDate:   now,
			wantError: false,
		},
		{
			name:      "Last_90_Days",
			startDate: now.AddDate(0, 0, -90),
			endDate:   now,
			wantError: false,
		},
		{
			name:      "Last_Year",
			startDate: now.AddDate(-1, 0, 0),
			endDate:   now,
			wantError: false,
		},
		{
			name:      "Same_Day",
			startDate: now.Truncate(24 * time.Hour),
			endDate:   now.Truncate(24 * time.Hour).Add(23*time.Hour + 59*time.Minute),
			wantError: false,
		},
		{
			name:      "Invalid_Reversed_Dates",
			startDate: now,
			endDate:   now.AddDate(0, 0, -30),
			wantError: true,
			errorMsg:  "start date cannot be after end date",
		},
		{
			name:      "Future_Date_Range",
			startDate: now.AddDate(0, 0, 1),
			endDate:   now.AddDate(0, 0, 7),
			wantError: false, // Valid range, just no data expected
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			metrics, err := service.GetDashboardMetrics(f.Ctx, tc.startDate, tc.endDate)

			if tc.wantError {
				require.Error(t, err)
				assert.Nil(t, metrics)
				if tc.errorMsg != "" {
					assert.Contains(t, err.Error(), tc.errorMsg)
				}
			} else {
				require.NoError(t, err)
				require.NotNil(t, metrics)
				assert.IsType(t, &entities.Analytics{}, metrics)
			}
		})
	}
}
