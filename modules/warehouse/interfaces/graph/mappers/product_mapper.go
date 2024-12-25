package mappers

import (
	"github.com/iota-agency/iota-sdk/modules/warehouse/domain/aggregates/product"
	model "github.com/iota-agency/iota-sdk/modules/warehouse/interfaces/graph/gqlmodels"
)

func ProductToGraphModel(entity *product.Product) *model.Product {
	var pos *model.WarehousePosition
	if entity.Position != nil {
		pos = PositionToGraphModel(entity.Position)
	}
	return &model.Product{
		ID:         int64(entity.ID),
		Status:     string(entity.Status),
		Rfid:       entity.Rfid,
		Position:   pos,
		PositionID: int64(entity.PositionID),
		CreatedAt:  entity.CreatedAt,
		UpdatedAt:  entity.UpdatedAt,
	}
}
