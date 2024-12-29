package position

import (
	"github.com/iota-uz/iota-sdk/modules/warehouse/domain/entities/unit"
	"github.com/iota-uz/iota-sdk/pkg/domain/entities/upload"
	"time"
)

type Position struct {
	ID        uint
	Title     string
	Barcode   string
	UnitID    uint
	Unit      unit.Unit
	InStock   uint
	Images    []upload.Upload
	CreatedAt time.Time
	UpdatedAt time.Time
}
