package testenv

import (
	"context"
	"sync"
	"time"
)

// ManualClock belongs to a dedicated environment and must be injected into the
// real services being tested; it does not alter SQL NOW() or the browser clock.
type ManualClock struct {
	mu  sync.RWMutex
	now time.Time
}

func NewManualClock(now time.Time) *ManualClock { return &ManualClock{now: now} }
func (c *ManualClock) Now() time.Time           { c.mu.RLock(); defer c.mu.RUnlock(); return c.now }
func (c *ManualClock) Advance(d time.Duration) error {
	if d < 0 {
		return failure("invalid_input", "clock cannot move backwards")
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = c.now.Add(d)
	return nil
}

// Gate makes races deterministic without wall-clock delays. Releasing a gate
// is idempotent; scope cleanup must release every gate owned by that scope.
type Gate struct {
	once      sync.Once
	release   chan struct{}
	entered   chan struct{}
	enterOnce sync.Once
}

func NewGate() *Gate { return &Gate{release: make(chan struct{}), entered: make(chan struct{})} }
func (g *Gate) Wait(ctx context.Context) error {
	g.enterOnce.Do(func() { close(g.entered) })
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-g.release:
		return nil
	}
}
func (g *Gate) Entered() <-chan struct{} { return g.entered }
func (g *Gate) Release()                 { g.once.Do(func() { close(g.release) }) }
