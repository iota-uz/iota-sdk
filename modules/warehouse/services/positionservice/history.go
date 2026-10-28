package positionservice

import (
	"strconv"

	"github.com/iota-uz/iota-sdk/modules/logging/history"
	"github.com/iota-uz/iota-sdk/modules/warehouse/domain/aggregates/position"
)

const (
	PositionResource = "warehouse.position"

	PositionCreated = "warehouse.position.created"
	PositionUpdated = "warehouse.position.updated"
	PositionDeleted = "warehouse.position.deleted"
)

// HistoryDefinitions are the business history event types of warehouse positions.
func HistoryDefinitions() []history.Definition {
	return history.Lifecycle("warehouse.position", "WarehousePositions.History", history.Resource{Name: PositionResource, Fields: []history.Field{
		{Name: "title", LabelKey: "WarehousePositions.Single.Title.Label", Type: history.ValueString},
		{Name: "barcode", LabelKey: "WarehousePositions.Single.Barcode.Label", Type: history.ValueString},
		{Name: "unit_id", LabelKey: "WarehousePositions.Single.Unit", Type: history.ValueInteger},
	}})
}

func positionSnapshot(p position.Position) history.Snapshot {
	s := history.Snapshot{
		"title":   p.Title(),
		"barcode": p.Barcode(),
	}
	if p.UnitID() != 0 {
		s["unit_id"] = p.UnitID()
	}
	return s
}

func positionID(id uint) string {
	return strconv.FormatUint(uint64(id), 10)
}
