package warehouse_position

import "time"

type Position struct {
	ID        int64
	Title     string
	Barcode   string
	UnitID    int64
	CreatedAt time.Time
	UpdatedAt time.Time
}
