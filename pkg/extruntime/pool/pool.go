package pool

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"

	"github.com/iota-uz/iota-sdk/pkg/serrors"
)

const (
	opNew    = serrors.Op("extruntime.pool.New")
	opInvoke = serrors.Op("extruntime.pool.Invoke")
	opClose  = serrors.Op("extruntime.pool.Close")
)

// Config configures a Pool. Zero values select the documented defaults.
type Config struct {
	// Size is the number of worker processes. Default 2.
	Size int
	// Concurrency is the number of invocations a single worker runs at once.
	// Default 1: a worker is a unit of CPU isolation, and invocations beyond
	// Size*Concurrency queue. Nested invocations (Invocation.ParentID) do not
	// take a slot, so a parent waiting for them can never deadlock the pool.
	Concurrency int
	// MaxQueue bounds waiting invocations; 0 means unbounded.
	MaxQueue int

	NodePath string   // default "node"
	NodeArgs []string // flags placed before the worker script
	// WorkerScript is the path of the host worker (dist/host/worker.mjs).
	WorkerScript string
	WorkDir      string // default: directory of the worker script

	NodeEnv string // default "production"
	TZ      string // default "UTC"
	// ExtraEnv entries must be IOTA_RUNTIME_* variables; the worker never
	// inherits the host environment.
	ExtraEnv []string

	StartupTimeout time.Duration // default 15s
	// CancelGrace is how long a canceled invocation may take to report done
	// before the worker is killed (it may be stuck in synchronous code).
	CancelGrace time.Duration // default 2s
	// ShutdownGrace bounds how long Close waits for workers to exit after
	// their stdin is closed. Default 2s.
	ShutdownGrace time.Duration

	RestartBackoffMin time.Duration // default 50ms
	RestartBackoffMax time.Duration // default 5s
	// StableAfter is how long a worker must live to reset restart backoff.
	StableAfter time.Duration // default 10s

	MaxFrameBytes int // default 64 MiB

	Logger *slog.Logger
}

func (c *Config) applyDefaults() error {
	if c.WorkerScript == "" {
		return serrors.New(serrors.Invalid, "extruntime pool: WorkerScript is required")
	}
	if c.Size <= 0 {
		c.Size = 2
	}
	if c.Concurrency <= 0 {
		c.Concurrency = 1
	}
	if c.NodePath == "" {
		c.NodePath = "node"
	}
	if c.NodeEnv == "" {
		c.NodeEnv = "production"
	}
	if c.TZ == "" {
		c.TZ = "UTC"
	}
	for _, kv := range c.ExtraEnv {
		if !strings.HasPrefix(kv, "IOTA_RUNTIME_") || !strings.Contains(kv, "=") {
			return serrors.New(serrors.Invalid, fmt.Sprintf("extruntime pool: env %q is not an IOTA_RUNTIME_* assignment", kv))
		}
	}
	if c.StartupTimeout <= 0 {
		c.StartupTimeout = 15 * time.Second
	}
	if c.CancelGrace <= 0 {
		c.CancelGrace = 2 * time.Second
	}
	if c.ShutdownGrace <= 0 {
		c.ShutdownGrace = 2 * time.Second
	}
	if c.RestartBackoffMin <= 0 {
		c.RestartBackoffMin = 50 * time.Millisecond
	}
	if c.RestartBackoffMax < c.RestartBackoffMin {
		c.RestartBackoffMax = 5 * time.Second
	}
	if c.StableAfter <= 0 {
		c.StableAfter = 10 * time.Second
	}
	if c.MaxFrameBytes <= 0 {
		c.MaxFrameBytes = 64 << 20
	}
	if c.Logger == nil {
		c.Logger = slog.Default()
	}
	return nil
}

// HostCalls serves the "call" frames of one invocation. A returned
// *WireError keeps its kind, code, params and issues across the boundary; any
// other error becomes an internal error.
type HostCalls interface {
	Call(ctx context.Context, method string, args json.RawMessage) (json.RawMessage, error)
}

// HostCallsFunc adapts a function to HostCalls.
type HostCallsFunc func(ctx context.Context, method string, args json.RawMessage) (json.RawMessage, error)

func (f HostCallsFunc) Call(ctx context.Context, method string, args json.RawMessage) (json.RawMessage, error) {
	return f(ctx, method, args)
}

