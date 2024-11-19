package mappers

import (
	"github.com/iota-agency/iota-sdk/internal/modules/warehouse/domain/aggregates/product"
	"github.com/iota-agency/iota-sdk/internal/modules/warehouse/domain/entities/position"
	"github.com/iota-agency/iota-sdk/internal/modules/warehouse/domain/entities/unit"
	"github.com/iota-agency/iota-sdk/internal/modules/warehouse/viewmodels"
	"strconv"
	"time"
)

func ProductToViewModel(entity *product.Product) *viewmodels.Product {
	var pos *viewmodels.Position
	if entity.Position != nil {
		pos = PositionToViewModel(entity.Position)
	}
	return &viewmodels.Product{
		ID:         strconv.FormatUint(uint64(entity.ID), 10),
		Status:     string(entity.Status),
		Rfid:       entity.Rfid,
		PositionID: strconv.FormatUint(uint64(entity.PositionID), 10),
		Position:   pos,
		CreatedAt:  entity.CreatedAt.Format(time.RFC3339),
		UpdatedAt:  entity.UpdatedAt.Format(time.RFC3339),
	}
}

func PositionToViewModel(entity *position.Position) *viewmodels.Position {
	return &viewmodels.Position{
		ID:        strconv.FormatUint(uint64(entity.ID), 10),
		Title:     entity.Title,
		Barcode:   entity.Barcode,
		UnitID:    strconv.FormatUint(uint64(entity.UnitID), 10),
		CreatedAt: entity.CreatedAt.Format(time.RFC3339),
		UpdatedAt: entity.UpdatedAt.Format(time.RFC3339),
	}
}

func UnitToViewModel(entity *unit.Unit) *viewmodels.Unit {
	return &viewmodels.Unit{
		ID:         strconv.FormatUint(uint64(entity.ID), 10),
		Title:      entity.Title,
		ShortTitle: entity.ShortTitle,
		CreatedAt:  entity.CreatedAt.Format(time.RFC3339),
		UpdatedAt:  entity.UpdatedAt.Format(time.RFC3339),
	}
}
