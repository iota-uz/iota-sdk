package orderservice

import (
	"context"

	"github.com/iota-uz/iota-sdk/modules/warehouse/domain/aggregates/order"
	"github.com/iota-uz/iota-sdk/modules/warehouse/domain/aggregates/product"
	"github.com/iota-uz/iota-sdk/modules/warehouse/permissions"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/eventbus"
)

type OrderService struct {
	repo        order.Repository
	productRepo product.Repository
	publisher   eventbus.EventBus
}

func NewOrderService(
	publisher eventbus.EventBus,
	orderRepo order.Repository,
	productRepo product.Repository,
) *OrderService {
	return &OrderService{
		repo:        orderRepo,
		productRepo: productRepo,
		publisher:   publisher,
	}
}

func (s *OrderService) GetByID(ctx context.Context, id uint) (order.Order, error) {
	if err := composables.CanUser(ctx, permissions.OrderRead); err != nil {
		return nil, err
	}
	return s.repo.GetByID(ctx, id)
}

func (s *OrderService) GetAll(ctx context.Context) ([]order.Order, error) {
	if err := composables.CanUser(ctx, permissions.OrderRead); err != nil {
		return nil, err
	}
	return s.repo.GetAll(ctx)
}

func (s *OrderService) GetPaginated(ctx context.Context, params *order.FindParams) ([]order.Order, error) {
	if err := composables.CanUser(ctx, permissions.OrderRead); err != nil {
		return nil, err
	}
	return s.repo.GetPaginated(ctx, params)
}

func (s *OrderService) FindByPositionID(ctx context.Context, queryOpts *product.FindByPositionParams) ([]product.Product, error) {
	return s.productRepo.FindByPositionID(ctx, queryOpts)
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
	return nil
}

func (s *OrderService) Complete(ctx context.Context, id uint) (order.Order, error) {
	//if err := composables.CanUser(ctx, permissions.OrderComplete); err != nil {
	//	return err
	//}
	entity, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	completedEntity, err := entity.Complete()
	if err != nil {
		return nil, err
	}
	if err := s.repo.Update(ctx, completedEntity); err != nil {
		return nil, err
	}
	return completedEntity, nil
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
	return nil
}

func (s *OrderService) Delete(ctx context.Context, id uint) (order.Order, error) {
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
	return entity, nil
}

func (s *OrderService) Count(ctx context.Context) (int64, error) {
	return s.repo.Count(ctx)
}
