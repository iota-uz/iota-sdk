package mappers

import (
	"github.com/iota-agency/iota-sdk/modules/warehouse/domain/entities/inventory"
	model "github.com/iota-agency/iota-sdk/modules/warehouse/interfaces/graph/gqlmodels"
)

func InventoryPositionToGraphModel(entity *inventory.Position) *model.InventoryPosition {
	return &model.InventoryPosition{
		ID:    int64(entity.ID),
		Title: entity.Title,
		Tags:  entity.RfidTags,
	}
}