// Invocation is one execution request.
type Invocation struct {
	// ID is generated when empty. Ctx.Invocation defaults to it.
	ID string
	// ParentID runs this invocation on the worker of a live invocation (the
	// nested invocation of tx.call) without taking a pool slot.
	ParentID string
	Artifact Artifact
	Kind     Kind
	Export   string
	Resource string
	Input    json.RawMessage
	Ctx      InvokeContext
	Calls    HostCalls
}

type runResult struct {
	value json.RawMessage
	ok    bool
	wire  *WireError
	lost  *WorkerLostError
}

type run struct {
	id        string
	w         *worker
	calls     HostCalls
	callCtx   context.Context
	cancel    context.CancelFunc
	wg        sync.WaitGroup
	suspended atomic.Bool
	result    chan runResult
}

func (r *run) finish(res runResult) {
	select {
	case r.result <- res:
	default:
	}
}

type waiter struct {
	grant chan *worker
}

// Pool is a fixed set of warm Node workers.
type Pool struct {
	cfg Config

	mu      sync.Mutex
	slots   []*worker
	waiters []*waiter
	closing bool

	runsWG sync.WaitGroup
	supWG  sync.WaitGroup
	stop   chan struct{}
}

// Stats is a point-in-time view of the pool.
type Stats struct {
	Size    int
	Ready   int
	Busy    int
	Queued  int
	PIDs    []int
	Closing bool
}

// New starts cfg.Size workers and returns once every one of them completed the
// protocol handshake. A worker that fails its first start fails New.
func New(cfg Config) (*Pool, error) {
	if err := cfg.applyDefaults(); err != nil {
		return nil, serrors.Wrap(opNew, err)
	}
	if cfg.WorkDir == "" {
		cfg.WorkDir = dirOf(cfg.WorkerScript)
	}
	p := &Pool{cfg: cfg, slots: make([]*worker, cfg.Size), stop: make(chan struct{})}

	first := make(chan error, cfg.Size)
	for i := 0; i < cfg.Size; i++ {
		p.supWG.Add(1)
		go p.supervise(i, first)
	}
	for i := 0; i < cfg.Size; i++ {
		if err := <-first; err != nil {
			_ = p.Close(context.Background())
			return nil, serrors.Wrap(opNew, err)
		}
	}
	return p, nil
}

func (p *Pool) supervise(slot int, first chan<- error) {
	defer p.supWG.Done()
	reportedFirst := false
	failures := 0
	for {
		select {
		case <-p.stop:
			if !reportedFirst {
				first <- ErrPoolClosed
			}
			return
		default:
		}

		w, err := p.spawn(slot)
		if err != nil {
			if !reportedFirst {
				first <- err
				return
			}
			p.cfg.Logger.Error("extruntime worker failed to start", "worker", slot, "error", err)
			failures++
		} else {
			if !p.markReady(w) {
				w.closeInput()
				<-w.exited
				if !reportedFirst {
					first <- ErrPoolClosed
				}
				return
			}
			if !reportedFirst {
				reportedFirst = true
				first <- nil
			}
			started := time.Now()
			<-w.exited
			if time.Since(started) < p.cfg.StableAfter {
				failures++
			} else {
				failures = 0
			}
			p.cfg.Logger.Warn("extruntime worker exited", "worker", slot, "pid", w.pid, "stderr", w.stderrTail())
		}

		delay := p.cfg.RestartBackoffMax
		if shift := max(failures-1, 0); shift < 16 {
			delay = min(p.cfg.RestartBackoffMin<<shift, p.cfg.RestartBackoffMax)
		}
		select {
		case <-p.stop:
			return
		case <-time.After(delay):
		}
	}
}

func (w *worker) closeInput() { _ = w.stdin.Close() }

func (p *Pool) markReady(w *worker) bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.closing {
		return false
	}
	p.slots[w.slot] = w
	p.dispatchLocked()
	return true
}

func (p *Pool) markDead(w *worker) {
	p.mu.Lock()
	w.dead = true
	if p.slots[w.slot] == w {
		p.slots[w.slot] = nil
	}
	p.mu.Unlock()
}

