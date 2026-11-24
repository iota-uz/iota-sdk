package manager_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/pykernel"
	"github.com/iota-uz/iota-sdk/pkg/pykernel/isolation"
	"github.com/iota-uz/iota-sdk/pkg/pykernel/lifecycle"
	"github.com/iota-uz/iota-sdk/pkg/pykernel/manager"
	"github.com/iota-uz/iota-sdk/pkg/pykernel/pykerneltest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newManagerWith(t *testing.T, policy lifecycle.LifecyclePolicy) *manager.Manager {
	t.Helper()
	m, err := manager.New(manager.Config{
		Policy:       policy,
		Sandbox:      pykerneltest.Sandbox(t),
		EnvAllowlist: []string{"PATH"},
		DisposeGrace: 2 * time.Second,
	})
	require.NoError(t, err)
	t.Cleanup(func() { _ = m.Shutdown(context.Background()) })
	return m
}

// run executes code and returns stdout and the terminal payload.
func run(t *testing.T, k pykernel.Kernel, req pykernel.ExecRequest) (string, *pykernel.ExecError, *pykernel.DonePayload) {
	t.Helper()
	ch, err := k.Exec(context.Background(), req)
	require.NoError(t, err)
	var sb strings.Builder
	var execErr *pykernel.ExecError
	var done *pykernel.DonePayload
	timeout := time.After(30 * time.Second)
	for {
		select {
		case ev, ok := <-ch:
			if !ok {
				return sb.String(), execErr, done
			}
			switch ev.Kind {
			case pykernel.EventStdout:
				sb.Write(ev.Stdout)
			case pykernel.EventError:
				execErr = ev.Err
			case pykernel.EventDone:
				done = ev.Done
			case pykernel.EventResult, pykernel.EventMetric, pykernel.EventLog, pykernel.EventTruncated:
			}
		case <-timeout:
			t.Fatal("exec did not finish")
		}
	}
}

func TestNew_FailsClosedWhenTheSandboxCannotBeVerified(t *testing.T) {
	_, err := manager.New(manager.Config{
		Policy:     lifecycle.Ephemeral(),
		PythonPath: "/nonexistent/python3",
		Sandbox:    isolation.Profile{Mode: isolation.ModeRequired},
	})
	require.Error(t, err)
	require.ErrorIs(t, err, isolation.ErrIsolationUnavailable)
	var inc *isolation.IncompleteError
	require.ErrorAs(t, err, &inc)
	assert.NotEmpty(t, inc.Missing)
}

// The sandbox is opt-in: a zero Sandbox starts an unconfined Manager instead
// of failing on a host without Landlock.
func TestNew_ZeroSandboxIsUnconfined(t *testing.T) {
	requirePython(t)
	m, err := manager.New(manager.Config{Policy: lifecycle.Ephemeral()})
	require.NoError(t, err)
	t.Cleanup(func() { _ = m.Shutdown(context.Background()) })
}

func TestExec_ReportsTerminationReasons(t *testing.T) {
	requirePython(t)
	m := newManagerWith(t, lifecycle.Ephemeral())
	cases := []struct {
		name    string
		req     pykernel.ExecRequest
		reason  pykernel.TerminationReason
		errType string
	}{
		{"completed", pykernel.ExecRequest{Code: "1 + 1"}, pykernel.ReasonCompleted, ""},
		{"error", pykernel.ExecRequest{Code: "raise ValueError('boom')"}, pykernel.ReasonError, "ValueError"},
		{"timeout", pykernel.ExecRequest{Code: "while True:\n    pass", Timeout: 300 * time.Millisecond}, pykernel.ReasonTimeout, "KernelCancelled"},
		{"kernel exited", pykernel.ExecRequest{Code: "import os\nos._exit(1)"}, pykernel.ReasonKernelExited, "KernelExited"},
		{"timeout wins over a swallowed interrupt", pykernel.ExecRequest{
			Code:    "import time\ntry:\n    while True:\n        time.sleep(0.01)\nexcept KeyboardInterrupt:\n    pass",
			Timeout: 300 * time.Millisecond,
		}, pykernel.ReasonTimeout, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			k, err := m.Acquire(context.Background(), &testSession{key: "reason-" + tc.name, tenant: uuid.New(), caps: emptyCaps(t), mode: pykernel.ModeApply, wd: t.TempDir()})
			require.NoError(t, err)
			defer func() { _ = m.Release(k) }()
			_, execErr, done := run(t, k, tc.req)
			require.NotNil(t, done, "exactly one EventDone ends every stream")
			assert.Equal(t, tc.reason, done.Reason)
			if tc.errType == "" {
				assert.Nil(t, execErr)
			} else {
				require.NotNil(t, execErr)
				assert.Equal(t, tc.errType, execErr.Type)
			}
		})
	}
}

