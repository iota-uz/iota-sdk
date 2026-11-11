package host_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/iota-uz/iota-sdk/pkg/extruntime/host"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/manifest"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/pool"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/spi"
)

const jsPrelude = `const item = { id: "acme.test.item" };
const domain = (code, params) => Object.assign(new Error(code), { kind: "domain", code, params });
`

func TestTwoPackagesAndVersionsInOnePoolSeeTheirOwnCaller(t *testing.T) {
	e := newEnv(t)
	who := func(pkg string, ver int) string {
		return jsPrelude + `export const who = async (ctx) => { await ctx.query.find(item); return { pkg: "` + pkg + `", ver: ` + itoa(ver) + `, user: ctx.user.id }; };`
	}
	out := obj("pkg", str(), "ver", num(), "user", num())
	e.publish("acme.a", "1.0.0", who("a", 1))
	e.publish("acme.a", "2.0.0", who("a", 2))
	e.publish("acme.b", "1.0.0", who("b", 1))
	e.contract(command("acme.a.who", false, nil, out), "acme.a", "1.0.0", "who")
	e.contract(command("acme.b.who", false, nil, out), "acme.b", "1.0.0", "who")

	run := func(contract string) host.Result {
		res, err := e.host.Invoke(context.Background(), e.req(contract, `null`))
		require.NoError(t, err)
		return res
	}
	require.JSONEq(t, `{"pkg":"a","ver":1,"user":7}`, string(run("acme.a.who").Output))
	require.JSONEq(t, `{"pkg":"b","ver":1,"user":7}`, string(run("acme.b.who").Output))

	_, err := e.bindings.Activate(context.Background(), "acme.a.who", "acme.a", "2.0.0", "who", nil)
	require.NoError(t, err)
	res := run("acme.a.who")
	require.JSONEq(t, `{"pkg":"a","ver":2,"user":7}`, string(res.Output))
	require.Equal(t, "2.0.0", res.Binding.Version)

	require.Equal(t, []spi.Caller{
		{Package: "acme.a", Version: "1.0.0"},
		{Package: "acme.b", Version: "1.0.0"},
		{Package: "acme.a", Version: "2.0.0"},
	}, e.store.callers())
	require.Equal(t, 1, e.pool.Stats().Ready, "everything ran in one warm worker")
}

func itoa(n int) string { b, _ := json.Marshal(n); return string(b) }

func TestOrdinaryCommandCallsOutsideTxThenCommitsViaTransaction(t *testing.T) {
	e := newEnv(t)
	var idleDuringCall atomic.Int64
	idleDuringCall.Store(-1)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		idleDuringCall.Store(int64(e.idleInTx()))
		_, _ = w.Write([]byte(`{"quote":42}`))
	}))
	defer srv.Close()

	e.publish("acme.a", "1.0.0", jsPrelude+`
export const renew = async (ctx, input) => {
  const quote = (await (await fetch(input.url)).json()).quote;
  const row = await ctx.transaction(async (tx) => tx.insert(item, { label: input.label + ":" + quote }));
  return { key: row.key, quote };
};`)
	e.contract(command("acme.a.renew", false, obj("url", str(), "label", str()), obj("key", str(), "quote", num())), "acme.a", "1.0.0", "renew")

	res, err := e.host.Invoke(context.Background(), e.req("acme.a.renew", `{"url":"`+srv.URL+`","label":"policy"}`))
	require.NoError(t, err)
	require.JSONEq(t, `{"key":"1","quote":42}`, string(res.Output))
	require.Equal(t, int64(0), idleDuringCall.Load(), "no Go transaction may be open while Node calls the network")
	require.Equal(t, []string{"policy:42"}, e.items())
	require.Equal(t, 1, res.CommittedTxCount)
	require.False(t, res.PartiallyCommitted)

	recs := e.invocations("acme.a.renew")
	require.Len(t, recs, 1)
	require.Equal(t, "ok", recs[0].Outcome)
	require.Equal(t, 1, recs[0].Committed, "counter is incremented inside the committed tx")
	require.Equal(t, []string{"acme.a", "1.0.0", "renew"}, []string{recs[0].Package, recs[0].Version, recs[0].Export})
	require.Equal(t, "ui", recs[0].TriggerKind)
	require.True(t, recs[0].Finished)
	require.Zero(t, e.idleInTx())
}

