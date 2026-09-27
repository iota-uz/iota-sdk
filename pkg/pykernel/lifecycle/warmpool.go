package lifecycle

import (
	"time"

	"github.com/iota-uz/iota-sdk/pkg/pykernel"
)

// WarmPool is the per-session policy for the interactive Ali REPL: a kernel is
// reused across execs (its namespace persists), parked on release, evicted
// after idleTTL, and the pool is capped at maxParallel live kernels.
func WarmPool(idleTTL time.Duration, maxParallel int) LifecyclePolicy {
	return warmPool{idleTTL: idleTTL, maxParallel: maxParallel}
}

type warmPool struct {
	idleTTL     time.Duration
	maxParallel int
}

func (warmPool) OnAcquire(string, PoolView) (Decision, error) { return ReuseIdle, nil }

func (warmPool) OnRelease(pykernel.KernelInfo) ReleaseAction { return Park }

func (w warmPool) ShouldEvict(info pykernel.KernelInfo, _ PoolView) bool {
	if info.InFlight {
		return false
	}
	return w.idleTTL > 0 && time.Since(info.LastActiveAt) > w.idleTTL
}

func (w warmPool) MaxParallel() int { return w.maxParallel }

func (warmPool) ResetMode() ResetMode { return KeepNamespace }
