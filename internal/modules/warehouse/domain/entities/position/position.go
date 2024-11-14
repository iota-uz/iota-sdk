package position

import "time"

type Position struct {
	ID        uint
	Title     string
	Barcode   string
	UnitID    uint
	CreatedAt time.Time
	UpdatedAt time.Time
}