func emptyCaps(t *testing.T) pykernel.CapabilitySet {
	t.Helper()
	caps, err := pykernel.NewCapabilitySet()
	require.NoError(t, err)
	return caps
}

func TestRespawn_ReportsNamespaceResetWithFilesPreserved(t *testing.T) {
	requirePython(t)
	wd := t.TempDir()
	tenant := uuid.New()
	sess := &testSession{key: "chat-1", tenant: tenant, caps: emptyCaps(t), mode: pykernel.ModeApply, wd: wd}

	first := newManagerWith(t, lifecycle.WarmPool(time.Hour, 4))
	k, err := first.Acquire(context.Background(), sess)
	require.NoError(t, err)
	info := k.Info()
	assert.False(t, info.NamespaceReset, "a brand-new session has nothing to reset")
	assert.False(t, info.FilesPreserved)
	_, execErr, _ := run(t, k, pykernel.ExecRequest{Code: "open('notes.txt', 'w').write('kept')\nanswer = 42"})
	require.Nil(t, execErr)
	// The host restarts: the process and its namespace are gone, the workdir is not.
	require.NoError(t, first.Shutdown(context.Background()))

	second := newManagerWith(t, lifecycle.WarmPool(time.Hour, 4))
	k, err = second.Acquire(context.Background(), sess)
	require.NoError(t, err)
	info = k.Info()
	assert.True(t, info.NamespaceReset)
	assert.True(t, info.FilesPreserved)
	out, execErr, done := run(t, k, pykernel.ExecRequest{Code: "print(open('notes.txt').read(), 'answer' in globals())"})
	require.Nil(t, execErr)
	assert.Equal(t, "kept False\n", out)
	require.NotNil(t, done)
	assert.True(t, done.NamespaceReset)
	assert.True(t, done.FilesPreserved)
	assert.False(t, k.Info().NamespaceReset, "the first exec acknowledges the reset")
	_, _, done = run(t, k, pykernel.ExecRequest{Code: "1"})
	assert.False(t, done.NamespaceReset)
}

type freshPool struct{ lifecycle.LifecyclePolicy }

func (freshPool) ResetMode() lifecycle.ResetMode { return lifecycle.FreshNamespace }

