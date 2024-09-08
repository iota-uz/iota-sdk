package product

import (
	"time"
)

type Product struct {
	ID         int64
	PositionID int64
	Rfid       string
	Status     *Status
	CreatedAt  time.Time
	UpdatedAt  time.Time
}
