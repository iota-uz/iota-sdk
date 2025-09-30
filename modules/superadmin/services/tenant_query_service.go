package services

import (
	"context"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/modules/superadmin/domain"
	"github.com/iota-uz/iota-sdk/modules/superadmin/domain/entities"
	"github.com/iota-uz/iota-sdk/pkg/repo"
	"github.com/pkg/errors"
)

// TenantQueryService provides business logic for tenant querying operations
type TenantQueryService struct {
	repo domain.AnalyticsQueryRepository
}

// NewTenantQueryService creates a new tenant query service
func NewTenantQueryService(repo domain.AnalyticsQueryRepository) *TenantQueryService {
	return &TenantQueryService{
		repo: repo,
	}
}

// FindTenants returns paginated list of tenants with user counts
// Supports optional search filtering by name or domain
func (s *TenantQueryService) FindTenants(ctx context.Context, limit, offset int, search string, sortBy domain.TenantSortBy) ([]*entities.TenantInfo, int, error) {
	if limit <= 0 {
		limit = 20 // Default page size
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

// GetByID retrieves a single tenant by ID with user count
func (s *TenantQueryService) GetByID(ctx context.Context, id uuid.UUID) (*entities.TenantInfo, error) {
	tenant, err := s.repo.GetTenantDetails(ctx, id)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get tenant by ID")
	}

	return tenant, nil
}

// GetAll retrieves all tenants (useful for exports)
func (s *TenantQueryService) GetAll(ctx context.Context) ([]*entities.TenantInfo, error) {
	// Use a large limit to get all tenants, default DESC sort
	sortBy := domain.TenantSortBy{Fields: []repo.SortByField[string]{{Field: "created_at", Ascending: false}}}
	tenants, _, err := s.repo.ListTenants(ctx, 10000, 0, sortBy)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get all tenants")
	}

	return tenants, nil
}
