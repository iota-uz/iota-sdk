package inventory

import (
	"time"

	"github.com/iota-agency/iota-sdk/modules/warehouse/domain/aggregates/position"
	"github.com/iota-agency/iota-sdk/pkg/domain/aggregates/user"
)

type Check struct {
	ID           uint
	Status       Status
	Name         string
	Results      []*CheckResult
	CreatedAt    time.Time
	FinishedAt   time.Time
	CreatedByID  uint
	CreatedBy    *user.User
	FinishedBy   *user.User
	FinishedByID uint
}

type Position struct {
	ID       uint
	Title    string
	Quantity int
	RfidTags []string
}

type CheckResult struct {
	ID               uint
	PositionID       uint
	Position         *position.Position
	ExpectedQuantity int
	ActualQuantity   int
	Difference       int
	CreatedAt        time.Time
}
