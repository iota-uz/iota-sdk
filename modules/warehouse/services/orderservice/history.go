package orderservice

import (
	"github.com/iota-uz/iota-sdk/modules/logging/domain/aggregates/businessevent"
	"github.com/iota-uz/iota-sdk/modules/logging/history"
	"github.com/iota-uz/iota-sdk/modules/warehouse/domain/aggregates/order"
)

const (
	OrderResource = "warehouse.order"

	OrderCreated   = "warehouse.order.created"
	OrderUpdated   = "warehouse.order.updated"
	OrderDeleted   = "warehouse.order.deleted"
	OrderCompleted = "warehouse.order.completed"
)

// HistoryDefinitions are the business history event types of warehouse orders.
func HistoryDefinitions() []history.Definition {
	resource := history.Resource{Name: OrderResource, Fields: []history.Field{
		{Name: "type", LabelKey: "WarehouseOrders.Single.Type", Type: history.ValueString},
		{Name: "status", LabelKey: "WarehouseOrders.Single.Status", Type: history.ValueString},
	}}
	return append(history.Lifecycle("warehouse.order", "WarehouseOrders.History", resource), history.Definition{
		Type:           OrderCompleted,
		Version:        1,
		Category:       businessevent.CategoryBusiness,
		TitleKey:       "WarehouseOrders.History.Completed.Title",
		DescriptionKey: "WarehouseOrders.History.Completed.Summary",
		Resources:      []history.Resource{resource},
	})
}

func orderSnapshot(o order.Order) history.Snapshot {
	return history.Snapshot{
		"type":   string(o.Type()),
		"status": string(o.Status()),
	}
}
