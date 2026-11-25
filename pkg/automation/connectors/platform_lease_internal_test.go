package connectors

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// The provider setup's deadline leaves a quarter of the committed lease: the lock wait before it counts against it,
// and a setup that could not end with that much left is not attempted.
func TestStartBudgetLeavesAQuarterOfTheLease(t *testing.T) {
	t.Parallel()
	assert.Equal(t, PlatformStartTimeout, startBudget(PlatformLease), "a full lease: the setup's own limit")
	assert.Equal(t, 30*time.Second, startBudget(time.Minute), "a lock wait of a minute shortens the deadline")
	assert.LessOrEqual(t, startBudget(platformStartMargin), time.Duration(0), "no setup with only the margin left")
	for _, left := range []time.Duration{PlatformLease, 90 * time.Second, time.Minute, 40 * time.Second} {
		if b := startBudget(left); b > 0 {
			assert.GreaterOrEqual(t, left-b, platformStartMargin, "a quarter of the lease is left at the deadline (%s)", left)
		}
	}
}
