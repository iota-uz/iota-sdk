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

// CallContextBinder is an optional Session extension for host-bound identity
// beyond the tenant, such as the chat user. The Manager freezes the binder of
// the Session a kernel is spawned for and applies it to the context of every
// capability call of that kernel, next to the tenant, so a capability can
// re-check the user's current permissions on each call. Bind an identity, not
// a permission snapshot: a change of the user's rights then takes effect in a
// warm session at its next call. A Session that binds a user must use a
// user-scoped Key, so a kernel is never reused for another user.
type CallContextBinder interface {
	BindCallContext(ctx context.Context) context.Context
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
	// NamespaceReset is true when the Python namespace the next exec sees is
	// not the one an earlier kernel of this session left: the kernel was
	// respawned (crash, eviction, host restart) or reset for reuse. The next
	// Exec acknowledges it and reports it in its DonePayload.
	NamespaceReset bool
	// FilesPreserved is true when the session workdir already held the state
	// of an earlier kernel when this one started, so files written by earlier
	// execs are intact even though the namespace is not. Results of earlier
	// execs are never recomputed by re-running their code.
	FilesPreserved bool
	// InFlight is true while an Exec is running.
	InFlight bool
	// Leased is true between Acquire and Release; the Manager never evicts a
	// leased kernel to make room for another one (an idle-TTL Sweep still
	// may, so a holder that never releases cannot pin a kernel forever).
	Leased bool
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
	// ErrWorkdirQuota is returned by Acquire when the session workdir holds
	// more entries than the Manager allows; the host must clean it up.
	ErrWorkdirQuota = errors.New("pykernel: session workdir is over its entry quota")
	// ErrRequestTooLarge is returned by Exec when the code (with its limits)
	// does not fit one bridge frame. Nothing was sent; the kernel is intact.
	ErrRequestTooLarge = errors.New("pykernel: exec request exceeds the maximum frame size")
	// ErrSessionMismatch is returned by Acquire when a live kernel for the key
	// was bound to a different tenant or Mode; keys must be scoped so that
	// never happens.
	ErrSessionMismatch = errors.New("pykernel: session key is bound to another tenant or mode")
)