func TestAtomicCommandUsesOneTxAndRollsBackBothWritesOnError(t *testing.T) {
	e := newEnv(t)
	e.publish("acme.a", "1.0.0", jsPrelude+`
export const activate = async (tx, input) => {
  await tx.insert(item, { label: "first" });
  await tx.insert(item, { label: "second" });
  if (input.fail) throw domain("acme.a.boom", { why: "asked" });
  return { count: await tx.count(item) };
};`)
	e.contract(command("acme.a.activate", true, obj("fail", boolean()), obj("count", num()), "acme.a.boom"), "acme.a", "1.0.0", "activate")

	_, err := e.host.Invoke(context.Background(), e.req("acme.a.activate", `{"fail":true}`))
	we, ok := pool.AsWireError(err)
	require.True(t, ok, "%v", err)
	require.Equal(t, pool.ErrKindDomain, we.Kind)
	require.Equal(t, "acme.a.boom", we.Code)
	require.JSONEq(t, `{"why":"asked"}`, string(we.Params))
	require.Empty(t, e.items(), "both writes rolled back")

	res, err := e.host.Invoke(context.Background(), e.req("acme.a.activate", `{"fail":false}`))
	require.NoError(t, err)
	require.JSONEq(t, `{"count":2}`, string(res.Output))
	require.Equal(t, []string{"first", "second"}, e.items())
	require.Equal(t, 1, res.CommittedTxCount)

	recs := e.invocations("acme.a.activate")
	require.Len(t, recs, 2)
	require.Equal(t, []string{"error", "ok"}, []string{recs[0].Outcome, recs[1].Outcome})
	require.Equal(t, []int{0, 1}, []int{recs[0].Committed, recs[1].Committed})
	require.False(t, recs[0].Partial)
}

func TestTxCallOfOperationJoinsTheTransaction(t *testing.T) {
	e := newEnv(t)
	e.publish("acme.a", "1.0.0", jsPrelude+`
export const bulk = async (tx, input) => {
  await tx.insert(item, { label: "outer" });
  const r = await tx.call({ id: "acme.a.addLabel" }, { label: "inner" });
  if (input.fail) throw domain("acme.a.boom", {});
  return { seen: r.seen };
};
export const addLabel = async (tx, input) => {
  await tx.insert(item, { label: input.label });
  return { seen: await tx.count(item) };
};`)
	e.contract(command("acme.a.bulk", true, obj("fail", boolean()), obj("seen", num()), "acme.a.boom"), "acme.a", "1.0.0", "bulk")
	e.contract(manifest.Resource{ID: "acme.a.addLabel", Kind: manifest.ResOperation, Input: obj("label", str()), Output: obj("seen", num())}, "acme.a", "1.0.0", "addLabel")

	_, err := e.host.Invoke(context.Background(), e.req("acme.a.bulk", `{"fail":true}`))
	require.Error(t, err)
	require.Empty(t, e.items(), "outer and joined inner write rolled back together")

	res, err := e.host.Invoke(context.Background(), e.req("acme.a.bulk", `{"fail":false}`))
	require.NoError(t, err)
	require.JSONEq(t, `{"seen":2}`, string(res.Output), "the operation saw the uncommitted outer write: same tx")
	require.Equal(t, []string{"outer", "inner"}, e.items())

	parent := e.invocations("acme.a.bulk")[1]
	child := e.invocations("acme.a.addLabel")[1]
	require.Equal(t, parent.ID, child.Parent)
	require.Equal(t, parent.Root, child.Root)
	require.Equal(t, 1, parent.Committed)
	require.Zero(t, child.Committed, "the joined tx belongs to the invocation that owns it")
}

