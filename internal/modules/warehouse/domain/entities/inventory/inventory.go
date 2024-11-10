package inventory

import (
	"time"
)

type Check struct {
	ID        int64
	Status    *Status
	Results   []*CheckResult
	CreatedAt time.Time
}

type CheckResult struct {
	ID               int64
	PositionID       int64
	ExpectedQuantity int
	ActualQuantity   int
	Difference       int
	CreatedAt        time.Time
}
