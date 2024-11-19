package product

import (
	"github.com/iota-agency/iota-sdk/pkg/modules/warehouse/domain/entities/position"
	"time"
)

type Product struct {
	ID         uint
	PositionID uint
	Rfid       string
	Status     Status
	Position   *position.Position
	CreatedAt  time.Time
	UpdatedAt  time.Time
}
