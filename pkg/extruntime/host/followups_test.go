package host_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/iota-uz/iota-sdk/pkg/extruntime/host"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/manifest"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/pool"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/spi"
)

// rawPrelude gives a hand-written artifact the means to bypass the SDK context
// objects altogether: it writes `call` frames of the wire protocol straight to
// the worker's stdout (fd 1) and reads the `reply` frames from stdin, exactly
// as a compromised or buggy package could. The host router has to hold on its
// own.
const rawPrelude = `import { writeSync } from "node:fs";
let rawId = 900000;
const rawWaiting = new Map();
let rawBuf = Buffer.alloc(0);
process.stdin.on("data", (chunk) => {
  rawBuf = Buffer.concat([rawBuf, chunk]);
  while (rawBuf.length >= 4) {
    const n = rawBuf.readUInt32BE(0);
    if (rawBuf.length < 4 + n) break;
    const frame = JSON.parse(rawBuf.subarray(4, 4 + n).toString("utf8"));
    rawBuf = rawBuf.subarray(4 + n);
    if (frame.t === "reply" && rawWaiting.has(frame.id)) {
      rawWaiting.get(frame.id)(frame.ok ? { ok: true, value: frame.value } : { ok: false, code: frame.error.code, kind: frame.error.kind });
      rawWaiting.delete(frame.id);
    }
  }
});
const rawCall = (inv, method, args) => new Promise((resolve) => {
  const id = ++rawId;
  rawWaiting.set(id, resolve);
  const payload = Buffer.from(JSON.stringify({ t: "call", inv, id, method, args }), "utf8");
  const head = Buffer.alloc(4);
  head.writeUInt32BE(payload.length, 0);
  writeSync(1, Buffer.concat([head, payload]));
});
const probe = async (fn) => {
  try { return { ok: true, value: await fn() }; } catch (e) { return { ok: false, code: e.code ?? null, kind: e.kind ?? null }; }
};
const WRITES = ["tx.begin", "tx.end", "tx.insert", "tx.update", "tx.delete", "tx.putOverlay", "tx.emit", "tx.notify", "tx.start", "tx.signal", "tx.call", "invoke.command", "invoke.signal", "wf.call", "wf.start", "wf.sleep"];
const rawWrites = async (inv) => {
  const out = {};
  for (const m of WRITES) {
    out[m] = (await rawCall(inv, m, { tx: "tx-x", entity: "acme.test.item", values: { label: "sneaky" }, key: "1", patch: {}, overlay: "o", baseKey: "1", event: "e", notice: "n", workflow: "w", input: null, instance: "i", name: "go", payload: {}, contract: "c", command: "c" })).code ?? "allowed";
  }
  return out;
};
const rejectWith = (params) => Object.assign(new Error("probe"), { kind: "validation", code: "probe", issues: [{ code: "probe", params }] });
`

func probeResult(t *testing.T, out json.RawMessage) map[string]any {
	t.Helper()
	var m map[string]any
	require.NoError(t, json.Unmarshal(out, &m), string(out))
	return m
}

