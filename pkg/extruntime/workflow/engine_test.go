package workflow_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/iota-uz/iota-sdk/pkg/extruntime/host"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/host/hosttest"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/manifest"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/spi"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/workflow"
)

func TestTransactionLoopRecordsDistinctOccurrencesAndReplaysWithoutDuplicates(t *testing.T) {
	env := hosttest.New(t)
	im := newImpls()
	publishFlow(t, env, im, "acme.a", "1.0.0", `
export const flow = async (wf, input) => {
  const keys = [];
  for (let i = 0; i < 3; i++) {
    keys.push(await wf.transaction("add", async (tx) => (await tx.insert(item, { label: input.who + i })).key));
  }
  const g = await wf.waitFor("wait", go);
  keys.push(await wf.transaction("after", async (tx) => (await tx.insert(item, { label: g.note })).key));
  return { keys };
};`)
	im.Activate(flowID, "acme.a", "1.0.0")
	env.Subjects.Grant(7)
	st := newStack(t, env, im)

	id := st.start(7, "k1", `{"who":"w"}`)
	require.Equal(t, 1, st.drain())
	require.Equal(t, workflow.StatusWaiting, st.status(id))
	require.Equal(t, []string{"w0", "w1", "w2"}, env.Items())
	h := st.history(id)
	for n := 0; n < 3; n++ {
		o, ok := occ(h, "add", n)
		require.True(t, ok, "occurrence add#%d", n)
		require.Equal(t, workflow.OccCompleted, o.Status)
	}
	require.Equal(t, 3, st.calls.count("wf.transaction.begin"))

	st.calls.reset()
	require.NoError(t, st.signal(id, "go", `{"note":"done"}`, ""))
	require.Equal(t, 1, st.drain())
	require.Equal(t, workflow.StatusCompleted, st.status(id))
	require.Equal(t, []string{"w0", "w1", "w2", "done"}, env.Items(), "replay did not repeat the committed DB effects")
	require.JSONEq(t, `{"keys":["1","2","3","4"]}`, string(st.instance(id).Result))
	require.Equal(t, 1, st.calls.count("wf.transaction.begin"), "the three completed transactions were answered inside Node from the journal")
	require.Equal(t, 1, st.calls.count("wf.waitFor"), "only the unmet wait reached Go on replay")
}

func TestStepRerunsAfterWorkerCrashWithTheSameIdempotencyKey(t *testing.T) {
	env := hosttest.New(t)
	im := newImpls()
	var mu sync.Mutex
	var keys []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		keys = append(keys, r.URL.Query().Get("key"))
		first := len(keys) == 1
		mu.Unlock()
		_, _ = fmt.Fprintf(w, `{"crash":%v}`, first)
	}))
	defer srv.Close()
	publishFlow(t, env, im, "acme.a", "1.0.0", `
export const flow = async (wf, input) => {
  return wf.step("sms", async ({ idempotencyKey }) => {
    const j = await (await fetch(input.url + "?key=" + encodeURIComponent(idempotencyKey))).json();
    if (j.crash) process.exit(1);
    return { sent: idempotencyKey };
  });
};`)
	im.Activate(flowID, "acme.a", "1.0.0")
	env.Subjects.Grant(7)
	st := newStack(t, env, im)

	id := st.start(7, "k", `{"who":"w","url":"`+srv.URL+`"}`)
	st.until(10*time.Second, func() bool { return st.status(id) == workflow.StatusCompleted }, "instance completes after the crashed run is replayed")

	mu.Lock()
	defer mu.Unlock()
	want := id.String() + ":sms:0"
	require.Equal(t, []string{want, want}, keys, "the effect ran twice (at-least-once) with the same provider idempotency key")
	o, ok := occ(st.history(id), "sms", 0)
	require.True(t, ok)
	require.Equal(t, 2, o.Attempts)
	require.JSONEq(t, `{"sent":"`+want+`"}`, string(st.instance(id).Result))
}

func TestHumanTaskDecisionContinuesAfterEngineRestart(t *testing.T) {
	env := hosttest.New(t)
	im := newImpls()
	publishFlow(t, env, im, "acme.a", "1.0.0", `
export const flow = async (wf, input) => {
  const d = await wf.task("approve", {
    subject: { entity: "acme.test.item", key: "1" },
    assignee: { entity: "core.user", key: "7" },
    decision: { type: { kind: "object", properties: [{ name: "approved", type: { kind: "bool" } }] } },
    timeout: "1h",
  });
  return d;
};`)
	im.Activate(flowID, "acme.a", "1.0.0")
	env.Subjects.Grant(7)
	env.Subjects.Grant(8)
	first := newStack(t, env, im)
	id := first.start(7, "k", `{"who":"w"}`)
	require.Equal(t, 1, first.drain())
	require.Equal(t, workflow.StatusWaiting, first.status(id))
	tasks := first.history(id).Tasks
	require.Len(t, tasks, 1)
	require.Equal(t, "open", tasks[0].Status)
	require.EqualValues(t, 7, tasks[0].AssigneeUserID)

	// Engine restart: new pool, host and engine on the same database.
	first.engine.Shutdown()
	require.NoError(t, first.pool.Close(context.Background()))
	second := newStack(t, env, im)
	require.Zero(t, second.drain(), "a waiting task does not run the instance")

	err := second.engine.DecideTask(context.Background(), env.Subject(8), tasks[0].ID, json.RawMessage(`{"approved":true}`))
	require.ErrorIs(t, err, workflow.ErrForbidden, "only the assignee (or a task admin) decides")
	err = second.engine.DecideTask(context.Background(), env.Subject(7), tasks[0].ID, json.RawMessage(`{"approved":"yes"}`))
	require.ErrorIs(t, err, workflow.ErrInvalid, "the decision must match the declared schema")
	require.NoError(t, second.engine.DecideTask(context.Background(), env.Subject(7), tasks[0].ID, json.RawMessage(`{"approved":true}`)))
	err = second.engine.DecideTask(context.Background(), env.Subject(7), tasks[0].ID, json.RawMessage(`{"approved":false}`))
	require.ErrorIs(t, err, workflow.ErrTaskClosed, "a second decision loses the compare-and-set")

	require.Equal(t, 1, second.drain())
	require.Equal(t, workflow.StatusCompleted, second.status(id))
	require.JSONEq(t, `{"outcome":"decided","decision":{"approved":true},"by":"7"}`, string(second.instance(id).Result))
}

