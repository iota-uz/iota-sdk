// Package services provides this package.
package services

import (
	"context"
	"strconv"

	"github.com/iota-uz/iota-sdk/modules/logging/history"
	"github.com/iota-uz/iota-sdk/modules/warehouse/domain/aggregates/position"
	"github.com/iota-uz/iota-sdk/modules/warehouse/domain/aggregates/product"
	"github.com/iota-uz/iota-sdk/modules/warehouse/domain/entities/inventory"
	"github.com/iota-uz/iota-sdk/modules/warehouse/permissions"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/eventbus"
)

type InventoryService struct {
	repo          inventory.Repository
	positionRepo  position.Repository
	productRepo   product.Repository
	publisher     eventbus.EventBus
	historyWriter history.Writer
}

func NewInventoryService(
	repo inventory.Repository,
	positionRepo position.Repository,
	productRepo product.Repository,
	publisher eventbus.EventBus,
	historyWriter history.Writer,
) *InventoryService {
	return &InventoryService{
		repo:          repo,
		productRepo:   productRepo,
		positionRepo:  positionRepo,
		publisher:     publisher,
		historyWriter: historyWriter,
	}
}

func (s *InventoryService) GetByID(ctx context.Context, id uint) (*inventory.Check, error) {
	if err := composables.CanUser(ctx, permissions.InventoryRead); err != nil {
		return nil, err
	}
	return s.repo.GetByID(ctx, id)
}

func (s *InventoryService) GetByIDWithDifference(ctx context.Context, id uint) (*inventory.Check, error) {
	if err := composables.CanUser(ctx, permissions.InventoryRead); err != nil {
		return nil, err
	}
	return s.repo.GetByIDWithDifference(ctx, id)
}

func (s *InventoryService) GetAll(ctx context.Context) ([]*inventory.Check, error) {
	if err := composables.CanUser(ctx, permissions.InventoryRead); err != nil {
		return nil, err
	}
	return s.repo.GetAll(ctx)
}

func (s *InventoryService) GetPaginated(
	ctx context.Context, params *inventory.FindParams,
) ([]*inventory.Check, error) {
	if err := composables.CanUser(ctx, permissions.InventoryRead); err != nil {
		return nil, err
	}
	return s.repo.GetPaginated(ctx, params)
}

func (s *InventoryService) Positions(ctx context.Context) ([]*inventory.Position, error) {
	return s.repo.Positions(ctx)
}

func (s *InventoryService) Create(ctx context.Context, data *inventory.CreateCheckDTO) (*inventory.Check, error) {
	if err := composables.CanUser(ctx, permissions.InventoryCreate); err != nil {
		return nil, err
	}
	user, err := composables.UseUser(ctx)
	if err != nil {
		return nil, err
	}
	entity, err := data.ToEntity(user)
	if err != nil {
		return nil, err
	}
	found := make(map[uint]uint)
	for _, pos := range data.Positions {
		found[pos.PositionID] = pos.Found
	}
	err = composables.InTx(ctx, func(txCtx context.Context) error {
		positions, err := s.repo.Positions(txCtx)
		if err != nil {
			return err
		}
		entity.Results = make([]*inventory.CheckResult, 0, len(positions))
		for _, pos := range positions {
			entity.AddResult(pos.ID, pos.Quantity, int(found[pos.ID]))
		}
		if err := s.repo.Create(txCtx, entity); err != nil {
			return err
		}
		return history.RecordMutation(txCtx, s.historyWriter, history.Mutation{
			Type: InventoryCheckCreated, Version: 1, Resource: InventoryCheckResource, ID: strconv.FormatUint(uint64(entity.ID), 10),
			After: inventoryCheckSnapshot(entity),
		})
	})
	if err != nil {
		return nil, err
	}
	createdEvent, err := inventory.NewCreatedEvent(ctx, *data, *entity)
	if err != nil {
		return nil, err
	}
	s.publisher.Publish(createdEvent)
	return entity, nil
}

func (s *InventoryService) Update(ctx context.Context, id uint, data *inventory.UpdateCheckDTO) error {
	if err := composables.CanUser(ctx, permissions.InventoryUpdate); err != nil {
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
		after := *before
		if entity.Name != "" {
			after.Name = entity.Name
		}
		return history.RecordMutation(txCtx, s.historyWriter, history.Mutation{
			Type: InventoryCheckUpdated, Version: 1, Resource: InventoryCheckResource, ID: strconv.FormatUint(uint64(id), 10),
			Before: inventoryCheckSnapshot(before), After: inventoryCheckSnapshot(&after),
		})
	})
	if err != nil {
		return err
	}
	updatedEvent, err := inventory.NewUpdatedEvent(ctx, *data, *entity)
	if err != nil {
		return err
	}
	s.publisher.Publish(updatedEvent)
	return nil
}

func (s *InventoryService) Delete(ctx context.Context, id uint) (*inventory.Check, error) {
	if err := composables.CanUser(ctx, permissions.InventoryDelete); err != nil {
		return nil, err
	}
	var entity *inventory.Check
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
			Type: InventoryCheckDeleted, Version: 1, Resource: InventoryCheckResource, ID: strconv.FormatUint(uint64(id), 10),
			Before: inventoryCheckSnapshot(entity),
		})
	})
	if err != nil {
		return nil, err
	}
	deletedEvent, err := inventory.NewDeletedEvent(ctx, *entity)
	if err != nil {
		return nil, err
	}
	s.publisher.Publish(deletedEvent)
	return entity, nil
}
func (s *InventoryService) Count(ctx context.Context) (uint, error) {
	return s.repo.Count(ctx)
}
