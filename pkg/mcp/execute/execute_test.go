package execute_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"os/exec"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/iota-uz/iota-sdk/pkg/mcp"
	"github.com/iota-uz/iota-sdk/pkg/mcp/execute"
	"github.com/iota-uz/iota-sdk/pkg/pykernel/lifecycle"
	"github.com/iota-uz/iota-sdk/pkg/pykernel/manager"
)

type echoIn struct {
	Text string `json:"text"`
}

type echoOut struct {
	Text  string `json:"text"`
	Bulky string `json:"bulky"`
}

type recordOut struct {
	OperationID string `json:"operation_id"`
}

type memoryArtifacts struct {
	mu    sync.Mutex
	files map[string][]byte
}

func (m *memoryArtifacts) Save(_ context.Context, f execute.ArtifactFile) (string, error) {
	body, err := io.ReadAll(f.Content)
	if err != nil {
		return "", err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	ref := "file-" + f.Name
	m.files[ref] = body
	return ref, nil
}

type fixture struct {
	engine    *mcp.Engine
	allowed   *atomic.Bool
	artifacts *memoryArtifacts
	ctx       context.Context
}

func newFixture(t *testing.T, withRuntime bool) *fixture {
	t.Helper()
	if _, err := exec.LookPath("python3"); err != nil && withRuntime {
		t.Skip("python3 not available")
	}
	allowed := &atomic.Bool{}
	allowed.Store(true)
	reg := mcp.NewRegistry()
	require.NoError(t, reg.Register(
		mcp.MustNew(mcp.Spec{Name: "echo", Description: "Echo text.", Annotations: mcp.Annotations{ReadOnly: true}},
			func(_ context.Context, in echoIn) (echoOut, error) {
				return echoOut{Text: in.Text, Bulky: "INTERMEDIATE-PAYLOAD"}, nil
			}),
		mcp.MustNew(mcp.Spec{Name: "record", Description: "Pretend to write.", Policy: mcp.Policy{Permissions: []string{"Record.Write"}}},
			func(_ context.Context, _ struct{}) (recordOut, error) {
				return recordOut{OperationID: "op-1"}, nil
			}),
	))
	authz := mcp.AuthorizerFunc(func(ctx context.Context, _ mcp.Principal, policy mcp.Policy) (context.Context, error) {
		if len(policy.Permissions) > 0 && !allowed.Load() {
			return nil, mcp.Fail(mcp.CodeForbidden, mcp.RecoveryRequestAccess, "missing permission")
		}
		return ctx, nil
	})
	engine, err := mcp.NewEngine(reg, authz)
	require.NoError(t, err)

	cfg := execute.Config{
		Engine:   engine,
		WorkRoot: t.TempDir(),
		Limits: execute.Limits{
			DefaultTimeout: 10 * time.Second, MaxTimeout: 30 * time.Second,
			DefaultOutputBytes: 4096, MaxOutputBytes: 8192, MaxCodeBytes: 10_000,
		},
		MaxArtifacts:     5,
		MaxArtifactBytes: 1 << 20,
	}
	artifacts := &memoryArtifacts{files: map[string][]byte{}}
	cfg.Artifacts = artifacts
	if withRuntime {
		m, err := manager.New(manager.Config{
			Policy:       lifecycle.WarmPool(time.Minute, 4),
			EnvAllowlist: []string{"PATH"},
			DisposeGrace: time.Second,
		})
		require.NoError(t, err)
		t.Cleanup(func() { _ = m.Shutdown(context.Background()) })
		cfg.Manager = m
	}
	runner, err := execute.NewRunner(cfg)
	require.NoError(t, err)
	require.NoError(t, reg.Register(runner.Tool()))

	ctx := mcp.WithPrincipal(context.Background(), mcp.Principal{
		TenantID: uuid.New(), UserID: 7, Scopes: mcp.AllScopes(),
	})
	return &fixture{engine: engine, allowed: allowed, artifacts: artifacts, ctx: ctx}
}

func (f *fixture) run(t *testing.T, ctx context.Context, in execute.Input) mcp.Result {
	t.Helper()
	raw, err := json.Marshal(in)
	require.NoError(t, err)
	return f.engine.Call(ctx, execute.ToolName, raw)
}

func decode(t *testing.T, res mcp.Result) execute.Result {
	t.Helper()
	require.True(t, res.OK, "unexpected failure: %+v", res.Error)
	var out execute.Result
	require.NoError(t, json.Unmarshal(res.Data, &out))
	return out
}

func TestExecute_ComposesToolsWithTopLevelAwait(t *testing.T) {
	f := newFixture(t, true)
	out := decode(t, f.run(t, f.ctx, execute.Input{Code: `
import sys
r = await tools.echo(text="hi")
w = await tools.record()
print("visible")
print("warn", file=sys.stderr)
emit({"echo": r["data"]["text"], "ok": w["ok"]})
`}))

	assert.Equal(t, execute.StatusSucceeded, out.Status)
	assert.Nil(t, out.Error)
	require.Len(t, out.Emitted, 1)
	assert.JSONEq(t, `{"echo":"hi","ok":true}`, string(out.Emitted[0]))
	assert.Equal(t, "visible\n", out.Stdout)
	assert.Equal(t, "warn\n", out.Stderr)
	assert.Equal(t, []string{"op-1"}, out.OperationIDs)
	assert.NotContains(t, out.Stdout+out.Stderr+string(out.Emitted[0]), "INTERMEDIATE-PAYLOAD")
	assert.False(t, out.Namespace.Reset)
	assert.NotEmpty(t, out.Namespace.Generation)
}

func TestExecute_StructuredOutcomes(t *testing.T) {
	cases := []struct {
		name      string
		input     execute.Input
		cancelIn  time.Duration
		status    string
		errorCode string
		check     func(t *testing.T, out execute.Result)
	}{
		{
			name:   "truncates stdout and emit",
			input:  execute.Input{Code: "print('x' * 5000)\nemit('y' * 5000)", MaxOutputBytes: 1000},
			status: execute.StatusSucceeded,
			check: func(t *testing.T, out execute.Result) {
				t.Helper()
				assert.True(t, out.Truncated.Stdout)
				assert.True(t, out.Truncated.Emitted)
				assert.False(t, out.Truncated.Stderr)
				assert.LessOrEqual(t, len(out.Stdout), 1000)
				assert.Empty(t, out.Emitted)
			},
		},
		{
			name:      "times out",
			input:     execute.Input{Code: "while True:\n    pass", TimeoutMS: 300},
			status:    execute.StatusTimedOut,
			errorCode: execute.ErrorTimeout,
		},
		{
			name:      "is cancelled by the caller",
			input:     execute.Input{Code: "import time\nwhile True:\n    time.sleep(0.01)"},
			cancelIn:  300 * time.Millisecond,
			status:    execute.StatusCancelled,
			errorCode: execute.ErrorCancelled,
		},
		{
			name:      "reports python errors with traceback",
			input:     execute.Input{Code: "raise ValueError('boom')"},
			status:    execute.StatusFailed,
			errorCode: execute.ErrorPython,
			check: func(t *testing.T, out execute.Result) {
				t.Helper()
				assert.Contains(t, out.Error.Message, "ValueError: boom")
				assert.Contains(t, out.Stderr, "Traceback")
			},
		},
		{
			name:   "hides execute from the tools namespace",
			input:  execute.Input{Code: "emit(hasattr(tools, 'execute'))\nemit(sorted(vars(tools)))"},
			status: execute.StatusSucceeded,
			check: func(t *testing.T, out execute.Result) {
				t.Helper()
				require.Len(t, out.Emitted, 2)
				assert.JSONEq(t, `false`, string(out.Emitted[0]))
				assert.JSONEq(t, `["echo","record"]`, string(out.Emitted[1]))
			},
		},
		{
			name:   "returns regular files from out and skips links",
			input:  execute.Input{Code: "import os\nopen('out/report.csv','w').write('a,b\\n')\nos.symlink('/etc/hosts', 'out/hosts')"},
			status: execute.StatusSucceeded,
			check: func(t *testing.T, out execute.Result) {
				t.Helper()
				require.Len(t, out.Artifacts, 1)
				assert.Equal(t, "report.csv", out.Artifacts[0].Name)
				assert.Equal(t, int64(4), out.Artifacts[0].SizeBytes)
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := newFixture(t, true)
			ctx := f.ctx
			if tc.cancelIn > 0 {
				var cancel context.CancelFunc
				ctx, cancel = context.WithCancel(ctx)
				time.AfterFunc(tc.cancelIn, cancel)
			}
			out := decode(t, f.run(t, ctx, tc.input))
			assert.Equal(t, tc.status, out.Status)
			if tc.errorCode == "" {
				assert.Nil(t, out.Error)
			} else {
				require.NotNil(t, out.Error)
				assert.Equal(t, tc.errorCode, out.Error.Code)
			}
			if tc.check != nil {
				tc.check(t, out)
			}
		})
	}
}

func TestExecute_BusySessionRejectsBeforeRunning(t *testing.T) {
	f := newFixture(t, true)
	started := make(chan struct{})
	done := make(chan mcp.Result, 1)
	go func() {
		close(started)
		done <- f.engine.Call(f.ctx, execute.ToolName, json.RawMessage(`{"code":"import time\ntime.sleep(1.5)"}`))
	}()
	<-started
	time.Sleep(700 * time.Millisecond)

	res := f.run(t, f.ctx, execute.Input{Code: "emit('second')"})
	require.False(t, res.OK)
	assert.Equal(t, mcp.CodeBusy, res.Error.Code)
	assert.Positive(t, res.Error.RetryAfterMS)
	assert.Equal(t, execute.StatusSucceeded, decode(t, <-done).Status)
}

func TestExecute_RevokedAccessAppliesToWarmSession(t *testing.T) {
	f := newFixture(t, true)
	first := decode(t, f.run(t, f.ctx, execute.Input{Code: "kept = 1\nr = await tools.record()\nemit(r['ok'])"}))
	assert.JSONEq(t, `true`, string(first.Emitted[0]))

	f.allowed.Store(false)
	second := decode(t, f.run(t, f.ctx, execute.Input{Code: "r = await tools.record()\nemit(kept)\nemit(r)"}))
	require.Len(t, second.Emitted, 2)
	assert.JSONEq(t, `1`, string(second.Emitted[0]), "the warm namespace survived")
	assert.Equal(t, first.Namespace.Generation, second.Namespace.Generation)
	var res mcp.Result
	require.NoError(t, json.Unmarshal(second.Emitted[1], &res))
	require.False(t, res.OK)
	assert.Equal(t, mcp.CodeForbidden, res.Error.Code)
	assert.Empty(t, second.OperationIDs)
}

func TestExecute_DisabledRuntimeIsDependencyUnavailable(t *testing.T) {
	f := newFixture(t, false)
	res := f.run(t, f.ctx, execute.Input{Code: "emit(1)"})
	require.False(t, res.OK)
	assert.Equal(t, mcp.CodeDependencyUnavailable, res.Error.Code)
	assert.Equal(t, mcp.RecoveryRetryLater, res.Error.Recovery)
}

func TestExecute_LimitsCanOnlyBeLowered(t *testing.T) {
	f := newFixture(t, false)
	cases := []struct {
		name  string
		input execute.Input
		code  mcp.Code
	}{
		{name: "timeout above maximum", input: execute.Input{Code: "1", TimeoutMS: 60_000}, code: mcp.CodeInvalidInput},
		{name: "zero output disables nothing", input: execute.Input{Code: "1", MaxOutputBytes: -1}, code: mcp.CodeInvalidInput},
		{name: "code above maximum", input: execute.Input{Code: string(bytes.Repeat([]byte("#"), 10_001))}, code: mcp.CodeInvalidInput},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			res := f.run(t, f.ctx, tc.input)
			require.False(t, res.OK)
			assert.Equal(t, tc.code, res.Error.Code)
		})
	}
}