// A task assigned to a permission is decided by anyone who holds it now, by the task admin, and by nobody else (not even
// the user who started the workflow); the stored kind and permission are what the management read models show.
func TestHumanTaskAssignedToAPermissionIsDecidedByItsHolders(t *testing.T) {
	env := hosttest.New(t)
	im := newImpls()
	const committee = "acme.reinsurance.committee"
	env.Catalog.Add(spi.ResolvedResource{Package: "acme.a", Version: "1.0.0", Resource: manifest.Resource{ID: committee, Kind: manifest.ResPermission}})
	publishFlow(t, env, im, "acme.a", "1.0.0", `
export const flow = async (wf, input) => {
  return wf.task("approve", {
    subject: { entity: "acme.test.item", key: "1" },
    assignee: { permission: "`+committee+`" },
    decision: { type: { kind: "object", properties: [{ name: "approved", type: { kind: "bool" } }] } },
  });
};`)
	im.Activate(flowID, "acme.a", "1.0.0")
	env.Subjects.Grant(7)                     // starts the workflow
	env.Subjects.Grant(8, committee)          // a committee member
	env.Subjects.Grant(9, "acme.other.thing") // holds other permissions
	env.Subjects.Grant(10, "tasks.admin")     // the task admin
	st := newStack(t, env, im)
	id := st.start(7, "k", `{"who":"w"}`)
	require.Equal(t, 1, st.drain())
	require.Equal(t, workflow.StatusWaiting, st.status(id))

	tasks := st.history(id).Tasks
	require.Len(t, tasks, 1)
	require.Equal(t, workflow.TaskAssigneePermission, tasks[0].AssigneeKind)
	require.Equal(t, committee, tasks[0].AssigneePermission)
	require.Zero(t, tasks[0].AssigneeUserID, "no user is the assignee")

	decide := func(user int64, doc string) error {
		return st.engine.DecideTask(context.Background(), env.Subject(user), tasks[0].ID, json.RawMessage(doc))
	}
	require.ErrorIs(t, decide(7, `{"approved":true}`), workflow.ErrForbidden, "the user who started it is not a member")
	err := decide(9, `{"approved":true}`)
	require.ErrorIs(t, err, workflow.ErrForbidden)
	require.ErrorContains(t, err, committee, "the refusal names the permission")
	require.ErrorIs(t, decide(8, `{"approved":"yes"}`), workflow.ErrInvalid, "the schema still applies")

	// Rights are judged now: a member who lost the permission is refused, the task admin is not.
	env.Subjects.Grant(8)
	require.ErrorIs(t, decide(8, `{"approved":true}`), workflow.ErrForbidden)
	env.Subjects.Grant(8, committee)
	require.NoError(t, decide(8, `{"approved":true}`))
	require.ErrorIs(t, decide(10, `{"approved":false}`), workflow.ErrTaskClosed)

	require.Equal(t, 1, st.drain())
	require.JSONEq(t, `{"outcome":"decided","decision":{"approved":true},"by":"8"}`, string(st.instance(id).Result), "the decider is the member")

	// The task admin may decide a permission task too.
	other := st.start(7, "k2", `{"who":"w"}`)
	require.Equal(t, 1, st.drain())
	require.NoError(t, st.engine.DecideTask(context.Background(), env.Subject(10, "tasks.admin"), st.history(other).Tasks[0].ID, json.RawMessage(`{"approved":false}`)))
}

func TestTaskAssignedToAnUndeclaredPermissionFailsTheCall(t *testing.T) {
	env := hosttest.New(t)
	im := newImpls()
	publishFlow(t, env, im, "acme.a", "1.0.0", `
export const flow = async (wf) => {
  let code = "";
  try {
    await wf.task("approve", { subject: { entity: "acme.test.item", key: "1" }, assignee: { permission: "acme.nope.committee" }, decision: { type: { kind: "bool" } } });
  } catch (e) { code = e.code; }
  return { code };
};`)
	im.Activate(flowID, "acme.a", "1.0.0")
	env.Subjects.Grant(7)
	st := newStack(t, env, im)
	id := st.start(7, "k", `{"who":"w"}`)
	require.Equal(t, 1, st.drain())
	require.JSONEq(t, `{"code":"workflow.invalidAssignee"}`, string(st.instance(id).Result))
	require.Empty(t, st.history(id).Tasks, "no task was created")
}

