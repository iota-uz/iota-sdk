package pykernel

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
)

// Session is the host-bound execution context for a kernel lease. Every
// security-relevant field is set host-side from the authenticated session/run
// and is immutable for the life of the lease; the kernel never supplies any of
// them (see the package doc's host-binds-context invariant).
type Session interface {
	// Key is the pool identity — an Ali chat-session id (warm reuse) or a
	// migration run id (ephemeral). Warm-pool keys must be tenant-scoped so a
	// kernel is never reused across tenants.
	Key() string
	// TenantID scopes every capability call; bound from the session.
	TenantID() uuid.UUID
	// Capabilities is the frozen set of host functions exposed into Python.
	Capabilities() CapabilitySet
	// Mode is plan vs apply; write capabilities are refused in plan mode.
	Mode() Mode
	// Workdir is the per-session/per-run scratch directory on a persistent
	// volume. It survives a kernel respawn even though the namespace does not.
	Workdir() string
}

// Manager owns the lifecycle of Python kernel subprocesses and is the single
// entry point both consumers share. Implementations are safe for concurrent
// use. The Manager binds no tenant/permission context itself — that travels on
// the Session passed to Acquire.
type Manager interface {
	// Acquire returns a Kernel bound to sess. Whether it reuses an idle kernel
	// keyed by sess.Key() or spawns a fresh one is decided by the lifecycle
	// policy. The capability set and Mode in sess are frozen onto the kernel and
	// cannot be escalated for the life of the lease. The returned Kernel is
	// leased: the caller releases it (warm policy) or disposes it (ephemeral).
	Acquire(ctx context.Context, sess Session) (Kernel, error)
	// Get returns the already-acquired kernel for key, or (nil, false). A host
	// reconciler uses it to find an in-flight kernel after a crash or redeploy.
	Get(key string) (Kernel, bool)
	// Release returns a kernel to the pool (warm policy) or disposes it
	// (ephemeral policy).
	Release(k Kernel) error
	// Evict forcibly terminates and removes the kernel for key (idle timeout,
	// LRU cap, tenant teardown). It is safe to call on an unknown key.
	Evict(key string) error
	// Shutdown drains the pool: it refuses new Acquire calls, lets in-flight
	// execs run up to a grace period, then kills survivors. Wire it to SIGTERM.
	Shutdown(ctx context.Context) error
}

// Kernel is a handle to one warm Python subprocess. At most one Exec runs at a
// time per Kernel; a concurrent Exec returns ErrKernelBusy.
type Kernel interface {
	// Exec runs code in the kernel's persistent namespace and streams events
	// (stdout chunks, partial results, metrics, logs, the final result, errors)
	// until the channel closes with a terminal EventDone. The namespace survives
	// across Exec calls; the workdir survives across kernel respawns.
	Exec(ctx context.Context, req ExecRequest) (<-chan ExecEvent, error)
	// Info reports liveness, last activity, and whether the namespace was reset
	// since the consumer last saw it (the post-respawn "namespace cleared, files
	// intact" signal surfaced to the model).
	Info() KernelInfo
	// Dispose terminates the subprocess and frees its slot. It is idempotent.
	Dispose(ctx context.Context) error
}

// KernelInfo is a liveness snapshot of a kernel.
type KernelInfo struct {
	// Key is the kernel's pool identity (the Session key it was acquired with).
	Key string
	// PID is the subprocess id, or 0 if not yet spawned / already disposed.
	PID int
	// LastActiveAt is when the kernel last started or finished an exec.
	LastActiveAt time.Time
	// NamespaceReset is true after a respawn until the consumer acknowledges it.
	NamespaceReset bool
	// InFlight is true while an Exec is running.
	InFlight bool
}

// Sentinel errors callers branch on.
var (
	// ErrKernelBusy is returned by Exec when an exec is already in flight.
	ErrKernelBusy = errors.New("pykernel: kernel has an in-flight exec")
	// ErrPoolExhausted is returned by Acquire when the parallel-kernel cap is
	// reached and no kernel can be evicted to make room.
	ErrPoolExhausted = errors.New("pykernel: parallel-kernel cap reached")
	// ErrKernelDisposed is returned when operating on a disposed kernel.
	ErrKernelDisposed = errors.New("pykernel: kernel disposed")
)
