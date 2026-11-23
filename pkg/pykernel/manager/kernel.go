package manager

import (
	"context"
	"io/fs"
	"path/filepath"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/pkg/pykernel"
	"github.com/iota-uz/iota-sdk/pkg/pykernel/bridge"
	"github.com/iota-uz/iota-sdk/pkg/pykernel/isolation"
)

const (
	// execBuffer is the per-exec event channel capacity. The last two slots
	// are reserved for the terminal events (a synthetic error and EventDone),
	// so the stream always ends with exactly one EventDone even when the
	// consumer stopped reading.
	execBuffer       = 256
	reservedTerminal = 2
	quotaPoll        = 500 * time.Millisecond
)

// kernelOptions are the per-kernel limits the Manager hands to a kernel.
type kernelOptions struct {
	outputCap      int
	grace          time.Duration
	wallClock      time.Duration
	workdirQuota   int64
	workdir        string
	nsReset        bool
	filesPreserved bool
}

// kernel is a live Python subprocess plus its bridge. It implements
// pykernel.Kernel and serves as the bridge.EventSink that fans kernel output
// into the active Exec's event channel.
type kernel struct {
	key    string
	tenant uuid.UUID
	mode   pykernel.Mode
	proc   isolation.Process
	br     bridge.Bridge
	disp   *dispatcher
	opts   kernelOptions

	serveCancel context.CancelFunc
	closed      chan struct{}
	closeOnce   sync.Once
	serveDone   chan struct{}
	ready       chan []byte

	mu             sync.Mutex
	disposed       bool
	serveExited    bool
	inFlight       bool
	nsReset        bool
	filesPreserved bool
	execs          int
	leases         int
	lastActive     time.Time
	pid            int
	active         *activeExec
}

// activeExec is the state of the one in-flight exec.
type activeExec struct {
	id      string
	ctx     context.Context
	cancel  context.CancelFunc
	ch      chan pykernel.ExecEvent
	done    chan struct{}
	stop    pykernel.TerminationReason
	errType string
	dropped int
	nsReset bool
	files   bool
}

var (
	_ pykernel.Kernel  = (*kernel)(nil)
	_ bridge.EventSink = (*kernel)(nil)
)

func newKernel(sess pykernel.Session, proc isolation.Process, br bridge.Bridge, opts kernelOptions) *kernel {
	k := &kernel{
		key:            sess.Key(),
		tenant:         sess.TenantID(),
		mode:           sess.Mode(),
		proc:           proc,
		br:             br,
		opts:           opts,
		closed:         make(chan struct{}),
		serveDone:      make(chan struct{}),
		ready:          make(chan []byte, 1),
		lastActive:     time.Now(),
		pid:            proc.PID(),
		nsReset:        opts.nsReset,
		filesPreserved: opts.filesPreserved,
	}
	k.disp = &dispatcher{caps: sess.Capabilities(), mode: sess.Mode(), tenant: sess.TenantID(), exec: k.execContext}
	if b, ok := sess.(pykernel.CallContextBinder); ok {
		k.disp.bind = b.BindCallContext
	}
	return k
}

// execContext returns the context capability calls of execID run under, as
// long as that exec is still the active one. It is cancelled when the exec is
// cancelled, times out or is stopped, and it carries the consumer's values
// only when the exec asked for BindContext.
func (k *kernel) execContext(execID string) (context.Context, bool) {
	k.mu.Lock()
	defer k.mu.Unlock()
	if execID == "" || k.active == nil || execID != k.active.id {
		return nil, false
	}
	return k.active.ctx, true
}

// start runs the bridge read loop for the kernel's lifetime.
func (k *kernel) start() {
	ctx, cancel := context.WithCancel(context.Background())
	k.serveCancel = cancel
	go func() {
		_ = k.br.Serve(ctx, k.disp, k)
		k.onServeExit()
		close(k.serveDone)
	}()
}

func (k *kernel) Info() pykernel.KernelInfo {
	k.mu.Lock()
	defer k.mu.Unlock()
	return k.infoLocked()
}

func (k *kernel) infoLocked() pykernel.KernelInfo {
	return pykernel.KernelInfo{
		Key:            k.key,
		PID:            k.pid,
		LastActiveAt:   k.lastActive,
		NamespaceReset: k.nsReset,
		FilesPreserved: k.filesPreserved,
		InFlight:       k.inFlight,
		Leased:         k.leases > 0,
	}
}

func (k *kernel) isDisposed() bool {
	k.mu.Lock()
	defer k.mu.Unlock()
	return k.disposed || k.serveExited
}

func (k *kernel) lease() {
	k.mu.Lock()
	k.leases++
	k.mu.Unlock()
}

func (k *kernel) unlease() {
	k.mu.Lock()
	if k.leases > 0 {
		k.leases--
	}
	k.mu.Unlock()
}

