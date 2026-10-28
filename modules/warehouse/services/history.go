package services

import (
	"github.com/iota-uz/iota-sdk/modules/logging/history"
	"github.com/iota-uz/iota-sdk/modules/warehouse/domain/entities/inventory"
	"github.com/iota-uz/iota-sdk/modules/warehouse/domain/entities/unit"
)

const (
	UnitResource           = "warehouse.unit"
	InventoryCheckResource = "warehouse.inventory_check"

	UnitCreated           = "warehouse.unit.created"
	UnitUpdated           = "warehouse.unit.updated"
	UnitDeleted           = "warehouse.unit.deleted"
	InventoryCheckCreated = "warehouse.inventory_check.created"
	InventoryCheckUpdated = "warehouse.inventory_check.updated"
	InventoryCheckDeleted = "warehouse.inventory_check.deleted"
)

// HistoryDefinitions are the business history event types of warehouse units and inventory checks.
func HistoryDefinitions() []history.Definition {
	defs := history.Lifecycle("warehouse.unit", "WarehouseUnits.History", history.Resource{Name: UnitResource, Fields: []history.Field{
		{Name: "title", LabelKey: "WarehouseUnits.Single.Title.Label", Type: history.ValueString},
		{Name: "short_title", LabelKey: "WarehouseUnits.Single.ShortTitle.Label", Type: history.ValueString},
	}})
	return append(defs, history.Lifecycle("warehouse.inventory_check", "WarehouseInventory.History", history.Resource{Name: InventoryCheckResource, Fields: []history.Field{
		{Name: "name", LabelKey: "WarehouseInventory.Single.Name.Label", Type: history.ValueString},
		{Name: "status", LabelKey: "WarehouseInventory.List.Status", Type: history.ValueString},
	}})...)
}

func unitSnapshot(u *unit.Unit) history.Snapshot {
	return history.Snapshot{
		"title":       u.Title,
		"short_title": u.ShortTitle,
	}
}

func inventoryCheckSnapshot(c *inventory.Check) history.Snapshot {
	return history.Snapshot{
		"name":   c.Name,
		"status": string(c.Status),
	}
}
