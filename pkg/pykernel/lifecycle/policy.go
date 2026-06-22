// Package lifecycle defines how a pykernel.Manager pools, reuses, and evicts
// kernels. It is the single seam that lets one kernel primitive serve two very
// different consumers without forking the manager:
//
//   - WarmPool — per-session reuse with idle eviction and an LRU cap, for the
//     interactive Ali REPL;
//   - Ephemeral — every Acquire spawns and every Release disposes, for the
//     batch data-migration engine, whose durable state lives host-side so a
//     kernel is safe to throw away.
//
// The Manager is policy-agnostic: it consults a LifecyclePolicy for every
// pooling decision and acts on the result. Concrete policies are provided
// alongside the Manager implementation.
package lifecycle

import "github.com/iota-uz/iota-sdk/pkg/pykernel"

// Decision is how OnAcquire wants the Manager to satisfy an Acquire.
type Decision int

const (
	// SpawnNew starts a fresh kernel subprocess.
	SpawnNew Decision = iota
	// ReuseIdle hands back an existing idle kernel matching the key.
	ReuseIdle
)

// ReleaseAction is how OnRelease wants the Manager to treat a released kernel.
type ReleaseAction int

const (
	// Dispose terminates the kernel and frees its slot.
	Dispose ReleaseAction = iota
	// Park returns the kernel to the idle pool for later reuse.
	Park
)

// ResetMode is whether a reused kernel keeps or clears its Python namespace.
type ResetMode int

const (
	// FreshNamespace always executes against a clean namespace.
	FreshNamespace ResetMode = iota
	// KeepNamespace preserves names/state across execs (interactive REPL).
	KeepNamespace
)

// PoolView is a read-only snapshot of the live kernel pool that a policy
// inspects to make eviction and reuse decisions.
type PoolView interface {
	// Len returns the number of live kernels.
	Len() int
	// Infos returns liveness snapshots for all live kernels.
	Infos() []pykernel.KernelInfo
}

// LifecyclePolicy decides how kernels are pooled, reused, evicted, and reset.
// Swapping the policy is the only difference between Ali's warm REPL and the
// migration engine's ephemeral runs. Implementations must be safe for
// concurrent use.
type LifecyclePolicy interface {
	// OnAcquire decides whether to reuse an idle kernel for key or spawn fresh.
	OnAcquire(key string, pool PoolView) (Decision, error)
	// OnRelease decides whether to park or dispose a kernel after its exec
	// completes.
	OnRelease(info pykernel.KernelInfo) ReleaseAction
	// ShouldEvict is consulted by the background sweeper (idle TTL, LRU cap).
	ShouldEvict(info pykernel.KernelInfo, pool PoolView) bool
	// MaxParallel caps the number of concurrently live kernels; the Manager
	// evicts (LRU) or rejects Acquire beyond it.
	MaxParallel() int
	// ResetMode reports whether reused kernels keep or clear their namespace.
	ResetMode() ResetMode
}
