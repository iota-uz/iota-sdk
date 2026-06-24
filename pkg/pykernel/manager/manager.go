// Package manager implements pykernel.Manager over sandboxed Python
// subprocesses: it spawns a kernel, wires the host↔kernel bridge, applies a
// lifecycle policy (warm pool or ephemeral), and routes capability calls
// through the central plan/apply authorization.
package manager

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/iota-uz/iota-sdk/pkg/pykernel"
	"github.com/iota-uz/iota-sdk/pkg/pykernel/bridge"
	"github.com/iota-uz/iota-sdk/pkg/pykernel/isolation"
	"github.com/iota-uz/iota-sdk/pkg/pykernel/lifecycle"
	"github.com/iota-uz/iota-sdk/pkg/pykernel/runtime"
)

const (
	defaultPython       = "python3"
	defaultOutputCap    = 50 * 1024
	defaultDisposeGrace = 5 * time.Second
)

var (
	errShuttingDown  = errors.New("pykernel/manager: manager is shutting down")
	errForeignKernel = errors.New("pykernel/manager: kernel not owned by this manager")
)

// Config configures a Manager. Policy is required; the rest have defaults.
type Config struct {
	// PythonPath is the interpreter to launch (default "python3").
	PythonPath string
	// Launcher launches the sandbox (default isolation.NewLauncher()).
	Launcher isolation.Launcher
	// Policy decides pooling/eviction (lifecycle.WarmPool or lifecycle.Ephemeral).
	Policy lifecycle.LifecyclePolicy
	// Limits are the OS resource caps applied to every kernel.
	Limits isolation.ResourceLimits
	// DefaultOutputCap bounds captured stdout+result bytes per exec (default 50KB).
	DefaultOutputCap int
	// DisposeGrace is how long a kernel may take to exit on dispose before
	// it is SIGKILLed (default 5s).
	DisposeGrace time.Duration
	// EnvAllowlist names host environment variables passed through to the
	// kernel (e.g. PATH, LANG). Secrets must NOT be listed.
	EnvAllowlist []string
	// ExtraEnv adds fixed, non-secret KEY=VALUE entries to the kernel env.
	ExtraEnv []string
	// KernelStderr, if set, receives kernel-process stderr (debugging only).
	KernelStderr io.Writer
}

// Manager owns the lifecycle of Python kernel subprocesses.
type Manager struct {
	cfg     Config
	mu      sync.Mutex
	kernels map[string]*kernel
	closing bool
}

var _ pykernel.Manager = (*Manager)(nil)

// New validates cfg, applies defaults, and returns a Manager.
func New(cfg Config) (*Manager, error) {
	if cfg.Policy == nil {
		return nil, errors.New("pykernel/manager: Policy is required")
	}
	if cfg.PythonPath == "" {
		cfg.PythonPath = defaultPython
	}
	if cfg.Launcher == nil {
		cfg.Launcher = isolation.NewLauncher()
	}
	if cfg.DefaultOutputCap == 0 {
		cfg.DefaultOutputCap = defaultOutputCap
	}
	if cfg.DisposeGrace == 0 {
		cfg.DisposeGrace = defaultDisposeGrace
	}
	return &Manager{cfg: cfg, kernels: make(map[string]*kernel)}, nil
}

func (m *Manager) Acquire(ctx context.Context, sess pykernel.Session) (pykernel.Kernel, error) {
	m.mu.Lock()
	if m.closing {
		m.mu.Unlock()
		return nil, errShuttingDown
	}
	decision, err := m.cfg.Policy.OnAcquire(sess.Key(), poolView{m.snapshotLocked()})
	if err != nil {
		m.mu.Unlock()
		return nil, err
	}
	if decision == lifecycle.ReuseIdle {
		if k, ok := m.kernels[sess.Key()]; ok {
			if !k.isDisposed() {
				m.mu.Unlock()
				return k, nil
			}
			// The entry is a dead kernel (its serve loop exited); evict it now so
			// it doesn't linger until Sweep, then fall through to spawn a fresh one.
			delete(m.kernels, sess.Key())
		}
	}
	if maxParallel := m.cfg.Policy.MaxParallel(); maxParallel > 0 && len(m.kernels) >= maxParallel {
		if !m.evictIdleLocked() {
			m.mu.Unlock()
			return nil, pykernel.ErrPoolExhausted
		}
	}
	m.mu.Unlock()

	// Spawn outside the lock: Launch + Accept can take time.
	k, err := m.spawn(ctx, sess)
	if err != nil {
		return nil, err
	}

	m.mu.Lock()
	if m.closing {
		m.mu.Unlock()
		_ = k.Dispose(context.Background())
		return nil, errShuttingDown
	}
	if existing, ok := m.kernels[sess.Key()]; ok && !existing.isDisposed() {
		// Lost a same-key spawn race against a live kernel: keep the established
		// one and discard our freshly-spawned spare.
		m.mu.Unlock()
		_ = k.Dispose(context.Background())
		return existing, nil
	}
	m.kernels[sess.Key()] = k
	m.mu.Unlock()
	return k, nil
}

