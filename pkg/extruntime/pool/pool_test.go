package pool_test

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/iota-uz/iota-sdk/pkg/extruntime/pool"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/pool/pooltest"
)

func newPool(t *testing.T, size int, mutate ...func(*pool.Config)) *pool.Pool {
	t.Helper()
	cfg := pool.Config{
		Size:         size,
		WorkerScript: pooltest.BuildWorker(t),
		CancelGrace:  300 * time.Millisecond,
		Logger:       slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError})),
	}
	for _, m := range mutate {
		m(&cfg)
	}
	p, err := pool.New(cfg)
	require.NoError(t, err)
	t.Cleanup(func() { _ = p.Close(context.Background()) })
	return p
}

type noCalls struct{}

func (noCalls) Call(context.Context, string, json.RawMessage) (json.RawMessage, error) {
	return nil, errors.New("no host calls expected")
}

func invoke(art pool.Artifact, kind pool.Kind, export string, input string, calls pool.HostCalls) pool.Invocation {
	if calls == nil {
		calls = noCalls{}
	}
	return pool.Invocation{
		Artifact: art,
		Kind:     kind,
		Export:   export,
		Resource: "acme.test." + export,
		Input:    json.RawMessage(input),
		Ctx:      pool.InvokeContext{Tenant: "t-1", User: pool.UserContext{ID: 7, Locale: "en"}},
		Calls:    calls,
	}
}

func TestInvokeEchoesInputAndContext(t *testing.T) {
	p := newPool(t, 1)
	art := pooltest.Artifact(t, `export const who = async (ctx, input) => ({ user: ctx.user.id, tenant: ctx.tenant, input });`)
	out, err := p.Invoke(context.Background(), invoke(art, pool.KindCommand, "who", `{"a":1}`, nil))
	require.NoError(t, err)
	require.JSONEq(t, `{"user":7,"tenant":"t-1","input":{"a":1}}`, string(out))
}

func TestHostCallsRoundTripWithHandleAndConsoleIsolation(t *testing.T) {
	p := newPool(t, 1)
	art := pooltest.Artifact(t, `
export const run = async (ctx, input) => {
  console.log("hello from package");
  process.stdout.write("raw write\n");
  const row = await ctx.query.get({ id: "acme.test.thing" }, { key: "k1" });
  const tx = await ctx.transaction(async (tx) => tx.insert("acme.test.thing", { n: 1 }));
  return { row, tx };
};`)
	var methods []string
	var mu sync.Mutex
	calls := pool.HostCallsFunc(func(_ context.Context, method string, args json.RawMessage) (json.RawMessage, error) {
		mu.Lock()
		methods = append(methods, method)
		mu.Unlock()
		switch method {
		case "q.get":
			require.JSONEq(t, `{"entity":"acme.test.thing","key":"k1"}`, string(args))
			return json.RawMessage(`{"key":"k1"}`), nil
		case "tx.begin":
			return json.RawMessage(`{"tx":"h-1"}`), nil
		case "tx.insert":
			require.JSONEq(t, `{"tx":"h-1","entity":"acme.test.thing","values":{"n":1}}`, string(args))
			return json.RawMessage(`{"key":"k2"}`), nil
		case "tx.end":
			require.JSONEq(t, `{"tx":"h-1","outcome":"commit"}`, string(args))
			return json.RawMessage(`null`), nil
		}
		return nil, errors.New("unexpected " + method)
	})
	out, err := p.Invoke(context.Background(), invoke(art, pool.KindCommand, "run", `null`, calls))
	require.NoError(t, err)
	require.JSONEq(t, `{"row":{"key":"k1"},"tx":{"key":"k2"}}`, string(out))
	require.Equal(t, []string{"q.get", "tx.begin", "tx.insert", "tx.end"}, methods)
}

func TestNestedTransactionRejectedAndRollbackOnThrow(t *testing.T) {
	p := newPool(t, 1)
	art := pooltest.Artifact(t, `
export const nested = async (ctx) => ctx.transaction(async () => ctx.transaction(async () => 1));
export const boom = async (ctx) => ctx.transaction(async () => { throw new Error("kaboom"); });`)
	var outcomes []string
	calls := pool.HostCallsFunc(func(_ context.Context, method string, args json.RawMessage) (json.RawMessage, error) {
		switch method {
		case "tx.begin":
			return json.RawMessage(`{"tx":"h"}`), nil
		case "tx.end":
			var a struct{ Outcome string }
			require.NoError(t, json.Unmarshal(args, &a))
			outcomes = append(outcomes, a.Outcome)
			return json.RawMessage(`null`), nil
		}
		return nil, errors.New("unexpected " + method)
	})
	_, err := p.Invoke(context.Background(), invoke(art, pool.KindCommand, "nested", `null`, calls))
	we, ok := pool.AsWireError(err)
	require.True(t, ok)
	require.Equal(t, "runtime.nestedTransaction", we.Code)

	_, err = p.Invoke(context.Background(), invoke(art, pool.KindCommand, "boom", `null`, calls))
	we, ok = pool.AsWireError(err)
	require.True(t, ok)
	require.Equal(t, pool.ErrKindInternal, we.Kind)
	require.Contains(t, we.Message, "kaboom")
	require.Equal(t, []string{"rollback", "rollback"}, outcomes)
}

