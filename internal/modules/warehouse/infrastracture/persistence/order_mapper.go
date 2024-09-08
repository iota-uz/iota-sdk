package persistence

import (
	"errors"
	"github.com/iota-agency/iota-erp/internal/modules/warehouse/domain/aggregates/order"
)

func toDBOrder(data *order.Order) (*Order, []*OrderItem) {
	var dbItems []*OrderItem
	for _, item := range data.Items {
		dbItems = append(dbItems, &OrderItem{
			ProductID: item.Product.ID,
			OrderID:   data.ID,
		})
	}
	return &Order{
		ID:        data.ID,
		Status:    data.Status.String(),
		Type:      data.Type.String(),
		CreatedAt: data.CreatedAt,
	}, dbItems
}

func toDomainOrder(dbOrder *Order, dbItems []*OrderItem, dbProduct []*Product) (*order.Order, error) {
	var items []*order.Item
	for _, item := range dbItems {
		var orderProduct *Product
		for _, p := range dbProduct {
			if p.ID == item.ProductID {
				orderProduct = p
				break
			}
		}
		if orderProduct == nil {
			return nil, errors.New("product not found")
		}
		p, err := toDomainProduct(orderProduct)
		if err != nil {
			return nil, err
		}
		items = append(items, &order.Item{
			Product: p,
		})
	}
	status, err := order.NewStatus(dbOrder.Status)
	if err != nil {
		return nil, err
	}
	typeEnum, err := order.NewType(dbOrder.Type)
	if err != nil {
		return nil, err
	}
	return &order.Order{
		ID:        dbOrder.ID,
		Status:    status,
		Type:      typeEnum,
		CreatedAt: dbOrder.CreatedAt,
		Items:     items,
	}, nil
}
