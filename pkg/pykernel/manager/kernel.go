package manager

import (
	"context"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/pkg/pykernel"
	"github.com/iota-uz/iota-sdk/pkg/pykernel/bridge"
	"github.com/iota-uz/iota-sdk/pkg/pykernel/isolation"
)

// kernel is a live Python subprocess plus its bridge. It implements
// pykernel.Kernel and serves as the bridge.EventSink that fans kernel output
// into the active Exec's event channel.
type kernel struct {
	key       string
	proc      isolation.Process
	br        bridge.Bridge
	disp      *dispatcher
	outputCap int
	grace     time.Duration

	serveCancel context.CancelFunc
	closed      chan struct{}
	closeOnce   sync.Once

	mu           sync.Mutex
	disposed     bool
	inFlight     bool
	nsReset      bool
	lastActive   time.Time
	pid          int
	activeExecID string
	activeCh     chan pykernel.ExecEvent
	activeDone   chan struct{}
}

var (
	_ pykernel.Kernel  = (*kernel)(nil)
	_ bridge.EventSink = (*kernel)(nil)
)

func newKernel(sess pykernel.Session, proc isolation.Process, br bridge.Bridge, outputCap int, grace time.Duration) *kernel {
	return &kernel{
		key:        sess.Key(),
		proc:       proc,
		br:         br,
		disp:       &dispatcher{caps: sess.Capabilities(), mode: sess.Mode(), tenant: sess.TenantID()},
		outputCap:  outputCap,
		grace:      grace,
		closed:     make(chan struct{}),
		lastActive: time.Now(),
		pid:        proc.PID(),
	}
}

// start runs the bridge read loop for the kernel's lifetime.
func (k *kernel) start() {
	ctx, cancel := context.WithCancel(context.Background())
	k.serveCancel = cancel
	go func() {
		_ = k.br.Serve(ctx, k.disp, k)
		k.onServeExit()
	}()
}

func (k *kernel) Info() pykernel.KernelInfo {
	k.mu.Lock()
	defer k.mu.Unlock()
	return pykernel.KernelInfo{
		Key:            k.key,
		PID:            k.pid,
		LastActiveAt:   k.lastActive,
		NamespaceReset: k.nsReset,
		InFlight:       k.inFlight,
	}
}

func (k *kernel) isDisposed() bool {
	k.mu.Lock()
	defer k.mu.Unlock()
	return k.disposed
}

func (k *kernel) Exec(ctx context.Context, req pykernel.ExecRequest) (<-chan pykernel.ExecEvent, error) {
	k.mu.Lock()
	if k.disposed {
		k.mu.Unlock()
		return nil, pykernel.ErrKernelDisposed
	}
	if k.inFlight {
		k.mu.Unlock()
		return nil, pykernel.ErrKernelBusy
	}
	execID := uuid.NewString()
	ch := make(chan pykernel.ExecEvent, 256)
	done := make(chan struct{})
	k.activeExecID = execID
	k.activeCh = ch
	k.activeDone = done
	k.inFlight = true
	k.lastActive = time.Now()
	k.mu.Unlock()

	outCap := req.OutputCap
	if outCap == 0 {
		outCap = k.outputCap
	}
	limits := bridge.Limits{OutputCap: outCap}
	if req.Timeout > 0 {
		limits.WallClockMS = req.Timeout.Milliseconds()
	}

	if err := k.br.Submit(ctx, execID, req.Code, limits); err != nil {
		k.mu.Lock()
		k.finishLocked(nil)
		k.mu.Unlock()
		return nil, err
	}

	go k.watch(ctx, execID, req.Timeout, done)
	return ch, nil
}

// Emit receives a kernel output notification (runs in the bridge read loop) and
// fans it into the active exec's channel, closing the channel on EventDone.
func (k *kernel) Emit(execID string, ev bridge.RawEvent) {
	out, terminal := translate(ev)

	k.mu.Lock()
	if k.activeCh == nil || execID != k.activeExecID {
		k.mu.Unlock()
		return
	}
	ch := k.activeCh
	k.mu.Unlock()

	// Buffered send with a closing escape hatch so an abandoned consumer can't
	// wedge the read loop.
	select {
	case ch <- out:
	case <-k.closed:
		return
	}

	if terminal {
		k.mu.Lock()
		if k.activeExecID == execID {
			k.finishLocked(nil)
		}
		k.mu.Unlock()
	}
}

// finishLocked closes the active exec channel (optionally emitting a final
// synthetic event first) and clears in-flight state. Caller holds k.mu. The
// channel is only ever closed here, and only from the bridge goroutine paths
// (Emit / onServeExit) or the Submit-failure path, so it is closed at most once.
func (k *kernel) finishLocked(synthetic *pykernel.ExecEvent) {
	if k.activeCh == nil {
		return
	}
	if synthetic != nil {
		select {
		case k.activeCh <- *synthetic:
		default:
		}
	}
	close(k.activeCh)
	k.activeCh = nil
	k.activeExecID = ""
	k.inFlight = false
	k.lastActive = time.Now()
	if k.activeDone != nil {
		close(k.activeDone)
		k.activeDone = nil
	}
}

// watch enforces ctx cancellation and the wall-clock timeout: cooperative
// cancel first, then SIGKILL after the dispose grace if the exec hasn't ended.
func (k *kernel) watch(ctx context.Context, execID string, timeout time.Duration, done <-chan struct{}) {
	var timer <-chan time.Time
	if timeout > 0 {
		t := time.NewTimer(timeout)
		defer t.Stop()
		timer = t.C
	}
	select {
	case <-done:
		return
	case <-k.closed:
		return
	case <-ctx.Done():
	case <-timer:
	}
	// ctx cancelled or timed out: cooperative cancel, then escalate to kill.
	_ = k.br.Cancel(context.Background(), execID)
	select {
	case <-done:
	case <-k.closed:
	case <-time.After(k.grace):
		_ = k.proc.Kill()
	}
}

// onServeExit runs after the bridge read loop returns (kernel died or was
// disposed). The kernel is marked disposed so the pool never hands back this
// corpse on a later Acquire. If an exec was still in flight, the consumer is
// unblocked with a synthetic error + closed channel.
func (k *kernel) onServeExit() {
	k.mu.Lock()
	defer k.mu.Unlock()
	k.disposed = true
	if k.activeCh != nil {
		k.finishLocked(&pykernel.ExecEvent{
			Kind:      pykernel.EventError,
			Err:       &pykernel.ExecError{Type: "KernelExited", Message: "kernel process exited before completing the exec"},
			Timestamp: time.Now(),
		})
	}
}

func (k *kernel) Dispose(_ context.Context) error {
	k.mu.Lock()
	if k.disposed {
		k.mu.Unlock()
		return nil
	}
	k.disposed = true
	k.mu.Unlock()

	// Unblock any in-flight Emit send, stop the read loop, and close the bridge
	// — closing the control socket makes the shim exit cleanly on EOF.
	k.closeOnce.Do(func() { close(k.closed) })
	if k.serveCancel != nil {
		k.serveCancel()
	}
	_ = k.br.Close()

	waitDone := make(chan struct{})
	go func() {
		_ = k.proc.Wait()
		close(waitDone)
	}()
	select {
	case <-waitDone:
	case <-time.After(k.grace):
		_ = k.proc.Kill()
		<-waitDone
	}
	return nil
}