func TestTxCallOfOrdinaryCommandIsRejectedAtRuntime(t *testing.T) {
	e := newEnv(t)
	e.publish("acme.a", "1.0.0", jsPrelude+`
export const sneaky = async (tx) => {
  await tx.insert(item, { label: "before" });
  await tx.call({ id: "acme.a.plain" }, {});
};
export const plain = async () => ({ ok: true });`)
	e.contract(command("acme.a.sneaky", true, nil, nil), "acme.a", "1.0.0", "sneaky")
	e.contract(command("acme.a.plain", false, nil, obj("ok", boolean())), "acme.a", "1.0.0", "plain")

	_, err := e.host.Invoke(context.Background(), e.req("acme.a.sneaky", `null`))
	we, ok := pool.AsWireError(err)
	require.True(t, ok)
	require.Equal(t, "runtime.ordinaryCommandInTx", we.Code)
	require.Empty(t, e.items())
	require.Empty(t, e.invocations("acme.a.plain"), "the ordinary command never ran")
}

func TestNativeEntityWritesAreRejected(t *testing.T) {
	e := newEnv(t)
	e.publish("acme.a", "1.0.0", `export const write = async (tx) => tx.insert({ id: "core.native" }, { x: 1 });`)
	e.contract(command("acme.a.write", true, nil, nil), "acme.a", "1.0.0", "write")
	_, err := e.host.Invoke(context.Background(), e.req("acme.a.write", `null`))
	we, ok := pool.AsWireError(err)
	require.True(t, ok)
	require.Equal(t, pool.ErrKindDenied, we.Kind)
	require.Equal(t, "runtime.notWritable", we.Code)
}

func TestWorkerKilledInsideTxRollsBackAndNextInvocationSucceeds(t *testing.T) {
	e := newEnv(t)
	e.publish("acme.a", "1.0.0", jsPrelude+`
export const killer = async (ctx, input) => {
  await ctx.transaction(async (tx) => tx.insert(item, { label: "committed" }));
  if (input.stage === "afterCommit") process.kill(process.pid, "SIGKILL");
  await ctx.transaction(async (tx) => {
    await tx.insert(item, { label: "uncommitted" });
    process.kill(process.pid, "SIGKILL");
    await new Promise(() => {});
  });
};
export const ok = async () => ({ ok: true });`)
	e.contract(command("acme.a.killer", false, obj("stage", str()), nil), "acme.a", "1.0.0", "killer")
	e.contract(command("acme.a.ok", false, nil, obj("ok", boolean())), "acme.a", "1.0.0", "ok")

	res, err := e.host.Invoke(context.Background(), e.req("acme.a.killer", `{"stage":"inTx"}`))
	require.ErrorIs(t, err, pool.ErrWorkerLost)
	require.Equal(t, host.OutcomeWorkerLost, res.Outcome)
	require.Equal(t, []string{"committed"}, e.items(), "the open transaction was rolled back, the committed one stays")
	require.True(t, res.PartiallyCommitted, "one ctx.transaction had committed before the loss")
	require.Zero(t, e.idleInTx())

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	next, err := e.host.Invoke(ctx, e.req("acme.a.ok", `null`))
	require.NoError(t, err, "Go keeps running and the pool restarted the worker")
	require.JSONEq(t, `{"ok":true}`, string(next.Output))

	res, err = e.host.Invoke(ctx, e.req("acme.a.killer", `{"stage":"afterCommit"}`))
	require.ErrorIs(t, err, pool.ErrWorkerLost)
	require.True(t, res.PartiallyCommitted)
	require.Equal(t, 1, res.CommittedTxCount)
	recs := e.invocations("acme.a.killer")
	require.Len(t, recs, 2)
	for _, r := range recs {
		require.Equal(t, "worker_lost", r.Outcome)
		require.Equal(t, 1, r.Committed, "the counter committed with the data, so a lost reply is diagnosable")
		require.True(t, r.Partial)
	}
	require.Equal(t, []string{"committed", "committed"}, e.items(), "not retried")
}

