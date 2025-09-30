package persistence

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/modules/superadmin/domain"
	"github.com/iota-uz/iota-sdk/modules/superadmin/domain/entities"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/jackc/pgx/v5"
	"github.com/pkg/errors"
)

type pgAnalyticsQueryRepository struct{}

func NewPgAnalyticsQueryRepository() domain.AnalyticsQueryRepository {
	return &pgAnalyticsQueryRepository{}
}

const (
	getDashboardMetricsSQL = `
		SELECT
			(SELECT COUNT(*) FROM tenants) as tenant_count,
			(SELECT COUNT(*) FROM users) as user_count,
			(SELECT COUNT(DISTINCT user_id) FROM sessions WHERE created_at >= $1 AND created_at < $1 + interval '1 day') as dau,
			(SELECT COUNT(DISTINCT user_id) FROM sessions WHERE created_at >= $1 AND created_at < $1 + interval '7 days') as wau,
			(SELECT COUNT(DISTINCT user_id) FROM sessions WHERE created_at >= $1 AND created_at < $1 + interval '30 days') as mau,
			(SELECT COUNT(*) FROM sessions WHERE created_at >= $1 AND created_at <= $2) as session_count
	`

	getTenantCountSQL = `SELECT COUNT(*) FROM tenants`

	getUserCountSQL = `SELECT COUNT(*) FROM users`

	getActiveUsersCountSQL = `
		SELECT COUNT(DISTINCT user_id)
		FROM sessions
		WHERE created_at >= $1
	`

	listTenantsQuery = `
		SELECT
			t.id,
			t.name,
			t.domain,
			COALESCE(u.user_count, 0) as user_count,
			t.created_at,
			t.updated_at
		FROM tenants t
		LEFT JOIN (
			SELECT tenant_id, COUNT(*) as user_count
			FROM users
			GROUP BY tenant_id
		) u ON t.id = u.tenant_id
	`

	countTenantsSQL = `SELECT COUNT(*) FROM tenants`

	searchTenantsQuery = `
		SELECT
			t.id,
			t.name,
			t.domain,
			COALESCE(u.user_count, 0) as user_count,
			t.created_at,
			t.updated_at
		FROM tenants t
		LEFT JOIN (
			SELECT tenant_id, COUNT(*) as user_count
			FROM users
			GROUP BY tenant_id
		) u ON t.id = u.tenant_id
		WHERE t.name ILIKE $1 OR t.domain ILIKE $1
	`

	countTenantsSearchSQL = `
		SELECT COUNT(*)
		FROM tenants
		WHERE name ILIKE $1 OR domain ILIKE $1
	`

	filterTenantsByDateRangeQuery = `
		SELECT
			t.id,
			t.name,
			t.domain,
			COALESCE(u.user_count, 0) as user_count,
			t.created_at,
			t.updated_at
		FROM tenants t
		LEFT JOIN (
			SELECT tenant_id, COUNT(*) as user_count
			FROM users
			GROUP BY tenant_id
		) u ON t.id = u.tenant_id
		WHERE t.created_at >= $1 AND t.created_at <= $2
	`

	countTenantsByDateRangeSQL = `
		SELECT COUNT(*)
		FROM tenants
		WHERE created_at >= $1 AND created_at <= $2
	`

	getTenantDetailsSQL = `
		SELECT
			t.id,
			t.name,
			t.domain,
			COALESCE(u.user_count, 0) as user_count,
			t.created_at,
			t.updated_at
		FROM tenants t
		LEFT JOIN (
			SELECT tenant_id, COUNT(*) as user_count
			FROM users
			GROUP BY tenant_id
		) u ON t.id = u.tenant_id
		WHERE t.id = $1
	`

	getUserSignupsTimeSeriesSQL = `
		SELECT
			DATE(created_at) as date,
			COUNT(*) as count
		FROM users
		WHERE created_at >= $1 AND created_at <= $2
		GROUP BY DATE(created_at)
		ORDER BY date ASC
	`

	getTenantSignupsTimeSeriesSQL = `
		SELECT
			DATE(created_at) as date,
			COUNT(*) as count
		FROM tenants
		WHERE created_at >= $1 AND created_at <= $2
		GROUP BY DATE(created_at)
		ORDER BY date ASC
	`
)

// buildOrderByClause generates ORDER BY clause for tenant queries
// Defaults to DESC (newest first) if no valid sort specified
func buildOrderByClause(sortField, sortOrder string) string {
	// Only allow created_at for security
	if sortField == "created_at" {
		if sortOrder == "asc" {
			return "ORDER BY t.created_at ASC"
		}
		return "ORDER BY t.created_at DESC"
	}
	// Default: newest first
	return "ORDER BY t.created_at DESC"
}

