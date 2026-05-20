package spotlight

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/sony/gobreaker/v2"
)

// BreakerState mirrors gobreaker's state but is exported under our own
// type so callers (admin dashboard, metrics) do not pull a transitive
// dependency on gobreaker.
type BreakerState int

const (
	BreakerStateClosed BreakerState = iota
	BreakerStateHalfOpen
	BreakerStateOpen
)

func (s BreakerState) String() string {
	switch s {
	case BreakerStateClosed:
		return "closed"
	case BreakerStateHalfOpen:
		return "half_open"
	case BreakerStateOpen:
		return "open"
	default:
		return "unknown"
	}
}

// EngineBreakerConfig tunes the behaviour of the Meili engine circuit
// breaker. Defaults track issue #2810 §3.6: >10 errors in 60s → open.
type EngineBreakerConfig struct {
	// Name is included in metrics + log lines so multiple breakers
	// (active engine, build engine, AI tool engine) can be distinguished.
	Name string
	// MaxRequests is the number of requests allowed in half-open state
	// before the breaker decides to close or re-open.
	MaxRequests uint32
	// Interval resets the counter while the breaker is closed. 0 means
	// counters never reset until a trip.
	Interval time.Duration
	// Timeout is how long the breaker stays open before transitioning
	// to half-open and allowing a probe.
	Timeout time.Duration
	// MinRequests is the lower bound on the rolling window before a
	// failure ratio is even considered.
	MinRequests uint32
	// FailureRatio in (0,1] is the trip threshold once MinRequests have
	// been observed within Interval.
	FailureRatio float64
	// OnStateChange is invoked synchronously on every transition.
	OnStateChange func(name string, from, to BreakerState)
}

// DefaultEngineBreakerConfig matches the issue spec: open after >10 errors
// in 60 s rolling window, half-open after 30 s timeout.
func DefaultEngineBreakerConfig(name string) EngineBreakerConfig {
	return EngineBreakerConfig{
		Name:         name,
		MaxRequests:  1,
		Interval:     60 * time.Second,
		Timeout:      30 * time.Second,
		MinRequests:  10,
		FailureRatio: 0.5,
	}
}

// EngineBreaker wraps a gobreaker.CircuitBreaker with our own surface so
// projectors can call Allow / RecordResult or Execute. It is safe for
// concurrent use; the underlying *gobreaker.CircuitBreaker is replaced
// wholesale by ResetManually, so all accesses go through cbMu.
type EngineBreaker struct {
	mu       sync.RWMutex
	cb       *gobreaker.CircuitBreaker[any]
	settings gobreaker.Settings // captured at construction; reused on reset
	manualMu chan struct{}      // semaphore reserved for ResetManually
}

// ErrBreakerOpen is returned by Execute / Allow when the breaker has
// tripped.
var ErrBreakerOpen = errors.New("spotlight engine breaker is open")

// NewEngineBreaker builds a breaker that trips on the supplied config.
func NewEngineBreaker(cfg EngineBreakerConfig) *EngineBreaker {
	if cfg.Name == "" {
		cfg.Name = "spotlight.meili"
	}
	if cfg.MaxRequests == 0 {
		cfg.MaxRequests = 1
	}
	if cfg.Interval == 0 {
		cfg.Interval = 60 * time.Second
	}
	if cfg.Timeout == 0 {
		cfg.Timeout = 30 * time.Second
	}
	if cfg.MinRequests == 0 {
		cfg.MinRequests = 10
	}
	if cfg.FailureRatio <= 0 || cfg.FailureRatio > 1 {
		cfg.FailureRatio = 0.5
	}
	settings := gobreaker.Settings{
		Name:        cfg.Name,
		MaxRequests: cfg.MaxRequests,
		Interval:    cfg.Interval,
		Timeout:     cfg.Timeout,
		ReadyToTrip: func(c gobreaker.Counts) bool {
			if c.Requests < cfg.MinRequests {
				return false
			}
			return float64(c.TotalFailures)/float64(c.Requests) >= cfg.FailureRatio
		},
	}
	if cfg.OnStateChange != nil {
		settings.OnStateChange = func(name string, from, to gobreaker.State) {
			cfg.OnStateChange(name, breakerStateFrom(from), breakerStateFrom(to))
		}
	}
	cb := gobreaker.NewCircuitBreaker[any](settings)
	return &EngineBreaker{
		cb:       cb,
		settings: settings,
		manualMu: make(chan struct{}, 1),
	}
}

func breakerStateFrom(s gobreaker.State) BreakerState {
	switch s {
	case gobreaker.StateClosed:
		return BreakerStateClosed
	case gobreaker.StateHalfOpen:
		return BreakerStateHalfOpen
	case gobreaker.StateOpen:
		return BreakerStateOpen
	default:
		return BreakerStateClosed
	}
}

// loadCB returns the current breaker pointer under the read lock.
func (b *EngineBreaker) loadCB() *gobreaker.CircuitBreaker[any] {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.cb
}

// State returns the current state. Useful for the /system/spotlight UI.
func (b *EngineBreaker) State() BreakerState {
	return breakerStateFrom(b.loadCB().State())
}

// Execute runs fn through the breaker. If the breaker is open the function
// is not invoked and ErrBreakerOpen is returned. Non-nil fn errors count
// as failures toward the trip threshold. The caller's context is honored
// before fn runs so a cancelled context short-circuits without consuming
// a breaker slot.
func (b *EngineBreaker) Execute(ctx context.Context, fn func() error) error {
	if ctx != nil {
		if err := ctx.Err(); err != nil {
			return err
		}
	}
	cb := b.loadCB()
	_, err := cb.Execute(func() (any, error) {
		if ctx != nil {
			if cerr := ctx.Err(); cerr != nil {
				return nil, cerr
			}
		}
		return nil, fn()
	})
	if errors.Is(err, gobreaker.ErrOpenState) {
		return ErrBreakerOpen
	}
	return err
}

// ResetManually forces the breaker to closed. Intended for the admin
// "reset" endpoint exposed on /system/spotlight/breaker/reset.
//
// gobreaker has no public reset; the workaround is to recreate the
// breaker with the original settings so Name, OnStateChange, MinRequests,
// FailureRatio and timeouts survive the reset. b.mu serializes the swap
// with concurrent State/Execute reads.
func (b *EngineBreaker) ResetManually() {
	select {
	case b.manualMu <- struct{}{}:
		defer func() { <-b.manualMu }()
	default:
		return // someone else is already resetting; theirs wins.
	}
	fresh := gobreaker.NewCircuitBreaker[any](b.settings)
	b.mu.Lock()
	prev := b.cb.State()
	b.cb = fresh
	onStateChange := b.settings.OnStateChange
	name := b.settings.Name
	b.mu.Unlock()
	// gobreaker only emits OnStateChange on transitions inside the
	// breaker; NewCircuitBreaker does not synthesize a Closed event. If
	// we were previously open or half-open, fire the transition ourselves
	// so metrics and admin UIs that track state stay accurate.
	if onStateChange != nil && prev != gobreaker.StateClosed {
		onStateChange(name, prev, gobreaker.StateClosed)
	}
}