func TestSelectedBindingIsStableWhileActiveBindingSwitches(t *testing.T) {
	e := newEnv(t, func(c *host.Config) {})
	slow := func(v int) string {
		return `export const run = async () => { await new Promise(r => setTimeout(r, 400)); return { v: ` + itoa(v) + ` }; };`
	}
	e.publish("acme.a", "1.0.0", slow(1))
	e.publish("acme.a", "2.0.0", slow(2))
	e.contract(command("acme.a.slow", false, nil, obj("v", num())), "acme.a", "1.0.0", "run")

	type outcome struct {
		res host.Result
		err error
	}
	first := make(chan outcome, 1)
	go func() {
		res, err := e.host.Invoke(context.Background(), e.req("acme.a.slow", `null`))
		first <- outcome{res, err}
	}()
	require.Eventually(t, func() bool { return len(e.invocations("acme.a.slow")) == 1 }, 5*time.Second, 10*time.Millisecond)

	_, err := e.bindings.Activate(context.Background(), "acme.a.slow", "acme.a", "2.0.0", "run", nil)
	require.NoError(t, err)

	got := <-first
	require.NoError(t, got.err)
	require.JSONEq(t, `{"v":1}`, string(got.res.Output), "the running invocation keeps its selected artifact")
	require.Equal(t, "1.0.0", got.res.Binding.Version)

	second, err := e.host.Invoke(context.Background(), e.req("acme.a.slow", `null`))
	require.NoError(t, err)
	require.JSONEq(t, `{"v":2}`, string(second.Output))

	recs := e.invocations("acme.a.slow")
	require.Equal(t, []string{"1.0.0", "2.0.0"}, []string{recs[0].Version, recs[1].Version})

	var open, total int
	require.NoError(t, e.db.QueryRow(context.Background(), `SELECT count(*) FILTER (WHERE active_to IS NULL), count(*) FROM extruntime.bindings WHERE target_id = 'acme.a.slow'`).Scan(&open, &total))
	require.Equal(t, []int{1, 2}, []int{open, total})
	_, err = e.db.Exec(context.Background(), `INSERT INTO extruntime.bindings (slot_kind, target_id, package, version, export) VALUES ('command', 'acme.a.slow', 'x', '1', 'run')`)
	require.Error(t, err, "a second open binding for one command is impossible")
}

func TestNoBindingMeansNoFallback(t *testing.T) {
	e := newEnv(t)
	e.catalog.res["acme.a.orphan"] = spi.ResolvedResource{Resource: command("acme.a.orphan", false, nil, nil)}
	_, err := e.host.Invoke(context.Background(), e.req("acme.a.orphan", `null`))
	require.ErrorIs(t, err, host.ErrNoBinding)
}