func TestFreshNamespace_ClearsNamesOnReuseButKeepsCapabilities(t *testing.T) {
	requirePython(t)
	echo := pykernel.CapabilityFunc("echo", pykernel.AccessRead, pykernel.CapabilitySignature{},
		func(context.Context, pykernel.CallArgs) (any, error) { return "ok", nil })
	caps, err := pykernel.NewCapabilitySet(echo)
	require.NoError(t, err)
	m := newManagerWith(t, freshPool{lifecycle.WarmPool(time.Hour, 4)})
	sess := &testSession{key: "fresh", tenant: uuid.New(), caps: caps, mode: pykernel.ModeApply, wd: t.TempDir()}

	k, err := m.Acquire(context.Background(), sess)
	require.NoError(t, err)
	_, execErr, _ := run(t, k, pykernel.ExecRequest{Code: "x = 1"})
	require.Nil(t, execErr)
	require.NoError(t, m.Release(k))

	again, err := m.Acquire(context.Background(), sess)
	require.NoError(t, err)
	if pykerneltest.Sandbox(t).Mode == isolation.ModeRequired {
		assert.NotSame(t, k, again, "a supervised kernel that ran code is replaced by a fresh process")
	} else {
		assert.Same(t, k, again, "an unsupervised kernel is reset in place")
	}
	assert.True(t, again.Info().NamespaceReset)
	out, execErr, done := run(t, again, pykernel.ExecRequest{Code: "print('x' in globals(), echo())"})
	require.Nil(t, execErr)
	assert.Equal(t, "False ok\n", out)
	assert.True(t, done.NamespaceReset)
	assert.True(t, done.FilesPreserved)
}

type boundSession struct {
	*testSession
	user string
}

type userKey struct{}

func (s boundSession) BindCallContext(ctx context.Context) context.Context {
	// A binder cannot replace the Session tenant: the Manager applies it last.
	ctx = composables.WithTenantID(ctx, uuid.New())
	return context.WithValue(ctx, userKey{}, s.user)
}

func TestCallContextBinder_InjectsIdentityBesideTheTenant(t *testing.T) {
	requirePython(t)
	type seen struct {
		user   any
		tenant uuid.UUID
	}
	got := make(chan seen, 1)
	whoami := pykernel.CapabilityFunc("whoami", pykernel.AccessRead, pykernel.CapabilitySignature{},
		func(ctx context.Context, _ pykernel.CallArgs) (any, error) {
			tid, _ := composables.UseTenantID(ctx)
			got <- seen{user: ctx.Value(userKey{}), tenant: tid}
			return nil, nil //nolint:nilnil // the capability only records its context
		})
	caps, err := pykernel.NewCapabilitySet(whoami)
	require.NoError(t, err)
	tenant := uuid.New()
	m := newManagerWith(t, lifecycle.Ephemeral())
	k, err := m.Acquire(context.Background(), boundSession{
		testSession: &testSession{key: "bound", tenant: tenant, caps: caps, mode: pykernel.ModePlan, wd: t.TempDir()},
		user:        "user-7",
	})
	require.NoError(t, err)
	defer func() { _ = m.Release(k) }()
	_, execErr, _ := run(t, k, pykernel.ExecRequest{Code: "whoami()"})
	require.Nil(t, execErr)
	s := <-got
	assert.Equal(t, "user-7", s.user)
	assert.Equal(t, tenant, s.tenant)
}

func TestExec_OversizedSubmissionKeepsHealthyKernel(t *testing.T) {
	requirePython(t)
	m := newManagerWith(t, lifecycle.WarmPool(time.Hour, 4))
	k, err := m.Acquire(context.Background(), &testSession{key: "oversized", tenant: uuid.New(), caps: emptyCaps(t), mode: pykernel.ModeApply, wd: t.TempDir()})
	require.NoError(t, err)
	defer func() { _ = m.Release(k) }()

	_, err = k.Exec(context.Background(), pykernel.ExecRequest{Code: "x = '" + strings.Repeat("a", 65<<20) + "'"})
	require.ErrorIs(t, err, pykernel.ErrRequestTooLarge, "refused as too large, not as a lost kernel")

	out, execErr, done := run(t, k, pykernel.ExecRequest{Code: "print(42)"})
	require.Nil(t, execErr)
	assert.Equal(t, "42\n", out)
	require.NotNil(t, done)
	assert.Equal(t, pykernel.ReasonCompleted, done.Reason, "the kernel is still healthy")
}