func TestTimerContinuesAfterEngineRestart(t *testing.T) {
	env := hosttest.New(t)
	im := newImpls()
	publishFlow(t, env, im, "acme.a", "1.0.0", `
export const flow = async (wf) => {
  await wf.sleep("cooldown", "700ms");
  return await wf.transaction("after", async (tx) => (await tx.insert(item, { label: "woke" })).key);
};`)
	im.Activate(flowID, "acme.a", "1.0.0")
	env.Subjects.Grant(7)
	first := newStack(t, env, im)
	id := first.start(7, "k", `{"who":"w"}`)
	began := time.Now()
	require.Equal(t, 1, first.drain())
	require.Equal(t, workflow.StatusWaiting, first.status(id))
	first.engine.Shutdown()
	require.NoError(t, first.pool.Close(context.Background()))

	second := newStack(t, env, im)
	require.Zero(t, second.drain(), "the timer has not fired yet")
	second.until(10*time.Second, func() bool { return second.status(id) == workflow.StatusCompleted }, "timer fires on the restarted engine")
	require.GreaterOrEqual(t, time.Since(began), 700*time.Millisecond)
	require.Equal(t, []string{"woke"}, env.Items())
}

func TestPublishingV2DoesNotChangeALiveV1InstanceButNewInstancesUseV2(t *testing.T) {
	env := hosttest.New(t)
	im := newImpls()
	src := func(v string) string {
		return `export const flow = async (wf, input) => { const g = await wf.waitFor("w", go); return { v: "` + v + `", note: g.note, who: input.who }; };`
	}
	publishFlow(t, env, im, "acme.a", "1.0.0", src("v1"))
	im.Activate(flowID, "acme.a", "1.0.0")
	env.Subjects.Grant(7)
	st := newStack(t, env, im)
	old := st.start(7, "old", `{"who":"old"}`)
	require.Equal(t, 1, st.drain())

	publishFlow(t, env, im, "acme.a", "2.0.0", src("v2"))
	im.Activate(flowID, "acme.a", "2.0.0") // activation is application-wide and takes effect immediately
	fresh := st.start(7, "new", `{"who":"new"}`)
	require.Equal(t, "1.0.0", st.instance(old).Version)
	require.Equal(t, "2.0.0", st.instance(fresh).Version)

	require.NoError(t, st.signal(old, "go", `{"note":"a"}`, ""))
	require.NoError(t, st.signal(fresh, "go", `{"note":"b"}`, ""))
	st.drain()
	st.drain()
	require.JSONEq(t, `{"v":"v1","note":"a","who":"old"}`, string(st.instance(old).Result), "the live instance replayed on its pinned v1 artifact")
	require.JSONEq(t, `{"v":"v2","note":"b","who":"new"}`, string(st.instance(fresh).Result))
}

func TestWfCallSelectsTheCurrentBindingAndReplayKeepsTheRecordedReceipt(t *testing.T) {
	env := hosttest.New(t)
	im := newImpls()
	env.Artifacts.Add(t, "acme.impl1", "1.0.0", `export const value = async () => ({ v: 1 });`)
	env.Artifacts.Add(t, "acme.impl2", "1.0.0", `export const value = async () => ({ v: 2 });`)
	env.Catalog.Add(spi.ResolvedResource{Package: "acme.a", Version: "1.0.0", Resource: hosttest.Command("acme.test.value", false, nil, hosttest.Obj("v", hosttest.Int()))})
	_, err := env.Bindings.Activate(context.Background(), "acme.test.value", "acme.impl1", "1.0.0", "value", nil)
	require.NoError(t, err)
	publishFlow(t, env, im, "acme.a", "1.0.0", `
const Value = { id: "acme.test.value" };
export const flow = async (wf) => {
  const first = await wf.call("first", Value, null);
  await wf.waitFor("w", go);
  const second = await wf.call("second", Value, null);
  return { first: first.v, second: second.v };
};`)
	im.Activate(flowID, "acme.a", "1.0.0")
	env.Subjects.Grant(7)
	st := newStack(t, env, im)

	id := st.start(7, "k", `{"who":"w"}`)
	require.Equal(t, 1, st.drain())
	require.Equal(t, 1, st.calls.count("wf.call"))

	// The implementation behind the command is replaced while the instance waits.
	_, err = env.Bindings.Activate(context.Background(), "acme.test.value", "acme.impl2", "1.0.0", "value", nil)
	require.NoError(t, err)
	require.NoError(t, st.signal(id, "go", `{"note":"n"}`, ""))
	require.Equal(t, 1, st.drain())

	require.Equal(t, workflow.StatusCompleted, st.status(id))
	require.JSONEq(t, `{"first":1,"second":2}`, string(st.instance(id).Result), "replay returned the old receipt; the new call used the new implementation")
	require.Equal(t, 2, st.calls.count("wf.call"), "the first call was answered inside Node on replay")

	h := st.history(id)
	var calls []workflow.InvocationInfo
	for _, inv := range h.Invocations {
		if inv.Contract == "acme.test.value" {
			calls = append(calls, inv)
		}
	}
	require.Len(t, calls, 2)
	require.Equal(t, "acme.impl1", calls[0].BindingPackage)
	require.Equal(t, "acme.impl2", calls[1].BindingPackage)
	first, _ := occ(h, "first", 0)
	require.Equal(t, calls[0].ID.String(), first.InvocationID)
	require.NotEmpty(t, first.ContractHash)
}

