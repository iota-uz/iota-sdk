package manager

import (
	"context"
	"net"
	"os"
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

type fakeProcess struct{ killed chan struct{} }

func newFakeProcess() *fakeProcess { return &fakeProcess{killed: make(chan struct{})} }

func (*fakeProcess) PID() int                 { return 4242 }
func (*fakeProcess) Signal(_ os.Signal) error { return nil }
func (p *fakeProcess) Kill() error {
	select {
	case <-p.killed:
	default:
		close(p.killed)
	}
	return nil
}
func (p *fakeProcess) Wait() error { return nil }

// fakeLauncher returns a fake process so Acquire can spawn a "live" kernel
// without a real Python subprocess. The kernel's bridge reads from a socketpair
// nothing writes to, so the serve loop blocks and the kernel stays live — which
// is all this test needs from a freshly-spawned kernel.
type fakeLauncher struct{}

func (l *fakeLauncher) Launch(_ context.Context, _ isolation.SandboxSpec) (isolation.Process, error) {
	return newFakeProcess(), nil
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
	k := newKernel(testSess(key, t.TempDir()), newFakeProcess(), bridge.New(hostConn), defaultOutputCap, 100*time.Millisecond)
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
