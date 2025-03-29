package position

import (
	"time"

	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/upload"
	"github.com/iota-uz/iota-sdk/modules/warehouse/domain/entities/unit"
)

type Position struct {
	ID        uint
	TenantID  uint
	Title     string
	Barcode   string
	UnitID    uint
	Unit      *unit.Unit
	InStock   uint
	Images    []upload.Upload
	CreatedAt time.Time
	UpdatedAt time.Time
}