func TestAtomicCallCommitsTheReceiptWithTheCommandAndRecordsDomainFailures(t *testing.T) {
	env := hosttest.New(t)
	im := newImpls()
	env.Artifacts.Add(t, "acme.impl", "1.0.0", `
export const add = async (tx, input) => {
  await tx.insert({ id: "acme.test.item" }, { label: input.label });
  if (input.fail) throw Object.assign(new Error("nope"), { kind: "domain", code: "acme.test.nope", params: {} });
  return { ok: true };
};`)
	env.Catalog.Add(spi.ResolvedResource{Package: "acme.a", Version: "1.0.0", Resource: hosttest.Command("acme.test.add", true,
		hosttest.Obj("label", hosttest.Str(), "fail", hosttest.Bool()), hosttest.Obj("ok", hosttest.Bool()), "acme.test.nope")})
	_, err := env.Bindings.Activate(context.Background(), "acme.test.add", "acme.impl", "1.0.0", "add", nil)
	require.NoError(t, err)
	publishFlow(t, env, im, "acme.a", "1.0.0", `
const Add = { id: "acme.test.add" };
export const flow = async (wf) => {
  await wf.call("good", Add, { label: "good", fail: false });
  let caught = "";
  try { await wf.call("bad", Add, { label: "bad", fail: true }); } catch (e) { caught = e.code; }
  await wf.waitFor("w", go);
  return { caught };
};`)
	im.Activate(flowID, "acme.a", "1.0.0")
	env.Subjects.Grant(7)
	st := newStack(t, env, im)
	id := st.start(7, "k", `{"who":"w"}`)
	require.Equal(t, 1, st.drain())
	require.Equal(t, []string{"good"}, env.Items(), "the failed atomic command rolled back; the good one committed with its receipt")
	h := st.history(id)
	good, _ := occ(h, "good", 0)
	require.Equal(t, workflow.OccCompleted, good.Status)
	bad, _ := occ(h, "bad", 0)
	require.Equal(t, workflow.OccFailed, bad.Status)

	require.NoError(t, st.signal(id, "go", `{"note":"n"}`, ""))
	require.Equal(t, 1, st.drain())
	require.JSONEq(t, `{"caught":"acme.test.nope"}`, string(st.instance(id).Result), "the replayed failure is the recorded one")
	require.Equal(t, []string{"good"}, env.Items())
}

func TestTerminateCancelsWaitsRejectsSignalsAndStartV2LinksHistory(t *testing.T) {
	env := hosttest.New(t)
	im := newImpls()
	src := func(v string) string {
		return `export const flow = async (wf, input) => {
  await wf.transaction("mark", async (tx) => tx.insert(item, { label: "` + v + `" }));
  const d = await wf.task("approve", { subject: { entity: "acme.test.item", key: "1" }, assignee: { entity: "core.user", key: "7" },
    decision: { type: { kind: "object" } }, timeout: "1h" });
  return { v: "` + v + `" };
};`
	}
	publishFlow(t, env, im, "acme.a", "1.0.0", src("v1"))
	im.Activate(flowID, "acme.a", "1.0.0")
	env.Subjects.Grant(7)
	st := newStack(t, env, im)
	id := st.start(7, "k", `{"who":"w"}`)
	require.Equal(t, 1, st.drain())
	require.Equal(t, workflow.StatusWaiting, st.status(id))

	_, err := st.engine.StartV2(context.Background(), env.Subject(7), id, "")
	require.ErrorIs(t, err, workflow.ErrNotAccepting, "a live instance is terminated before a new version starts")

	require.NoError(t, st.engine.Terminate(context.Background(), env.Tenant, id))
	h := st.history(id)
	require.Equal(t, workflow.StatusTerminated, h.Instance.Status)
	require.Equal(t, "cancelled", h.Tasks[0].Status, "open tasks are cancelled")
	task, _ := occ(h, "approve", 0)
	require.Equal(t, workflow.OccCancelled, task.Status, "timers are cancelled")
	require.ErrorIs(t, st.signal(id, "go", `{"note":"n"}`, ""), workflow.ErrNotAccepting)
	require.ErrorIs(t, st.engine.DecideTask(context.Background(), env.Subject(7), h.Tasks[0].ID, json.RawMessage(`{}`)), workflow.ErrTaskClosed)
	require.Zero(t, st.drain(), "a terminated instance never runs again")
	require.Equal(t, []string{"v1"}, env.Items(), "done effects stay: no compensation")

	publishFlow(t, env, im, "acme.a", "2.0.0", src("v2"))
	v2, err := st.engine.StartV2(context.Background(), env.Subject(7), id, "2.0.0")
	require.NoError(t, err)
	next := st.instance(v2)
	require.NotNil(t, next.PredecessorID)
	require.Equal(t, id, *next.PredecessorID)
	require.Nil(t, next.StartKey, "start-v2 has no start key")
	require.Equal(t, "2.0.0", next.Version)
	require.JSONEq(t, `{"who":"w"}`, string(next.Input))
	require.Equal(t, 1, st.drain())
	require.Equal(t, []string{"v1", "v2"}, env.Items())
	list, err := st.engine.List(context.Background(), workflow.ListFilter{TenantID: env.Tenant})
	require.NoError(t, err)
	require.Len(t, list, 2)
}

