// Package services provides this package.
package services

import (
	"context"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/department"
	"github.com/iota-uz/iota-sdk/modules/core/infrastructure/query"
	"github.com/iota-uz/iota-sdk/modules/core/permissions"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/eventbus"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
)

// DepartmentService provides operations for managing departments.
type DepartmentService struct {
	repo      department.Repository
	orgQuery  query.OrgQueryRepository
	publisher eventbus.EventBus
}

// NewDepartmentService creates a new department service instance. orgQuery
// supplies the recursive subtree walk used for hierarchy cycle detection on
// write.
func NewDepartmentService(
	repo department.Repository,
	orgQuery query.OrgQueryRepository,
	publisher eventbus.EventBus,
) *DepartmentService {
	return &DepartmentService{
		repo:      repo,
		orgQuery:  orgQuery,
		publisher: publisher,
	}
}

// Count returns the total number of departments.
func (s *DepartmentService) Count(ctx context.Context, params *department.FindParams) (int64, error) {
	if err := composables.CanUser(ctx, permissions.DepartmentRead); err != nil {
		return 0, err
	}
	return s.repo.Count(ctx, params)
}

// GetPaginated returns a paginated list of departments.
func (s *DepartmentService) GetPaginated(
	ctx context.Context,
	params *department.FindParams,
) ([]department.Department, error) {
	if err := composables.CanUser(ctx, permissions.DepartmentRead); err != nil {
		return nil, err
	}
	return s.repo.GetPaginated(ctx, params)
}

// GetByID returns a department by its ID.
func (s *DepartmentService) GetByID(ctx context.Context, id uuid.UUID) (department.Department, error) {
	if err := composables.CanUser(ctx, permissions.DepartmentRead); err != nil {
		return nil, err
	}
	return s.repo.GetByID(ctx, id)
}

// GetAll returns all departments.
func (s *DepartmentService) GetAll(ctx context.Context) ([]department.Department, error) {
	if err := composables.CanUser(ctx, permissions.DepartmentRead); err != nil {
		return nil, err
	}
	return s.repo.GetPaginated(ctx, &department.FindParams{
		Limit: 1000,
	})
}

// Create creates a new department.
func (s *DepartmentService) Create(ctx context.Context, d department.Department) (department.Department, error) {
	const op serrors.Op = "DepartmentService.Create"
	if err := composables.CanUser(ctx, permissions.DepartmentCreate); err != nil {
		return nil, err
	}

	tenantID, err := composables.UseTenantID(ctx)
	if err != nil {
		return nil, serrors.E(op, err)
	}

	actor, err := composables.UseUser(ctx)
	if err != nil {
		return nil, err
	}

	var saved department.Department
	err = composables.InTx(ctx, func(txCtx context.Context) error {
		if err := ValidateDepartment(txCtx, op, tenantID, d, s.repo, s.orgQuery.DepartmentSubtree); err != nil {
			return err
		}
		saved, err = s.repo.Save(txCtx, d)
		return err
	})
	if err != nil {
		return nil, err
	}

	s.publisher.Publish(department.NewCreatedEvent(saved, actor))
	return saved, nil
}

// Update updates an existing department.
func (s *DepartmentService) Update(ctx context.Context, d department.Department) (department.Department, error) {
	const op serrors.Op = "DepartmentService.Update"
	if err := composables.CanUser(ctx, permissions.DepartmentUpdate); err != nil {
		return nil, err
	}

	tenantID, err := composables.UseTenantID(ctx)
	if err != nil {
		return nil, serrors.E(op, err)
	}

	actor, err := composables.UseUser(ctx)
	if err != nil {
		return nil, err
	}

	var oldDepartment department.Department
	var updated department.Department
	err = composables.InTx(ctx, func(txCtx context.Context) error {
		var err error
		oldDepartment, err = s.repo.GetByID(txCtx, d.ID())
		if err != nil {
			return err
		}
		if err := ValidateDepartment(txCtx, op, tenantID, d, s.repo, s.orgQuery.DepartmentSubtree); err != nil {
			return err
		}
		updated, err = s.repo.Save(txCtx, d)
		return err
	})
	if err != nil {
		return nil, err
	}

	s.publisher.Publish(department.NewUpdatedEvent(oldDepartment, updated, actor))
	return updated, nil
}

// Delete removes a department by its ID.
func (s *DepartmentService) Delete(ctx context.Context, id uuid.UUID) error {
	if err := composables.CanUser(ctx, permissions.DepartmentDelete); err != nil {
		return err
	}

	actor, err := composables.UseUser(ctx)
	if err != nil {
		return err
	}

	var d department.Department
	err = composables.InTx(ctx, func(txCtx context.Context) error {
		var err error
		d, err = s.repo.GetByID(txCtx, id)
		if err != nil {
			return err
		}
		return s.repo.Delete(txCtx, id)
	})
	if err != nil {
		return err
	}

	s.publisher.Publish(department.NewDeletedEvent(d, actor))
	return nil
}