func (r *pgAnalyticsQueryRepository) GetDashboardMetrics(ctx context.Context, startDate, endDate time.Time) (*entities.Analytics, error) {
	tx, err := composables.UseTx(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get transaction")
	}

	var metrics entities.Analytics
	err = tx.QueryRow(ctx, getDashboardMetricsSQL, startDate, endDate).Scan(
		&metrics.TenantCount,
		&metrics.UserCount,
		&metrics.DAU,
		&metrics.WAU,
		&metrics.MAU,
		&metrics.SessionCount,
	)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get dashboard metrics")
	}

	return &metrics, nil
}

func (r *pgAnalyticsQueryRepository) GetTenantCount(ctx context.Context) (int, error) {
	tx, err := composables.UseTx(ctx)
	if err != nil {
		return 0, errors.Wrap(err, "failed to get transaction")
	}

	var count int
	err = tx.QueryRow(ctx, getTenantCountSQL).Scan(&count)
	if err != nil {
		return 0, errors.Wrap(err, "failed to get tenant count")
	}

	return count, nil
}

func (r *pgAnalyticsQueryRepository) GetUserCount(ctx context.Context) (int, error) {
	tx, err := composables.UseTx(ctx)
	if err != nil {
		return 0, errors.Wrap(err, "failed to get transaction")
	}

	var count int
	err = tx.QueryRow(ctx, getUserCountSQL).Scan(&count)
	if err != nil {
		return 0, errors.Wrap(err, "failed to get user count")
	}

	return count, nil
}

func (r *pgAnalyticsQueryRepository) GetActiveUsersCount(ctx context.Context, since time.Time) (int, error) {
	tx, err := composables.UseTx(ctx)
	if err != nil {
		return 0, errors.Wrap(err, "failed to get transaction")
	}

	var count int
	err = tx.QueryRow(ctx, getActiveUsersCountSQL, since).Scan(&count)
	if err != nil {
		return 0, errors.Wrap(err, "failed to get active users count")
	}

	return count, nil
}

func (r *pgAnalyticsQueryRepository) ListTenants(ctx context.Context, limit, offset int, sortField, sortOrder string) ([]*entities.TenantInfo, int, error) {
	tx, err := composables.UseTx(ctx)
	if err != nil {
		return nil, 0, errors.Wrap(err, "failed to get transaction")
	}

	// Get total count
	var total int
	err = tx.QueryRow(ctx, countTenantsSQL).Scan(&total)
	if err != nil {
		return nil, 0, errors.Wrap(err, "failed to count tenants")
	}

	// Build query with sorting
	orderBy := buildOrderByClause(sortField, sortOrder)
	query := listTenantsQuery + " " + orderBy + " LIMIT $1 OFFSET $2"

	// Get tenants with user counts
	rows, err := tx.Query(ctx, query, limit, offset)
	if err != nil {
		return nil, 0, errors.Wrap(err, "failed to list tenants")
	}
	defer rows.Close()

	tenants, err := r.scanTenants(rows)
	if err != nil {
		return nil, 0, err
	}

	return tenants, total, nil
}

func (r *pgAnalyticsQueryRepository) SearchTenants(ctx context.Context, search string, limit, offset int, sortField, sortOrder string) ([]*entities.TenantInfo, int, error) {
	tx, err := composables.UseTx(ctx)
	if err != nil {
		return nil, 0, errors.Wrap(err, "failed to get transaction")
	}

	// Add wildcards for ILIKE pattern matching
	searchPattern := "%" + search + "%"

	// Get total count with search filter
	var total int
	err = tx.QueryRow(ctx, countTenantsSearchSQL, searchPattern).Scan(&total)
	if err != nil {
		return nil, 0, errors.Wrap(err, "failed to count tenants with search")
	}

	// Build query with sorting
	orderBy := buildOrderByClause(sortField, sortOrder)
	query := searchTenantsQuery + " " + orderBy + " LIMIT $1 OFFSET $2"

	// Get tenants matching search
	rows, err := tx.Query(ctx, query, searchPattern, limit, offset)
	if err != nil {
		return nil, 0, errors.Wrap(err, "failed to search tenants")
	}
	defer rows.Close()

	tenants, err := r.scanTenants(rows)
	if err != nil {
		return nil, 0, err
	}

	return tenants, total, nil
}

