package services

import (
	"context"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/user"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/pkg/errors"
)

// TenantUsersService provides business logic for querying users within a specific tenant
type TenantUsersService struct {
	userRepo user.Repository
}

// NewTenantUsersService creates a new tenant users service
func NewTenantUsersService(userRepo user.Repository) *TenantUsersService {
	return &TenantUsersService{
		userRepo: userRepo,
	}
}

// GetUsersByTenantID retrieves paginated users for a specific tenant
// This bypasses normal tenant scoping to allow superadmin cross-tenant access
func (s *TenantUsersService) GetUsersByTenantID(
	ctx context.Context,
	tenantID uuid.UUID,
	limit, offset int,
	search string,
	sortBy user.SortBy,
) ([]user.User, int, error) {
	// Validate inputs
	if limit <= 0 {
		limit = 20 // Default page size
	}
	if offset < 0 {
		offset = 0
	}

	// Create find params without tenant filter
	// The tenant filtering is done via context (see below)
	params := &user.FindParams{
		Limit:   limit,
		Offset:  offset,
		SortBy:  sortBy,
		Search:  search,
		Filters: []user.Filter{},
	}

	// CRITICAL: Create a new context WITH the target tenant ID
	// This allows superadmin to query users for a specific tenant
	// We use a clean context and add only the tenant we want to query
	ctxWithTenant := composables.WithTenantID(context.Background(), tenantID)

	// Copy transaction from original context if it exists
	// This ensures the query runs within the same transaction for consistency
	if tx, err := composables.UseTx(ctx); err == nil {
		if pgxTx, ok := tx.(pgx.Tx); ok {
			ctxWithTenant = composables.WithTx(ctxWithTenant, pgxTx)
		} else if pool, ok := tx.(*pgxpool.Pool); ok {
			ctxWithTenant = composables.WithPool(ctxWithTenant, pool)
		}
	}

	// Get users using repository
	users, err := s.userRepo.GetPaginated(ctxWithTenant, params)
	if err != nil {
		return nil, 0, errors.Wrap(err, "failed to get users by tenant ID")
	}

	// Get total count
	count, err := s.userRepo.Count(ctxWithTenant, params)
	if err != nil {
		return nil, 0, errors.Wrap(err, "failed to count users by tenant ID")
	}

	return users, int(count), nil
}