func TestStructuredErrorsInputValidationAndPermission(t *testing.T) {
	e := newEnv(t)
	e.publish("acme.a", "1.0.0", jsPrelude+`
export const declared = async () => { throw domain("acme.a.notActive", { status: "draft" }); };
export const undeclared = async () => { throw domain("acme.a.surprise", {}); };
export const badOut = async () => ({ wrong: true });`)
	declared := command("acme.a.declared", false, obj("n", num()), obj("ok", boolean()), "acme.a.notActive")
	declared.Permission = "acme.a.manage"
	e.contract(declared, "acme.a", "1.0.0", "declared")
	e.contract(command("acme.a.undeclared", false, nil, nil, "acme.a.notActive"), "acme.a", "1.0.0", "undeclared")
	e.contract(command("acme.a.badOut", false, nil, obj("ok", boolean())), "acme.a", "1.0.0", "badOut")

	_, err := e.host.Invoke(context.Background(), e.req("acme.a.declared", `{"n":1}`))
	we, _ := pool.AsWireError(err)
	require.Equal(t, pool.ErrKindDenied, we.Kind)

	_, err = e.host.Invoke(context.Background(), e.req("acme.a.declared", `{"n":"x"}`, "acme.a.manage"))
	we, _ = pool.AsWireError(err)
	require.Equal(t, pool.ErrKindValidation, we.Kind)
	require.Len(t, we.Issues, 1)
	require.JSONEq(t, `["n"]`, string(we.Issues[0].Path))
	require.Equal(t, "input", we.Issues[0].Phase)
	require.Empty(t, e.invocations("acme.a.declared"), "rejected before dispatch: no worker, no binding, no record")

	res, err := e.host.Invoke(context.Background(), e.req("acme.a.declared", `{"n":1}`, "acme.a.manage"))
	we, _ = pool.AsWireError(err)
	require.Equal(t, pool.ErrKindDomain, we.Kind)
	require.Equal(t, "acme.a.notActive", we.Code)
	require.JSONEq(t, `{"status":"draft"}`, string(we.Params))
	require.Equal(t, host.OutcomeError, res.Outcome)

	_, err = e.host.Invoke(context.Background(), e.req("acme.a.undeclared", `null`))
	we, _ = pool.AsWireError(err)
	require.Equal(t, pool.ErrKindInternal, we.Kind)
	require.Equal(t, "runtime.undeclaredDomainError", we.Code)

	_, err = e.host.Invoke(context.Background(), e.req("acme.a.badOut", `null`))
	we, _ = pool.AsWireError(err)
	require.Equal(t, "runtime.invalidOutput", we.Code)
}

func TestDeadlineRollsBackOpenTransaction(t *testing.T) {
	e := newEnv(t)
	e.publish("acme.a", "1.0.0", jsPrelude+`
export const hang = async (ctx) => ctx.transaction(async (tx) => {
  await tx.insert(item, { label: "never" });
  await new Promise(() => {});
});
export const ok = async () => ({ ok: true });`)
	e.contract(command("acme.a.hang", false, nil, nil), "acme.a", "1.0.0", "hang")
	e.contract(command("acme.a.ok", false, nil, obj("ok", boolean())), "acme.a", "1.0.0", "ok")

	ctx, cancel := context.WithTimeout(context.Background(), 400*time.Millisecond)
	defer cancel()
	res, err := e.host.Invoke(ctx, e.req("acme.a.hang", `null`))
	require.ErrorIs(t, err, context.DeadlineExceeded)
	require.Equal(t, host.OutcomeError, res.Outcome)
	require.Empty(t, e.items())
	require.Zero(t, e.idleInTx())

	ctx2, cancel2 := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel2()
	_, err = e.host.Invoke(ctx2, e.req("acme.a.ok", `null`))
	require.NoError(t, err)
}

func TestHostCallsOnOneHandleAreSerialized(t *testing.T) {
	e := newEnv(t)
	e.publish("acme.a", "1.0.0", jsPrelude+`
export const burst = async (tx) => {
  await Promise.all(Array.from({ length: 25 }, (_, i) => tx.insert(item, { label: "r" + i })));
  return { count: await tx.count(item) };
};`)
	e.contract(command("acme.a.burst", true, nil, obj("count", num())), "acme.a", "1.0.0", "burst")
	res, err := e.host.Invoke(context.Background(), e.req("acme.a.burst", `null`))
	require.NoError(t, err)
	require.JSONEq(t, `{"count":25}`, string(res.Output))
	require.Len(t, e.items(), 25)
}

func TestCommandCallsAnotherCommandThroughItsActiveBinding(t *testing.T) {
	e := newEnv(t)
	e.publish("acme.a", "1.0.0", `
export const outer = async (ctx) => ({ inner: await ctx.call({ id: "acme.b.inner" }, { n: 3 }) });`)
	e.publish("acme.b", "1.0.0", `export const inner = async (ctx, input) => ({ tripled: input.n * 3, pkg: "b" });`)
	e.contract(command("acme.a.outer", false, nil, nil), "acme.a", "1.0.0", "outer")
	e.contract(command("acme.b.inner", false, obj("n", num()), obj("tripled", num(), "pkg", str())), "acme.b", "1.0.0", "inner")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	res, err := e.host.Invoke(ctx, e.req("acme.a.outer", `null`))
	require.NoError(t, err, "nested invocation must not deadlock the one-worker pool")
	require.JSONEq(t, `{"inner":{"tripled":9,"pkg":"b"}}`, string(res.Output))
	outer, inner := e.invocations("acme.a.outer")[0], e.invocations("acme.b.inner")[0]
	require.Equal(t, outer.ID, inner.Parent)
	require.Equal(t, outer.Root, inner.Root)
}

