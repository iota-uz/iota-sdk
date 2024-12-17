package mappers

import (
	"github.com/iota-agency/iota-sdk/modules/warehouse/domain/aggregates/order"
	model "github.com/iota-agency/iota-sdk/modules/warehouse/interfaces/graph/gqlmodels"
	"github.com/iota-agency/iota-sdk/pkg/mapping"
)

func OrderItemsToGraphModel(item order.Item) *model.OrderItem {
	return &model.OrderItem{
		Position: PositionToGraphModel(item.Position),
		Products: mapping.MapViewModels(item.Products, ProductToGraphModel),
	}
}

func OrderToGraphModel(o *order.Order) *model.Order {
	return &model.Order{
		ID:        int64(o.ID),
		Type:      string(o.Type),
		Status:    string(o.Status),
		Items:     mapping.MapViewModels(o.Items, OrderItemsToGraphModel),
		CreatedAt: o.CreatedAt,
	}
}