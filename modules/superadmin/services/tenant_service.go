package services

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/modules/superadmin/domain"
	"github.com/iota-uz/iota-sdk/modules/superadmin/domain/entities"
	"github.com/pkg/errors"
)

// TenantService provides business logic for tenant management operations
type TenantService struct {
	repo domain.AnalyticsQueryRepository
}

// NewTenantService creates a new tenant service
func NewTenantService(repo domain.AnalyticsQueryRepository) *TenantService {
	return &TenantService{
		repo: repo,
	}
}

// ListTenants returns a paginated list of tenants with user counts
// If limit is 0 or negative, defaults to 50
func (s *TenantService) ListTenants(ctx context.Context, limit, offset int) ([]*entities.TenantInfo, int, error) {
	// Validate and set default limit
	if limit <= 0 {
		limit = 50
	}
	if limit > 1000 {
		return nil, 0, errors.New("limit cannot exceed 1000")
	}

	// Validate offset
	if offset < 0 {
		offset = 0
	}

	tenants, total, err := s.repo.ListTenants(ctx, limit, offset, "", "")
	if err != nil {
		return nil, 0, errors.Wrap(err, "failed to list tenants")
	}

	return tenants, total, nil
}

// FilterByDateRange returns tenants created within the specified date range
// If startDate is zero, returns all tenants from the beginning
// If endDate is zero, defaults to now
func (s *TenantService) FilterByDateRange(ctx context.Context, startDate, endDate time.Time, limit, offset int, sortField, sortOrder string) ([]*entities.TenantInfo, int, error) {
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
		limit = 50
	}
	if limit > 1000 {
		return nil, 0, errors.New("limit cannot exceed 1000")
	}

	// Validate offset
	if offset < 0 {
		offset = 0
	}

	tenants, total, err := s.repo.FilterTenantsByDateRange(ctx, startDate, endDate, limit, offset, sortField, sortOrder)
	if err != nil {
		return nil, 0, errors.Wrap(err, "failed to filter tenants by date range")
	}

	return tenants, total, nil
}

// GetTenantDetails returns detailed information about a specific tenant
func (s *TenantService) GetTenantDetails(ctx context.Context, tenantID uuid.UUID) (*entities.TenantInfo, error) {
	if tenantID == uuid.Nil {
		return nil, errors.New("tenant ID cannot be nil")
	}

	tenant, err := s.repo.GetTenantDetails(ctx, tenantID)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get tenant details")
	}

	return tenant, nil
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
	tenants, total, err := s.repo.ListTenants(ctx, 1000, 0, "", "")
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