func TestStaleRunnerCannotRecordAfterLeaseTakeover(t *testing.T) {
	env := hosttest.New(t)
	im := newImpls()
	release := make(chan struct{})
	var mu sync.Mutex
	hits := 0
	entered := make(chan struct{}, 2)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		mu.Lock()
		hits++
		n := hits
		mu.Unlock()
		entered <- struct{}{}
		if n == 1 {
			<-release
			_, _ = w.Write([]byte(`{"v":"A"}`))
			return
		}
		_, _ = w.Write([]byte(`{"v":"B"}`))
	}))
	defer srv.Close()
	publishFlow(t, env, im, "acme.a", "1.0.0", `
export const flow = async (wf, input) => {
  const v = await wf.step("slow", async () => (await (await fetch(input.url)).json()).v);
  await wf.transaction("after", async (tx) => tx.insert(item, { label: "after:" + v }));
  return v;
};`)
	im.Activate(flowID, "acme.a", "1.0.0")
	env.Subjects.Grant(7)
	// Runner A never renews its lease: it is the stale runner.
	a := newStack(t, env, im, func(c *workflow.Config, _ *host.Config) { c.Heartbeat = time.Hour; c.LeaseTTL = time.Hour })
	b := newStack(t, env, im)
	id := a.start(7, "k", `{"who":"w","url":"`+srv.URL+`"}`)

	aDone := make(chan int, 1)
	go func() { n, _ := a.engine.RunOnce(context.Background()); aDone <- n }()
	<-entered // A is inside the step effect, holding epoch 1

	_, err := env.DB.Exec(context.Background(), `UPDATE extruntime.workflow_instances SET lease_until = now() - interval '1 second' WHERE id = $1`, id)
	require.NoError(t, err)
	require.Equal(t, 1, b.drain(), "B takes the expired lease over (epoch 2) and finishes the instance")
	require.Equal(t, workflow.StatusCompleted, b.status(id))
	require.EqualValues(t, 2, b.instance(id).LeaseEpoch)

	close(release) // A's effect now returns and A tries to record
	select {
	case <-aDone:
	case <-time.After(10 * time.Second):
		t.Fatal("stale runner did not stop")
	}
	h := b.history(id)
	step, _ := occ(h, "slow", 0)
	require.JSONEq(t, `"B"`, string(step.Result), "the stale runner's result was fenced out")
	require.JSONEq(t, `"B"`, string(h.Instance.Result))
	require.Equal(t, []string{"after:B"}, env.Items())
	require.Equal(t, workflow.StatusCompleted, h.Instance.Status)
}

func TestNondeterminismBlocksTheInstance(t *testing.T) {
	env := hosttest.New(t)
	im := newImpls()
	publishFlow(t, env, im, "acme.a", "1.0.0", `
let runs = 0;
export const flow = async (wf) => {
  runs++;
  if (runs === 1) { await wf.step("a", async () => 1); } else { await wf.transaction("a", async (tx) => tx.insert(item, { label: "x" })); }
  await wf.waitFor("w", go);
  return runs;
};`)
	im.Activate(flowID, "acme.a", "1.0.0")
	env.Subjects.Grant(7)
	st := newStack(t, env, im)
	id := st.start(7, "k", `{"who":"w"}`)
	require.Equal(t, 1, st.drain())
	require.NoError(t, st.signal(id, "go", `{"note":"n"}`, ""))
	require.Equal(t, 1, st.drain())

	inst := st.instance(id)
	require.Equal(t, workflow.StatusBlocked, inst.Status)
	require.Equal(t, workflow.BlockNondeterminism, inst.BlockedReason)
	require.Contains(t, inst.BlockedDetail, "a#0")
	require.Empty(t, env.Items(), "the diverging code did not run its effect")
	require.Zero(t, st.drain(), "a blocked instance is not claimed")
}

