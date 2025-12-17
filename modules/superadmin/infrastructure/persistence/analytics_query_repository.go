package persistence

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/modules/superadmin/domain"
	"github.com/iota-uz/iota-sdk/modules/superadmin/domain/entities"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/repo"
	"github.com/jackc/pgx/v5"
	"github.com/pkg/errors"
)

// Field type aliases for sorting and filtering
type Field = string

const (
	FieldID        Field = "id"
	FieldName      Field = "name"
	FieldEmail     Field = "email"
	FieldPhone     Field = "phone"
	FieldDomain    Field = "domain"
	FieldUserCount Field = "user_count"
	FieldCreatedAt Field = "created_at"
	FieldUpdatedAt Field = "updated_at"
)

type SortBy = repo.SortBy[Field]
type Filter = repo.FieldFilter[Field]

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
			t.email,
			t.phone,
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
			t.email,
			t.phone,
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
			t.email,
			t.phone,
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
			t.email,
			t.phone,
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

// fieldMapping returns the mapping between logical field names and SQL column names
func (r *pgAnalyticsQueryRepository) fieldMapping() map[Field]string {
	return map[Field]string{
		FieldID:        "t.id",
		FieldName:      "t.name",
		FieldEmail:     "t.email",
		FieldPhone:     "t.phone",
		FieldDomain:    "t.domain",
		FieldUserCount: "COALESCE(u.user_count, 0)",
		FieldCreatedAt: "t.created_at",
		FieldUpdatedAt: "t.updated_at",
	}
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

func (r *pgAnalyticsQueryRepository) ListTenants(ctx context.Context, limit, offset int, sortBy SortBy) ([]*entities.TenantInfo, int, error) {
	tx, err := composables.UseTx(ctx)
	if err != nil {
		return nil, 0, errors.Wrap(err, "failed to get transaction")
	}

	// Validate limit parameter
	if limit < 0 {
		return nil, 0, errors.New("limit cannot be negative")
	}

	// Get total count
	var total int
	err = tx.QueryRow(ctx, countTenantsSQL).Scan(&total)
	if err != nil {
		return nil, 0, errors.Wrap(err, "failed to count tenants")
	}

	// Handle limit=0 case - return empty slice
	if limit == 0 {
		return []*entities.TenantInfo{}, total, nil
	}

	// Default sort if empty
	if len(sortBy.Fields) == 0 {
		sortBy = SortBy{Fields: []repo.SortByField[Field]{{Field: FieldCreatedAt, Ascending: false}}}
	}

	// Build query with sorting
	orderBy := sortBy.ToSQL(r.fieldMapping())
	// If no valid sort fields, use default
	if orderBy == "" {
		sortBy = SortBy{Fields: []repo.SortByField[Field]{{Field: FieldCreatedAt, Ascending: false}}}
		orderBy = sortBy.ToSQL(r.fieldMapping())
	}

	query := repo.Join(
		listTenantsQuery,
		orderBy,
		repo.FormatLimitOffset(limit, offset),
	)

	// Get tenants with user counts
	rows, err := tx.Query(ctx, query)
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

func (r *pgAnalyticsQueryRepository) SearchTenants(ctx context.Context, search string, limit, offset int, sortBy SortBy) ([]*entities.TenantInfo, int, error) {
	tx, err := composables.UseTx(ctx)
	if err != nil {
		return nil, 0, errors.Wrap(err, "failed to get transaction")
	}

	// Validate limit parameter
	if limit < 0 {
		return nil, 0, errors.New("limit cannot be negative")
	}

	// Add wildcards for ILIKE pattern matching
	searchPattern := "%" + search + "%"

	// Get total count with search filter
	var total int
	err = tx.QueryRow(ctx, countTenantsSearchSQL, searchPattern).Scan(&total)
	if err != nil {
		return nil, 0, errors.Wrap(err, "failed to count tenants with search")
	}

	// Handle limit=0 case - return empty slice
	if limit == 0 {
		return []*entities.TenantInfo{}, total, nil
	}

	// Default sort if empty
	if len(sortBy.Fields) == 0 {
		sortBy = SortBy{Fields: []repo.SortByField[Field]{{Field: FieldCreatedAt, Ascending: false}}}
	}

	// Build query with sorting
	orderBy := sortBy.ToSQL(r.fieldMapping())
	// If no valid sort fields, use default
	if orderBy == "" {
		sortBy = SortBy{Fields: []repo.SortByField[Field]{{Field: FieldCreatedAt, Ascending: false}}}
		orderBy = sortBy.ToSQL(r.fieldMapping())
	}

	query := repo.Join(
		searchTenantsQuery,
		orderBy,
		repo.FormatLimitOffset(limit, offset),
	)

	// Get tenants matching search
	rows, err := tx.Query(ctx, query, searchPattern)
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

func (r *pgAnalyticsQueryRepository) FilterTenantsByDateRange(ctx context.Context, startDate, endDate time.Time, limit, offset int, sortBy SortBy) ([]*entities.TenantInfo, int, error) {
	tx, err := composables.UseTx(ctx)
	if err != nil {
		return nil, 0, errors.Wrap(err, "failed to get transaction")
	}

	// Validate limit parameter
	if limit < 0 {
		return nil, 0, errors.New("limit cannot be negative")
	}

	// Get total count for date range
	var total int
	err = tx.QueryRow(ctx, countTenantsByDateRangeSQL, startDate, endDate).Scan(&total)
	if err != nil {
		return nil, 0, errors.Wrap(err, "failed to count tenants by date range")
	}

	// Handle limit=0 case - return empty slice
	if limit == 0 {
		return []*entities.TenantInfo{}, total, nil
	}

	// Default sort if empty
	if len(sortBy.Fields) == 0 {
		sortBy = SortBy{Fields: []repo.SortByField[Field]{{Field: FieldCreatedAt, Ascending: false}}}
	}

	// Build query with sorting
	orderBy := sortBy.ToSQL(r.fieldMapping())
	// If no valid sort fields, use default
	if orderBy == "" {
		sortBy = SortBy{Fields: []repo.SortByField[Field]{{Field: FieldCreatedAt, Ascending: false}}}
		orderBy = sortBy.ToSQL(r.fieldMapping())
	}

	query := repo.Join(
		filterTenantsByDateRangeQuery,
		orderBy,
		repo.FormatLimitOffset(limit, offset),
	)

	// Get tenants within date range
	rows, err := tx.Query(ctx, query, startDate, endDate)
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
		&tenant.Email,
		&tenant.Phone,
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
			&tenant.Email,
			&tenant.Phone,
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
