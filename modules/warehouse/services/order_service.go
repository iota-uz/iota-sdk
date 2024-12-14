package services

import (
	"context"
	"github.com/iota-agency/iota-sdk/modules/warehouse/domain/aggregates/order"
	"github.com/iota-agency/iota-sdk/modules/warehouse/domain/aggregates/product"
	"github.com/iota-agency/iota-sdk/modules/warehouse/permissions"
	"github.com/iota-agency/iota-sdk/modules/warehouse/persistence"
	"github.com/iota-agency/iota-sdk/pkg/composables"
	"github.com/iota-agency/iota-sdk/pkg/event"
)

type OrderService struct {
	repo        order.Repository
	productRepo product.Repository
	publisher   event.Publisher
}

func NewOrderService(publisher event.Publisher) *OrderService {
	return &OrderService{
		repo:        persistence.NewOrderRepository(),
		productRepo: persistence.NewProductRepository(),
		publisher:   publisher,
	}
}

func (s *OrderService) GetByID(ctx context.Context, id uint) (*order.Order, error) {
	if err := composables.CanUser(ctx, permissions.OrderRead); err != nil {
		return nil, err
	}
	return s.repo.GetByID(ctx, id)
}

func (s *OrderService) GetAll(ctx context.Context) ([]*order.Order, error) {
	if err := composables.CanUser(ctx, permissions.OrderRead); err != nil {
		return nil, err
	}
	return s.repo.GetAll(ctx)
}

func (s *OrderService) GetPaginated(ctx context.Context, params *order.FindParams) ([]*order.Order, error) {
	if err := composables.CanUser(ctx, permissions.OrderRead); err != nil {
		return nil, err
	}
	return s.repo.GetPaginated(ctx, params)
}

func (s *OrderService) GetOldestProducts(ctx context.Context, positionID uint, quantity uint) ([]*product.Product, error) {
	queryOpts := &product.QueryOptions{
		SortBy: []string{"created_at asc"},
		Limit:  int(quantity),
	}
	return s.productRepo.GetByPositionID(ctx, positionID, queryOpts)
}

func (s *OrderService) Create(ctx context.Context, data order.CreateDTO) error {
	if err := composables.CanUser(ctx, permissions.OrderCreate); err != nil {
		return err
	}
	entity, err := data.ToEntity()
	if err != nil {
		return err
	}
	if err := s.repo.Create(ctx, entity); err != nil {
		return err
	}
	// TODO: Uncomment this code after creating the event
	//createdEvent, err := order.NewCreatedEvent(ctx, *data, *entity)
	//if err != nil {
	//	return err
	//}
	//s.publisher.Publish(createdEvent)
	return nil
}

func (s *OrderService) Update(ctx context.Context, id uint, data order.UpdateDTO) error {
	if err := composables.CanUser(ctx, permissions.OrderUpdate); err != nil {
		return err
	}
	entity, err := data.ToEntity(id)
	if err != nil {
		return err
	}
	if err := s.repo.Update(ctx, entity); err != nil {
		return err
	}
	// TODO: Uncomment this code after creating the event
	//updatedEvent, err := order.NewUpdatedEvent(ctx, *data, *entity)
	//if err != nil {
	//	return err
	//}
	//s.publisher.Publish(updatedEvent)
	return nil
}

func (s *OrderService) Delete(ctx context.Context, id uint) (*order.Order, error) {
	if err := composables.CanUser(ctx, permissions.OrderDelete); err != nil {
		return nil, err
	}
	entity, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if err := s.repo.Delete(ctx, id); err != nil {
		return nil, err
	}
	// TODO: Uncomment this code after creating the event
	//deletedEvent, err := order.NewDeletedEvent(ctx, *entity)
	//if err != nil {
	//	return err
	//}
	//s.publisher.Publish(deletedEvent)
	return entity, nil
}

func (s *OrderService) Count(ctx context.Context) (int64, error) {
	return s.repo.Count(ctx)
}
