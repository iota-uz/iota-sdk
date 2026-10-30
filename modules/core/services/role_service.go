// Package services provides this package.
package services

import (
	"context"
	"strconv"

	"github.com/iota-uz/iota-sdk/modules/core/permissions"
	"github.com/iota-uz/iota-sdk/modules/logging/history"
	"github.com/iota-uz/iota-sdk/pkg/composables"

	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/role"
	"github.com/iota-uz/iota-sdk/pkg/eventbus"
)

type RoleService struct {
	repo          role.Repository
	publisher     eventbus.EventBus
	policy        *PrivilegeGrantPolicy
	historyWriter history.Writer
}

func NewRoleService(repo role.Repository, publisher eventbus.EventBus, policy *PrivilegeGrantPolicy, historyWriter history.Writer) *RoleService {
	return &RoleService{
		repo:          repo,
		publisher:     publisher,
		policy:        policy,
		historyWriter: historyWriter,
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

func (s *RoleService) Create(ctx context.Context, data role.Role) (role.Role, error) {
	err := composables.CanUser(ctx, permissions.RoleCreate)
	if err != nil {
		return nil, err
	}

	createdEvent, err := role.NewCreatedEvent(ctx, data)
	if err != nil {
		return nil, err
	}

	var createdRole role.Role
	err = composables.InTx(ctx, func(txCtx context.Context) error {
		var err error
		data, err = s.policy.AuthorizeRoleCreate(txCtx, data)
		if err != nil {
			return err
		}
		createdRole, err = s.repo.Create(txCtx, data)
		if err != nil {
			return err
		}
		return recordRoleChange(txCtx, s.historyWriter, nil, createdRole)
	})
	if err != nil {
		return nil, err
	}
	createdEvent.Result = createdRole

	s.publisher.Publish(createdEvent)

	return createdRole, nil
}

func (s *RoleService) Update(ctx context.Context, data role.Role) error {
	err := composables.CanUser(ctx, permissions.RoleUpdate)
	if err != nil {
		return err
	}

	updatedEvent, err := role.NewUpdatedEvent(ctx, data)
	if err != nil {
		return err
	}

	var updatedRole role.Role
	err = composables.InTx(ctx, func(ctx context.Context) error {
		var err error
		data, err = s.policy.AuthorizeRoleUpdate(ctx, data)
		if err != nil {
			return err
		}
		before, err := s.repo.GetByID(ctx, data.ID())
		if err != nil {
			return err
		}
		updatedRole, err = s.repo.Update(ctx, data)
		if err != nil {
			return err
		}
		return recordRoleChange(ctx, s.historyWriter, before, updatedRole)
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

	deletedEvent, err := role.NewDeletedEvent(ctx)
	if err != nil {
		return err
	}

	var deletedRole role.Role
	err = composables.InTx(ctx, func(txCtx context.Context) error {
		entity, err := s.policy.AuthorizeRoleDelete(txCtx, id)
		if err != nil {
			return err
		}
		if err := s.repo.Delete(txCtx, id); err != nil {
			return err
		}
		deletedRole = entity
		return history.RecordMutation(txCtx, s.historyWriter, history.Mutation{
			Type: RoleDeleted, Version: 1, Resource: RoleResource, ID: strconv.FormatUint(uint64(id), 10),
			Before: roleSnapshot(entity),
		})
	})
	if err != nil {
		return err
	}
	deletedEvent.Result = deletedRole
	s.publisher.Publish(deletedEvent)

	return nil
}