func (r *pgAnalyticsQueryRepository) FilterTenantsByDateRange(ctx context.Context, startDate, endDate time.Time, limit, offset int, sortField, sortOrder string) ([]*entities.TenantInfo, int, error) {
	tx, err := composables.UseTx(ctx)
	if err != nil {
		return nil, 0, errors.Wrap(err, "failed to get transaction")
	}

	// Get total count for date range
	var total int
	err = tx.QueryRow(ctx, countTenantsByDateRangeSQL, startDate, endDate).Scan(&total)
	if err != nil {
		return nil, 0, errors.Wrap(err, "failed to count tenants by date range")
	}

	// Build query with sorting
	orderBy := buildOrderByClause(sortField, sortOrder)
	query := filterTenantsByDateRangeQuery + " " + orderBy + " LIMIT $1 OFFSET $2"

	// Get tenants within date range
	rows, err := tx.Query(ctx, query, startDate, endDate, limit, offset)
	if err != nil {
		return nil, 0, errors.Wrap(err, "failed to filter tenants by date range")
	}
	defer rows.Close()

	tenants, err := r.scanTenants(rows)
	if err != nil {
		return nil, 0, err
	}

	return tenants, total, nil
}

func (r *pgAnalyticsQueryRepository) GetTenantDetails(ctx context.Context, tenantID uuid.UUID) (*entities.TenantInfo, error) {
	tx, err := composables.UseTx(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get transaction")
	}

	var tenant entities.TenantInfo
	err = tx.QueryRow(ctx, getTenantDetailsSQL, tenantID).Scan(
		&tenant.ID,
		&tenant.Name,
		&tenant.Domain,
		&tenant.UserCount,
		&tenant.CreatedAt,
		&tenant.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, errors.New("tenant not found")
		}
		return nil, errors.Wrap(err, "failed to get tenant details")
	}

	return &tenant, nil
}

func (r *pgAnalyticsQueryRepository) GetUserSignupsTimeSeries(ctx context.Context, startDate, endDate time.Time) ([]entities.TimeSeriesDataPoint, error) {
	tx, err := composables.UseTx(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get transaction")
	}

	rows, err := tx.Query(ctx, getUserSignupsTimeSeriesSQL, startDate, endDate)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get user signups time series")
	}
	defer rows.Close()

	var dataPoints []entities.TimeSeriesDataPoint
	for rows.Next() {
		var dp entities.TimeSeriesDataPoint
		err := rows.Scan(&dp.Date, &dp.Count)
		if err != nil {
			return nil, errors.Wrap(err, "failed to scan time series data point")
		}
		dataPoints = append(dataPoints, dp)
	}

	if err := rows.Err(); err != nil {
		return nil, errors.Wrap(err, "error iterating time series rows")
	}

	return dataPoints, nil
}

func (r *pgAnalyticsQueryRepository) GetTenantSignupsTimeSeries(ctx context.Context, startDate, endDate time.Time) ([]entities.TimeSeriesDataPoint, error) {
	tx, err := composables.UseTx(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get transaction")
	}

	rows, err := tx.Query(ctx, getTenantSignupsTimeSeriesSQL, startDate, endDate)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get tenant signups time series")
	}
	defer rows.Close()

	var dataPoints []entities.TimeSeriesDataPoint
	for rows.Next() {
		var dp entities.TimeSeriesDataPoint
		err := rows.Scan(&dp.Date, &dp.Count)
		if err != nil {
			return nil, errors.Wrap(err, "failed to scan time series data point")
		}
		dataPoints = append(dataPoints, dp)
	}

	if err := rows.Err(); err != nil {
		return nil, errors.Wrap(err, "error iterating time series rows")
	}

	return dataPoints, nil
}

func (r *pgAnalyticsQueryRepository) scanTenants(rows pgx.Rows) ([]*entities.TenantInfo, error) {
	var tenants []*entities.TenantInfo

	for rows.Next() {
		var tenant entities.TenantInfo
		err := rows.Scan(
			&tenant.ID,
			&tenant.Name,
			&tenant.Domain,
			&tenant.UserCount,
			&tenant.CreatedAt,
			&tenant.UpdatedAt,
		)
		if err != nil {
			return nil, errors.Wrap(err, "failed to scan tenant")
		}
		tenants = append(tenants, &tenant)
	}

	if err := rows.Err(); err != nil {
		return nil, errors.Wrap(err, "error iterating tenant rows")
	}

	return tenants, nil
}
