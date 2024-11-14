package product

import (
	"time"
)

type Product struct {
	ID         uint
	PositionID uint
	Rfid       string
	Status     Status
	CreatedAt  time.Time
	UpdatedAt  time.Time
}
