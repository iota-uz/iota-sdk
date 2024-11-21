package mappers

import (
	"github.com/iota-agency/iota-sdk/modules/warehouse/domain/aggregates/position"
	"github.com/iota-agency/iota-sdk/modules/warehouse/domain/aggregates/product"
	"github.com/iota-agency/iota-sdk/modules/warehouse/domain/entities/unit"
	viewmodels2 "github.com/iota-agency/iota-sdk/modules/warehouse/viewmodels"
	"strconv"
	"time"
)

func ProductToViewModel(entity *product.Product) *viewmodels2.Product {
	var pos *viewmodels2.Position
	if entity.Position != nil {
		pos = PositionToViewModel(entity.Position)
	}
	return &viewmodels2.Product{
		ID:         strconv.FormatUint(uint64(entity.ID), 10),
		Status:     string(entity.Status),
		Rfid:       entity.Rfid,
		PositionID: strconv.FormatUint(uint64(entity.PositionID), 10),
		Position:   pos,
		CreatedAt:  entity.CreatedAt.Format(time.RFC3339),
		UpdatedAt:  entity.UpdatedAt.Format(time.RFC3339),
	}
}

func PositionToViewModel(entity *position.Position) *viewmodels2.Position {
	return &viewmodels2.Position{
		ID:        strconv.FormatUint(uint64(entity.ID), 10),
		Title:     entity.Title,
		Barcode:   entity.Barcode,
		UnitID:    strconv.FormatUint(uint64(entity.UnitID), 10),
		Unit:      *UnitToViewModel(&entity.Unit),
		CreatedAt: entity.CreatedAt.Format(time.RFC3339),
		UpdatedAt: entity.UpdatedAt.Format(time.RFC3339),
	}
}

func UnitToViewModel(entity *unit.Unit) *viewmodels2.Unit {
	return &viewmodels2.Unit{
		ID:         strconv.FormatUint(uint64(entity.ID), 10),
		Title:      entity.Title,
		ShortTitle: entity.ShortTitle,
		CreatedAt:  entity.CreatedAt.Format(time.RFC3339),
		UpdatedAt:  entity.UpdatedAt.Format(time.RFC3339),
	}
}
