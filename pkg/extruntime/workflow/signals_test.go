package workflow_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/iota-uz/iota-sdk/pkg/extruntime/host"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/host/hosttest"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/pool"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/spi"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/workflow"
)

// signalSource: a waiting receiver (`flow`) and the commands that signal it.
const signalSource = `
export const flow = async (wf) => {
  const g = await wf.waitFor("wait", go);
  return { note: g.note };
};
const target = (input) => ({ workflow: input.workflow ?? "acme.test.flow", instance: input.instance });
export const ping = async (tx, input) => {
  await tx.insert(item, { label: "sent:" + input.note });
  await tx.signal(target(input), "go", { note: input.note });
  if (input.fail) throw Object.assign(new Error("boom"), { kind: "domain", code: "acme.a.boom" });
  return {};
};
export const pingOrdinary = async (ctx, input) => {
  await ctx.signal(target(input), "go", { note: input.note });
  if (input.fail) throw Object.assign(new Error("boom"), { kind: "domain", code: "acme.a.boom" });
  return {};
};
export const probe = async (tx, input) => {
  let result;
  try {
    await tx.signal(target(input), input.name, input.payload);
    result = { ok: true };
  } catch (e) {
    result = { ok: false, code: e.code, kind: e.kind };
  }
  await tx.insert(item, { label: "after:" + input.name });
  return result;
};`

type signalFixture struct {
	env  *hosttest.Env
	st   *stack
	flow uuid.UUID
}

func newSignalFixture(t *testing.T) *signalFixture {
	t.Helper()
	env := hosttest.New(t)
	im := newImpls()
	publishFlow(t, env, im, "acme.a", "1.0.0", signalSource)
	im.Activate(flowID, "acme.a", "1.0.0")
	env.Subjects.Grant(7, "flow.signal")
	st := newStack(t, env, im)
	for _, c := range []struct {
		id, export string
		atomic     bool
	}{{"acme.a.ping", "ping", true}, {"acme.a.probe", "probe", true}, {"acme.a.pingOrdinary", "pingOrdinary", false}} {
		env.Catalog.Add(spi.ResolvedResource{Package: "acme.a", Version: "1.0.0", Resource: hosttest.Command(c.id, c.atomic, nil, nil, "acme.a.boom")})
		_, err := env.Bindings.Activate(context.Background(), c.id, "acme.a", "1.0.0", c.export, nil)
		require.NoError(t, err)
	}
	// The workflow now demands a permission to be signalled.
	res, _, _ := env.Catalog.Resolve(context.Background(), flowID)
	res.Resource.Permission = "flow.signal"
	env.Catalog.Add(res)

	f := &signalFixture{env: env, st: st}
	f.flow = st.start(7, "recv", `{"who":"w"}`)
	require.Equal(t, 1, st.drain())
	require.Equal(t, workflow.StatusWaiting, st.status(f.flow))
	return f
}

func (f *signalFixture) invoke(subject spi.Subject, contract string, input map[string]any) (host.Result, error) {
	in, err := json.Marshal(input)
	require.NoError(f.st.t, err)
	return f.st.host.Invoke(context.Background(), host.Request{Contract: contract, Input: in, Subject: subject, Trigger: host.Trigger{Kind: host.TriggerUI, Ref: "test"}})
}

func (f *signalFixture) user(perms ...string) spi.Subject { return f.env.Subject(7, perms...) }

func (f *signalFixture) probe(subject spi.Subject, instance, name string, payload any, extra ...string) map[string]any {
	f.st.t.Helper()
	in := map[string]any{"instance": instance, "name": name, "payload": payload}
	if len(extra) > 0 {
		in["workflow"] = extra[0]
	}
	res, err := f.invoke(subject, "acme.a.probe", in)
	require.NoError(f.st.t, err)
	var out map[string]any
	require.NoError(f.st.t, json.Unmarshal(res.Output, &out))
	return out
}

func TestTxSignalIsWrittenAtomicallyWithTheDataAndWakesTheInstance(t *testing.T) {
	f := newSignalFixture(t)
	st, env := f.st, f.env

	_, err := f.invoke(f.user("flow.signal"), "acme.a.ping", map[string]any{"instance": f.flow.String(), "note": "lost", "fail": true})
	we, ok := pool.AsWireError(err)
	require.True(t, ok, "%v", err)
	require.Equal(t, "acme.a.boom", we.Code)
	require.Empty(t, env.Items(), "data rolled back")
	require.Empty(t, st.history(f.flow).Signals, "and the signal with it")
	require.Zero(t, st.drain(), "nothing to wake")

	_, err = f.invoke(f.user("flow.signal"), "acme.a.ping", map[string]any{"instance": f.flow.String(), "note": "hello"})
	require.NoError(t, err)
	require.Equal(t, []string{"sent:hello"}, env.Items())
	sigs := st.history(f.flow).Signals
	require.Len(t, sigs, 1)
	require.Equal(t, "go", sigs[0].Name)
	require.JSONEq(t, `{"note":"hello"}`, string(sigs[0].Payload))
	require.NotNil(t, sigs[0].SenderUserID)
	require.Equal(t, int64(7), *sigs[0].SenderUserID)

	require.Equal(t, 1, st.drain(), "the signal woke the waiting instance")
	require.Equal(t, workflow.StatusCompleted, st.status(f.flow))
	require.JSONEq(t, `{"note":"hello"}`, string(st.instance(f.flow).Result))
}