func TestTxQueryAndQueryQueryRunTheActiveImplementationInTheCallersReadView(t *testing.T) {
	e := newEnv(t)
	e.publish("acme.a", "1.0.0", jsPrelude+rawPrelude+`
export const txProbe = async (tx, input) => {
  if (input.insert) await tx.insert(item, { label: "u" });
  return probe(() => tx.query({ id: input.query }, input.arg));
};
export const qProbe = async (ctx, input) => probe(() => ctx.query({ id: input.query }, input.arg));
export const countItems = async (q, input) => ({ n: (await q.count(item)) + input.min });
export const badOutput = async () => ({ n: "x" });
export const snap = async (ctx, input) => {
  const before = await ctx.query.count(item);
  await fetch(input.url);
  const viaQuery = await ctx.query({ id: "acme.a.countItems" }, { min: 0 });
  const direct = await ctx.query.count(item);
  return { before, viaQuery: viaQuery.n, direct };
};`)
	e.contract(command("acme.a.txProbe", true, nil, nil), "acme.a", "1.0.0", "txProbe")
	e.contract(command("acme.a.qProbe", false, nil, nil), "acme.a", "1.0.0", "qProbe")
	e.contract(command("acme.a.snap", false, obj("url", str()), nil), "acme.a", "1.0.0", "snap")
	countIn, countOut := obj("min", num()), obj("n", num())
	e.contract(manifest.Resource{ID: "acme.a.countItems", Kind: manifest.ResQuery, Input: countIn, Output: countOut, Permission: "acme.read"}, "acme.a", "1.0.0", "countItems")
	e.contract(manifest.Resource{ID: "acme.a.badOutput", Kind: manifest.ResQuery, Output: countOut}, "acme.a", "1.0.0", "badOutput")
	e.catalog.res["acme.a.nativeList"] = spi.ResolvedResource{Native: true, Resource: manifest.Resource{ID: "acme.a.nativeList", Kind: manifest.ResQuery}}
	e.catalog.res["acme.a.unbound"] = spi.ResolvedResource{Package: "acme.a", Version: "1.0.0", Resource: manifest.Resource{ID: "acme.a.unbound", Kind: manifest.ResQuery}}

	run := func(contract, query, arg string, insert bool, perms ...string) map[string]any {
		t.Helper()
		in, _ := json.Marshal(map[string]any{"query": query, "arg": json.RawMessage(arg), "insert": insert})
		res, err := e.host.Invoke(context.Background(), e.req(contract, string(in), perms...))
		require.NoError(t, err)
		return probeResult(t, res.Output)
	}
	failure := func(got map[string]any) [2]any { return [2]any{got["code"], got["kind"]} }

	t.Run("tx.query sees the uncommitted rows of the transaction it joins", func(t *testing.T) {
		got := run("acme.a.txProbe", "acme.a.countItems", `{"min":10}`, true, "acme.read")
		require.Equal(t, map[string]any{"ok": true, "value": map[string]any{"n": float64(11)}}, got, "1 uncommitted row + min")
		require.Equal(t, []string{"u"}, e.items(), "the atomic command committed")

		parent := e.invocations("acme.a.txProbe")
		nested := e.invocations("acme.a.countItems")
		require.Len(t, parent, 1)
		require.Len(t, nested, 1)
		require.Equal(t, parent[0].ID, nested[0].Parent, "the nested query is recorded with its parent")
		require.Equal(t, parent[0].Root, nested[0].Root)
		require.Equal(t, []string{"ok", "acme.a", "countItems"}, []string{nested[0].Outcome, nested[0].Package, nested[0].Export})
	})

	t.Run("input and output are validated against the contract", func(t *testing.T) {
		require.Equal(t, [2]any{"runtime.invalidValue", "validation"}, failure(run("acme.a.txProbe", "acme.a.countItems", `{"min":"x"}`, false, "acme.read")))
		require.Equal(t, [2]any{"runtime.invalidOutput", "internal"}, failure(run("acme.a.txProbe", "acme.a.badOutput", `null`, false)))
	})

	t.Run("the query permission is enforced for the caller", func(t *testing.T) {
		require.Equal(t, [2]any{"runtime.permissionDenied", "denied"}, failure(run("acme.a.txProbe", "acme.a.countItems", `{"min":0}`, false)))
		require.Equal(t, [2]any{"runtime.permissionDenied", "denied"}, failure(run("acme.a.qProbe", "acme.a.countItems", `{"min":0}`, false)))
	})

	t.Run("native and unimplemented queries report nativeQueryUnavailable; non-queries are refused", func(t *testing.T) {
		for _, via := range []string{"acme.a.txProbe", "acme.a.qProbe"} {
			call := func(q string) map[string]any { return run(via, q, `null`, false) }
			require.Equal(t, [2]any{host.CodeNativeQueryUnavailable, "internal"}, failure(call("acme.a.nativeList")), via)
			require.Equal(t, [2]any{host.CodeNativeQueryUnavailable, "internal"}, failure(call("acme.a.unbound")), via)
			require.Equal(t, [2]any{host.CodeNotQuery, "internal"}, failure(call("acme.a.txProbe")), via)
		}
	})

	t.Run("q.query reads the same snapshot as the rest of the invocation", func(t *testing.T) {
		before := len(e.items())
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			_, err := e.db.Exec(context.Background(), `INSERT INTO host_items (tenant_id, label) VALUES ($1, 'concurrent')`, e.tenant)
			require.NoError(t, err)
			_, _ = w.Write([]byte(`{}`))
		}))
		defer srv.Close()
		res, err := e.host.Invoke(context.Background(), e.req("acme.a.snap", `{"url":"`+srv.URL+`"}`, "acme.read"))
		require.NoError(t, err)
		require.JSONEq(t, `{"before":`+itoa(before)+`,"viaQuery":`+itoa(before)+`,"direct":`+itoa(before)+`}`, string(res.Output),
			"a row committed meanwhile is invisible to the query run inside the snapshot")
		require.Len(t, e.items(), before+1)
		require.Zero(t, e.idleInTx())
	})
}

