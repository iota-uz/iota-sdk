package position

import (
	"github.com/iota-agency/iota-sdk/modules/warehouse/domain/entities/unit"
	"time"
)

type Position struct {
	ID        uint
	Title     string
	Barcode   string
	UnitID    uint
	Unit      unit.Unit
	CreatedAt time.Time
	UpdatedAt time.Time
}
