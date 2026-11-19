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

// Set moves the clock to a scenario's logical time. The registry calls it when
// a dedicated scenario is prepared.
func (c *ManualClock) Set(now time.Time) { c.mu.Lock(); defer c.mu.Unlock(); c.now = now }

// UseClock makes r control clock: dedicated scenarios set it to Input.Now and
// the scope holding the dedicated lease may advance it. Inject the same clock
// into the real services under test; production keeps SystemClock.
func (r *Registry) UseClock(clock *ManualClock) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.clock = clock
}

// AdvanceClock moves the environment clock for the scope that holds the
// dedicated lease. Shared environments refuse: the clock is global state.
func (r *Registry) AdvanceClock(scopeID string, d time.Duration) (time.Time, error) {
	// The lease check and the mutation share Registry.mu, the lock that hands
	// the lease over, so an advance finishes under its lease or fails.
	r.mu.Lock()
	defer r.mu.Unlock()
	if err := r.ownedLocked(scopeID); err != nil {
		return time.Time{}, err
	}
	if r.clock == nil || !r.dedicated {
		return time.Time{}, failure("missing_capability", "clock control requires a dedicated environment with a manual clock")
	}
	if r.exclusiveScope != scopeID {
		return time.Time{}, failure("scope_conflict", "clock control requires the dedicated scenario lease")
	}
	return r.clock.advance(d)
}

// ClockNow returns the environment clock, or false when none is controlled.
func (r *Registry) ClockNow() (time.Time, bool) {
	r.mu.Lock()
	clock := r.clock
	r.mu.Unlock()
	if clock == nil {
		return time.Time{}, false
	}
	return clock.Now(), true
}

func (c *ManualClock) Advance(d time.Duration) error {
	_, err := c.advance(d)
	return err
}

// advance returns the time produced by this advance, read in the same
// critical section as the increment.
func (c *ManualClock) advance(d time.Duration) (time.Time, error) {
	if d < 0 {
		return time.Time{}, failure("invalid_input", "clock cannot move backwards")
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = c.now.Add(d)
	return c.now, nil
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
