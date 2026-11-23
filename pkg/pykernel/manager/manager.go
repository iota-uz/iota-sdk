// Package manager implements pykernel.Manager over sandboxed Python
// subprocesses: it spawns a kernel, wires the host↔kernel bridge, applies a
// lifecycle policy (warm pool or ephemeral), and routes capability calls
// through the central plan/apply authorization.
//
// A Manager is confined by default (isolation.ModeRequired): New verifies with
// isolation.Check that a real child gets every required boundary and fails
// with the list of missing ones otherwise, and every spawned kernel must
// report the same boundaries in its kernel.ready handshake before it is
// handed out.
package manager

import (
	"context"
	"encoding/json"
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
	defaultStartTimeout = 20 * time.Second
	stateDir            = ".pykernel"
	maxSeenKeys         = 4096
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
	// Sandbox is the OS confinement of every kernel. Its zero value is
	// isolation.ModeRequired: New fails closed unless every required boundary
	// is available.
	Sandbox isolation.Profile
	// Limits are the OS resource caps applied to every kernel. For a confined
	// Manager every zero field takes isolation.DefaultLimits.
	Limits isolation.ResourceLimits
	// RuntimeDir is a host-owned directory for the shim, outside every kernel
	// workdir (default: a fresh temporary directory removed on Shutdown).
	RuntimeDir string
	// DefaultOutputCap bounds captured stdout+result bytes per exec (default 50KB).
	DefaultOutputCap int
	// DisposeGrace is how long a kernel may take to stop a cancelled exec
	// before it is SIGKILLed (default 5s).
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
	cfg        Config
	shimPath   string
	ownRuntime bool
	report     isolation.Report

	mu      sync.Mutex
	kernels map[string]*kernel
	seen    map[string]struct{}
	closing bool
	// pending counts spawns that passed the capacity check but are not
	// registered yet; it is part of the MaxParallel count.
	pending int
}

var _ pykernel.Manager = (*Manager)(nil)

// New validates cfg, applies defaults, verifies the sandbox and returns a
// Manager. For a confined Manager it returns an error wrapping
// isolation.ErrIsolationUnavailable, listing every missing boundary, when the
// host cannot confine a kernel completely.
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
	if cfg.Sandbox.StartTimeout == 0 {
		cfg.Sandbox.StartTimeout = defaultStartTimeout
	}
	switch cfg.Sandbox.Mode {
	case isolation.ModeRequired, isolation.ModeInsecureDisabled:
	default:
		// Never fall back to the unconfined path for a value we do not know.
		return nil, fmt.Errorf("pykernel/manager: unknown sandbox mode %v", cfg.Sandbox.Mode)
	}
	confined := cfg.Sandbox.Mode == isolation.ModeRequired
	if confined {
		cfg.Limits = cfg.Limits.WithDefaults()
	}

	m := &Manager{cfg: cfg, kernels: make(map[string]*kernel), seen: make(map[string]struct{})}
	if m.cfg.RuntimeDir == "" {
		dir, err := os.MkdirTemp("", "pykernel-runtime-")
		if err != nil {
			return nil, fmt.Errorf("pykernel/manager: runtime dir: %w", err)
		}
		m.cfg.RuntimeDir, m.ownRuntime = dir, true
	}
	if cfg.Sandbox.RunAs != nil {
		// The kernel uid reads the shim; it must never be able to write it.
		if err := os.Chmod(m.cfg.RuntimeDir, 0o755); err != nil {
			m.cleanup()
			return nil, fmt.Errorf("pykernel/manager: runtime dir: %w", err)
		}
	}
	shim, err := runtime.Write(m.cfg.RuntimeDir)
	if err != nil {
		m.cleanup()
		return nil, err
	}
	m.shimPath = shim

	if confined {
		ctx, cancel := context.WithTimeout(context.Background(), cfg.Sandbox.StartTimeout)
		m.report = isolation.Check(ctx, isolation.CheckOptions{
			PythonPath: cfg.PythonPath, Profile: cfg.Sandbox, Limits: cfg.Limits, Launcher: cfg.Launcher,
		})
		cancel()
		if err := m.report.Err(); err != nil {
			m.cleanup()
			return nil, fmt.Errorf("pykernel/manager: %w", err)
		}
	}
	return m, nil
}

// Isolation returns the sandbox report New verified (empty for an unconfined
// Manager). A consumer enables untrusted execution only when its Err is nil.
func (m *Manager) Isolation() isolation.Report { return m.report }

// Limits returns the resource caps every kernel of this Manager gets.
func (m *Manager) Limits() isolation.ResourceLimits { return m.cfg.Limits }

func (m *Manager) confined() bool { return m.cfg.Sandbox.Mode == isolation.ModeRequired }