func TestSubscriptionRunWritesIntentsInCallerTxAndDerivesStartKeys(t *testing.T) {
	e := newEnv(t)
	e.publish("acme.a", "1.0.0", jsPrelude+`
export const onRenewed = async (tx, event) => {
  const subject = { entity: "acme.test.item", key: "1" };
  await tx.emit({ id: "acme.a.renewed.v1" }, { subject, payload: { n: event.n } });
  await tx.start({ id: "acme.a.followUp" }, { n: 1 });
  await tx.start({ id: "acme.a.followUp" }, { n: 2 });
  await tx.start({ id: "acme.a.followUp" }, { n: 3 }, { key: "explicit" });
  await tx.notify({ id: "acme.a.reminder.v1" }, { subject, payload: { n: 1 }, to: [{ kind: "user", user: { entity: "core.user", key: "42" } }], channels: ["sms"] });
  return null;
};
export const noKey = async (tx) => tx.start({ id: "acme.a.followUp" }, {});`)
	art, _, err := e.art.ResolveArtifact(context.Background(), "acme.a", "1.0.0")
	require.NoError(t, err)
	binding := host.Binding{Package: "acme.a", Version: "1.0.0", Artifact: art, Export: "onRenewed"}

	runIn := func(commit bool, export, base string) (host.Result, error) {
		tx, err := e.db.Begin(context.Background())
		require.NoError(t, err)
		b := binding
		b.Export = export
		res, err := e.host.Run(context.Background(), host.RunRequest{
			Export:       host.Export{Kind: pool.KindSubscription, Resource: "acme.a.onRenewed", Binding: b},
			Input:        json.RawMessage(`{"n":5}`),
			Subject:      e.subject(),
			Trigger:      host.Trigger{Kind: host.TriggerSubscription, Ref: "acme.a.onRenewed"},
			Tx:           tx,
			Producer:     spi.Producer{EventID: "evt-9"},
			StartKeyBase: base,
		})
		if commit && err == nil {
			require.NoError(t, tx.Commit(context.Background()))
		} else {
			require.NoError(t, tx.Rollback(context.Background()))
		}
		return res, err
	}
	outboxRows := func() int {
		var n int
		require.NoError(t, e.db.QueryRow(context.Background(), `SELECT count(*) FROM host_outbox`).Scan(&n))
		return n
	}

	_, err = runIn(false, "onRenewed", "subscription:acme.a.onRenewed:evt-9")
	require.NoError(t, err)
	require.Zero(t, outboxRows(), "the event was written in the caller's tx, which rolled back")

	res, err := runIn(true, "onRenewed", "subscription:acme.a.onRenewed:evt-9")
	require.NoError(t, err)
	require.Equal(t, 1, outboxRows())
	require.Zero(t, res.CommittedTxCount, "the caller owns the commit")

	require.Len(t, e.outbox.events, 2)
	ev := e.outbox.events[1]
	require.Equal(t, "acme.a.renewed.v1", ev.Event)
	require.JSONEq(t, `{"n":5}`, string(ev.Payload))
	require.Equal(t, &spi.ObjectRef{Entity: "acme.test.item", Key: "1"}, ev.Subject)
	require.Equal(t, res.InvocationID, ev.Producer.InvocationID)
	require.Equal(t, "evt-9", ev.Producer.EventID)
	require.Equal(t, spi.Caller{Package: "acme.a", Version: "1.0.0"}, e.outbox.scopes[1].Caller)

	keys := []string{}
	for _, s := range e.starter.starts[3:] {
		keys = append(keys, s.Key)
	}
	require.Equal(t, []string{"subscription:acme.a.onRenewed:evt-9#1", "subscription:acme.a.onRenewed:evt-9#2", "explicit"}, keys)

	require.Len(t, e.notifier.notices, 2)
	n := e.notifier.notices[1]
	require.Equal(t, "1.0.0", n.Version, "notice pins the package version of its templates")
	require.Equal(t, []spi.Recipient{{Kind: spi.RecipientUser, UserID: 42}}, n.Recipients)
	require.Equal(t, []string{"sms"}, n.Channels)

	_, err = runIn(false, "noKey", "")
	we, ok := pool.AsWireError(err)
	require.True(t, ok)
	require.Equal(t, "runtime.startKeyRequired", we.Code)
}