func TestGuardsAndQueriesAreReadOnlyEvenForAnArtifactThatBypassesTheSDKContext(t *testing.T) {
	e := newEnv(t)
	guard := rawPrelude + `
export const check = async (ctx) => { throw rejectWith(await rawWrites(ctx.invocation.id)); };`
	e.publish("acme.a", "1.0.0", jsPrelude+`
export const renew = async (ctx) => ({ ok: true });
export const add = async (tx) => ({ ok: true });
export const fill = async (ctx) => ctx.transaction(async (tx) => tx.insert(item, { label: "row" }));`)
	e.publish("acme.q", "1.0.0", jsPrelude+rawPrelude+`
export const sneaky = async (q) => ({ writes: await rawWrites(q.invocation.id) });`)
	e.contract(command("acme.a.renew", false, nil, nil), "acme.a", "1.0.0", "renew")
	e.contract(command("acme.a.add", true, nil, nil), "acme.a", "1.0.0", "add")
	e.contract(command("acme.a.fill", false, nil, nil), "acme.a", "1.0.0", "fill")
	e.contract(manifest.Resource{ID: "acme.q.sneaky", Kind: manifest.ResQuery}, "acme.q", "1.0.0", "sneaky")

	readOnly := func(t *testing.T, params json.RawMessage) {
		t.Helper()
		var got map[string]string
		require.NoError(t, json.Unmarshal(params, &got))
		require.Len(t, got, 16)
		for method, code := range got {
			require.Equal(t, host.CodeReadOnlyContext, code, method)
		}
	}

	for _, tc := range []struct {
		name, target, contract string
		phase                  manifest.GuardPhase
	}{
		{"precheck of an ordinary command", "acme.a.renew", "acme.a.renew", manifest.GuardInput},
		{"operation guard of an atomic command", "acme.a.add", "acme.a.add", manifest.GuardOperation},
		{"write rule of an entity", itemEntity, "acme.a.fill", manifest.GuardWrite},
	} {
		t.Run(tc.name, func(t *testing.T) {
			e.catalog.guards = nil
			e.addGuard("acme.g.sneaky", tc.target, tc.phase, "acme.g", guard)
			_, err := e.host.Invoke(context.Background(), e.req(tc.contract, `null`))
			issues := issuesOf(t, err)
			require.Len(t, issues, 1)
			require.Equal(t, "probe", issues[0].Code)
			readOnly(t, issues[0].Params)
			require.Empty(t, e.items(), "nothing was written")
			require.Empty(t, e.signalRows())
			require.Empty(t, e.outbox.events)
			require.Zero(t, e.idleInTx())
		})
	}

	t.Run("a query invocation", func(t *testing.T) {
		res, err := e.host.Invoke(context.Background(), e.req("acme.q.sneaky", `null`))
		require.NoError(t, err)
		var out struct{ Writes json.RawMessage }
		require.NoError(t, json.Unmarshal(res.Output, &out))
		readOnly(t, out.Writes)
		require.Empty(t, e.items())
	})
}