func TestFreshNamespace_ReplacesAKernelThatKeepsAnEarlierHoldersThread(t *testing.T) {
	requirePython(t)
	if pykerneltest.Sandbox(t).Mode != isolation.ModeRequired {
		t.Skip("leftover work is only observable for supervised kernels")
	}
	m := newManagerWith(t, freshPool{lifecycle.WarmPool(time.Hour, 4)})
	sess := &testSession{key: "fresh-thread", tenant: uuid.New(), caps: emptyCaps(t), mode: pykernel.ModeApply, wd: t.TempDir()}
	k, err := m.Acquire(context.Background(), sess)
	require.NoError(t, err)
	_, execErr, _ := run(t, k, pykernel.ExecRequest{Code: "import threading, time\nthreading.Thread(target=time.sleep, args=(3600,), daemon=True).start()"})
	require.Nil(t, execErr)
	require.NoError(t, m.Release(k))

	again, err := m.Acquire(context.Background(), sess)
	require.NoError(t, err)
	assert.NotSame(t, k, again, "the earlier holder's thread must not be handed to the next one")
	assert.True(t, again.Info().NamespaceReset)
	out, execErr, _ := run(t, again, pykernel.ExecRequest{Code: "import threading\nprint(threading.active_count())"})
	require.Nil(t, execErr)
	assert.Equal(t, "2\n", out)
}

func TestExec_WorkerClosingItsBridgeIsKilled(t *testing.T) {
	requirePython(t)
	m := newManagerWith(t, lifecycle.Ephemeral())
	k, err := m.Acquire(context.Background(), &testSession{key: "bridge-exit", tenant: uuid.New(), caps: emptyCaps(t), mode: pykernel.ModeApply, wd: t.TempDir()})
	require.NoError(t, err)
	defer func() { _ = m.Release(k) }()
	pid := k.Info().PID
	_, _, done := run(t, k, pykernel.ExecRequest{Code: "import socket\nsocket.socket(fileno=3).shutdown(socket.SHUT_RDWR)\nwhile True:\n    pass", Timeout: time.Minute})
	require.NotNil(t, done)
	assert.Equal(t, pykernel.ReasonKernelExited, done.Reason)
	// The terminal event comes only after the group is confirmed gone.
	assert.True(t, errors.Is(syscall.Kill(-pid, 0), syscall.ESRCH) || isZombie(pid),
		"a kernel without its bridge must be gone before its exec is reported ended")
}

func isZombie(pid int) bool {
	raw, err := os.ReadFile(fmt.Sprintf("/proc/%d/stat", pid))
	if err != nil {
		return true
	}
	s := string(raw)
	i := strings.LastIndexByte(s, ')')
	return i >= 0 && len(s) > i+2 && s[i+2] == 'Z'
}

func TestExec_RejectedRequestKeepsTheResetNotice(t *testing.T) {
	requirePython(t)
	wd := t.TempDir()
	sess := &testSession{key: "reset-notice", tenant: uuid.New(), caps: emptyCaps(t), mode: pykernel.ModeApply, wd: wd}
	first := newManagerWith(t, lifecycle.WarmPool(time.Hour, 4))
	k, err := first.Acquire(context.Background(), sess)
	require.NoError(t, err)
	_, execErr, _ := run(t, k, pykernel.ExecRequest{Code: "x = 1"})
	require.Nil(t, execErr)
	require.NoError(t, first.Shutdown(context.Background()))

	second := newManagerWith(t, lifecycle.WarmPool(time.Hour, 4))
	k, err = second.Acquire(context.Background(), sess)
	require.NoError(t, err)
	require.True(t, k.Info().NamespaceReset)
	_, err = k.Exec(context.Background(), pykernel.ExecRequest{Code: "y = '" + strings.Repeat("a", 65<<20) + "'"})
	require.ErrorIs(t, err, pykernel.ErrRequestTooLarge)
	assert.True(t, k.Info().NamespaceReset, "a refused request must not consume the reset notice")
	_, execErr, done := run(t, k, pykernel.ExecRequest{Code: "1"})
	require.Nil(t, execErr)
	assert.True(t, done.NamespaceReset)
}
