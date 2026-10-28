// Package services provides this package.
package services

import (
	"context"
	"strconv"

	"github.com/iota-uz/iota-sdk/modules/logging/history"
	"github.com/iota-uz/iota-sdk/modules/warehouse/domain/entities/unit"
	"github.com/iota-uz/iota-sdk/modules/warehouse/permissions"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/eventbus"
)

type UnitService struct {
	repo          unit.Repository
	publisher     eventbus.EventBus
	historyWriter history.Writer
}

func NewUnitService(
	repo unit.Repository,
	publisher eventbus.EventBus,
	historyWriter history.Writer,
) *UnitService {
	return &UnitService{
		repo:          repo,
		publisher:     publisher,
		historyWriter: historyWriter,
	}
}

func (s *UnitService) GetByID(ctx context.Context, id uint) (*unit.Unit, error) {
	if err := composables.CanUser(ctx, permissions.UnitRead); err != nil {
		return nil, err
	}
	return s.repo.GetByID(ctx, id)
}

func (s *UnitService) GetByTitleOrShortTitle(ctx context.Context, name string) (*unit.Unit, error) {
	if err := composables.CanUser(ctx, permissions.UnitRead); err != nil {
		return nil, err
	}
	return s.repo.GetByTitleOrShortTitle(ctx, name)
}

func (s *UnitService) GetAll(ctx context.Context) ([]*unit.Unit, error) {
	if err := composables.CanUser(ctx, permissions.UnitRead); err != nil {
		return nil, err
	}
	return s.repo.GetAll(ctx)
}

func (s *UnitService) GetPaginated(
	ctx context.Context, params *unit.FindParams,
) ([]*unit.Unit, error) {
	if err := composables.CanUser(ctx, permissions.ProductRead); err != nil {
		return nil, err
	}
	return s.repo.GetPaginated(ctx, params)
}

func (s *UnitService) Create(ctx context.Context, data *unit.CreateDTO) (*unit.Unit, error) {
	if err := composables.CanUser(ctx, permissions.UnitCreate); err != nil {
		return nil, err
	}
	entity, err := data.ToEntity()
	if err != nil {
		return nil, err
	}
	err = composables.InTx(ctx, func(txCtx context.Context) error {
		if err := s.repo.Create(txCtx, entity); err != nil {
			return err
		}
		return history.RecordMutation(txCtx, s.historyWriter, history.Mutation{
			Type: UnitCreated, Version: 1, Resource: UnitResource, ID: strconv.FormatUint(uint64(entity.ID), 10),
			After: unitSnapshot(entity),
		})
	})
	if err != nil {
		return nil, err
	}
	createdEvent, err := unit.NewCreatedEvent(ctx, *data, *entity)
	if err != nil {
		return nil, err
	}
	s.publisher.Publish(createdEvent)
	return entity, nil
}

func (s *UnitService) Update(ctx context.Context, id uint, data *unit.UpdateDTO) error {
	if err := composables.CanUser(ctx, permissions.UnitUpdate); err != nil {
		return err
	}
	entity, err := data.ToEntity(id)
	if err != nil {
		return err
	}
	err = composables.InTx(ctx, func(txCtx context.Context) error {
		before, err := s.repo.GetByID(txCtx, id)
		if err != nil {
			return err
		}
		if err := s.repo.Update(txCtx, entity); err != nil {
			return err
		}
		return history.RecordMutation(txCtx, s.historyWriter, history.Mutation{
			Type: UnitUpdated, Version: 1, Resource: UnitResource, ID: strconv.FormatUint(uint64(id), 10),
			Before: unitSnapshot(before), After: unitSnapshot(entity),
		})
	})
	if err != nil {
		return err
	}
	updatedEvent, err := unit.NewUpdatedEvent(ctx, *data, *entity)
	if err != nil {
		return err
	}
	s.publisher.Publish(updatedEvent)
	return nil
}

func (s *UnitService) Delete(ctx context.Context, id uint) (*unit.Unit, error) {
	if err := composables.CanUser(ctx, permissions.UnitDelete); err != nil {
		return nil, err
	}
	var entity *unit.Unit
	err := composables.InTx(ctx, func(txCtx context.Context) error {
		var err error
		entity, err = s.repo.GetByID(txCtx, id)
		if err != nil {
			return err
		}
		if err := s.repo.Delete(txCtx, id); err != nil {
			return err
		}
		return history.RecordMutation(txCtx, s.historyWriter, history.Mutation{
			Type: UnitDeleted, Version: 1, Resource: UnitResource, ID: strconv.FormatUint(uint64(id), 10),
			Before: unitSnapshot(entity),
		})
	})
	if err != nil {
		return nil, err
	}
	deletedEvent, err := unit.NewDeletedEvent(ctx, *entity)
	if err != nil {
		return nil, err
	}
	s.publisher.Publish(deletedEvent)
	return entity, nil
}
func (s *UnitService) Count(ctx context.Context) (uint, error) {
	return s.repo.Count(ctx)
}
