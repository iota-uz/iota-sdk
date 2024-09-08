package persistence

import (
	"github.com/iota-agency/iota-erp/internal/modules/warehouse/domain/entities/unit"
)

func toDBUnit(unit *unit.Unit) *Unit {
	return &Unit{
		ID:        unit.ID,
		Name:      unit.Name,
		CreatedAt: unit.CreatedAt,
		UpdatedAt: unit.UpdatedAt,
	}
}

func toDomainUnit(dbUnit *Unit) *unit.Unit {
	return &unit.Unit{
		ID:        dbUnit.ID,
		Name:      dbUnit.Name,
		CreatedAt: dbUnit.CreatedAt,
		UpdatedAt: dbUnit.UpdatedAt,
	}
}
