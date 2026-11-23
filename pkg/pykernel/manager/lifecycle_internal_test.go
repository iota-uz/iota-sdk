package manager

import (
	"context"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/pkg/pykernel"
	"github.com/iota-uz/iota-sdk/pkg/pykernel/bridge"
	"github.com/iota-uz/iota-sdk/pkg/pykernel/isolation"
	"github.com/iota-uz/iota-sdk/pkg/pykernel/lifecycle"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// These are white-box tests (package manager) that exercise the disposed-kernel
// lifecycle without spawning real Python: they use a fake launcher/process and a
// real bridge over net.Pipe, so serve-exit can be simulated by closing the pipe.

type fakeProcess struct {
	killed chan struct{}
	// childFile, if set, is this fake process's retained copy of the inherited
	// child socket fd. Holding it open keeps the socketpair peer alive so the
	// spawned kernel's Serve blocks on read (modeling a real inherited child);
	// it is closed when the process is killed/waited.
	childFile *os.File
}

func newFakeProcess() *fakeProcess { return &fakeProcess{killed: make(chan struct{})} }

func (*fakeProcess) PID() int                 { return 4242 }
func (*fakeProcess) Signal(_ os.Signal) error { return nil }
func (p *fakeProcess) Kill() error {
	select {
	case <-p.killed:
	default:
		close(p.killed)
	}
	if p.childFile != nil {
		_ = p.childFile.Close()
	}
	return nil
}
func (p *fakeProcess) Wait() error {
	if p.childFile != nil {
		_ = p.childFile.Close()
	}
	return nil
}

// fakeLauncher returns a fake process so Acquire can spawn a "live" kernel
// without a real Python subprocess. It OWNS the inherited child socket fd
// (spec.ExtraFiles[0]) so that spawn()'s post-Launch close of the parent's copy
// doesn't tear down the socketpair peer: the retained fd keeps the peer open, so
// the spawned kernel's Serve blocks on read and stays live, modeling a real
// inherited child. The fd is released when the process is killed/waited.
type fakeLauncher struct{}

func (l *fakeLauncher) Launch(_ context.Context, spec isolation.SandboxSpec) (isolation.Process, error) {
	p := newFakeProcess()
	if len(spec.ExtraFiles) > 0 && spec.ExtraFiles[0] != nil {
		// Dup the fd so we own a copy independent of the parent's, which spawn()
		// closes right after Launch.
		fd, err := syscall.Dup(int(spec.ExtraFiles[0].Fd()))
		if err != nil {
			return nil, err
		}
		p.childFile = os.NewFile(uintptr(fd), "fake-child-socket")
	}
	return p, nil
}

func testSess(key, wd string) pykernel.Session {
	caps, _ := pykernel.NewCapabilitySet()
	return &fakeSession{key: key, tenant: uuid.New(), caps: caps, mode: pykernel.ModeApply, wd: wd}
}

type fakeSession struct {
	key    string
	tenant uuid.UUID
	caps   pykernel.CapabilitySet
	mode   pykernel.Mode
	wd     string
}

func (s *fakeSession) Key() string                          { return s.key }
func (s *fakeSession) TenantID() uuid.UUID                  { return s.tenant }
func (s *fakeSession) Capabilities() pykernel.CapabilitySet { return s.caps }
func (s *fakeSession) Mode() pykernel.Mode                  { return s.mode }
func (s *fakeSession) Workdir() string                      { return s.wd }

// newPipeKernel builds a kernel whose bridge runs over a net.Pipe, starts its
// serve loop, and returns the kernel plus the kernel-side conn (close it to kill
// the serve loop and trigger onServeExit).
func newPipeKernel(t *testing.T, key string) (*kernel, net.Conn) {
	t.Helper()
	hostConn, kernConn := net.Pipe()
	k := newKernel(testSess(key, t.TempDir()), newFakeProcess(), bridge.New(hostConn), kernelOptions{outputCap: defaultOutputCap, grace: 100 * time.Millisecond})
	k.start()
	return k, kernConn
}

func TestOnServeExit_MarksKernelDisposed(t *testing.T) {
	t.Parallel()

	k, kernConn := newPipeKernel(t, "k1")
	require.False(t, k.isDisposed(), "kernel should start live")

	// Killing the kernel side makes readFrame return EOF, ending Serve and
	// running onServeExit.
	require.NoError(t, kernConn.Close())

	require.Eventually(t, k.isDisposed, time.Second, 5*time.Millisecond,
		"a kernel whose serve loop exited must be marked disposed")
}

func TestAcquire_DoesNotReturnDeadReuseEntry(t *testing.T) {
	t.Parallel()

	m, err := New(Config{
		Policy:       lifecycle.WarmPool(time.Hour, 8), // reuse-idle policy
		Launcher:     &fakeLauncher{},
		Sandbox:      isolation.Profile{Mode: isolation.ModeInsecureDisabled},
		DisposeGrace: 100 * time.Millisecond,
	})
	require.NoError(t, err)
	t.Cleanup(func() { _ = m.Shutdown(context.Background()) })

	// Seed a dead kernel under the reuse key, mimicking a crashed-then-evicted-on-
	// reuse situation.
	dead, kernConn := newPipeKernel(t, "dead-key")
	require.NoError(t, kernConn.Close())
	require.Eventually(t, dead.isDisposed, time.Second, 5*time.Millisecond)

	m.mu.Lock()
	m.kernels["dead-key"] = dead
	m.mu.Unlock()

	// Acquire for the same key must NOT hand back the corpse.
	got, err := m.Acquire(context.Background(), testSess("dead-key", t.TempDir()))
	require.NoError(t, err)
	gotK := got.(*kernel)
	assert.NotSame(t, dead, gotK, "Acquire must not return the dead kernel")
	assert.False(t, gotK.isDisposed(), "Acquire must return a live kernel")

	// And the dead entry must have been evicted from the map (replaced by the new
	// one), not left lingering.
	m.mu.Lock()
	cur := m.kernels["dead-key"]
	m.mu.Unlock()
	assert.Same(t, gotK, cur, "the dead entry must be replaced by the fresh kernel")
}

func newFakeManager(t *testing.T, policy lifecycle.LifecyclePolicy) *Manager {
	t.Helper()
	m, err := New(Config{
		Policy:       policy,
		Launcher:     &fakeLauncher{},
		Sandbox:      isolation.Profile{Mode: isolation.ModeInsecureDisabled},
		DisposeGrace: 100 * time.Millisecond,
	})
	require.NoError(t, err)
	t.Cleanup(func() { _ = m.Shutdown(context.Background()) })
	return m
}

func TestAcquire_EvictsLRUIdleKernelAtCapacity(t *testing.T) {
	t.Parallel()
	m := newFakeManager(t, lifecycle.WarmPool(time.Hour, 2))

	a, err := m.Acquire(context.Background(), testSess("a", t.TempDir()))
	require.NoError(t, err)
	require.NoError(t, m.Release(a))
	time.Sleep(5 * time.Millisecond)
	b, err := m.Acquire(context.Background(), testSess("b", t.TempDir()))
	require.NoError(t, err)
	require.NoError(t, m.Release(b))

	// The pool is full of idle kernels younger than the TTL: the least
	// recently used one (a) makes room — torn down before c starts.
	aProc := a.(*kernel).proc.(*fakeProcess)
	c, err := m.Acquire(context.Background(), testSess("c", t.TempDir()))
	require.NoError(t, err)
	select {
	case <-aProc.killed:
	default:
		t.Fatal("the evicted kernel must be torn down before its replacement is handed out")
	}
	_, aLive := m.Get("a")
	_, bLive := m.Get("b")
	assert.False(t, aLive, "the LRU kernel is evicted")
	assert.True(t, bLive)

	// b is idle but c is leased: only b may go.
	_, err = m.Acquire(context.Background(), testSess("d", t.TempDir()))
	require.NoError(t, err)
	_, cLive := m.Get("c")
	assert.True(t, cLive, "a leased kernel is never evicted for room")

	// Now c and d are both leased: the pool is exhausted.
	_, err = m.Acquire(context.Background(), testSess("e", t.TempDir()))
	require.ErrorIs(t, err, pykernel.ErrPoolExhausted)
	require.NoError(t, m.Release(c))
}

func TestAcquire_RefusesReuseAcrossTenants(t *testing.T) {
	t.Parallel()
	m := newFakeManager(t, lifecycle.WarmPool(time.Hour, 4))
	wd := t.TempDir()
	k, err := m.Acquire(context.Background(), testSess("shared-key", wd))
	require.NoError(t, err)
	require.NoError(t, m.Release(k))

	_, err = m.Acquire(context.Background(), testSess("shared-key", wd)) // testSess mints a new tenant
	require.ErrorIs(t, err, pykernel.ErrSessionMismatch)
}

func TestAcquire_RefusesWorkdirOverEntryQuota(t *testing.T) {
	t.Parallel()
	m, err := New(Config{
		Policy:       lifecycle.Ephemeral(),
		Launcher:     &fakeLauncher{},
		Sandbox:      isolation.Profile{Mode: isolation.ModeInsecureDisabled},
		Limits:       isolation.ResourceLimits{WorkdirEntries: 5},
		DisposeGrace: 100 * time.Millisecond,
	})
	require.NoError(t, err)
	t.Cleanup(func() { _ = m.Shutdown(context.Background()) })
	wd := t.TempDir()
	for i := 0; i < 10; i++ {
		require.NoError(t, os.WriteFile(filepath.Join(wd, fmt.Sprintf("f%d", i)), nil, 0o600))
	}
	_, err = m.Acquire(context.Background(), testSess("full", wd))
	require.ErrorIs(t, err, pykernel.ErrWorkdirQuota)
}

func TestNew_RejectsUnknownSandboxMode(t *testing.T) {
	t.Parallel()
	_, err := New(Config{Policy: lifecycle.Ephemeral(), Sandbox: isolation.Profile{Mode: isolation.Mode(42)}})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown sandbox mode")
}

// slowLauncher holds every launch until release is closed, so concurrent
// Acquires are all in their spawn at once.
type slowLauncher struct {
	fakeLauncher
	started chan struct{}
	release chan struct{}
}

func (l *slowLauncher) Launch(ctx context.Context, spec isolation.SandboxSpec) (isolation.Process, error) {
	l.started <- struct{}{}
	<-l.release
	return l.fakeLauncher.Launch(ctx, spec)
}

func TestAcquire_ConcurrentSpawnsRespectMaxParallel(t *testing.T) {
	t.Parallel()
	l := &slowLauncher{started: make(chan struct{}, 8), release: make(chan struct{})}
	m, err := New(Config{
		Policy: lifecycle.WarmPool(time.Hour, 2), Launcher: l,
		Sandbox: isolation.Profile{Mode: isolation.ModeInsecureDisabled}, DisposeGrace: 100 * time.Millisecond,
	})
	require.NoError(t, err)
	t.Cleanup(func() { _ = m.Shutdown(context.Background()) })

	errs := make(chan error, 4)
	for i := 0; i < 4; i++ {
		go func(i int) {
			_, err := m.Acquire(context.Background(), testSess(fmt.Sprintf("k%d", i), t.TempDir()))
			errs <- err
		}(i)
	}
	<-l.started
	<-l.started
	// Two spawns hold both slots; the other two must be refused, not launched.
	var exhausted int
	for i := 0; i < 2; i++ {
		require.ErrorIs(t, <-errs, pykernel.ErrPoolExhausted)
		exhausted++
	}
	close(l.release)
	for i := 0; i < 2; i++ {
		require.NoError(t, <-errs)
	}
	assert.Equal(t, 2, exhausted)
	assert.Len(t, l.started, 0, "no third launch happened")
}

func TestSpawn_RestartRecordIsHostOwned(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	wd := filepath.Join(root, "session")
	m := newFakeManager(t, lifecycle.Ephemeral())

	k, err := m.Acquire(context.Background(), testSess("s1", wd))
	require.NoError(t, err)
	assert.False(t, k.Info().NamespaceReset)
	require.NoError(t, m.Release(k))
	_, err = os.Stat(filepath.Join(root, ".session.pykernel"))
	require.NoError(t, err, "the record lives next to the workdir, out of the kernel's reach")

	// The kernel wiping its own state directory changes nothing.
	require.NoError(t, os.RemoveAll(filepath.Join(wd, stateDir)))
	k, err = m.Acquire(context.Background(), testSess("s2", wd))
	require.NoError(t, err)
	assert.True(t, k.Info().NamespaceReset)
	assert.True(t, k.Info().FilesPreserved)
	require.NoError(t, m.Release(k))

	// A recreated workdir is a reset without preserved files.
	require.NoError(t, os.RemoveAll(wd))
	k, err = m.Acquire(context.Background(), testSess("s3", wd))
	require.NoError(t, err)
	assert.True(t, k.Info().NamespaceReset)
	assert.False(t, k.Info().FilesPreserved)
	require.NoError(t, m.Release(k))
}

// closingProcess is a fake process whose kill closes the kernel's end of the
// control channel, as a real dying process does.
type closingProcess struct {
	*fakeProcess
	conn net.Conn
}

func (p *closingProcess) Kill() error {
	_ = p.conn.Close()
	return p.fakeProcess.Kill()
}

func TestExec_StoppedSubmissionReportsItsOutcome(t *testing.T) {
	t.Parallel()
	hostConn, kernConn := net.Pipe() // nobody reads the kernel side: Submit blocks
	proc := &closingProcess{fakeProcess: newFakeProcess(), conn: kernConn}
	k := newKernel(testSess("stuck", t.TempDir()), proc, bridge.New(hostConn),
		kernelOptions{outputCap: defaultOutputCap, grace: 50 * time.Millisecond})
	k.start()
	t.Cleanup(func() { _ = k.Dispose(context.Background()) })

	ctx, cancel := context.WithCancel(context.Background())
	time.AfterFunc(100*time.Millisecond, cancel)
	ch, err := k.Exec(ctx, pykernel.ExecRequest{Code: "1"})
	require.NoError(t, err, "a stopped submission is an outcome, not a transport error")
	var done *pykernel.DonePayload
	for ev := range ch {
		if ev.Kind == pykernel.EventDone {
			done = ev.Done
		}
	}
	require.NotNil(t, done)
	assert.Equal(t, pykernel.ReasonCancelled, done.Reason)
}

// lingeringProcess does not finish Wait until released, like a process group
// still dying after its kill.
type lingeringProcess struct {
	*fakeProcess
	release chan struct{}
}

func (p *lingeringProcess) Wait() error {
	<-p.release
	return p.fakeProcess.Wait()
}

type lingeringLauncher struct {
	fakeLauncher
	procs chan *lingeringProcess
}

func (l *lingeringLauncher) Launch(ctx context.Context, spec isolation.SandboxSpec) (isolation.Process, error) {
	base, err := l.fakeLauncher.Launch(ctx, spec)
	if err != nil {
		return nil, err
	}
	p := &lingeringProcess{fakeProcess: base.(*fakeProcess), release: make(chan struct{})}
	l.procs <- p
	return p, nil
}

func TestAcquire_ReplacementWaitsForTheEvictedGroupToEnd(t *testing.T) {
	t.Parallel()
	l := &lingeringLauncher{procs: make(chan *lingeringProcess, 4)}
	m, err := New(Config{
		Policy: lifecycle.WarmPool(time.Hour, 1), Launcher: l,
		Sandbox: isolation.Profile{Mode: isolation.ModeInsecureDisabled}, DisposeGrace: 50 * time.Millisecond,
	})
	require.NoError(t, err)
	t.Cleanup(func() {
		for {
			select {
			case p := <-l.procs:
				close(p.release)
			default:
				return
			}
		}
	})
	a, err := m.Acquire(context.Background(), testSess("a", t.TempDir()))
	require.NoError(t, err)
	first := <-l.procs
	require.NoError(t, m.Release(a))

	got := make(chan error, 1)
	go func() {
		_, err := m.Acquire(context.Background(), testSess("b", t.TempDir()))
		got <- err
	}()
	select {
	case err := <-got:
		t.Fatalf("the replacement started while the evicted group was alive (err=%v)", err)
	case <-time.After(300 * time.Millisecond):
	}
	close(first.release)
	require.NoError(t, <-got)
}

func TestExec_KernelDyingDuringSubmitReportsKernelExited(t *testing.T) {
	t.Parallel()
	hostConn, kernConn := net.Pipe() // the kernel never reads, then dies
	k := newKernel(testSess("dying", t.TempDir()), newFakeProcess(), bridge.New(hostConn),
		kernelOptions{outputCap: defaultOutputCap, grace: 50 * time.Millisecond})
	k.start()
	t.Cleanup(func() { _ = k.Dispose(context.Background()) })
	time.AfterFunc(100*time.Millisecond, func() { _ = kernConn.Close() })

	ch, err := k.Exec(context.Background(), pykernel.ExecRequest{Code: "1"})
	require.NoError(t, err, "a dead kernel is an outcome, not a transport error")
	var done *pykernel.DonePayload
	for ev := range ch {
		if ev.Kind == pykernel.EventDone {
			done = ev.Done
		}
	}
	require.NotNil(t, done)
	assert.Equal(t, pykernel.ReasonKernelExited, done.Reason)
}
