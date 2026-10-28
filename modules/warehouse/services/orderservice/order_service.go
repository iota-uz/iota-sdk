// Package orderservice provides this package.
package orderservice

import (
	"context"
	"strconv"

	"github.com/iota-uz/iota-sdk/modules/logging/history"
	"github.com/iota-uz/iota-sdk/modules/warehouse/domain/aggregates/order"
	"github.com/iota-uz/iota-sdk/modules/warehouse/domain/aggregates/product"
	"github.com/iota-uz/iota-sdk/modules/warehouse/permissions"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/eventbus"
)

type OrderService struct {
	repo          order.Repository
	productRepo   product.Repository
	publisher     eventbus.EventBus
	historyWriter history.Writer
}

func NewOrderService(
	publisher eventbus.EventBus,
	orderRepo order.Repository,
	productRepo product.Repository,
	historyWriter history.Writer,
) *OrderService {
	return &OrderService{
		repo:          orderRepo,
		productRepo:   productRepo,
		publisher:     publisher,
		historyWriter: historyWriter,
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
	return composables.InTx(ctx, func(txCtx context.Context) error {
		created, err := s.repo.Create(txCtx, entity)
		if err != nil {
			return err
		}
		return s.record(txCtx, OrderCreated, created.ID(), nil, created)
	})
}

func (s *OrderService) Complete(ctx context.Context, id uint) (order.Order, error) {
	//if err := composables.CanUser(ctx, permissions.OrderComplete); err != nil {
	//	return err
	//}
	var completedEntity order.Order
	err := composables.InTx(ctx, func(txCtx context.Context) error {
		entity, err := s.repo.GetByID(txCtx, id)
		if err != nil {
			return err
		}
		completedEntity, err = entity.Complete()
		if err != nil {
			return err
		}
		if err := s.repo.Update(txCtx, completedEntity); err != nil {
			return err
		}
		return s.record(txCtx, OrderCompleted, id, entity, completedEntity)
	})
	if err != nil {
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
	return composables.InTx(ctx, func(txCtx context.Context) error {
		before, err := s.repo.GetByID(txCtx, id)
		if err != nil {
			return err
		}
		if err := s.repo.Update(txCtx, entity); err != nil {
			return err
		}
		return s.record(txCtx, OrderUpdated, id, before, entity)
	})
}

func (s *OrderService) Delete(ctx context.Context, id uint) (order.Order, error) {
	if err := composables.CanUser(ctx, permissions.OrderDelete); err != nil {
		return nil, err
	}
	var entity order.Order
	err := composables.InTx(ctx, func(txCtx context.Context) error {
		var err error
		entity, err = s.repo.GetByID(txCtx, id)
		if err != nil {
			return err
		}
		if err := s.repo.Delete(txCtx, id); err != nil {
			return err
		}
		return s.record(txCtx, OrderDeleted, id, entity, nil)
	})
	if err != nil {
		return nil, err
	}
	return entity, nil
}

func (s *OrderService) record(ctx context.Context, eventType string, id uint, before, after order.Order) error {
	m := history.Mutation{Type: eventType, Version: 1, Resource: OrderResource, ID: strconv.FormatUint(uint64(id), 10)}
	if before != nil {
		m.Before = orderSnapshot(before)
	}
	if after != nil {
		m.After = orderSnapshot(after)
	}
	return history.RecordMutation(ctx, s.historyWriter, m)
}

func (s *OrderService) Count(ctx context.Context) (int64, error) {
	return s.repo.Count(ctx)
}
