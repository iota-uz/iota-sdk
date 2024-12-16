package inventory

import (
	"time"
)

type Check struct {
	ID         uint
	Status     Status
	Type       Type
	Name       string
	Results    []*CheckResult
	CreatedAt  time.Time
	FinishedAt time.Time
	CreatedBy  uint
	FinishedBy uint
}

type CheckResult struct {
	ID               uint
	PositionID       uint
	ExpectedQuantity int
	ActualQuantity   int
	Difference       int
	CreatedAt        time.Time
}