// resetNamespace clears the Python namespace before the next exec (policy
// FreshNamespace on reuse). It is a no-op for a kernel that never ran code.
func (k *kernel) resetNamespace() error {
	k.mu.Lock()
	if k.execs == 0 || k.inFlight {
		k.mu.Unlock()
		return nil
	}
	k.nsReset = true
	k.filesPreserved = true
	k.mu.Unlock()
	return k.br.ResetNamespace(context.Background())
}

func (k *kernel) Exec(ctx context.Context, req pykernel.ExecRequest) (<-chan pykernel.ExecEvent, error) {
	timeout := req.Timeout
	if w := k.opts.wallClock; w > 0 && (timeout <= 0 || timeout > w) {
		timeout = w
	}
	var quotaStart int64
	if k.opts.workdirQuota > 0 {
		quotaStart = dirSize(k.opts.workdir)
	}

	k.mu.Lock()
	if k.disposed || k.serveExited {
		k.mu.Unlock()
		return nil, pykernel.ErrKernelDisposed
	}
	if k.inFlight {
		k.mu.Unlock()
		return nil, pykernel.ErrKernelBusy
	}
	base := context.Background()
	if req.BindContext {
		base = ctx
	}
	callCtx, cancel := context.WithCancel(base)
	a := &activeExec{
		id:      uuid.NewString(),
		ctx:     callCtx,
		cancel:  cancel,
		ch:      make(chan pykernel.ExecEvent, execBuffer),
		done:    make(chan struct{}),
		nsReset: k.nsReset,
		files:   k.filesPreserved,
	}
	k.active = a
	k.nsReset = false // acknowledged by this exec; reported in its DonePayload
	k.execs++
	k.inFlight = true
	k.lastActive = time.Now()
	k.mu.Unlock()

	outCap := req.OutputCap
	if outCap == 0 {
		outCap = k.opts.outputCap
	}
	limits := bridge.Limits{OutputCap: outCap, SeparateStderr: req.SeparateStderr}
	if timeout > 0 {
		limits.WallClockMS = timeout.Milliseconds()
	}

	if err := k.br.Submit(ctx, a.id, req.Code, limits); err != nil {
		k.mu.Lock()
		if k.active == a {
			k.finishLocked()
		}
		k.mu.Unlock()
		return nil, err
	}

	go k.watch(ctx, a, timeout, quotaStart)
	return a.ch, nil
}

// Emit receives a kernel output notification (runs in the bridge read loop) and
// fans it into the active exec's channel. It never blocks: when a consumer
// stops reading, non-terminal events are dropped and counted, and the terminal
// EventDone still fits in the reserved slots, so the read loop, the slot and
// the next exec are never held hostage by a stalled consumer.
func (k *kernel) Emit(execID string, ev bridge.RawEvent) {
	if ev.Kind == bridge.KindReady {
		select {
		case k.ready <- append([]byte(nil), ev.Params...):
		default:
		}
		return
	}
	out, terminal := translate(ev)

	k.mu.Lock()
	defer k.mu.Unlock()
	a := k.active
	if a == nil || execID != a.id {
		return
	}
	if out.Kind == pykernel.EventError && out.Err != nil {
		a.errType = out.Err.Type
	}
	if terminal {
		out.Done = k.donePayloadLocked(a, out.Done, "")
		k.finishLocked(out)
		return
	}
	if len(a.ch) >= cap(a.ch)-reservedTerminal {
		a.dropped++
		return
	}
	a.ch <- out // cannot block: this goroutine is the only sender and a slot is free
}

// donePayloadLocked completes the shim's terminal summary with what only the
// host knows: why the exec ended, dropped events and the namespace state.
func (k *kernel) donePayloadLocked(a *activeExec, base *pykernel.DonePayload, exited pykernel.TerminationReason) *pykernel.DonePayload {
	d := pykernel.DonePayload{}
	if base != nil {
		d = *base
	}
	d.EventsDropped = a.dropped
	d.NamespaceReset = a.nsReset
	d.FilesPreserved = a.files
	switch {
	case exited != "":
		d.Reason = exited
	case a.errType == "":
		d.Reason = pykernel.ReasonCompleted
	case a.errType == "KernelCancelled" && a.stop != "":
		d.Reason = a.stop
	case a.errType == "KernelCancelled":
		d.Reason = pykernel.ReasonCancelled
	default:
		d.Reason = pykernel.ReasonError
	}
	return &d
}

// finishLocked sends the given terminal events, closes the active exec channel
// and clears in-flight state. Caller holds k.mu. It is reached only from the
// bridge read goroutine (Emit, onServeExit) or the Submit-failure path, so the
// channel is closed exactly once and the sends never block (reserved slots).
func (k *kernel) finishLocked(events ...pykernel.ExecEvent) {
	a := k.active
	if a == nil {
		return
	}
	for _, ev := range events {
		select {
		case a.ch <- ev:
		default:
		}
	}
	close(a.ch)
	a.cancel()
	close(a.done)
	k.active = nil
	k.inFlight = false
	k.lastActive = time.Now()
}

