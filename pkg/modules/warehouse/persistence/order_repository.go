package persistence

import (
	"context"
	"github.com/iota-agency/iota-sdk/pkg/composables"
	"github.com/iota-agency/iota-sdk/pkg/graphql/helpers"
	"github.com/iota-agency/iota-sdk/pkg/service"

	"github.com/iota-agency/iota-sdk/pkg/domain/aggregates/order"
	"github.com/iota-agency/iota-sdk/pkg/modules/warehouse/persistence/models"
)

type GormOrderRepository struct{}

func NewOrderRepository() order.Repository {
	return &GormOrderRepository{}
}

func (g *GormOrderRepository) GetPaginated(
	ctx context.Context, limit, offset int,
	sortBy []string,
) ([]*order.Order, error) {
	tx, ok := composables.UseTx(ctx)
	if !ok {
		return nil, service.ErrNoTx
	}
	q := tx.Limit(limit).Offset(offset)
	q, err := helpers.ApplySort(q, sortBy)
	if err != nil {
		return nil, err
	}
	var entities []*models.WarehouseOrder
	if err := q.Find(&entities).Error; err != nil {
		return nil, err
	}
	orders := make([]*order.Order, len(entities))
	for i, entity := range entities {
		// TODO: proper implementation
		o, err := g.GetByID(ctx, entity.ID)
		if err != nil {
			return nil, err
		}
		orders[i] = o
	}
	return orders, nil
}

func (g *GormOrderRepository) Count(ctx context.Context) (int64, error) {
	tx, ok := composables.UseTx(ctx)
	if !ok {
		return 0, service.ErrNoTx
	}
	var count int64
	if err := tx.Model(&models.WarehouseOrder{}).Count(&count).Error; err != nil { //nolint:exhaustruct
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

	orders := make([]*order.Order, len(entities))
	for i, entity := range entities {
		// TODO: proper implementation
		o, err := g.GetByID(ctx, entity.ID)
		if err != nil {
			return nil, err
		}
		orders[i] = o
	}
	return orders, nil
}

func (g *GormOrderRepository) GetByID(ctx context.Context, id uint) (*order.Order, error) {
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
	ids := make([]uint, len(orderItems))
	for i, item := range orderItems {
		ids[i] = item.ProductID
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
	if err := tx.Where("order_id = ?", or.ID).Delete(&models.OrderItem{}).Error; err != nil { //nolint:exhaustruct
		return err
	}
	for _, item := range orderItems {
		if err := tx.Create(item).Error; err != nil {
			return err
		}
	}
	return nil
}

func (g *GormOrderRepository) Delete(ctx context.Context, id uint) error {
	tx, ok := composables.UseTx(ctx)
	if !ok {
		return service.ErrNoTx
	}
	if err := tx.Where("id = ?", id).Delete(&models.WarehouseOrder{}).Error; err != nil { //nolint:exhaustruct
		return err
	}
	return nil
}
