// Package services provides this package.
package services

import (
	"context"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/modules/core/infrastructure/query"
	"github.com/iota-uz/iota-sdk/modules/core/permissions"
	"github.com/iota-uz/iota-sdk/pkg/composables"
)

// OrgQuery is the consumer-facing read API for organizational membership and
// hierarchy lookups. It is the foundation that permission-scoping code (e.g.
// EDO granular permissions) builds on. All lookups are tenant-scoped.
type OrgQuery interface {
	// UserDepartments returns the departments the user holds a position in.
	UserDepartments(ctx context.Context, userID uint) ([]uuid.UUID, error)
	// UserManagedDepartments returns the departments where the user holds a
	// manager position. When includeSubtree is true the result also contains
	// every descendant department.
	UserManagedDepartments(ctx context.Context, userID uint, includeSubtree bool) ([]uuid.UUID, error)
	// DepartmentSubtree returns the department and all of its descendants.
	DepartmentSubtree(ctx context.Context, deptID uuid.UUID) ([]uuid.UUID, error)
}

// OrgQueryService implements OrgQuery on top of the org query repository.
type OrgQueryService struct {
	repo query.OrgQueryRepository
}

// NewOrgQueryService creates a new org query service instance.
func NewOrgQueryService(repo query.OrgQueryRepository) *OrgQueryService {
	return &OrgQueryService{repo: repo}
}

var _ OrgQuery = (*OrgQueryService)(nil)

func (s *OrgQueryService) UserDepartments(ctx context.Context, userID uint) ([]uuid.UUID, error) {
	if err := composables.CanUser(ctx, permissions.DepartmentRead); err != nil {
		return nil, err
	}
	return s.repo.UserDepartments(ctx, userID)
}

func (s *OrgQueryService) UserManagedDepartments(
	ctx context.Context,
	userID uint,
	includeSubtree bool,
) ([]uuid.UUID, error) {
	if err := composables.CanUser(ctx, permissions.DepartmentRead); err != nil {
		return nil, err
	}
	return s.repo.UserManagedDepartments(ctx, userID, includeSubtree)
}

func (s *OrgQueryService) DepartmentSubtree(ctx context.Context, deptID uuid.UUID) ([]uuid.UUID, error) {
	if err := composables.CanUser(ctx, permissions.DepartmentRead); err != nil {
		return nil, err
	}
	return s.repo.DepartmentSubtree(ctx, deptID)
}
