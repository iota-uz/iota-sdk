package productservice

import (
	"strconv"

	"github.com/iota-uz/iota-sdk/modules/logging/history"
	"github.com/iota-uz/iota-sdk/modules/warehouse/domain/aggregates/product"
)

const (
	ProductResource = "warehouse.product"

	ProductCreated = "warehouse.product.created"
	ProductUpdated = "warehouse.product.updated"
	ProductDeleted = "warehouse.product.deleted"
)

// HistoryDefinitions are the business history event types of warehouse products.
func HistoryDefinitions() []history.Definition {
	return history.Lifecycle("warehouse.product", "Products.History", history.Resource{Name: ProductResource, Fields: []history.Field{
		{Name: "rfid", LabelKey: "Products.Single.Rfid", Type: history.ValueString},
		{Name: "status", LabelKey: "Products.Single.Status", Type: history.ValueString},
		{Name: "position_id", LabelKey: "Products.Single.Position", Type: history.ValueInteger},
	}})
}

func productSnapshot(p product.Product) history.Snapshot {
	s := history.Snapshot{
		"rfid":   p.Rfid(),
		"status": string(p.Status()),
	}
	if p.PositionID() != 0 {
		s["position_id"] = p.PositionID()
	}
	return s
}

func productID(p product.Product) string {
	return strconv.FormatUint(uint64(p.ID()), 10)
}
