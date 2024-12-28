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

	userpersistence "github.com/iota-agency/iota-sdk/modules/core/infrastructure/persistence"
)

type InventoryService struct {
	repo         inventory.Repository
	positionRepo position.Repository
	productRepo  product.Repository
	publisher    event.Publisher
}

func NewInventoryService(publisher event.Publisher) *InventoryService {
	unitRepo := persistence.NewUnitRepository()
	positionRepo := persistence.NewPositionRepository(unitRepo)
	userRepo := userpersistence.NewUserRepository()
	return &InventoryService{
		repo:         persistence.NewInventoryRepository(userRepo, positionRepo),
		productRepo:  persistence.NewProductRepository(positionRepo),
		positionRepo: positionRepo,
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

func (s *InventoryService) Positions(ctx context.Context) ([]*inventory.Position, error) {
	return s.repo.Positions(ctx)
}

func (s *InventoryService) Create(ctx context.Context, data *inventory.CreateCheckDTO) (*inventory.Check, error) {
	tx, err := composables.UsePoolTx(ctx)
	if err != nil {
		return nil, err
	}
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
	found := make(map[uint]uint)
	for _, pos := range data.Positions {
		found[pos.PositionID] = pos.Found
	}
	positions, err := s.repo.Positions(ctx)
	if err != nil {
		return nil, err
	}
	entity.Results = make([]*inventory.CheckResult, 0, len(positions))
	for _, pos := range positions {
		entity.AddResult(pos.ID, pos.Quantity, int(found[pos.ID]))
	}
	if err := s.repo.Create(ctx, entity); err != nil {
		return nil, err
	}
	createdEvent, err := inventory.NewCreatedEvent(ctx, *data, *entity)
	if err != nil {
		return nil, err
	}
	s.publisher.Publish(createdEvent)
	return entity, tx.Commit(ctx)
}

func (s *InventoryService) Update(ctx context.Context, id uint, data *inventory.UpdateCheckDTO) error {
	tx, err := composables.UsePoolTx(ctx)
	if err != nil {
		return err
	}
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
	return tx.Commit(ctx)
}

func (s *InventoryService) Delete(ctx context.Context, id uint) (*inventory.Check, error) {
	tx, err := composables.UsePoolTx(ctx)
	if err != nil {
		return nil, err
	}
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
	return entity, tx.Commit(ctx)
}
func (s *InventoryService) Count(ctx context.Context) (uint, error) {
	return s.repo.Count(ctx)
}