// dispatchLocked hands free capacity to queued invocations in FIFO order.
func (p *Pool) dispatchLocked() {
	for len(p.waiters) > 0 {
		w := p.pickLocked()
		if w == nil {
			return
		}
		wt := p.waiters[0]
		p.waiters = p.waiters[1:]
		w.used++
		wt.grant <- w
	}
}

func (p *Pool) pickLocked() *worker {
	var best *worker
	for _, w := range p.slots {
		if w == nil || w.dead || w.used >= p.cfg.Concurrency {
			continue
		}
		if best == nil || w.used < best.used {
			best = w
		}
	}
	return best
}

func (p *Pool) acquire(ctx context.Context) (*worker, error) {
	p.mu.Lock()
	if p.closing {
		p.mu.Unlock()
		return nil, ErrPoolClosed
	}
	if len(p.waiters) == 0 {
		if w := p.pickLocked(); w != nil {
			w.used++
			p.mu.Unlock()
			return w, nil
		}
	}
	if p.cfg.MaxQueue > 0 && len(p.waiters) >= p.cfg.MaxQueue {
		p.mu.Unlock()
		return nil, ErrQueueFull
	}
	wt := &waiter{grant: make(chan *worker, 1)}
	p.waiters = append(p.waiters, wt)
	p.mu.Unlock()

	select {
	case w := <-wt.grant:
		if w == nil {
			return nil, ErrPoolClosed
		}
		return w, nil
	case <-ctx.Done():
		p.mu.Lock()
		for i, x := range p.waiters {
			if x == wt {
				p.waiters = append(p.waiters[:i], p.waiters[i+1:]...)
				p.mu.Unlock()
				return nil, ctx.Err()
			}
		}
		p.mu.Unlock()
		if w := <-wt.grant; w != nil {
			p.release(w)
		}
		return nil, ctx.Err()
	}
}

func (p *Pool) release(w *worker) {
	p.mu.Lock()
	if w.used > 0 {
		w.used--
	}
	p.dispatchLocked()
	p.mu.Unlock()
}

// Invoke runs one invocation to completion. Failures keep their structure:
// *WireError for errors raised by the handler, *WorkerLostError (matching
// ErrWorkerLost) when the process died, or the context error when ctx ended.
// The pool never re-runs an invocation.
func (p *Pool) Invoke(ctx context.Context, inv Invocation) (json.RawMessage, error) {
	if inv.Calls == nil {
		return nil, serrors.New(serrors.Invalid, "extruntime pool: Invocation.Calls is required")
	}
	if inv.ID == "" {
		inv.ID = "inv-" + uuid.NewString()
	}
	if inv.Ctx.Invocation == "" {
		inv.Ctx.Invocation = inv.ID
	}
	if inv.Ctx.Deadline == "" {
		if dl, ok := ctx.Deadline(); ok {
			inv.Ctx.Deadline = formatDeadline(dl)
		}
	}

	p.mu.Lock()
	if p.closing {
		p.mu.Unlock()
		return nil, serrors.Wrap(opInvoke, ErrPoolClosed)
	}
	p.runsWG.Add(1)
	p.mu.Unlock()
	defer p.runsWG.Done()

	for {
		w, slotted, err := p.place(ctx, inv.ParentID)
		if err != nil {
			return nil, serrors.Wrap(opInvoke, err)
		}
		val, retry, err := p.runOn(ctx, w, slotted, inv)
		if retry {
			continue
		}
		return val, err
	}
}