func (m *Manager) Get(key string) (pykernel.Kernel, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	k, ok := m.kernels[key]
	if !ok {
		return nil, false
	}
	return k, true
}

func (m *Manager) Release(kk pykernel.Kernel) error {
	k, ok := kk.(*kernel)
	if !ok {
		return errForeignKernel
	}
	if m.cfg.Policy.OnRelease(k.Info()) == lifecycle.Dispose {
		m.mu.Lock()
		if m.kernels[k.key] == k {
			delete(m.kernels, k.key)
		}
		m.mu.Unlock()
		return k.Dispose(context.Background())
	}
	return nil // Park: leave it parked in the pool for reuse.
}

func (m *Manager) Evict(key string) error {
	m.mu.Lock()
	k, ok := m.kernels[key]
	if ok {
		delete(m.kernels, key)
	}
	m.mu.Unlock()
	if !ok {
		return nil
	}
	return k.Dispose(context.Background())
}

func (m *Manager) Shutdown(ctx context.Context) error {
	m.mu.Lock()
	m.closing = true
	victims := make([]*kernel, 0, len(m.kernels))
	for key, k := range m.kernels {
		victims = append(victims, k)
		delete(m.kernels, key)
	}
	m.mu.Unlock()
	for _, k := range victims {
		_ = k.Dispose(ctx)
	}
	return nil
}

// Sweep disposes kernels the policy considers evictable (idle TTL, etc.).
// A periodic task can call it; it is a no-op for the ephemeral policy.
func (m *Manager) Sweep() {
	m.mu.Lock()
	view := poolView{m.snapshotLocked()}
	var victims []*kernel
	for key, k := range m.kernels {
		if m.cfg.Policy.ShouldEvict(k.Info(), view) {
			victims = append(victims, k)
			delete(m.kernels, key)
		}
	}
	m.mu.Unlock()
	for _, k := range victims {
		_ = k.Dispose(context.Background())
	}
}

func (m *Manager) snapshotLocked() []pykernel.KernelInfo {
	out := make([]pykernel.KernelInfo, 0, len(m.kernels))
	for _, k := range m.kernels {
		out = append(out, k.Info())
	}
	return out
}

// evictIdleLocked disposes one evictable kernel to make room; caller holds mu.
func (m *Manager) evictIdleLocked() bool {
	view := poolView{m.snapshotLocked()}
	for key, k := range m.kernels {
		if m.cfg.Policy.ShouldEvict(k.Info(), view) {
			delete(m.kernels, key)
			go func(k *kernel) { _ = k.Dispose(context.Background()) }(k)
			return true
		}
	}
	return false
}

type poolView struct{ infos []pykernel.KernelInfo }

func (p poolView) Len() int                     { return len(p.infos) }
func (p poolView) Infos() []pykernel.KernelInfo { return p.infos }

func (m *Manager) spawn(ctx context.Context, sess pykernel.Session) (*kernel, error) {
	workdir := sess.Workdir()
	if workdir == "" {
		return nil, errors.New("pykernel/manager: session workdir is empty")
	}
	ctrlDir := filepath.Join(workdir, ".pykernel")
	shimPath, err := runtime.Write(ctrlDir)
	if err != nil {
		return nil, err
	}

	capsJSON, err := capSpecsJSON(sess.Capabilities())
	if err != nil {
		return nil, err
	}

	// The control socket is an inherited fd (fd 3 in the child), not a path.
	hostConn, childFile, err := socketPair()
	if err != nil {
		return nil, fmt.Errorf("pykernel/manager: socketpair: %w", err)
	}

	env := append([]string{}, isolation.AllowedEnv(m.cfg.EnvAllowlist...)...)
	env = append(env,
		"PYKERNEL_SOCKET_FD=3",
		"PYKERNEL_CAPABILITIES="+string(capsJSON),
		"HOME="+workdir,
	)
	env = append(env, m.cfg.ExtraEnv...)

	proc, err := m.cfg.Launcher.Launch(ctx, isolation.SandboxSpec{
		Command:    []string{m.cfg.PythonPath, shimPath},
		Workdir:    workdir,
		Env:        env,
		Limits:     m.cfg.Limits,
		Stderr:     m.cfg.KernelStderr,
		ExtraFiles: []*os.File{childFile},
	})
	// The child has inherited the fd; the parent closes its copy regardless.
	_ = childFile.Close()
	if err != nil {
		_ = hostConn.Close()
		return nil, err
	}

	k := newKernel(sess, proc, bridge.New(hostConn), m.cfg.DefaultOutputCap, m.cfg.DisposeGrace)
	k.start()
	return k, nil
}
