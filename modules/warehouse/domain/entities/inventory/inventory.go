package inventory

import (
	"time"

	"github.com/iota-agency/iota-sdk/pkg/domain/aggregates/user"
)

type Check struct {
	ID           uint
	Status       Status
	Type         Type
	Name         string
	Results      []*CheckResult
	CreatedAt    time.Time
	FinishedAt   time.Time
	CreatedByID  uint
	CreatedBy    *user.User
	FinishedBy   *user.User
	FinishedByID uint
}

type CheckResult struct {
	ID               uint
	PositionID       uint
	ExpectedQuantity int
	ActualQuantity   int
	Difference       int
	CreatedAt        time.Time
}