// place picks the worker for an invocation: the parent's worker for nested
// invocations, otherwise a slot from the queue.
func (p *Pool) place(ctx context.Context, parentID string) (*worker, bool, error) {
	if parentID == "" {
		w, err := p.acquire(ctx)
		return w, true, err
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.closing {
		return nil, false, ErrPoolClosed
	}
	for _, w := range p.slots {
		if w != nil && w.lookup(parentID) != nil {
			return w, false, nil
		}
	}
	return nil, false, serrors.New(serrors.NotFound, "extruntime pool: parent invocation "+parentID+" is not running")
}

// runOn executes inv on w. retry is true when w died before the invocation
// was sent, which is safe to place again.
func (p *Pool) runOn(ctx context.Context, w *worker, slotted bool, inv Invocation) (json.RawMessage, bool, error) {
	if slotted {
		defer p.release(w)
	}
	callCtx, cancel := context.WithCancel(ctx)
	r := &run{id: inv.ID, w: w, calls: inv.Calls, callCtx: callCtx, cancel: cancel, result: make(chan runResult, 1)}
	defer func() {
		cancel()
		r.wg.Wait()
		w.unregister(r.id)
	}()

	if !w.register(r) {
		if !slotted {
			return nil, false, serrors.Wrap(opInvoke, &WorkerLostError{Worker: w.slot, PID: w.pid, Invocation: inv.ID, Cause: "parent worker exited", Stderr: w.stderrTail()})
		}
		return nil, true, nil
	}

	frame := invokeFrame{T: "invoke", ID: inv.ID, Artifact: inv.Artifact, Kind: inv.Kind, Export: inv.Export, Resource: inv.Resource, Input: inv.Input, Ctx: inv.Ctx}
	if len(frame.Input) == 0 {
		frame.Input = json.RawMessage("null")
	}
	if err := w.send(frame); err != nil {
		w.kill()
		<-w.exited
		return nil, false, serrors.Wrap(opInvoke, &WorkerLostError{Worker: w.slot, PID: w.pid, Invocation: inv.ID, Cause: "write to worker failed: " + err.Error(), Stderr: w.stderrTail()})
	}

	select {
	case res := <-r.result:
		val, err := p.outcome(res)
		return val, false, err
	case <-ctx.Done():
		_ = w.send(cancelFrame{T: "cancel", ID: inv.ID})
		grace := time.NewTimer(p.cfg.CancelGrace)
		defer grace.Stop()
		select {
		case <-r.result:
		case <-grace.C:
			p.cfg.Logger.Warn("extruntime worker ignored cancel; killing", "worker", w.slot, "invocation", inv.ID)
			w.kill()
			<-w.exited
		}
		return nil, false, serrors.Wrap(opInvoke, ctx.Err())
	}
}

func (p *Pool) outcome(res runResult) (json.RawMessage, error) {
	switch {
	case res.lost != nil:
		return nil, serrors.Wrap(opInvoke, res.lost)
	case !res.ok && res.wire != nil:
		return nil, res.wire
	case !res.ok:
		return nil, serrors.Wrap(opInvoke, Internal(CodeHostCall, "worker reported failure without error"))
	default:
		return res.value, nil
	}
}

// Stats returns a snapshot of the pool.
func (p *Pool) Stats() Stats {
	p.mu.Lock()
	defer p.mu.Unlock()
	st := Stats{Size: p.cfg.Size, Queued: len(p.waiters), Closing: p.closing}
	for _, w := range p.slots {
		if w == nil || w.dead {
			continue
		}
		st.Ready++
		st.PIDs = append(st.PIDs, w.pid)
		if w.used > 0 {
			st.Busy++
		}
	}
	return st
}

// Close stops accepting invocations, lets running ones finish until ctx ends,
// then closes every worker's stdin (the shutdown signal) and kills what does
// not exit within ShutdownGrace.
func (p *Pool) Close(ctx context.Context) error {
	p.mu.Lock()
	if p.closing {
		p.mu.Unlock()
		return nil
	}
	p.closing = true
	waiters := p.waiters
	p.waiters = nil
	close(p.stop)
	p.mu.Unlock()
	for _, wt := range waiters {
		wt.grant <- nil
	}

	drained := make(chan struct{})
	go func() { p.runsWG.Wait(); close(drained) }()
	var err error
	select {
	case <-drained:
	case <-ctx.Done():
		err = serrors.WrapContext(opClose, ctx.Err(), "in-flight invocations did not finish; workers are being killed")
	}

	p.mu.Lock()
	live := make([]*worker, 0, len(p.slots))
	for _, w := range p.slots {
		if w != nil {
			live = append(live, w)
		}
	}
	p.mu.Unlock()
	for _, w := range live {
		w.closeInput()
	}
	deadline := time.NewTimer(p.cfg.ShutdownGrace)
	defer deadline.Stop()
	for _, w := range live {
		select {
		case <-w.exited:
		case <-deadline.C:
			w.kill()
			<-w.exited
		}
	}
	p.supWG.Wait()
	return err
}

func dirOf(path string) string {
	if i := strings.LastIndexAny(path, `/\`); i >= 0 {
		return path[:i]
	}
	return "."
}