func (m *Manager) cleanup() {
	if m.ownRuntime {
		_ = os.RemoveAll(m.cfg.RuntimeDir)
	}
}

func (m *Manager) Acquire(ctx context.Context, sess pykernel.Session) (pykernel.Kernel, error) {
	if err := m.prepareWorkdir(sess.Workdir()); err != nil {
		return nil, err
	}
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
				needReset, err := m.leaseForReuseLocked(k, sess)
				m.mu.Unlock()
				if err != nil {
					return nil, err
				}
				return m.finishReuse(k, needReset)
			}
			// The entry is a dead kernel (its serve loop exited); evict it now so
			// it doesn't linger until Sweep, then fall through to spawn a fresh one.
			// Dispose it asynchronously so its process is reaped, not leaked.
			delete(m.kernels, sess.Key())
			go func(k *kernel) { _ = k.Dispose(context.Background()) }(k)
		}
	}
	if maxParallel := m.cfg.Policy.MaxParallel(); maxParallel > 0 && len(m.kernels)+m.pending >= maxParallel {
		if !m.evictIdleLocked() {
			m.mu.Unlock()
			return nil, pykernel.ErrPoolExhausted
		}
	}
	_, seen := m.seen[sess.Key()]
	m.pending++ // the slot is reserved until the kernel is registered or the spawn fails
	m.mu.Unlock()

	// Spawn outside the lock: Launch + the ready handshake can take time.
	k, err := m.spawn(ctx, sess, seen)
	m.mu.Lock()
	m.pending--
	if err != nil {
		m.mu.Unlock()
		return nil, err
	}
	if m.closing {
		m.mu.Unlock()
		_ = k.Dispose(context.Background())
		return nil, errShuttingDown
	}
	if existing, ok := m.kernels[sess.Key()]; ok {
		if !existing.isDisposed() {
			// Lost a same-key spawn race against a live kernel: keep the established
			// one and discard our freshly-spawned spare. A policy that spawns per
			// Acquire never shares a kernel between two holders.
			var needReset bool
			err := fmt.Errorf("pykernel/manager: key %q is already leased: %w", sess.Key(), pykernel.ErrKernelBusy)
			if decision != lifecycle.SpawnNew {
				needReset, err = m.leaseForReuseLocked(existing, sess)
			}
			m.mu.Unlock()
			_ = k.Dispose(context.Background())
			if err != nil {
				return nil, err
			}
			return m.finishReuse(existing, needReset)
		}
		// The entry is a dead kernel: replace it with our fresh one and dispose
		// the stale entry asynchronously so its process is reaped, not leaked.
		go func(k *kernel) { _ = k.Dispose(context.Background()) }(existing)
	}
	m.kernels[sess.Key()] = k
	if len(m.seen) >= maxSeenKeys {
		m.seen = make(map[string]struct{})
	}
	m.seen[sess.Key()] = struct{}{}
	k.lease()
	m.mu.Unlock()
	return k, nil
}

// leaseForReuseLocked leases a live pooled kernel for sess; caller holds m.mu,
// so the lease is atomic with eviction and Release. The tenant and Mode frozen
// on the kernel at spawn must match: a key is never reused across tenants or
// from a plan run into an apply run. A FreshNamespace kernel is never shared.
// It reports whether the namespace must be reset before the kernel is handed
// out.
func (m *Manager) leaseForReuseLocked(k *kernel, sess pykernel.Session) (bool, error) {
	if k.tenant != sess.TenantID() || k.mode != sess.Mode() {
		return false, pykernel.ErrSessionMismatch
	}
	return k.leaseForReuse(m.cfg.Policy.ResetMode() == lifecycle.FreshNamespace)
}

// finishReuse resets the namespace of a just-leased kernel when required. The
// write happens with no lock held and is bounded: a kernel that does not read
// its control socket is disposed instead of wedging the Manager.
func (m *Manager) finishReuse(k *kernel, needReset bool) (pykernel.Kernel, error) {
	if !needReset {
		return k, nil
	}
	done := make(chan error, 1)
	go func() { done <- k.br.ResetNamespace(context.Background()) }()
	var err error
	select {
	case err = <-done:
	case <-time.After(m.cfg.Sandbox.StartTimeout):
		err = errors.New("kernel does not read its control socket")
	}
	if err != nil {
		m.dropKernel(k)
		return nil, fmt.Errorf("pykernel/manager: reset namespace: %w", err)
	}
	k.markReset()
	return k, nil
}

// dropKernel removes k from the pool and disposes it.
func (m *Manager) dropKernel(k *kernel) {
	m.mu.Lock()
	if m.kernels[k.key] == k {
		delete(m.kernels, k.key)
	}
	m.mu.Unlock()
	_ = k.Dispose(context.Background())
}

