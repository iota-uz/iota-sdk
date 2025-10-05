package services

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/modules/superadmin/domain"
	"github.com/iota-uz/iota-sdk/modules/superadmin/domain/entities"
	"github.com/iota-uz/iota-sdk/pkg/repo"
	"github.com/pkg/errors"
)

// TenantService provides business logic for tenant management and querying operations
type TenantService struct {
	repo domain.AnalyticsQueryRepository
}

// NewTenantService creates a new tenant service
func NewTenantService(repo domain.AnalyticsQueryRepository) *TenantService {
	return &TenantService{
		repo: repo,
	}
}

// FindTenants returns paginated list of tenants with user counts
// Supports optional search filtering by name or domain
func (s *TenantService) FindTenants(ctx context.Context, limit, offset int, search string, sortBy domain.TenantSortBy) ([]*entities.TenantInfo, int, error) {
	if limit <= 0 {
		limit = DefaultPageSize
	}
	if offset < 0 {
		offset = 0
	}

	// Use search if provided
	if search != "" {
		tenants, total, err := s.repo.SearchTenants(ctx, search, limit, offset, sortBy)
		if err != nil {
			return nil, 0, errors.Wrap(err, "failed to search tenants")
		}
		return tenants, total, nil
	}

	// Fall back to listing all tenants
	tenants, total, err := s.repo.ListTenants(ctx, limit, offset, sortBy)
	if err != nil {
		return nil, 0, errors.Wrap(err, "failed to find tenants")
	}

	return tenants, total, nil
}

// ListTenants returns a paginated list of tenants with user counts
// If limit is 0 or negative, defaults to DefaultPageSize
func (s *TenantService) ListTenants(ctx context.Context, limit, offset int) ([]*entities.TenantInfo, int, error) {
	// Validate and set default limit
	if limit <= 0 {
		limit = DefaultPageSize
	}
	if limit > MaxPageSize {
		return nil, 0, errors.New("limit cannot exceed 1000")
	}

	// Validate offset
	if offset < 0 {
		offset = 0
	}

	// Default DESC sort
	sortBy := domain.TenantSortBy{Fields: []repo.SortByField[string]{{Field: "created_at", Ascending: false}}}
	tenants, total, err := s.repo.ListTenants(ctx, limit, offset, sortBy)
	if err != nil {
		return nil, 0, errors.Wrap(err, "failed to list tenants")
	}

	return tenants, total, nil
}

// FilterByDateRange returns tenants created within the specified date range
// If startDate is zero, returns all tenants from the beginning
// If endDate is zero, defaults to now
func (s *TenantService) FilterByDateRange(ctx context.Context, startDate, endDate time.Time, limit, offset int, sortBy domain.TenantSortBy) ([]*entities.TenantInfo, int, error) {
	// Set default date range if not provided
	if startDate.IsZero() {
		startDate = time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)
	}
	if endDate.IsZero() {
		endDate = time.Now()
	}

	// Validate date range
	if startDate.After(endDate) {
		return nil, 0, errors.New("start date cannot be after end date")
	}

	// Validate and set default limit
	if limit <= 0 {
		limit = DefaultPageSize
	}
	if limit > MaxPageSize {
		return nil, 0, errors.New("limit cannot exceed 1000")
	}

	// Validate offset
	if offset < 0 {
		offset = 0
	}

	tenants, total, err := s.repo.FilterTenantsByDateRange(ctx, startDate, endDate, limit, offset, sortBy)
	if err != nil {
		return nil, 0, errors.Wrap(err, "failed to filter tenants by date range")
	}

	return tenants, total, nil
}

// GetByID retrieves a single tenant by ID with user count
func (s *TenantService) GetByID(ctx context.Context, id uuid.UUID) (*entities.TenantInfo, error) {
	if id == uuid.Nil {
		return nil, errors.New("tenant ID cannot be nil")
	}

	tenant, err := s.repo.GetTenantDetails(ctx, id)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get tenant by ID")
	}

	return tenant, nil
}

// GetTenantDetails returns detailed information about a specific tenant
// Deprecated: Use GetByID instead
func (s *TenantService) GetTenantDetails(ctx context.Context, tenantID uuid.UUID) (*entities.TenantInfo, error) {
	return s.GetByID(ctx, tenantID)
}

// GetAll retrieves all tenants (useful for exports)
func (s *TenantService) GetAll(ctx context.Context) ([]*entities.TenantInfo, error) {
	// Use a large limit to get all tenants, default DESC sort
	sortBy := domain.TenantSortBy{Fields: []repo.SortByField[string]{{Field: "created_at", Ascending: false}}}
	tenants, _, err := s.repo.ListTenants(ctx, DefaultLargeLimit, 0, sortBy)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get all tenants")
	}

	return tenants, nil
}

// ExcelExportData represents data ready for Excel export
type ExcelExportData struct {
	Headers []string
	Rows    [][]interface{}
}

// PrepareExcelExport prepares tenant data for Excel export
func (s *TenantService) PrepareExcelExport(ctx context.Context, tenants []*entities.TenantInfo) (*ExcelExportData, error) {
	if len(tenants) == 0 {
		return &ExcelExportData{
			Headers: []string{"ID", "Name", "Domain", "User Count", "Created At", "Updated At"},
			Rows:    [][]interface{}{},
		}, nil
	}

	data := &ExcelExportData{
		Headers: []string{"ID", "Name", "Domain", "User Count", "Created At", "Updated At"},
		Rows:    make([][]interface{}, 0, len(tenants)),
	}

	for _, tenant := range tenants {
		row := []interface{}{
			tenant.ID.String(),
			tenant.Name,
			tenant.Domain,
			tenant.UserCount,
			tenant.CreatedAt.Format(time.RFC3339),
			tenant.UpdatedAt.Format(time.RFC3339),
		}
		data.Rows = append(data.Rows, row)
	}

	return data, nil
}

// GetTenantsSummary returns a summary of tenant statistics
func (s *TenantService) GetTenantsSummary(ctx context.Context) (string, error) {
	// Default DESC sort
	sortBy := domain.TenantSortBy{Fields: []repo.SortByField[string]{{Field: "created_at", Ascending: false}}}
	tenants, total, err := s.repo.ListTenants(ctx, MaxPageSize, 0, sortBy)
	if err != nil {
		return "", errors.Wrap(err, "failed to get tenants for summary")
	}

	totalUsers := 0
	for _, tenant := range tenants {
		totalUsers += tenant.UserCount
	}

	avgUsersPerTenant := float64(0)
	if total > 0 {
		avgUsersPerTenant = float64(totalUsers) / float64(total)
	}

	return fmt.Sprintf("Total Tenants: %d, Total Users: %d, Average Users per Tenant: %.2f",
		total, totalUsers, avgUsersPerTenant), nil
}