func TestTwoVersionsSideBySideInOnePool(t *testing.T) {
	p := newPool(t, 1)
	v1 := pooltest.Artifact(t, `let n = 0; export const ver = async (ctx) => ({ v: "pkgA@1", user: ctx.user.id, calls: ++n });`)
	v2 := pooltest.Artifact(t, `let n = 0; export const ver = async (ctx) => ({ v: "pkgA@2", user: ctx.user.id, calls: ++n });`)
	other := pooltest.Artifact(t, `export const ver = async (ctx) => ({ v: "pkgB@1", tenant: ctx.tenant });`)
	for i, want := range []string{`{"v":"pkgA@1","user":7,"calls":1}`, `{"v":"pkgA@2","user":7,"calls":1}`, `{"v":"pkgA@1","user":7,"calls":2}`, `{"v":"pkgB@1","tenant":"t-1"}`, `{"v":"pkgA@2","user":7,"calls":2}`} {
		art := []pool.Artifact{v1, v2, v1, other, v2}[i]
		out, err := p.Invoke(context.Background(), invoke(art, pool.KindCommand, "ver", `null`, nil))
		require.NoError(t, err)
		require.JSONEq(t, want, string(out), "step %d", i)
	}
	require.Equal(t, 1, p.Stats().Ready)
}

func TestMissingArtifactAndExportAreExplicit(t *testing.T) {
	p := newPool(t, 1)
	missing := pool.Artifact{Hash: "deadbeef", Dir: filepath.Join(t.TempDir(), "nope")}
	_, err := p.Invoke(context.Background(), invoke(missing, pool.KindCommand, "x", `null`, nil))
	require.ErrorIs(t, err, pool.ErrArtifactMissing)

	art := pooltest.Artifact(t, `export const present = async () => 1; export const notFn = 5;`)
	_, err = p.Invoke(context.Background(), invoke(art, pool.KindCommand, "absent", `null`, nil))
	require.ErrorIs(t, err, pool.ErrExportMissing)
	_, err = p.Invoke(context.Background(), invoke(art, pool.KindCommand, "notFn", `null`, nil))
	require.ErrorIs(t, err, pool.ErrExportMissing)

	out, err := p.Invoke(context.Background(), invoke(art, pool.KindCommand, "present", `null`, nil))
	require.NoError(t, err)
	require.JSONEq(t, `1`, string(out))
}

func TestUnsupportedProtocolRejectsWorker(t *testing.T) {
	script := filepath.Join(t.TempDir(), "fake.mjs")
	require.NoError(t, os.WriteFile(script, []byte(`
const body = Buffer.from(JSON.stringify({ t: "ready", protocol: "iota.runtime/2", pid: process.pid }));
const head = Buffer.alloc(4); head.writeUInt32BE(body.length);
process.stdin.once("data", () => process.stdout.write(Buffer.concat([head, body])));
`), 0o644))
	_, err := pool.New(pool.Config{Size: 1, WorkerScript: script})
	require.ErrorIs(t, err, pool.ErrProtocol)
}

func TestStructuredErrorsRoundTrip(t *testing.T) {
	p := newPool(t, 1)
	art := pooltest.Artifact(t, `
export const thrower = async () => {
  throw Object.assign(new Error("not active"), { kind: "domain", code: "acme.test.notActive", params: { status: "draft" } });
};
export const validator = async () => {
  throw { kind: "validation", issues: [{ code: "acme.test.tooLong", phase: "input", source: "g1", path: ["notes", 0], params: { max: 10 } }] };
};
export const catcher = async (ctx) => {
  try { await ctx.query.get("e", { key: "k" }); } catch (e) { return { kind: e.kind, code: e.code, params: e.params, issues: e.issues }; }
};`)
	_, err := p.Invoke(context.Background(), invoke(art, pool.KindCommand, "thrower", `null`, nil))
	we, ok := pool.AsWireError(err)
	require.True(t, ok)
	require.Equal(t, pool.ErrKindDomain, we.Kind)
	require.Equal(t, "acme.test.notActive", we.Code)
	require.JSONEq(t, `{"status":"draft"}`, string(we.Params))

	_, err = p.Invoke(context.Background(), invoke(art, pool.KindCommand, "validator", `null`, nil))
	we, ok = pool.AsWireError(err)
	require.True(t, ok)
	require.Equal(t, pool.ErrKindValidation, we.Kind)
	require.Len(t, we.Issues, 1)
	require.Equal(t, "acme.test.tooLong", we.Issues[0].Code)
	require.JSONEq(t, `["notes",0]`, string(we.Issues[0].Path))

	calls := pool.HostCallsFunc(func(context.Context, string, json.RawMessage) (json.RawMessage, error) {
		return nil, &pool.WireError{Kind: pool.ErrKindDomain, Code: "acme.test.denied2", Params: json.RawMessage(`{"x":1}`), Issues: []pool.Issue{{Code: "i1", Phase: "write", Path: json.RawMessage(`["a"]`)}}}
	})
	out, err := p.Invoke(context.Background(), invoke(art, pool.KindCommand, "catcher", `null`, calls))
	require.NoError(t, err)
	require.JSONEq(t, `{"kind":"domain","code":"acme.test.denied2","params":{"x":1},"issues":[{"code":"i1","phase":"write","path":["a"]}]}`, string(out))
}

