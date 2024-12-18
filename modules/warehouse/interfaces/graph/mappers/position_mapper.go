package mappers

import (
	"github.com/iota-agency/iota-sdk/modules/warehouse/domain/aggregates/position"
	model "github.com/iota-agency/iota-sdk/modules/warehouse/interfaces/graph/gqlmodels"
)

func PositionToGraphModel(item *position.Position) *model.WarehousePosition {
	return &model.WarehousePosition{
		ID:        int64(item.ID),
		Title:     item.Title,
		Barcode:   item.Barcode,
		CreatedAt: item.CreatedAt,
		UpdatedAt: item.UpdatedAt,
	}
}