func TestInvokeSignalCommitsInItsOwnShortTransaction(t *testing.T) {
	f := newSignalFixture(t)
	_, err := f.invoke(f.user("flow.signal"), "acme.a.pingOrdinary", map[string]any{"instance": f.flow.String(), "note": "own-tx", "fail": true})
	require.Error(t, err, "the command failed afterwards")
	require.Len(t, f.st.history(f.flow).Signals, 1, "the signal had already been committed")
	require.Equal(t, 1, f.st.drain())
	require.JSONEq(t, `{"note":"own-tx"}`, string(f.st.instance(f.flow).Result))
}

func TestSignalErrorsAreTypedAndLeaveTheTransactionUsable(t *testing.T) {
	f := newSignalFixture(t)
	id := f.flow.String()
	allowed := f.user("flow.signal")
	fail := func(code, kind string) map[string]any { return map[string]any{"ok": false, "code": code, "kind": kind} }

	cases := []struct {
		name string
		got  map[string]any
		want map[string]any
	}{
		{"unknown instance", f.probe(allowed, uuid.NewString(), "go", map[string]string{"note": "x"}), fail(workflow.CodeInstanceNotFound, "internal")},
		{"not an instance id", f.probe(allowed, "nope", "go", map[string]string{"note": "x"}), fail(workflow.CodeInstanceNotFound, "internal")},
		{"undeclared signal", f.probe(allowed, id, "stop", map[string]string{"note": "x"}), fail(workflow.CodeUnknownSignal, "internal")},
		{"payload does not match the schema", f.probe(allowed, id, "go", map[string]int{"note": 1}), fail("runtime.invalidValue", "validation")},
		{"another workflow", f.probe(allowed, id, "go", map[string]string{"note": "x"}, "acme.test.other"), fail(workflow.CodeInstanceMismatch, "internal")},
		{"user without the permission", f.probe(f.env.Subject(8, "flow.signal"), id, "go", map[string]string{"note": "x"}), fail(workflow.CodeSignalDenied, "denied")},
	}
	for _, c := range cases {
		require.Equal(t, c.want, c.got, c.name)
	}
	require.Empty(t, f.st.history(f.flow).Signals, "no rejected signal was written")
	require.Len(t, f.env.Items(), len(cases), "the transaction stayed usable after every rejection: the insert after each probe committed")

	t.Run("another tenant cannot see the instance", func(t *testing.T) {
		other := spi.Subject{TenantID: f.env.NewTenant(), UserID: 7, Kind: spi.SubjectInitiator, Permissions: []string{"flow.signal"}}
		require.Equal(t, fail(workflow.CodeInstanceNotFound, "internal"), f.probe(other, id, "go", map[string]string{"note": "x"}))
		require.Empty(t, f.st.history(f.flow).Signals)
	})

	t.Run("rights are the current ones, not those the session carried", func(t *testing.T) {
		f.env.Subjects.Grant(7) // revoked after the session started
		require.Equal(t, fail(workflow.CodeSignalDenied, "denied"), f.probe(allowed, id, "go", map[string]string{"note": "x"}))
		f.env.Subjects.Block(7, "disabled")
		require.Equal(t, fail(workflow.CodeSignalDenied, "denied"), f.probe(allowed, id, "go", map[string]string{"note": "x"}))
		f.env.Subjects.Grant(7, "flow.signal")
		require.Equal(t, map[string]any{"ok": true}, f.probe(allowed, id, "go", map[string]string{"note": "now"}))
		require.Len(t, f.st.history(f.flow).Signals, 1)
	})
}

func TestSignalToATerminatedOrFinishedInstanceIsRejectedWithATypedError(t *testing.T) {
	f := newSignalFixture(t)
	allowed := f.user("flow.signal")
	payload := map[string]string{"note": "x"}
	notAccepting := map[string]any{"ok": false, "code": workflow.CodeInstanceNotAccepting, "kind": "internal"}

	require.NoError(t, f.st.engine.Terminate(context.Background(), f.env.Tenant, f.flow))
	require.Equal(t, notAccepting, f.probe(allowed, f.flow.String(), "go", payload), "terminated")

	done := f.st.start(7, "recv2", `{"who":"w"}`)
	require.Equal(t, 1, f.st.drain())
	require.NoError(t, f.st.signal(done, "go", `{"note":"fin"}`, ""))
	require.Equal(t, 1, f.st.drain())
	require.Equal(t, workflow.StatusCompleted, f.st.status(done))
	require.Equal(t, notAccepting, f.probe(allowed, done.String(), "go", payload), "completed")
	require.Empty(t, f.st.history(f.flow).Signals)
}

func TestSignalInTxRepeatedWithTheSameKeyIsANoOp(t *testing.T) {
	f := newSignalFixture(t)
	for range 2 {
		tx, err := f.env.DB.Begin(context.Background())
		require.NoError(t, err)
		require.NoError(t, f.st.engine.SignalInTx(context.Background(), tx, spi.Scope{Subject: f.user("flow.signal")}, host.SignalIntent{
			Workflow: flowID, Instance: f.flow.String(), Name: "go", Payload: json.RawMessage(`{"note":"k"}`), Key: "dedupe-1",
		}))
		require.NoError(t, tx.Commit(context.Background()))
	}
	require.Len(t, f.st.history(f.flow).Signals, 1)
}