func TestChangedCallInputIsNondeterminism(t *testing.T) {
	env := hosttest.New(t)
	im := newImpls()
	env.Artifacts.Add(t, "acme.impl", "1.0.0", `export const add = async (tx, input) => { await tx.insert({ id: "acme.test.item" }, { label: input.label }); return { ok: true }; };`)
	env.Catalog.Add(spi.ResolvedResource{Package: "acme.a", Version: "1.0.0", Resource: hosttest.Command("acme.test.add", true, hosttest.Obj("label", hosttest.Str()), hosttest.Obj("ok", hosttest.Bool()))})
	_, err := env.Bindings.Activate(context.Background(), "acme.test.add", "acme.impl", "1.0.0", "add", nil)
	require.NoError(t, err)
	publishFlow(t, env, im, "acme.a", "1.0.0", `
let runs = 0;
export const flow = async (wf) => {
  runs++;
  await wf.transaction("t", { id: "acme.test.add" }, { label: "run" + runs });
  await wf.waitFor("w", go);
};`)
	im.Activate(flowID, "acme.a", "1.0.0")
	env.Subjects.Grant(7)
	st := newStack(t, env, im)
	id := st.start(7, "k", `{"who":"w"}`)
	require.Equal(t, 1, st.drain())
	require.NoError(t, st.signal(id, "go", `{"note":"n"}`, ""))
	st.drain()
	inst := st.instance(id)
	require.Equal(t, workflow.StatusBlocked, inst.Status)
	require.Equal(t, workflow.BlockNondeterminism, inst.BlockedReason)
	require.Equal(t, []string{"run1"}, env.Items())
}

func TestCurrentRightsAreLoadedOnEveryRunAndADisabledSubjectBlocks(t *testing.T) {
	env := hosttest.New(t)
	im := newImpls()
	publishFlow(t, env, im, "acme.a", "1.0.0", `
export const flow = async (wf) => {
  await wf.waitFor("w", go);
  return { can: wf.user.has("claims.approve") };
};`)
	im.Activate(flowID, "acme.a", "1.0.0")
	env.Subjects.Grant(7)
	st := newStack(t, env, im)
	id := st.start(7, "k", `{"who":"w"}`)
	require.Equal(t, 1, st.drain())

	env.Subjects.Grant(7, "claims.approve")
	env.Subjects.Block(7, "account disabled")
	require.NoError(t, st.signal(id, "go", `{"note":"n"}`, ""))
	require.Equal(t, 1, st.drain())
	inst := st.instance(id)
	require.Equal(t, workflow.StatusBlocked, inst.Status)
	require.Equal(t, workflow.BlockSubject, inst.BlockedReason)
	require.Contains(t, inst.BlockedDetail, "account disabled")

	env.Subjects.Grant(7, "claims.approve")
	require.NoError(t, st.engine.Resume(context.Background(), env.Tenant, id))
	require.Equal(t, 1, st.drain())
	require.JSONEq(t, `{"can":true}`, string(st.instance(id).Result), "the run acts with the subject's rights as they are now")
	require.GreaterOrEqual(t, env.Subjects.Loads(7), 3, "rights were loaded for the first run, the blocked run and the resumed run")
}

func TestAmbiguousOrdinaryCommandBlocksWithCommitCountThenResolveAndResume(t *testing.T) {
	env := hosttest.New(t)
	im := newImpls()
	env.Artifacts.Add(t, "acme.impl", "1.0.0", `
export const crashy = async (ctx) => {
  await ctx.transaction(async (tx) => tx.insert({ id: "acme.test.item" }, { label: "partial" }));
  process.exit(1);
};`)
	env.Catalog.Add(spi.ResolvedResource{Package: "acme.a", Version: "1.0.0", Resource: hosttest.Command("acme.test.crashy", false, nil, hosttest.Obj("v", hosttest.Int()))})
	_, err := env.Bindings.Activate(context.Background(), "acme.test.crashy", "acme.impl", "1.0.0", "crashy", nil)
	require.NoError(t, err)
	publishFlow(t, env, im, "acme.a", "1.0.0", `
export const flow = async (wf) => {
  const r = await wf.call("c", { id: "acme.test.crashy" }, null);
  return r;
};`)
	im.Activate(flowID, "acme.a", "1.0.0")
	env.Subjects.Grant(7)
	st := newStack(t, env, im)
	id := st.start(7, "k", `{"who":"w"}`)
	st.until(10*time.Second, func() bool { return st.status(id) == workflow.StatusBlocked }, "ambiguous call blocks the instance")

	inst := st.instance(id)
	require.Equal(t, workflow.BlockAmbiguous, inst.BlockedReason)
	require.Contains(t, inst.BlockedDetail, "committed 1 transaction")
	require.Equal(t, []string{"partial"}, env.Items(), "the command was not blindly re-dispatched")
	o, _ := occ(st.history(id), "c", 0)
	require.Equal(t, workflow.OccStarted, o.Status)

	// Retry accepts the risk: the call is dispatched again as a new invocation.
	require.NoError(t, st.engine.Retry(context.Background(), env.Tenant, id, "c", 0))
	require.NoError(t, st.engine.Resume(context.Background(), env.Tenant, id))
	st.until(10*time.Second, func() bool { return st.status(id) == workflow.StatusBlocked }, "the retried call crashes again and is ambiguous again")
	require.Equal(t, []string{"partial", "partial"}, env.Items(), "retry re-executed the command (the duplicate was accepted by the operator)")

	require.NoError(t, st.engine.Resolve(context.Background(), env.Tenant, id, "c", 0, json.RawMessage(`{"v":7}`)))
	require.NoError(t, st.engine.Resume(context.Background(), env.Tenant, id))
	require.Equal(t, 1, st.drain())
	require.Equal(t, workflow.StatusCompleted, st.status(id))
	require.JSONEq(t, `{"v":7}`, string(st.instance(id).Result), "the operator's result stands in for the unknown outcome")
	require.Equal(t, []string{"partial", "partial"}, env.Items())
}

