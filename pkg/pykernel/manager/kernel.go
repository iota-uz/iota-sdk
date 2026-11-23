package manager

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"
	"sync"
	"syscall"
	"time"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/pkg/pykernel"
	"github.com/iota-uz/iota-sdk/pkg/pykernel/bridge"
	"github.com/iota-uz/iota-sdk/pkg/pykernel/isolation"
)

const (
	// execBuffer is the per-exec event channel capacity. The last three slots
	// are reserved for the first error and the terminal events,
	// so the stream always ends with exactly one EventDone even when the
	// consumer stopped reading.
	execBuffer       = 256
	reservedTerminal = 3 // the first EventError, a synthetic error, EventDone
	quotaPoll        = 500 * time.Millisecond
	exitDrain        = 300 * time.Millisecond
	maxStreams       = 16 // emulated stream writes waiting per kernel
	settlePoll       = 5 * time.Millisecond
)

// kernelOptions are the per-kernel limits the Manager hands to a kernel.
type kernelOptions struct {
	outputCap      int
	grace          time.Duration
	wallClock      time.Duration
	workdirQuota   int64
	workdirEntries int64
	fileSize       int64
	// supervised: a supervisor enforces the quota at allocation time, so the
	// polling watcher is not started.
	supervised     bool
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
	// gone is closed once the kernel's whole process group has terminated
	// and been reaped; every disposer can wait on it.
	gone      chan struct{}
	waitOnce  sync.Once
	frozen    bool // supervised kernels are stopped (SIGSTOP) while no exec runs
	serveDone chan struct{}
	ready     chan []byte

	mu             sync.Mutex
	disposed       bool
	serveExited    bool
	inFlight       bool
	nsReset        bool
	filesPreserved bool
	execs          int
	resets         int
	readySeen      bool // the kernel reported ready: spawning is gated from now on
	streams        int  // emulated stream writes in flight
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
	err     *pykernel.ExecError
	// claimed is the worker's own exec.done. In a supervised kernel it is
	// untrusted: the exec ends only when the host has seen the kernel come
	// to rest (settle); until then the watchdog stays armed.
	claimed *pykernel.ExecEvent
	quota   bool // the supervisor refused an allocation of this exec
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
		gone:           make(chan struct{}),
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
	if k.opts.workdirQuota > 0 && !k.opts.supervised {
		go k.watchQuota()
	}
	// The stream ends when the process dies, whatever the read loop is doing
	// (it may be stuck in a capability handler that ignores its context).
	if p, ok := k.proc.(interface{ Exited() <-chan struct{} }); ok {
		go func() {
			select {
			case <-p.Exited():
				// Let the read loop drain frames the kernel sent before it
				// died; force the end only if it is stuck.
				select {
				case <-k.serveDone:
				case <-time.After(exitDrain):
					_ = k.br.Close()
					k.onServeExit()
				}
			case <-k.closed:
			}
		}()
	}
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

// unlease drops one lease and reports how many remain.
func (k *kernel) unlease() int {
	k.mu.Lock()
	defer k.mu.Unlock()
	if k.leases > 0 {
		k.leases--
	}
	return k.leases
}

// leaseForReuse takes a lease on a pooled kernel. With fresh (policy
// FreshNamespace) the kernel must be unshared and idle, and the result says
// whether it ran code, so its namespace must be reset before use.
func (k *kernel) leaseForReuse(fresh bool) (bool, error) {
	k.mu.Lock()
	defer k.mu.Unlock()
	if fresh && (k.inFlight || k.leases > 0) {
		return false, pykernel.ErrKernelBusy
	}
	if fresh && k.execs > 0 && k.opts.supervised && !pristine(k.pid) {
		return false, errNotPristine
	}
	k.leases++
	return fresh && k.execs > 0, nil
}

// settle ends exec a after the worker claimed it was done, once the host
// itself sees the kernel at rest: processes the exec started are killed, and
// no thread of the kernel may be running. A kernel that keeps running is left
// to the watchdog, which stops it at the exec's deadline with ReasonTimeout.
func (k *kernel) settle(a *activeExec) {
	for calm := 0; calm < 2; {
		select {
		case <-a.done:
			return
		case <-k.closed:
			return
		case <-time.After(settlePoll):
		}
		if atRest(k.pid) {
			calm++
		} else {
			calm = 0
		}
	}
	k.mu.Lock()
	defer k.mu.Unlock()
	if k.active != a || a.claimed == nil {
		return
	}
	// The kernel is stopped before the consumer learns the exec ended: work
	// it left behind (a sleeping thread, a timer) cannot resume outside an
	// exec, whose watchdog, quota and capability context then bound it.
	if !k.freezeLocked() {
		_ = k.proc.Kill() // cannot be held at rest: never leave it running
		return
	}
	out := *a.claimed
	out.Done = k.donePayloadLocked(a, out.Done, "")
	k.finishLocked(out)
}

// freezeLocked stops the kernel's process group and confirms every member is
// stopped. Caller holds k.mu.
func (k *kernel) freezeLocked() bool {
	for i := 0; i < 100; i++ {
		_ = k.proc.Signal(syscall.SIGSTOP)
		if groupStopped(k.pid) {
			k.frozen = true
			return true
		}
		time.Sleep(time.Millisecond)
	}
	return false
}

// thawLocked resumes a frozen kernel for an exec. Caller holds k.mu.
func (k *kernel) thawLocked() {
	if k.frozen {
		_ = k.proc.Signal(syscall.SIGCONT)
		k.frozen = false
	}
}

// noteQuota records that the supervisor refused an allocation of the
// running exec, so its failure is reported as ReasonQuotaExceeded.
func (k *kernel) noteQuota() {
	k.mu.Lock()
	if k.active != nil {
		k.active.quota = true
	}
	k.mu.Unlock()
}

// activity reports whether no exec runs, how many have run, and when the
// kernel was last active.
func (k *kernel) activity() (bool, int, time.Time) {
	k.mu.Lock()
	defer k.mu.Unlock()
	return k.active == nil, k.execs + k.resets, k.lastActive
}

// spawnAllowed reports whether the kernel may create a thread or process:
// before it is ready (its own startup) and while an exec runs.
func (k *kernel) spawnAllowed() bool {
	k.mu.Lock()
	defer k.mu.Unlock()
	return !k.readySeen || k.active != nil
}

// addStream claims one of the kernel's emulated stream-write slots.
func (k *kernel) addStream() bool {
	k.mu.Lock()
	defer k.mu.Unlock()
	if k.streams >= maxStreams {
		return false
	}
	k.streams++
	return true
}

func (k *kernel) doneStream() {
	k.mu.Lock()
	k.streams--
	k.mu.Unlock()
}

// killIdle kills a kernel that keeps working while no exec runs.
func (k *kernel) killIdle() { _ = k.proc.Kill() }

// markReset records a namespace reset the kernel has been told to perform.
func (k *kernel) markReset() {
	k.mu.Lock()
	// Clearing a namespace runs user __del__ code: host-requested work, not
	// idle activity.
	k.resets++
	k.lastActive = time.Now()
	k.nsReset = true
	k.filesPreserved = true
	k.mu.Unlock()
}

func (k *kernel) Exec(ctx context.Context, req pykernel.ExecRequest) (<-chan pykernel.ExecEvent, error) {
	timeout := req.Timeout
	if w := k.opts.wallClock; w > 0 && (timeout <= 0 || timeout > w) {
		timeout = w
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
	k.thawLocked()
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

	// The watchdog is armed before the write: a kernel that stops reading its
	// socket blocks Submit, and only the watchdog's kill can unblock it.
	go k.watch(ctx, a, timeout)
	if err := k.br.Submit(ctx, a.id, req.Code, limits); err != nil {
		k.mu.Lock()
		defer k.mu.Unlock()
		if errors.Is(err, bridge.ErrFrameTooLarge) {
			// Rejected before anything was written: the kernel is healthy,
			// the request is the problem.
			if k.active == a {
				k.finishLocked()
			}
			return nil, fmt.Errorf("pykernel/manager: %w", pykernel.ErrRequestTooLarge)
		}
		if k.active == a && a.stop != "" {
			// The watchdog stopped a kernel that would not take the code:
			// the consumer gets the typed outcome, not a transport error.
			now := time.Now()
			errEv := pykernel.ExecEvent{Kind: pykernel.EventError, Timestamp: now,
				Err: &pykernel.ExecError{Type: "KernelCancelled", Message: "the exec was stopped before the kernel accepted it"}}
			a.err = errEv.Err
			doneEv := pykernel.ExecEvent{Kind: pykernel.EventDone, Done: k.donePayloadLocked(a, nil, a.stop), Timestamp: now}
			k.finishLocked(errEv, doneEv)
			return a.ch, nil
		}
		if k.active != a {
			// The exec already ended (stopped, or the kernel died while the
			// code was being sent): its closed stream carries the outcome.
			return a.ch, nil
		}
		// A broken submission means the kernel is dying: give its exit
		// path (process watcher, read loop) the chance to finish the exec,
		// and finish it as kernel_exited ourselves if it does not.
		k.mu.Unlock()
		select {
		case <-a.done:
		case <-time.After(exitDrain + time.Second):
		}
		k.mu.Lock()
		if k.active == a {
			now := time.Now()
			errEv := pykernel.ExecEvent{Kind: pykernel.EventError, Timestamp: now,
				Err: &pykernel.ExecError{Type: "KernelExited", Message: "the kernel could not take the code: " + err.Error()}}
			a.err = errEv.Err
			reason := pykernel.ReasonKernelExited
			if a.stop != "" {
				reason = a.stop
			}
			doneEv := pykernel.ExecEvent{Kind: pykernel.EventDone, Done: k.donePayloadLocked(a, nil, reason), Timestamp: now}
			k.finishLocked(errEv, doneEv)
		}
		return a.ch, nil
	}
	return a.ch, nil
}

// Emit receives a kernel output notification (runs in the bridge read loop) and
// fans it into the active exec's channel. It never blocks: when a consumer
// stops reading, non-terminal events are dropped and counted, and the terminal
// EventDone still fits in the reserved slots, so the read loop, the slot and
// the next exec are never held hostage by a stalled consumer.
func (k *kernel) Emit(execID string, ev bridge.RawEvent) {
	if ev.Kind == bridge.KindReady {
		k.mu.Lock()
		k.readySeen = true
		k.mu.Unlock()
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
	if terminal {
		if !k.opts.supervised {
			out.Done = k.donePayloadLocked(a, out.Done, "")
			k.finishLocked(out)
			return
		}
		if a.claimed == nil {
			a.claimed = &out
			go k.settle(a)
		}
		return
	}
	room := cap(a.ch) - len(a.ch)
	if out.Kind == pykernel.EventError && out.Err != nil {
		a.errType = out.Err.Type
		if a.err == nil {
			// The first error is structured failure detail: it keeps a
			// reserved slot and is repeated in DonePayload.Error.
			a.err = out.Err
			if room > reservedTerminal-1 {
				a.ch <- out
				return
			}
		}
		a.dropped++
		return
	}
	if room <= reservedTerminal {
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
	d.Error = a.err
	d.NamespaceReset = a.nsReset
	d.FilesPreserved = a.files
	switch {
	case exited != "":
		d.Reason = exited
	case a.stop != "":
		// A host-initiated stop wins even when the code swallowed the
		// interrupt and returned normally.
		d.Reason = a.stop
	case a.quota && a.errType != "":
		d.Reason = pykernel.ReasonQuotaExceeded
	case a.errType == "":
		d.Reason = pykernel.ReasonCompleted
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
func (k *kernel) watch(ctx context.Context, a *activeExec, timeout time.Duration) {
	var timer <-chan time.Time
	if timeout > 0 {
		t := time.NewTimer(timeout)
		defer t.Stop()
		timer = t.C
	}
	var reason pykernel.TerminationReason
	select {
	case <-a.done:
		return
	case <-k.closed:
		return
	case <-ctx.Done():
		reason = pykernel.ReasonCancelled
	case <-timer:
		reason = pykernel.ReasonTimeout
	}
	k.stop(a, reason)
}

// stop ends exec a for reason: its capability calls are cancelled at once,
// the code gets a cooperative interrupt, and the process is killed if the
// exec has not ended after the grace.
func (k *kernel) stop(a *activeExec, reason pykernel.TerminationReason) {
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

// watchQuota measures the workdir for the kernel's whole life — also between
// execs, where a background thread or forked child may keep writing. Whenever
// usage is over the quota and grew since the previous check (a session that
// starts over quota may still delete files), the running exec is stopped with
// ReasonQuotaExceeded, or, with no exec running, the kernel is killed.
func (k *kernel) watchQuota() {
	t := time.NewTicker(quotaPoll)
	defer t.Stop()
	prev := dirUsage(k.opts.workdir, k.opts.workdirEntries)
	for {
		select {
		case <-k.closed:
			return
		case <-k.serveDone:
			return
		case <-t.C:
		}
		cur := dirUsage(k.opts.workdir, k.opts.workdirEntries)
		// Past the entry cap the walk stops, so growth is no longer
		// measurable: a saturated workdir counts as growing.
		saturated := k.opts.workdirEntries > 0 && cur.entries > k.opts.workdirEntries
		over := saturated || cur.bytes > k.opts.workdirQuota
		grew := saturated || cur.bytes > prev.bytes || cur.entries > prev.entries
		prev = cur
		if !over || !grew {
			continue
		}
		select {
		case <-k.closed:
			return // disposed during the walk: the process may be reaped
		default:
		}
		k.mu.Lock()
		a := k.active
		k.mu.Unlock()
		if a != nil {
			k.stop(a, pykernel.ReasonQuotaExceeded)
		} else {
			_ = k.proc.Kill() // reap-safe: a no-op once the leader is reaped
		}
	}
}

// onServeExit runs after the bridge read loop returns (kernel died or was
// disposed). It marks the kernel serve-exited (not fully disposed) so the pool
// treats it as unusable via isDisposed, while leaving full teardown to Dispose
// — the process may still be alive and must still be reaped. If an exec was
// still in flight, the consumer is unblocked with a synthetic error and a
// terminal EventDone carrying the reason.
func (k *kernel) onServeExit() {
	// Without its bridge the kernel can never be used again, and code in it
	// must not outlive the exec's watchdog: stop the whole group now.
	_ = k.proc.Kill()
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
	if a.err == nil {
		a.err = errEv.Err
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
	k.reap()
	select {
	case <-k.gone:
	case <-time.After(k.opts.grace):
		// Wait is stuck only if a process ignores SIGKILL (D state); the
		// reaper keeps going and gone closes when it is done.
	}
	return nil
}

// reap starts, once, the wait for the kernel's whole process group (Wait
// empties the group before reaping the leader) and the reaping of members
// reparented to the host; gone closes after both.
func (k *kernel) reap() {
	k.waitOnce.Do(func() {
		go func() {
			_ = k.proc.Wait()
			// Descendants orphaned by the kill are reparented to the host
			// when it is PID 1 of a container; reap them as well.
			for isolation.ReapGroup(k.pid) > 0 {
				time.Sleep(20 * time.Millisecond)
			}
			close(k.gone)
		}()
	})
}

// Gone is closed when the kernel's process group has terminated.
func (k *kernel) Gone() <-chan struct{} { return k.gone }

type usage struct {
	bytes   int64
	entries int64
}

// dirUsage sums the apparent size of regular files and counts entries below
// dir without following symlinks. It stops counting once entries passes
// maxEntries (when set), so a tree of millions of entries costs a bounded walk.
func dirUsage(dir string, maxEntries int64) usage {
	var u usage
	_ = filepath.WalkDir(dir, func(_ string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil //nolint:nilerr // a vanished entry is not an error for a size estimate
		}
		u.entries++
		if maxEntries > 0 && u.entries > maxEntries {
			return filepath.SkipAll
		}
		if d.Type().IsRegular() {
			if info, err := d.Info(); err == nil {
				u.bytes += info.Size()
			}
		}
		return nil
	})
	return u
}
