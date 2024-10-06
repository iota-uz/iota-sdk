package persistence

import (
	"context"
	"github.com/iota-agency/iota-erp/internal/infrastracture/persistence/models"

	"github.com/iota-agency/iota-erp/internal/domain/aggregates/order"
	"github.com/iota-agency/iota-erp/sdk/composables"
	"github.com/iota-agency/iota-erp/sdk/graphql/helpers"
	"github.com/iota-agency/iota-erp/sdk/service"
)

type GormOrderRepository struct{}

func NewGormOrderRepository() order.Repository {
	return &GormOrderRepository{}
}

func (g *GormOrderRepository) GetPaginated(ctx context.Context, limit, offset int, sortBy []string) ([]*order.Order, error) {
	tx, ok := composables.UseTx(ctx)
	if !ok {
		return nil, service.ErrNoTx
	}
	q := tx.Limit(limit).Offset(offset)
	q, err := helpers.ApplySort(q, sortBy, &order.Order{})
	if err != nil {
		return nil, err
	}
	var entities []*models.WarehouseOrder
	if err := q.Find(&entities).Error; err != nil {
		return nil, err
	}
	var orders []*order.Order
	for _, entity := range entities {
		// TODO: proper implementation
		o, err := g.GetByID(ctx, entity.ID)
		if err != nil {
			return nil, err
		}
		orders = append(orders, o)
	}
	return orders, nil
}

func (g *GormOrderRepository) Count(ctx context.Context) (int64, error) {
	tx, ok := composables.UseTx(ctx)
	if !ok {
		return 0, service.ErrNoTx
	}
	var count int64
	if err := tx.Model(&models.WarehouseOrder{}).Count(&count).Error; err != nil {
		return 0, err
	}
	return count, nil
}

func (g *GormOrderRepository) GetAll(ctx context.Context) ([]*order.Order, error) {
	tx, ok := composables.UseTx(ctx)
	if !ok {
		return nil, service.ErrNoTx
	}
	var entities []*models.WarehouseOrder
	if err := tx.Find(&entities).Error; err != nil {
		return nil, err
	}

	var orders []*order.Order
	for _, entity := range entities {
		// TODO: proper implementation
		o, err := g.GetByID(ctx, entity.ID)
		if err != nil {
			return nil, err
		}
		orders = append(orders, o)
	}
	return orders, nil
}

func (g *GormOrderRepository) GetByID(ctx context.Context, id int64) (*order.Order, error) {
	tx, ok := composables.UseTx(ctx)
	if !ok {
		return nil, service.ErrNoTx
	}
	var entity models.WarehouseOrder
	if err := tx.Where("id = ?", id).First(&entity).Error; err != nil {
		return nil, err
	}
	var orderItems []*models.OrderItem
	if err := tx.Where("order_id = ?", entity.ID).Find(&orderItems).Error; err != nil {
		return nil, err
	}
	var ids []int64
	for _, item := range orderItems {
		ids = append(ids, item.ProductID)
	}
	var products []*models.WarehouseProduct
	if err := tx.Where("id = ?", entity.ID).Find(&products, ids).Error; err != nil {
		return nil, err
	}
	o, err := toDomainOrder(&entity, orderItems, products)
	if err != nil {
		return nil, err
	}
	return o, nil
}

func (g *GormOrderRepository) Create(ctx context.Context, data *order.Order) error {
	tx, ok := composables.UseTx(ctx)
	if !ok {
		return service.ErrNoTx
	}
	or, orderItems := toDBOrder(data)
	if err := tx.Create(or).Error; err != nil {
		return err
	}
	for _, item := range orderItems {
		if err := tx.Create(item).Error; err != nil {
			return err
		}
	}
	return nil
}

func (g *GormOrderRepository) Update(ctx context.Context, data *order.Order) error {
	tx, ok := composables.UseTx(ctx)
	if !ok {
		return service.ErrNoTx
	}
	or, orderItems := toDBOrder(data)
	if err := tx.Save(or).Error; err != nil {
		return err
	}
	if err := tx.Where("order_id = ?", or.ID).Delete(&models.OrderItem{}).Error; err != nil {
		return err
	}
	for _, item := range orderItems {
		if err := tx.Create(item).Error; err != nil {
			return err
		}
	}
	return nil
}

func (g *GormOrderRepository) Delete(ctx context.Context, id int64) error {
	tx, ok := composables.UseTx(ctx)
	if !ok {
		return service.ErrNoTx
	}
	if err := tx.Where("id = ?", id).Delete(&models.WarehouseOrder{}).Error; err != nil {
		return err
	}
	return nil
}
