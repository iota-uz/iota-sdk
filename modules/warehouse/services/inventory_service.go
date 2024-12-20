package services

import (
	"context"

	"github.com/iota-agency/iota-sdk/modules/warehouse/domain/aggregates/position"
	"github.com/iota-agency/iota-sdk/modules/warehouse/domain/aggregates/product"
	"github.com/iota-agency/iota-sdk/modules/warehouse/domain/entities/inventory"
	"github.com/iota-agency/iota-sdk/modules/warehouse/permissions"
	"github.com/iota-agency/iota-sdk/modules/warehouse/persistence"
	"github.com/iota-agency/iota-sdk/pkg/composables"
	"github.com/iota-agency/iota-sdk/pkg/event"
)

type InventoryService struct {
	repo         inventory.Repository
	positionRepo position.Repository
	productRepo  product.Repository
	publisher    event.Publisher
}

func NewInventoryService(publisher event.Publisher) *InventoryService {
	return &InventoryService{
		repo:         persistence.NewInventoryRepository(),
		positionRepo: persistence.NewPositionRepository(),
		productRepo:  persistence.NewProductRepository(),
		publisher:    publisher,
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

func (s *InventoryService) Create(ctx context.Context, data *inventory.CreateCheckDTO) (*inventory.Check, error) {
	if err := composables.CanUser(ctx, permissions.InventoryCreate); err != nil {
		return nil, err
	}
	user, err := composables.UseUser(ctx)
	if err != nil {
		return nil, err
	}
	entity, err := data.ToEntity(user.ID)
	if err != nil {
		return nil, err
	}
	if entity.Type.Get() == inventory.Full {
		entity.Results = make([]*inventory.CheckResult, 0)
		positionIds, err := s.positionRepo.GetAllPositionIds(ctx)
		if err != nil {
			return nil, err
		}
		for _, id := range positionIds {
			entity.Results = append(entity.Results, &inventory.CheckResult{
				PositionID: id,
			})
		}
	}
	for _, result := range entity.Results {
		quantity, err := s.productRepo.CountWithFilters(ctx, &product.CountParams{
			PositionID: result.PositionID,
			Status:     product.InStock,
		})
		if err != nil {
			return nil, err
		}
		result.ExpectedQuantity = int(quantity)
	}
	if err := s.repo.Create(ctx, entity); err != nil {
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
	if err := s.repo.Update(ctx, entity); err != nil {
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
	entity, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if err := s.repo.Delete(ctx, id); err != nil {
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
