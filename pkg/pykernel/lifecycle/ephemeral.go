package lifecycle

import "github.com/iota-uz/iota-sdk/pkg/pykernel"

// Ephemeral is the per-run policy for the data-migration engine: every Acquire
// spawns a fresh kernel and every Release disposes it. Durable state lives
// host-side, so a kernel is safe to throw away after its run. The background
// sweeper never evicts (the run owns disposal).
func Ephemeral() LifecyclePolicy { return ephemeral{} }

type ephemeral struct{}

func (ephemeral) OnAcquire(string, PoolView) (Decision, error) { return SpawnNew, nil }

func (ephemeral) OnRelease(pykernel.KernelInfo) ReleaseAction { return Dispose }

func (ephemeral) ShouldEvict(pykernel.KernelInfo, PoolView) bool { return false }

func (ephemeral) MaxParallel() int { return 0 } // 0 = unbounded

func (ephemeral) ResetMode() ResetMode { return FreshNamespace }
