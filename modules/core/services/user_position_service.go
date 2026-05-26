// Package services provides this package.
package services

import (
	"context"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/department"
	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/user"
	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/userposition"
	"github.com/iota-uz/iota-sdk/modules/core/permissions"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/eventbus"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
)

// UserPositionService provides operations for managing user positions.
type UserPositionService struct {
	repo      userposition.Repository
	deptRepo  department.Repository
	userRepo  user.Repository
	publisher eventbus.EventBus
}

// NewUserPositionService creates a new user position service instance.
// deptRepo and userRepo are used to validate that a position's department and
// user references stay within the caller's tenant.
func NewUserPositionService(
	repo userposition.Repository,
	deptRepo department.Repository,
	userRepo user.Repository,
	publisher eventbus.EventBus,
) *UserPositionService {
	return &UserPositionService{
		repo:      repo,
		deptRepo:  deptRepo,
		userRepo:  userRepo,
		publisher: publisher,
	}
}

// Count returns the total number of user positions.
func (s *UserPositionService) Count(ctx context.Context, params *userposition.FindParams) (int64, error) {
	if err := composables.CanUser(ctx, permissions.PositionRead); err != nil {
		return 0, err
	}
	return s.repo.Count(ctx, params)
}

// GetPaginated returns a paginated list of user positions.
func (s *UserPositionService) GetPaginated(
	ctx context.Context,
	params *userposition.FindParams,
) ([]userposition.UserPosition, error) {
	if err := composables.CanUser(ctx, permissions.PositionRead); err != nil {
		return nil, err
	}
	return s.repo.GetPaginated(ctx, params)
}

// GetByID returns a user position by its ID.
func (s *UserPositionService) GetByID(ctx context.Context, id uuid.UUID) (userposition.UserPosition, error) {
	if err := composables.CanUser(ctx, permissions.PositionRead); err != nil {
		return nil, err
	}
	return s.repo.GetByID(ctx, id)
}

// Create creates a new user position.
func (s *UserPositionService) Create(
	ctx context.Context,
	p userposition.UserPosition,
) (userposition.UserPosition, error) {
	const op serrors.Op = "UserPositionService.Create"
	if err := composables.CanUser(ctx, permissions.PositionCreate); err != nil {
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

	var saved userposition.UserPosition
	err = composables.InTx(ctx, func(txCtx context.Context) error {
		if err := ValidateUserPosition(txCtx, op, tenantID, p, s.deptRepo, s.userRepo); err != nil {
			return err
		}
		saved, err = s.repo.Save(txCtx, p)
		return err
	})
	if err != nil {
		return nil, err
	}

	s.publisher.Publish(userposition.NewCreatedEvent(saved, actor))
	return saved, nil
}

// Update updates an existing user position.
func (s *UserPositionService) Update(
	ctx context.Context,
	p userposition.UserPosition,
) (userposition.UserPosition, error) {
	const op serrors.Op = "UserPositionService.Update"
	if err := composables.CanUser(ctx, permissions.PositionUpdate); err != nil {
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

	var oldPosition userposition.UserPosition
	var updated userposition.UserPosition
	err = composables.InTx(ctx, func(txCtx context.Context) error {
		var err error
		oldPosition, err = s.repo.GetByID(txCtx, p.ID())
		if err != nil {
			return err
		}
		if err := ValidateUserPosition(txCtx, op, tenantID, p, s.deptRepo, s.userRepo); err != nil {
			return err
		}
		updated, err = s.repo.Save(txCtx, p)
		return err
	})
	if err != nil {
		return nil, err
	}

	s.publisher.Publish(userposition.NewUpdatedEvent(oldPosition, updated, actor))
	return updated, nil
}

// Delete removes a user position by its ID.
func (s *UserPositionService) Delete(ctx context.Context, id uuid.UUID) error {
	if err := composables.CanUser(ctx, permissions.PositionDelete); err != nil {
		return err
	}

	actor, err := composables.UseUser(ctx)
	if err != nil {
		return err
	}

	var p userposition.UserPosition
	err = composables.InTx(ctx, func(txCtx context.Context) error {
		var err error
		p, err = s.repo.GetByID(txCtx, id)
		if err != nil {
			return err
		}
		return s.repo.Delete(txCtx, id)
	})
	if err != nil {
		return err
	}

	s.publisher.Publish(userposition.NewDeletedEvent(p, actor))
	return nil
}