func TestOrdinaryIntentsAreRefusedWhileAGoTransactionIsOpenInTheSameInvocation(t *testing.T) {
	e := newEnv(t)
	e.publish("acme.a", "1.0.0", jsPrelude+rawPrelude+`
const signal = { workflow: "acme.w", instance: "i-1", name: "go", payload: {} };
export const noop = async () => ({});
export const sdk = async (ctx) => ctx.transaction(async () => ({
  call: (await probe(() => ctx.call({ id: "acme.a.noop" }, null))).code,
  signal: (await probe(() => ctx.signal(signal, "go", {}))).code,
}));
export const raw = async (ctx) => {
  const inv = ctx.invocation.id;
  const begun = await rawCall(inv, "tx.begin", {});
  const during = {
    call: (await rawCall(inv, "invoke.command", { command: "acme.a.noop", input: null })).code,
    signal: (await rawCall(inv, "invoke.signal", signal)).code,
  };
  await rawCall(inv, "tx.end", { tx: begun.value.tx, outcome: "rollback" });
  const after = {
    call: (await rawCall(inv, "invoke.command", { command: "acme.a.noop", input: null })).ok,
    signal: (await rawCall(inv, "invoke.signal", signal)).ok,
  };
  return { during, after };
};
export const rawAtomic = async (tx) => ({
  call: (await rawCall(tx.invocation.id, "invoke.command", { command: "acme.a.noop", input: null })).code,
  signal: (await rawCall(tx.invocation.id, "invoke.signal", signal)).code,
});
export const rawFlow = async (wf) => {
  const inv = await wf.now("inv");
  const begun = await rawCall(inv, "wf.transaction.begin", { name: "t", n: 0 });
  const during = (await rawCall(inv, "wf.call", { name: "c", n: 0, command: "acme.a.noop", input: null })).code;
  await rawCall(inv, "wf.transaction.end", { name: "t", n: 0, tx: begun.value.tx, outcome: "rollback" });
  const after = (await rawCall(inv, "wf.call", { name: "c", n: 0, command: "acme.a.noop", input: null })).ok;
  return { during, after };
};`)
	e.contract(command("acme.a.noop", false, nil, nil), "acme.a", "1.0.0", "noop")
	e.contract(command("acme.a.sdk", false, nil, nil), "acme.a", "1.0.0", "sdk")
	e.contract(command("acme.a.raw", false, nil, nil), "acme.a", "1.0.0", "raw")
	e.contract(command("acme.a.rawAtomic", true, nil, nil), "acme.a", "1.0.0", "rawAtomic")

	t.Run("through the SDK context (worker side)", func(t *testing.T) {
		res, err := e.host.Invoke(context.Background(), e.req("acme.a.sdk", `null`))
		require.NoError(t, err)
		require.JSONEq(t, `{"call":"runtime.intentInTransaction","signal":"runtime.intentInTransaction"}`, string(res.Output))
		require.Empty(t, e.invocations("acme.a.noop"), "no nested invocation was started")
		require.Empty(t, e.signalRows())
	})

	t.Run("bypassing the SDK context (host side)", func(t *testing.T) {
		res, err := e.host.Invoke(context.Background(), e.req("acme.a.raw", `null`))
		require.NoError(t, err)
		require.JSONEq(t, `{"during":{"call":"runtime.intentInTransaction","signal":"runtime.intentInTransaction"},"after":{"call":true,"signal":true}}`, string(res.Output))
		require.Len(t, e.invocations("acme.a.noop"), 1, "only the call after the transaction ended ran")
		require.Equal(t, []string{"go:{}"}, e.signalRows())
	})

	t.Run("an atomic command holds a transaction for its whole run", func(t *testing.T) {
		res, err := e.host.Invoke(context.Background(), e.req("acme.a.rawAtomic", `null`))
		require.NoError(t, err)
		require.JSONEq(t, `{"call":"runtime.intentInTransaction","signal":"runtime.intentInTransaction"}`, string(res.Output))
		require.Len(t, e.invocations("acme.a.noop"), 1)
	})

	t.Run("wf.call while a wf.transaction is open", func(t *testing.T) {
		art, secrets, err := e.art.ResolveArtifact(context.Background(), "acme.a", "1.0.0")
		require.NoError(t, err)
		res, err := e.host.Run(context.Background(), host.RunRequest{
			Export: host.Export{Kind: pool.KindWorkflow, Resource: "acme.a.flow", Binding: host.Binding{
				Package: "acme.a", Version: "1.0.0", Export: "rawFlow", Artifact: art, Secrets: secrets,
			}},
			Input:   json.RawMessage(`null`),
			Subject: e.subject(),
			Trigger: host.Trigger{Kind: host.TriggerWorkflow, Ref: "inst"},
		})
		require.NoError(t, err)
		require.JSONEq(t, `{"during":"runtime.intentInTransaction","after":true}`, string(res.Output))
		require.Len(t, e.invocations("acme.a.noop"), 2, "the wf.call after the transaction ended ran")
	})
	require.Zero(t, e.idleInTx())
}