func TestWorkerKilledMidInvocationIsTypedAndPoolRecovers(t *testing.T) {
	p := newPool(t, 1, func(c *pool.Config) { c.RestartBackoffMin = 20 * time.Millisecond })
	art := pooltest.Artifact(t, `
export const die = async () => { process.kill(process.pid, "SIGKILL"); await new Promise(() => {}); };
export const ok = async () => "alive";`)
	_, err := p.Invoke(context.Background(), invoke(art, pool.KindCommand, "die", `null`, nil))
	require.ErrorIs(t, err, pool.ErrWorkerLost)
	var lost *pool.WorkerLostError
	require.ErrorAs(t, err, &lost)
	require.NotEmpty(t, lost.Invocation)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	out, err := p.Invoke(ctx, invoke(art, pool.KindCommand, "ok", `null`, nil))
	require.NoError(t, err)
	require.JSONEq(t, `"alive"`, string(out))
}

func TestDeadlineCancelsCooperativeAndKillsStuckWorker(t *testing.T) {
	p := newPool(t, 1, func(c *pool.Config) { c.RestartBackoffMin = 20 * time.Millisecond })
	art := pooltest.Artifact(t, `
export const wait = async (ctx) => ctx.query.get("e", { key: "k" });
export const spin = async () => { for (;;) {} };
export const ok = async () => "alive";`)

	calls := pool.HostCallsFunc(func(ctx context.Context, _ string, _ json.RawMessage) (json.RawMessage, error) {
		<-ctx.Done()
		return nil, ctx.Err()
	})
	ctx, cancel := context.WithTimeout(context.Background(), 150*time.Millisecond)
	_, err := p.Invoke(ctx, invoke(art, pool.KindCommand, "wait", `null`, calls))
	cancel()
	require.ErrorIs(t, err, context.DeadlineExceeded)

	ctx, cancel = context.WithTimeout(context.Background(), 150*time.Millisecond)
	_, err = p.Invoke(ctx, invoke(art, pool.KindCommand, "spin", `null`, nil))
	cancel()
	require.ErrorIs(t, err, context.DeadlineExceeded)

	ctx, cancel = context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	out, err := p.Invoke(ctx, invoke(art, pool.KindCommand, "ok", `null`, nil))
	require.NoError(t, err)
	require.JSONEq(t, `"alive"`, string(out))
}

func TestQueueWhenAllWorkersBusy(t *testing.T) {
	p := newPool(t, 1)
	art := pooltest.Artifact(t, `export const slow = async (ctx, input) => { await new Promise(r => setTimeout(r, 150)); return input; };`)
	var wg sync.WaitGroup
	started := time.Now()
	results := make([]string, 3)
	for i := range results {
		wg.Add(1)
		go func() {
			defer wg.Done()
			out, err := p.Invoke(context.Background(), invoke(art, pool.KindCommand, "slow", `"x"`, nil))
			require.NoError(t, err)
			results[i] = string(out)
		}()
		time.Sleep(20 * time.Millisecond)
		if i == 1 {
			require.Equal(t, 1, p.Stats().Queued)
		}
	}
	wg.Wait()
	require.GreaterOrEqual(t, time.Since(started), 450*time.Millisecond, "three 150ms invocations on one worker run serially")
	require.Equal(t, []string{`"x"`, `"x"`, `"x"`}, results)
}

func TestQueueLimit(t *testing.T) {
	p := newPool(t, 1, func(c *pool.Config) { c.MaxQueue = 1 })
	art := pooltest.Artifact(t, `export const slow = async () => { await new Promise(r => setTimeout(r, 300)); return 1; };`)
	var wg sync.WaitGroup
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := p.Invoke(context.Background(), invoke(art, pool.KindCommand, "slow", `null`, nil))
			require.NoError(t, err)
		}()
		time.Sleep(50 * time.Millisecond)
	}
	_, err := p.Invoke(context.Background(), invoke(art, pool.KindCommand, "slow", `null`, nil))
	require.ErrorIs(t, err, pool.ErrQueueFull)
	wg.Wait()
}