// setStop records why the host is stopping exec a, unless it already ended.
func (k *kernel) setStop(a *activeExec, reason pykernel.TerminationReason) bool {
	k.mu.Lock()
	defer k.mu.Unlock()
	if k.active != a {
		return false
	}
	if a.stop == "" {
		a.stop = reason
	}
	a.cancel() // capability calls of this exec observe the stop at once
	return true
}

// watch enforces ctx cancellation, the wall-clock timeout and the workdir
// quota: cooperative cancel first, then SIGKILL after the dispose grace if the
// exec hasn't ended.
func (k *kernel) watch(ctx context.Context, a *activeExec, timeout time.Duration, quotaStart int64) {
	var timer <-chan time.Time
	if timeout > 0 {
		t := time.NewTimer(timeout)
		defer t.Stop()
		timer = t.C
	}
	var tick <-chan time.Time
	if k.opts.workdirQuota > 0 {
		t := time.NewTicker(quotaPoll)
		defer t.Stop()
		tick = t.C
	}
	var reason pykernel.TerminationReason
	for reason == "" {
		select {
		case <-a.done:
			return
		case <-k.closed:
			return
		case <-ctx.Done():
			reason = pykernel.ReasonCancelled
		case <-timer:
			reason = pykernel.ReasonTimeout
		case <-tick:
			if size := dirSize(k.opts.workdir); size > k.opts.workdirQuota && size > quotaStart {
				reason = pykernel.ReasonQuotaExceeded
			}
		}
	}
	if !k.setStop(a, reason) {
		return
	}
	// Run the cancel in a goroutine so a stalled Cancel write can't block the
	// grace/Kill escalation below.
	go func() { _ = k.br.Cancel(context.Background(), a.id) }()
	grace := k.opts.grace
	if reason == pykernel.ReasonQuotaExceeded {
		grace = 0 // a writer must not get more time to keep writing
	}
	select {
	case <-a.done:
	case <-k.closed:
	case <-time.After(grace):
		_ = k.proc.Kill()
	}
}

// onServeExit runs after the bridge read loop returns (kernel died or was
// disposed). It marks the kernel serve-exited (not fully disposed) so the pool
// treats it as unusable via isDisposed, while leaving full teardown to Dispose
// — the process may still be alive and must still be reaped. If an exec was
// still in flight, the consumer is unblocked with a synthetic error and a
// terminal EventDone carrying the reason.
func (k *kernel) onServeExit() {
	k.mu.Lock()
	defer k.mu.Unlock()
	k.serveExited = true
	a := k.active
	if a == nil {
		return
	}
	reason := a.stop
	if reason == "" {
		reason = pykernel.ReasonKernelExited
	}
	now := time.Now()
	errEv := pykernel.ExecEvent{
		Kind:      pykernel.EventError,
		Err:       &pykernel.ExecError{Type: "KernelExited", Message: "kernel process exited before completing the exec"},
		Timestamp: now,
	}
	doneEv := pykernel.ExecEvent{Kind: pykernel.EventDone, Done: k.donePayloadLocked(a, nil, reason), Timestamp: now}
	k.finishLocked(errEv, doneEv)
}

func (k *kernel) Dispose(_ context.Context) error {
	k.mu.Lock()
	if k.disposed {
		k.mu.Unlock()
		return nil
	}
	k.disposed = true
	k.mu.Unlock()

	// Stop the read loop and close the bridge, then kill the whole process
	// group before reaping the leader: the unreaped leader keeps the group id
	// reserved, so the kill cannot hit an unrelated group, and children the
	// kernel forked (which cannot leave the group) die with it. Durable state
	// is already in the workdir, so there is nothing to flush gracefully.
	k.closeOnce.Do(func() { close(k.closed) })
	if k.serveCancel != nil {
		k.serveCancel()
	}
	_ = k.br.Close()
	_ = k.proc.Kill()

	waitDone := make(chan struct{})
	go func() {
		_ = k.proc.Wait()
		close(waitDone)
	}()
	select {
	case <-waitDone:
	case <-time.After(k.opts.grace):
		// Wait is stuck only if the process ignores SIGKILL (D state);
		// leave the reaping to the goroutine.
	}
	return nil
}

// dirSize sums the apparent size of regular files below dir without following
// symlinks.
func dirSize(dir string) int64 {
	var total int64
	_ = filepath.WalkDir(dir, func(_ string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil //nolint:nilerr // a vanished entry is not an error for a size estimate
		}
		if d.Type().IsRegular() {
			if info, err := d.Info(); err == nil {
				total += info.Size()
			}
		}
		return nil
	})
	return total
}