// prepareWorkdir creates the workdir and its state directory and, when the
// kernel runs as another uid, hands host-created entries to it.
func (m *Manager) prepareWorkdir(workdir string) error {
	if workdir == "" {
		return errors.New("pykernel/manager: session workdir is empty")
	}
	if n := m.cfg.Limits.WorkdirEntries; n > 0 && dirUsage(workdir, n).entries > n {
		return fmt.Errorf("pykernel/manager: %w", pykernel.ErrWorkdirQuota)
	}
	if err := prepareState(workdir, m.runAs()); err != nil {
		return fmt.Errorf("pykernel/manager: workdir: %w", err)
	}
	return nil
}

func (m *Manager) runAs() *isolation.Credential {
	if m.confined() {
		return m.cfg.Sandbox.RunAs
	}
	return nil
}

func (m *Manager) Get(key string) (pykernel.Kernel, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	k, ok := m.kernels[key]
	if !ok || k.isDisposed() {
		return nil, false
	}
	return k, true
}

func (m *Manager) Release(kk pykernel.Kernel) error {
	k, ok := kk.(*kernel)
	if !ok {
		return errForeignKernel
	}
	// Unlease and the dispose decision happen under m.mu, so a concurrent
	// reuse either leased first (and the kernel stays) or finds it gone.
	m.mu.Lock()
	if k.unlease() > 0 {
		m.mu.Unlock()
		return nil // another holder still uses it
	}
	if m.cfg.Policy.OnRelease(k.Info()) != lifecycle.Dispose {
		m.mu.Unlock()
		return nil // Park: leave it parked in the pool for reuse.
	}
	if m.kernels[k.key] == k {
		delete(m.kernels, k.key)
	}
	m.mu.Unlock()
	return k.Dispose(context.Background())
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
	m.cleanup()
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

// evictIdleLocked disposes one kernel to make room; caller holds mu. It takes
// a kernel the policy's ShouldEvict selects, else, for a PressureEvicter
// policy, the least-recently-used kernel that is neither leased nor running.
func (m *Manager) evictIdleLocked() bool {
	view := poolView{m.snapshotLocked()}
	var lruKey string
	var lru *kernel
	var lruAt time.Time
	for key, k := range m.kernels {
		info := k.Info()
		if info.InFlight || info.Leased {
			continue // never take a kernel from its holder to make room
		}
		if m.cfg.Policy.ShouldEvict(info, view) {
			m.evictLocked(key, k)
			return true
		}
		if lru == nil || info.LastActiveAt.Before(lruAt) {
			lruKey, lru, lruAt = key, k, info.LastActiveAt
		}
	}
	if pe, ok := m.cfg.Policy.(lifecycle.PressureEvicter); ok && pe.EvictLRUOnPressure() && lru != nil {
		m.evictLocked(lruKey, lru)
		return true
	}
	return false
}

func (m *Manager) evictLocked(key string, k *kernel) {
	delete(m.kernels, key)
	go func(k *kernel) { _ = k.Dispose(context.Background()) }(k)
}

type poolView struct{ infos []pykernel.KernelInfo }

func (p poolView) Len() int                     { return len(p.infos) }
func (p poolView) Infos() []pykernel.KernelInfo { return p.infos }

// kernelState is the host-owned record that a kernel ran on a workdir. It
// lives next to the workdir (".<name>.pykernel"), where no kernel can write,
// and names the workdir's inode: a later kernel (after a crash, eviction or
// host restart) reports that its namespace was reset, and that the files were
// preserved only when it is still the same directory.
type kernelState struct {
	StartedAt time.Time `json:"started_at"`
	Dev       uint64    `json:"dev"`
	Ino       uint64    `json:"ino"`
}

func statePath(workdir string) string {
	clean := filepath.Clean(workdir)
	return filepath.Join(filepath.Dir(clean), "."+filepath.Base(clean)+".pykernel")
}

// recordKernel reads the previous record of workdir and replaces it with one
// for the kernel about to start. It returns whether an earlier kernel ran
// there and whether the directory it used still exists.
func recordKernel(workdir string) (ranBefore, sameDir bool, err error) {
	dev, ino, err := dirIdentity(workdir)
	if err != nil {
		return false, false, err
	}
	path := statePath(workdir)
	if raw, rerr := os.ReadFile(path); rerr == nil {
		var prev kernelState
		if json.Unmarshal(raw, &prev) == nil {
			ranBefore = true
			sameDir = prev.Dev == dev && prev.Ino == ino
		}
	}
	raw, err := json.Marshal(kernelState{StartedAt: time.Now().UTC(), Dev: dev, Ino: ino})
	if err != nil {
		return false, false, err
	}
	if _, err := isolation.WriteFileAtomic(filepath.Dir(path), filepath.Base(path), raw); err != nil {
		return false, false, err
	}
	return ranBefore, sameDir, nil
}

func (m *Manager) spawn(ctx context.Context, sess pykernel.Session, seen bool) (*kernel, error) {
	workdir := sess.Workdir()
	ctrl := filepath.Join(workdir, stateDir)
	if err := prepareState(workdir, m.runAs()); err != nil {
		return nil, fmt.Errorf("pykernel/manager: workdir: %w", err)
	}
	ranBefore, filesPreserved, err := recordKernel(workdir)
	if err != nil {
		return nil, fmt.Errorf("pykernel/manager: kernel record: %w", err)
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
		"TMPDIR="+filepath.Join(ctrl, "tmp"),
	)
	env = append(env, m.cfg.ExtraEnv...)

	spec := isolation.SandboxSpec{
		// -s: no user site-packages. HOME is the writable workdir, so a user
		// site there would run code before the next kernel confines itself.
		Command:    []string{m.cfg.PythonPath, "-s", m.shimPath},
		Workdir:    workdir,
		Env:        env,
		Limits:     m.cfg.Limits,
		Stderr:     m.cfg.KernelStderr,
		ExtraFiles: []*os.File{childFile},
	}
	if m.confined() {
		readPaths := append(isolation.DefaultReadPaths(), m.cfg.RuntimeDir)
		spec.Confinement = &isolation.Confinement{
			Workdir:   workdir,
			ReadPaths: append(readPaths, m.cfg.Sandbox.ReadPaths...),
			RunAs:     m.cfg.Sandbox.RunAs,
		}
	}
	proc, err := m.cfg.Launcher.Launch(ctx, spec)
	// The child has inherited the fd; the parent closes its copy regardless.
	_ = childFile.Close()
	if err != nil {
		_ = hostConn.Close()
		return nil, err
	}

	listener := -1
	if m.confined() {
		// The kernel's first byte carries the quota supervisor's listener.
		if listener, err = recvListener(hostConn, m.cfg.Sandbox.StartTimeout); err != nil || listener < 0 {
			_ = proc.Kill()
			_ = hostConn.Close()
			_ = proc.Wait()
			if err == nil {
				err = errors.New("kernel did not hand over the quota supervisor")
			}
			return nil, fmt.Errorf("pykernel/manager: %w: %w", isolation.ErrIsolationUnavailable, err)
		}
	}
	k := newKernel(sess, proc, bridge.New(hostConn), kernelOptions{
		supervised:     m.confined(),
		outputCap:      m.cfg.DefaultOutputCap,
		grace:          m.cfg.DisposeGrace,
		wallClock:      m.cfg.Limits.WallClock,
		workdirQuota:   m.cfg.Limits.WorkdirBytes,
		workdirEntries: m.cfg.Limits.WorkdirEntries,
		workdir:        workdir,
		nsReset:        seen || ranBefore,
		filesPreserved: filesPreserved,
	})
	if m.confined() {
		if err := startSupervisor(k, listener, m.runAs()); err != nil {
			_ = k.Dispose(context.Background())
			return nil, fmt.Errorf("pykernel/manager: %w: quota supervisor: %w", isolation.ErrIsolationUnavailable, err)
		}
	}
	k.start()
	if m.confined() {
		if err := m.awaitReady(ctx, k); err != nil {
			_ = k.Dispose(context.Background())
			return nil, err
		}
	}
	return k, nil
}

// awaitReady waits for the kernel's kernel.ready handshake and refuses a
// kernel whose sandbox report is missing a required boundary.
func (m *Manager) awaitReady(ctx context.Context, k *kernel) error {
	timer := time.NewTimer(m.cfg.Sandbox.StartTimeout)
	defer timer.Stop()
	select {
	case raw := <-k.ready:
		var hello struct {
			Sandbox json.RawMessage `json:"sandbox"`
		}
		if err := json.Unmarshal(raw, &hello); err != nil || len(hello.Sandbox) == 0 || string(hello.Sandbox) == "null" {
			return fmt.Errorf("pykernel/manager: %w: kernel reported no sandbox", isolation.ErrIsolationUnavailable)
		}
		rep, err := isolation.ParseChildReport(hello.Sandbox)
		if err != nil {
			return fmt.Errorf("pykernel/manager: %w", err)
		}
		if err := rep.Err(); err != nil {
			return fmt.Errorf("pykernel/manager: %w", err)
		}
		return nil
	case <-k.serveDone:
		return fmt.Errorf("pykernel/manager: %w: kernel exited before reporting ready", isolation.ErrIsolationUnavailable)
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return fmt.Errorf("pykernel/manager: kernel did not report ready within %s", m.cfg.Sandbox.StartTimeout)
	}
}
