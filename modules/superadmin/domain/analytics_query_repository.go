package domain

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/modules/superadmin/domain/entities"
)

// AnalyticsQueryRepository provides read-only analytics queries for superadmin
type AnalyticsQueryRepository interface {
	// GetDashboardMetrics returns system-wide metrics for specified date range
	GetDashboardMetrics(ctx context.Context, startDate, endDate time.Time) (*entities.Analytics, error)

	// GetTenantCount returns total number of tenants
	GetTenantCount(ctx context.Context) (int, error)

	// GetUserCount returns total number of users across all tenants
	GetUserCount(ctx context.Context) (int, error)

	// GetActiveUsersCount returns count of active users (DAU/WAU/MAU)
	GetActiveUsersCount(ctx context.Context, since time.Time) (int, error)

	// ListTenants returns all tenants with user counts
	ListTenants(ctx context.Context, limit, offset int, sortField, sortOrder string) ([]*entities.TenantInfo, int, error)

	// SearchTenants returns tenants matching search criteria with user counts
	SearchTenants(ctx context.Context, search string, limit, offset int, sortField, sortOrder string) ([]*entities.TenantInfo, int, error)

	// FilterTenantsByDateRange returns tenants created within date range
	FilterTenantsByDateRange(ctx context.Context, startDate, endDate time.Time, limit, offset int, sortField, sortOrder string) ([]*entities.TenantInfo, int, error)

	// GetTenantDetails returns detailed information about a specific tenant
	GetTenantDetails(ctx context.Context, tenantID uuid.UUID) (*entities.TenantInfo, error)

	// GetUserSignupsTimeSeries returns daily user signup counts for the date range
	GetUserSignupsTimeSeries(ctx context.Context, startDate, endDate time.Time) ([]entities.TimeSeriesDataPoint, error)

	// GetTenantSignupsTimeSeries returns daily tenant signup counts for the date range
	GetTenantSignupsTimeSeries(ctx context.Context, startDate, endDate time.Time) ([]entities.TimeSeriesDataPoint, error)
}