func TestNestedInvocationOnParentWorkerDoesNotDeadlock(t *testing.T) {
	p := newPool(t, 1)
	art := pooltest.Artifact(t, `
export const outer = async (ctx) => ({ inner: await ctx.call({ id: "acme.test.inner" }, { n: 2 }) });
export const inner = async (ctx, input) => ({ doubled: input.n * 2 });`)
	var calls pool.HostCallsFunc
	calls = func(ctx context.Context, method string, args json.RawMessage) (json.RawMessage, error) {
		require.Equal(t, "invoke.command", method)
		parent := invoke(art, pool.KindCommand, "inner", `{"n":2}`, noCalls{})
		parent.ParentID = ctx.Value(parentKey{}).(string)
		return p.Invoke(ctx, parent)
	}
	in := invoke(art, pool.KindCommand, "outer", `null`, nil)
	in.ID = "root-1"
	in.Calls = pool.HostCallsFunc(func(ctx context.Context, method string, args json.RawMessage) (json.RawMessage, error) {
		return calls(context.WithValue(ctx, parentKey{}, "root-1"), method, args)
	})
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	out, err := p.Invoke(ctx, in)
	require.NoError(t, err)
	require.JSONEq(t, `{"inner":{"doubled":4}}`, string(out))
}

type parentKey struct{}

func TestSuspendUnwindsAndRejectsFurtherHostCalls(t *testing.T) {
	p := newPool(t, 1)
	art := pooltest.Artifact(t, `
export const wf = async (wf) => {
  try { await wf.sleep("cooldown", "1d"); } catch (e) { /* swallowed on purpose */ }
  try { await wf.step("after", async () => 1); } catch (e) { await wf.sleep("again", "1d").catch(() => {}); }
  return "finished normally";
};`)
	var methods []string
	calls := pool.HostCallsFunc(func(_ context.Context, method string, _ json.RawMessage) (json.RawMessage, error) {
		methods = append(methods, method)
		if method == "wf.sleep" {
			return nil, &pool.WireError{Kind: pool.ErrKindSuspend, Suspend: json.RawMessage(`{"wakeAt":"2030-01-01T00:00:00Z"}`)}
		}
		return json.RawMessage(`{"replay":true,"value":1}`), nil
	})
	_, err := p.Invoke(context.Background(), invoke(art, pool.KindWorkflow, "wf", `null`, calls))
	require.True(t, pool.IsSuspend(err), "got %v", err)
	we, _ := pool.AsWireError(err)
	require.JSONEq(t, `{"wakeAt":"2030-01-01T00:00:00Z"}`, string(we.Suspend))
	require.Equal(t, []string{"wf.sleep"}, methods, "no host call may reach Go after suspend")
}

func TestCloseDrainsInFlightThenRejectsNew(t *testing.T) {
	p := newPool(t, 1)
	art := pooltest.Artifact(t, `export const slow = async () => { await new Promise(r => setTimeout(r, 200)); return "done"; };`)
	res := make(chan error, 1)
	go func() {
		_, err := p.Invoke(context.Background(), invoke(art, pool.KindCommand, "slow", `null`, nil))
		res <- err
	}()
	time.Sleep(60 * time.Millisecond)
	require.NoError(t, p.Close(context.Background()))
	require.NoError(t, <-res)
	_, err := p.Invoke(context.Background(), invoke(art, pool.KindCommand, "slow", `null`, nil))
	require.ErrorIs(t, err, pool.ErrPoolClosed)
	require.Zero(t, p.Stats().Ready)
}

func TestWorkerEnvIsMinimal(t *testing.T) {
	t.Setenv("SECRET_FROM_HOST", "leak")
	p := newPool(t, 1, func(c *pool.Config) { c.ExtraEnv = []string{"IOTA_RUNTIME_FLAG=on"} })
	art := pooltest.Artifact(t, `export const env = async () => Object.keys(process.env).sort();`)
	out, err := p.Invoke(context.Background(), invoke(art, pool.KindCommand, "env", `null`, nil))
	require.NoError(t, err)
	var keys []string
	require.NoError(t, json.Unmarshal(out, &keys))
	require.Equal(t, []string{"IOTA_RUNTIME_FLAG", "IOTA_RUNTIME_PROTOCOL", "IOTA_RUNTIME_WORKER", "NODE_ENV", "PATH", "TZ"}, keys)

	_, err = pool.New(pool.Config{WorkerScript: "x", ExtraEnv: []string{"HOME=/root"}})
	require.Error(t, err)
}