func TestWorkflowHostCallsAreForwardedAndSuspendUnwinds(t *testing.T) {
	e := newEnv(t)
	e.publish("acme.a", "1.0.0", jsPrelude+`
export const flow = async (wf, input) => {
  const created = await wf.transaction("load", async (tx) => tx.insert(item, { label: "from-wf" }));
  try { await wf.sleep("cooldown", "1d"); } catch (e) { /* swallowed: must not matter */ }
  try { await wf.sleep("again", "1d"); } catch (e) { /* host calls after suspend never reach Go */ }
  return "unreachable " + created.key;
};`)
	art, _, err := e.art.ResolveArtifact(context.Background(), "acme.a", "1.0.0")
	require.NoError(t, err)

	res, err := e.host.Run(context.Background(), host.RunRequest{
		Export:  host.Export{Kind: pool.KindWorkflow, Resource: "acme.a.flow", Binding: host.Binding{Package: "acme.a", Version: "1.0.0", Artifact: art, Export: "flow"}},
		Input:   json.RawMessage(`{}`),
		Subject: e.subject(),
		Trigger: host.Trigger{Kind: host.TriggerWorkflow, Ref: "wfi-1"},
	})
	require.True(t, pool.IsSuspend(err), "%v", err)
	we, _ := pool.AsWireError(err)
	require.JSONEq(t, `{"wakeAt":"2031-01-01T00:00:00Z"}`, string(we.Suspend))
	require.Equal(t, host.OutcomeOK, res.Outcome, "suspension is the normal end of a run")
	require.Equal(t, []string{"wf.transaction.begin", "wf.transaction.end", "wf.sleep"}, e.wf.methods)
	require.Equal(t, []string{"from-wf"}, e.items(), "wf.transaction committed through the Go-owned tx")
	require.Equal(t, 1, res.CommittedTxCount)
	require.Equal(t, spi.Caller{Package: "acme.a", Version: "1.0.0"}, e.wf.scopes[0].Caller)
}

func TestSecretsAreLimitedToDeclaredCapabilities(t *testing.T) {
	e := newEnv(t, func(c *host.Config) { c.Secrets = secretDouble{} })
	e.art.secrets["acme.a"] = []string{"SMS_TOKEN"}
	e.publish("acme.a", "1.0.0", `
export const read = async (ctx) => {
  const ok = await ctx.secrets.get("SMS_TOKEN");
  let denied = null;
  try { await ctx.secrets.get("OTHER"); } catch (e) { denied = e.kind + ":" + e.code; }
  return { ok, denied };
};`)
	e.contract(command("acme.a.read", false, nil, obj("ok", str(), "denied", str())), "acme.a", "1.0.0", "read")
	res, err := e.host.Invoke(context.Background(), e.req("acme.a.read", `null`))
	require.NoError(t, err)
	require.JSONEq(t, `{"ok":"secret:SMS_TOKEN","denied":"denied:runtime.secretNotDeclared"}`, string(res.Output))
}

type secretDouble struct{}

func (secretDouble) Secret(_ context.Context, _ spi.Scope, name string) (string, error) {
	return "secret:" + name, nil
}