func TestSignalsAreValidatedDedupedAndConsumedInOrder(t *testing.T) {
	env := hosttest.New(t)
	im := newImpls()
	publishFlow(t, env, im, "acme.a", "1.0.0", `
export const flow = async (wf) => {
  const a = await wf.waitFor("first", go);
  const b = await wf.waitFor("second", go);
  const c = await wf.waitFor("third", go, { timeout: "300ms" });
  return { a: a.note, b: b.note, c };
};`)
	im.Activate(flowID, "acme.a", "1.0.0")
	env.Subjects.Grant(7)
	st := newStack(t, env, im)
	id := st.start(7, "k", `{"who":"w"}`)
	require.Equal(t, 1, st.drain())

	require.ErrorIs(t, st.signal(id, "unknown", `{}`, ""), workflow.ErrInvalid, "undeclared signal")
	require.ErrorIs(t, st.signal(id, "go", `{"note":1}`, ""), workflow.ErrInvalid, "payload does not match the declared signal type")
	require.NoError(t, st.signal(id, "go", `{"note":"one"}`, "key-1"))
	require.NoError(t, st.signal(id, "go", `{"note":"one-again"}`, "key-1"), "same key is a no-op")
	require.NoError(t, st.signal(id, "go", `{"note":"two"}`, "key-2"))
	require.Equal(t, 1, st.drain())

	require.Equal(t, workflow.StatusWaiting, st.status(id), "the third wait has a timer")
	st.until(10*time.Second, func() bool { return st.status(id) == workflow.StatusCompleted }, "timeout path completes")
	require.JSONEq(t, `{"a":"one","b":"two","c":{"outcome":"timeout"}}`, string(st.instance(id).Result))
	h := st.history(id)
	require.Len(t, h.Signals, 2)
	require.Equal(t, "first", h.Signals[0].ConsumedName)
	require.Equal(t, "second", h.Signals[1].ConsumedName)
	require.ErrorIs(t, st.signal(id, "go", `{"note":"late"}`, ""), workflow.ErrNotAccepting, "a finished instance rejects signals")
}

func TestTaskTimeoutWinsTheCompareAndSetOverALateDecision(t *testing.T) {
	env := hosttest.New(t)
	im := newImpls()
	publishFlow(t, env, im, "acme.a", "1.0.0", `
export const flow = async (wf) => wf.task("approve", {
  subject: { entity: "acme.test.item", key: "1" }, assignee: { userId: "7" },
  decision: { type: { kind: "object" } }, timeout: "300ms" });`)
	im.Activate(flowID, "acme.a", "1.0.0")
	env.Subjects.Grant(7)
	st := newStack(t, env, im)
	id := st.start(7, "k", `{"who":"w"}`)
	require.Equal(t, 1, st.drain())
	taskID := st.history(id).Tasks[0].ID
	st.until(10*time.Second, func() bool { return st.status(id) == workflow.StatusCompleted }, "task expires")
	require.JSONEq(t, `{"outcome":"timeout"}`, string(st.instance(id).Result))
	err := st.engine.DecideTask(context.Background(), env.Subject(7), taskID, json.RawMessage(`{}`))
	require.ErrorIs(t, err, workflow.ErrTaskClosed)
	require.Equal(t, "expired", st.history(id).Tasks[0].Status)
}

func TestStartedChildIsDedupedAcrossReplayAndHasAParentLink(t *testing.T) {
	env := hosttest.New(t)
	im := newImpls()
	publishFlow(t, env, im, "acme.a", "1.0.0", `
export const flow = async (wf, input) => {
  if (input.who === "kid") return "kid-done";
  const child = await wf.start("child", { id: "acme.test.flow" }, { who: "kid" });
  await wf.waitFor("w", go);
  return child.instance;
};`)
	im.Activate(flowID, "acme.a", "1.0.0")
	env.Subjects.Grant(7)
	st := newStack(t, env, im)
	parent := st.start(7, "k", `{"who":"parent"}`)
	st.drain() // parent runs and starts the child; the child then runs too
	require.NoError(t, st.signal(parent, "go", `{"note":"n"}`, ""))
	st.drain()

	list, err := st.engine.List(context.Background(), workflow.ListFilter{TenantID: env.Tenant})
	require.NoError(t, err)
	require.Len(t, list, 2, "replay did not start a second child")
	var child workflow.Instance
	for _, i := range list {
		if i.ID != parent {
			child = i
		}
	}
	require.NotNil(t, child.ParentInstanceID)
	require.Equal(t, parent, *child.ParentInstanceID)
	require.Equal(t, workflow.StatusCompleted, child.Status)
	require.JSONEq(t, `"`+child.ID.String()+`"`, string(st.instance(parent).Result))
	require.Equal(t, parent.String()+":child:0", *child.StartKey)
}

