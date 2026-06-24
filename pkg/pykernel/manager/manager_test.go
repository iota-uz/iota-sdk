package manager_test

import (
	"context"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/pykernel"
	"github.com/iota-uz/iota-sdk/pkg/pykernel/lifecycle"
	"github.com/iota-uz/iota-sdk/pkg/pykernel/manager"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type testSession struct {
	key    string
	tenant uuid.UUID
	caps   pykernel.CapabilitySet
	mode   pykernel.Mode
	wd     string
}

func (s *testSession) Key() string                          { return s.key }
func (s *testSession) TenantID() uuid.UUID                  { return s.tenant }
func (s *testSession) Capabilities() pykernel.CapabilitySet { return s.caps }
func (s *testSession) Mode() pykernel.Mode                  { return s.mode }
func (s *testSession) Workdir() string                      { return s.wd }

func requirePython(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("python3"); err != nil {
		t.Skip("python3 not available")
	}
}

func newManager(t *testing.T) *manager.Manager {
	t.Helper()
	m, err := manager.New(manager.Config{
		Policy:       lifecycle.Ephemeral(),
		EnvAllowlist: []string{"PATH"},
		DisposeGrace: 2 * time.Second,
	})
	require.NoError(t, err)
	t.Cleanup(func() { _ = m.Shutdown(context.Background()) })
	return m
}

// drain collects events until the channel closes or the deadline passes.
func drain(t *testing.T, ch <-chan pykernel.ExecEvent, d time.Duration) (string, string, bool, *pykernel.ExecError) {
	t.Helper()
	var sb strings.Builder
	var result string
	var gotDone bool
	var errEv *pykernel.ExecError
	deadline := time.After(d)
	for {
		select {
		case ev, ok := <-ch:
			if !ok {
				return sb.String(), result, gotDone, errEv
			}
			switch ev.Kind {
			case pykernel.EventStdout:
				sb.Write(ev.Stdout)
			case pykernel.EventResult:
				result = ev.Result.Text
			case pykernel.EventError:
				errEv = ev.Err
			case pykernel.EventDone:
				gotDone = true
			case pykernel.EventMetric, pykernel.EventLog:
				// Not asserted by these tests; ignore.
			}
		case <-deadline:
			t.Fatal("timed out draining exec events")
		}
	}
}

func TestManager_ExecStreamsStdoutResultAndCapability(t *testing.T) {
	requirePython(t)

	var sawTenant uuid.UUID
	tenant := uuid.New()
	echo := pykernel.CapabilityFunc("echo", pykernel.AccessRead,
		pykernel.CapabilitySignature{Params: []pykernel.ParamSpec{{Name: "msg", Type: "str"}}},
		func(ctx context.Context, args pykernel.CallArgs) (any, error) {
			if tid, err := composables.UseTenantID(ctx); err == nil {
				sawTenant = tid
			}
			return args["msg"], nil
		})
	caps, err := pykernel.NewCapabilitySet(echo)
	require.NoError(t, err)

	m := newManager(t)
	sess := &testSession{key: "run-1", tenant: tenant, caps: caps, mode: pykernel.ModeApply, wd: t.TempDir()}
	k, err := m.Acquire(context.Background(), sess)
	require.NoError(t, err)

	ch, err := k.Exec(context.Background(), pykernel.ExecRequest{
		Code: "print('hello')\nr = echo(msg='world')\nprint(r)\nr",
	})
	require.NoError(t, err)

	stdout, result, gotDone, errEv := drain(t, ch, 20*time.Second)
	require.Nil(t, errEv, "exec should not error")
	assert.True(t, gotDone, "should receive EventDone")
	assert.Contains(t, stdout, "hello")
	assert.Contains(t, stdout, "world")
	assert.Equal(t, "'world'", result, "final expression repr should be returned")
	assert.Equal(t, tenant, sawTenant, "tenant must be bound host-side into the capability context")

	require.NoError(t, m.Release(k)) // ephemeral → dispose
}

func TestManager_PlanModeRefusesWrite(t *testing.T) {
	requirePython(t)

	invoked := false
	write := pykernel.CapabilityFunc("do_write", pykernel.AccessWrite,
		pykernel.CapabilitySignature{},
		func(context.Context, pykernel.CallArgs) (any, error) {
			invoked = true
			return "wrote", nil
		})
	caps, err := pykernel.NewCapabilitySet(write)
	require.NoError(t, err)

	m := newManager(t)
	sess := &testSession{key: "run-2", tenant: uuid.New(), caps: caps, mode: pykernel.ModePlan, wd: t.TempDir()}
	k, err := m.Acquire(context.Background(), sess)
	require.NoError(t, err)

	ch, err := k.Exec(context.Background(), pykernel.ExecRequest{
		Code: "try:\n    do_write()\nexcept Exception as e:\n    print(type(e).__name__)",
	})
	require.NoError(t, err)

	stdout, _, gotDone, errEv := drain(t, ch, 20*time.Second)
	require.Nil(t, errEv)
	assert.True(t, gotDone)
	assert.Contains(t, stdout, "PlanModeViolation", "a write in plan mode must raise PlanModeViolation in the kernel")
	assert.False(t, invoked, "the host capability handler must never run for a refused write")

	require.NoError(t, m.Release(k))
}

func TestManager_CancelTerminatesExec(t *testing.T) {
	requirePython(t)

	caps, err := pykernel.NewCapabilitySet()
	require.NoError(t, err)

	m := newManager(t)
	sess := &testSession{key: "run-3", tenant: uuid.New(), caps: caps, mode: pykernel.ModeApply, wd: t.TempDir()}
	k, err := m.Acquire(context.Background(), sess)
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	ch, err := k.Exec(ctx, pykernel.ExecRequest{Code: "while True:\n    pass"})
	require.NoError(t, err)

	time.Sleep(250 * time.Millisecond)
	cancel()

	// The channel must close (cooperative cancel, or kill after the grace).
	deadline := time.After(15 * time.Second)
	for {
		select {
		case _, ok := <-ch:
			if !ok {
				require.NoError(t, m.Release(k))
				return
			}
		case <-deadline:
			t.Fatal("exec did not terminate after cancellation")
		}
	}
}
