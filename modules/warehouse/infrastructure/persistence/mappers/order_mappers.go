package mappers

import (
	"github.com/iota-uz/iota-sdk/modules/warehouse/domain/aggregates/order"
	"github.com/iota-uz/iota-sdk/modules/warehouse/infrastructure/persistence/models"
)

func ToDBOrder(entity order.Order) (*models.WarehouseOrder, []*models.WarehouseOrderItem, []*models.WarehouseProduct, error) {
	var dbOrderItems []*models.WarehouseOrderItem
	var dbProducts []*models.WarehouseProduct
	for _, item := range entity.Items() {
		for _, domainProduct := range item.Products() {
			dbOrderItems = append(dbOrderItems, &models.WarehouseOrderItem{
				WarehouseProductID: domainProduct.ID,
				WarehouseOrderID:   entity.ID(),
			})
			dbProduct, err := ToDBProduct(domainProduct)
			if err != nil {
				return nil, nil, nil, err
			}
			dbProducts = append(dbProducts, dbProduct)
		}
	}

	dbOrder := &models.WarehouseOrder{
		ID:        entity.ID(),
		Status:    string(entity.Status()),
		Type:      string(entity.Type()),
		CreatedAt: entity.CreatedAt(),
	}
	return dbOrder, dbOrderItems, dbProducts, nil
}

func ToDomainOrder(dbOrder *models.WarehouseOrder) (order.Order, error) {
	status, err := order.NewStatus(dbOrder.Status)
	if err != nil {
		return nil, err
	}
	orderType, err := order.NewType(dbOrder.Type)
	if err != nil {
		return nil, err
	}
	return order.NewWithID(dbOrder.ID, orderType, status, dbOrder.CreatedAt), nil
}
