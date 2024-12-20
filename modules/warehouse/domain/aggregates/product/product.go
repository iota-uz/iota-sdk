package product

import (
	"github.com/iota-agency/iota-sdk/modules/warehouse/domain/aggregates/position"
	"time"
)

func New(rfid string, positionID uint, status Status, position *position.Position) *Product {
	return &Product{
		PositionID: positionID,
		Rfid:       rfid,
		Status:     status,
		Position:   position,
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}
}

type Product struct {
	ID         uint
	PositionID uint
	Rfid       string
	Status     Status
	Position   *position.Position
	CreatedAt  time.Time
	UpdatedAt  time.Time
}