func TestTxSignalIsAtomicWithTheDataAndInvokeSignalIsItsOwnTransaction(t *testing.T) {
	e := newEnv(t)
	e.publish("acme.a", "1.0.0", jsPrelude+`
const target = { workflow: "acme.w", instance: "inst-1" };
export const atomic = async (tx, input) => {
  await tx.insert(item, { label: "data" });
  await tx.signal(target, "go", { note: "n" });
  if (input.fail) throw domain("acme.a.boom", {});
  return {};
};
export const ordinary = async (ctx, input) => {
  await ctx.signal(target, "go", { note: "o" });
  if (input.fail) throw domain("acme.a.boom", {});
  return {};
};
export const caught = async (ctx) => {
  try { await ctx.signal(target, "go", {}); return { code: null }; } catch (e) { return { code: e.code, kind: e.kind }; }
};`)
	in := obj("fail", boolean())
	e.contract(command("acme.a.atomic", true, in, obj(), "acme.a.boom"), "acme.a", "1.0.0", "atomic")
	e.contract(command("acme.a.ordinary", false, in, obj(), "acme.a.boom"), "acme.a", "1.0.0", "ordinary")
	e.contract(command("acme.a.caught", false, nil, nil), "acme.a", "1.0.0", "caught")

	_, err := e.host.Invoke(context.Background(), e.req("acme.a.atomic", `{"fail":true}`))
	we, ok := pool.AsWireError(err)
	require.True(t, ok)
	require.Equal(t, "acme.a.boom", we.Code)
	require.Empty(t, e.items(), "data rolled back")
	require.Empty(t, e.signalRows(), "and so did the signal written in the same transaction")

	res, err := e.host.Invoke(context.Background(), e.req("acme.a.atomic", `{"fail":false}`))
	require.NoError(t, err)
	require.Equal(t, []string{"data"}, e.items())
	require.Equal(t, []string{`go:{"note": "n"}`}, e.signalRows())
	require.Len(t, e.signals.intents, 2, "the rolled-back attempt reached the engine too")
	intent := e.signals.intents[1]
	require.Equal(t, "acme.w", intent.Workflow)
	require.Equal(t, "inst-1", intent.Instance)
	require.Equal(t, "go", intent.Name)
	require.JSONEq(t, `{"note":"n"}`, string(intent.Payload))
	require.Equal(t, res.InvocationID, intent.Producer.InvocationID)
	require.Equal(t, int64(7), e.signals.scopes[1].Subject.UserID)
	require.Equal(t, spi.Caller{Package: "acme.a", Version: "1.0.0"}, e.signals.scopes[1].Caller)

	_, err = e.host.Invoke(context.Background(), e.req("acme.a.ordinary", `{"fail":true}`))
	require.Error(t, err)
	require.Equal(t, []string{`go:{"note": "n"}`, `go:{"note": "o"}`}, e.signalRows(), "invoke.signal committed in its own transaction, whatever happened next")

	e.signals.mu.Lock()
	e.signals.fail = &pool.WireError{Kind: pool.ErrKindInternal, Code: "workflow.instanceNotFound", Message: "no such instance"}
	e.signals.mu.Unlock()
	res, err = e.host.Invoke(context.Background(), e.req("acme.a.caught", `null`))
	require.NoError(t, err)
	require.JSONEq(t, `{"code":"workflow.instanceNotFound","kind":"internal"}`, string(res.Output), "typed engine errors reach the package unchanged")
	require.Len(t, e.signalRows(), 2)
	require.Zero(t, e.idleInTx())
}

func TestSignalsFailExplicitlyWithoutAnEngine(t *testing.T) {
	e := newEnv(t, func(c *host.Config) { c.Signals = nil })
	e.publish("acme.a", "1.0.0", jsPrelude+`
export const caught = async (ctx) => {
  try { await ctx.signal({ workflow: "w", instance: "i" }, "go", {}); return { code: null }; } catch (e) { return { code: e.code }; }
};`)
	e.contract(command("acme.a.caught", false, nil, nil), "acme.a", "1.0.0", "caught")
	res, err := e.host.Invoke(context.Background(), e.req("acme.a.caught", `null`))
	require.NoError(t, err)
	require.JSONEq(t, `{"code":"runtime.notConfigured"}`, string(res.Output))
}

func TestAResultWithNULStillFinishesTheInvocationRecord(t *testing.T) {
	e := newEnv(t)
	e.publish("acme.a", "1.0.0", jsPrelude+`export const bin = async () => ({ s: "a\u0000b" });`)
	e.contract(command("acme.a.bin", false, nil, nil), "acme.a", "1.0.0", "bin")
	res, err := e.host.Invoke(context.Background(), e.req("acme.a.bin", `null`))
	require.NoError(t, err)
	require.JSONEq(t, `{"s":"a\u0000b"}`, string(res.Output), "the caller gets the real value")
	recs := e.invocations("acme.a.bin")
	require.Len(t, recs, 1)
	require.Equal(t, "ok", recs[0].Outcome, "the outcome is recorded though jsonb cannot hold U+0000")
	require.True(t, recs[0].Finished)
	require.Contains(t, *recs[0].Result, "unrecordable")
}
