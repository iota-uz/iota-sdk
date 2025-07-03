package inventory

import (
	"time"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/user"

	"github.com/iota-uz/iota-sdk/modules/warehouse/domain/aggregates/position"
)

type Check struct {
	ID           uint
	TenantID     uuid.UUID
	Status       Status
	Name         string
	Results      []*CheckResult
	CreatedAt    time.Time
	FinishedAt   time.Time
	CreatedByID  uint
	CreatedBy    user.User
	FinishedBy   user.User
	FinishedByID uint
}

func (c *Check) AddResult(positionID uint, expected, actual int) {
	c.Results = append(c.Results, &CheckResult{
		PositionID:       positionID,
		TenantID:         c.TenantID,
		ExpectedQuantity: expected,
		ActualQuantity:   actual,
		Difference:       expected - actual,
		CreatedAt:        time.Now(),
	})
}

type Position struct {
	ID       uint
	Title    string
	Quantity int
	RfidTags []string
}

type CheckResult struct {
	ID               uint
	TenantID         uuid.UUID
	PositionID       uint
	Position         position.Position
	ExpectedQuantity int
	ActualQuantity   int
	Difference       int
	CreatedAt        time.Time
}
