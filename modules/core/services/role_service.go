package services

import (
	"context"

	"github.com/iota-uz/iota-sdk/modules/core/permissions"
	"github.com/iota-uz/iota-sdk/pkg/composables"

	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/role"
	"github.com/iota-uz/iota-sdk/pkg/eventbus"
)

type RoleService struct {
	repo      role.Repository
	publisher eventbus.EventBus
}

func NewRoleService(repo role.Repository, publisher eventbus.EventBus) *RoleService {
	return &RoleService{
		repo:      repo,
		publisher: publisher,
	}
}

func (s *RoleService) Count(ctx context.Context, params *role.FindParams) (int64, error) {
	return s.repo.Count(ctx, params)
}

func (s *RoleService) GetAll(ctx context.Context) ([]role.Role, error) {
	return s.repo.GetAll(ctx)
}

func (s *RoleService) GetByID(ctx context.Context, id uint) (role.Role, error) {
	return s.repo.GetByID(ctx, id)
}

func (s *RoleService) GetPaginated(ctx context.Context, params *role.FindParams) ([]role.Role, error) {
	return s.repo.GetPaginated(ctx, params)
}

func (s *RoleService) Create(ctx context.Context, data role.Role) error {
	err := composables.CanUser(ctx, permissions.RoleCreate)
	if err != nil {
		return err
	}

	createdEvent, err := role.NewCreatedEvent(ctx, data)
	if err != nil {
		return err
	}

	var createdRole role.Role
	err = composables.InTx(ctx, func(txCtx context.Context) error {
		if created, err := s.repo.Create(txCtx, data); err != nil {
			return err
		} else {
			createdRole = created
		}
		return nil
	})
	if err != nil {
		return err
	}
	createdEvent.Result = createdRole

	s.publisher.Publish(createdEvent)

	return nil
}

func (s *RoleService) Update(ctx context.Context, data role.Role) error {
	err := composables.CanUser(ctx, permissions.RoleUpdate)
	if err != nil {
		return err
	}

	if !data.CanUpdate() {
		return composables.ErrForbidden
	}

	updatedEvent, err := role.NewUpdatedEvent(ctx, data)
	if err != nil {
		return err
	}

	var updatedRole role.Role
	err = composables.InTx(ctx, func(ctx context.Context) error {
		if roleAfterUpdate, err := s.repo.Update(ctx, data); err != nil {
			return err
		} else {
			updatedRole = roleAfterUpdate
		}
		return nil
	})
	if err != nil {
		return err
	}

	updatedEvent.Result = updatedRole

	s.publisher.Publish(updatedEvent)

	return nil
}

func (s *RoleService) Delete(ctx context.Context, id uint) error {
	err := composables.CanUser(ctx, permissions.RoleDelete)
	if err != nil {
		return err
	}

	entity, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return err
	}

	if !entity.CanDelete() {
		return composables.ErrForbidden
	}

	deletedEvent, err := role.NewDeletedEvent(ctx)
	if err != nil {
		return err
	}

	var deletedRole role.Role
	err = composables.InTx(ctx, func(txCtx context.Context) error {
		if err := s.repo.Delete(txCtx, id); err != nil {
			return err
		} else {
			deletedRole = entity
		}
		return nil
	})
	if err != nil {
		return err
	}
	deletedEvent.Result = deletedRole
	s.publisher.Publish(deletedEvent)

	return nil
}