func TestConcurrentPrimitivesGetOccurrencesAtCallTimeAndSuspendAfterTheyRecord(t *testing.T) {
	env := hosttest.New(t)
	im := newImpls()
	publishFlow(t, env, im, "acme.a", "1.0.0", `
export const flow = async (wf) => {
  const slow = (v) => async () => { await new Promise((r) => setTimeout(r, 80)); return v; };
  const results = await Promise.all([wf.step("x", slow("a")), wf.step("x", slow("b")), wf.sleep("nap", "1h")]);
  return results;
};`)
	im.Activate(flowID, "acme.a", "1.0.0")
	env.Subjects.Grant(7)
	st := newStack(t, env, im)
	id := st.start(7, "k", `{"who":"w"}`)
	require.Equal(t, 1, st.drain())
	h := st.history(id)
	x0, ok0 := occ(h, "x", 0)
	x1, ok1 := occ(h, "x", 1)
	require.True(t, ok0 && ok1, "two steps with one name get distinct occurrences")
	require.Equal(t, workflow.OccCompleted, x0.Status, "the run suspended only after the in-flight steps recorded")
	require.Equal(t, workflow.OccCompleted, x1.Status)
	require.JSONEq(t, `"a"`, string(x0.Result))
	require.JSONEq(t, `"b"`, string(x1.Result))
	nap, _ := occ(h, "nap", 0)
	require.Equal(t, workflow.OccWaiting, nap.Status)
	require.Equal(t, workflow.StatusWaiting, h.Instance.Status)
}

func TestNowAndUuidAreJournaledAcrossReplays(t *testing.T) {
	env := hosttest.New(t)
	im := newImpls()
	publishFlow(t, env, im, "acme.a", "1.0.0", `
export const flow = async (wf) => {
  const at = await wf.now("t");
  const id = await wf.uuid("u");
  await wf.waitFor("w", go);
  return { at, id, again: await wf.now("t") };
};`)
	im.Activate(flowID, "acme.a", "1.0.0")
	env.Subjects.Grant(7)
	st := newStack(t, env, im)
	id := st.start(7, "k", `{"who":"w"}`)
	require.Equal(t, 1, st.drain())
	h := st.history(id)
	at0, _ := occ(h, "t", 0)
	u0, _ := occ(h, "u", 0)
	require.NoError(t, st.signal(id, "go", `{"note":"n"}`, ""))
	require.Equal(t, 1, st.drain())
	var res struct {
		At    string `json:"at"`
		ID    string `json:"id"`
		Again string `json:"again"`
	}
	require.NoError(t, json.Unmarshal(st.instance(id).Result, &res))
	require.JSONEq(t, string(at0.Result), `"`+res.At+`"`, "the replay returned the journaled clock reading")
	require.JSONEq(t, string(u0.Result), `"`+res.ID+`"`)
	require.NotEqual(t, res.At, res.Again, "a second now() under the same name is a new occurrence")
}

func TestExpiredLeaseOfACrashedRunnerIsReclaimedAndReplayed(t *testing.T) {
	env := hosttest.New(t)
	im := newImpls()
	publishFlow(t, env, im, "acme.a", "1.0.0", `
export const flow = async (wf) => wf.transaction("once", async (tx) => (await tx.insert(item, { label: "once" })).key);`)
	im.Activate(flowID, "acme.a", "1.0.0")
	env.Subjects.Grant(7)
	st := newStack(t, env, im)
	id := st.start(7, "k", `{"who":"w"}`)
	// A runner claimed the instance and died: running with a live lease.
	_, err := env.DB.Exec(context.Background(), `UPDATE extruntime.workflow_instances SET lease_owner = 'dead', lease_until = now() + interval '1 hour', lease_epoch = 4 WHERE id = $1`, id)
	require.NoError(t, err)
	require.Zero(t, st.drain(), "a live lease is respected")
	_, err = env.DB.Exec(context.Background(), `UPDATE extruntime.workflow_instances SET lease_until = now() - interval '1 second' WHERE id = $1`, id)
	require.NoError(t, err)
	require.Equal(t, 1, st.drain())
	inst := st.instance(id)
	require.Equal(t, workflow.StatusCompleted, inst.Status)
	require.EqualValues(t, 5, inst.LeaseEpoch)
	require.Equal(t, []string{"once"}, env.Items())
}

func TestTwoEnginesNeverRunTheSameInstanceAtOnce(t *testing.T) {
	env := hosttest.New(t)
	im := newImpls()
	publishFlow(t, env, im, "acme.a", "1.0.0", `
export const flow = async (wf, input) => wf.transaction("once", async (tx) => (await tx.insert(item, { label: input.who })).key);`)
	im.Activate(flowID, "acme.a", "1.0.0")
	env.Subjects.Grant(7)
	a := newStack(t, env, im)
	b := newStack(t, env, im)
	for i := 0; i < 6; i++ {
		a.start(7, fmt.Sprintf("k%d", i), fmt.Sprintf(`{"who":"w%d"}`, i))
	}
	var wg sync.WaitGroup
	for _, s := range []*stack{a, b} {
		wg.Add(1)
		go func() { defer wg.Done(); s.drain() }()
	}
	wg.Wait()
	items := env.Items()
	require.Len(t, items, 6, "each instance ran exactly once across both engines")
	require.ElementsMatch(t, []string{"w0", "w1", "w2", "w3", "w4", "w5"}, items)
}
