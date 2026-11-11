package workflow_test

import (
	"context"
	"encoding/json"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"

	"github.com/iota-uz/iota-sdk/pkg/extruntime/host"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/host/hosttest"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/manifest"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/pool"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/spi"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/workflow"
)

// A tx.start inside wf.transaction derives its dedupe key from the
// transaction's occurrence, so a later transaction of a replayed run never
// collides with the child an earlier one started.
func TestTxStartKeyIsScopedToTheTransactionOccurrence(t *testing.T) {
	env := hosttest.New(t)
	im := newImpls()
	publishFlow(t, env, im, "acme.a", "1.0.0", `
export const flow = async (wf, input) => {
  if (input.who !== "parent") return input.who;
  const a = await wf.transaction("a", async (tx) => (await tx.start({ id: "acme.test.flow" }, { who: "kidA" })).instance);
  await wf.waitFor("w", go);
  const b = await wf.transaction("b", async (tx) => (await tx.start({ id: "acme.test.flow" }, { who: "kidB" })).instance);
  return { a, b };
};`)
	im.Activate(flowID, "acme.a", "1.0.0")
	env.Subjects.Grant(7)
	st := newStack(t, env, im)
	parent := st.start(7, "k", `{"who":"parent"}`)
	st.drain()
	require.NoError(t, st.signal(parent, "go", `{"note":"n"}`, ""))
	st.drain()
	require.Equal(t, workflow.StatusCompleted, st.status(parent))

	var out struct{ A, B string }
	require.NoError(t, json.Unmarshal(st.instance(parent).Result, &out))
	list, err := st.engine.List(context.Background(), workflow.ListFilter{TenantID: env.Tenant})
	require.NoError(t, err)
	require.NotEqual(t, out.A, out.B, "transaction b started kidB, not the instance of transaction a")
	require.Len(t, list, 3, "parent + kidA + kidB")
	keys := map[string]string{}
	for _, i := range list {
		var in struct{ Who string }
		require.NoError(t, json.Unmarshal(i.Input, &in))
		if i.StartKey != nil {
			keys[in.Who] = *i.StartKey
		}
	}
	require.Equal(t, parent.String()+":a:0#1", keys["kidA"], "the key is instance:name:n plus the sequence inside the occurrence")
	require.Equal(t, parent.String()+":b:0#1", keys["kidB"])
}

// A decision that commits while the run is suspending must wake the instance:
// DecideTask serializes with the run on the instance row.
func TestTaskDecisionCommittedWhileTheRunSuspendsWakesTheInstance(t *testing.T) {
	env := hosttest.New(t)
	im := newImpls()
	publishFlow(t, env, im, "acme.a", "1.0.0", `
export const flow = async (wf, input) => {
  return wf.task("approve", {
    subject: { entity: "acme.test.item", key: "1" },
    assignee: { entity: "core.user", key: "7" },
    decision: { type: { kind: "object", properties: [{ name: "approved", type: { kind: "bool" } }] } },
  });
};`)
	im.Activate(flowID, "acme.a", "1.0.0")
	env.Subjects.Grant(7)
	st := newStack(t, env, im)
	ctx := context.Background()

	// Test-only: widen the window between finishSuspended's wake computation
	// and its commit. The trigger changes no data.
	_, err := env.DB.Exec(ctx, `CREATE FUNCTION scratch_pause() RETURNS trigger AS $$
		BEGIN IF NEW.status = 'waiting' AND OLD.status = 'running' THEN PERFORM pg_advisory_xact_lock(424242); END IF; RETURN NEW; END $$ LANGUAGE plpgsql`)
	require.NoError(t, err)
	_, err = env.DB.Exec(ctx, `CREATE TRIGGER scratch_pause BEFORE UPDATE ON extruntime.workflow_instances FOR EACH ROW EXECUTE FUNCTION scratch_pause()`)
	require.NoError(t, err)
	gate, err := env.DB.Acquire(ctx)
	require.NoError(t, err)
	defer gate.Release()
	_, err = gate.Exec(ctx, `SELECT pg_advisory_lock(424242)`)
	require.NoError(t, err)

	id := st.start(7, "k", `{"who":"w"}`)
	done := make(chan struct{})
	go func() { defer close(done); _, _ = st.engine.RunOnce(ctx) }()
	waitLockWaiters(t, ctx, env.DB, 1)

	tasks := st.history(id).Tasks
	require.Len(t, tasks, 1)
	decided := make(chan error, 1)
	go func() {
		decided <- st.engine.DecideTask(ctx, env.Subject(7), tasks[0].ID, json.RawMessage(`{"approved":true}`))
	}()
	// The decision queues behind the suspending run on the instance row (an
	// implementation that takes no lock returns at once instead).
	var decideErr error
	early := false
	deadline := time.Now().Add(20 * time.Second)
	for {
		select {
		case decideErr = <-decided:
			early = true
		default:
		}
		if early || lockWaiters(t, ctx, env.DB) >= 2 {
			break
		}
		require.True(t, time.Now().Before(deadline), "the decision neither completed nor queued behind the run")
		time.Sleep(20 * time.Millisecond)
	}

	_, err = gate.Exec(ctx, `SELECT pg_advisory_unlock(424242)`)
	require.NoError(t, err)
	<-done
	if !early {
		decideErr = <-decided
	}
	require.NoError(t, decideErr)

	require.Equal(t, "decided", st.history(id).Tasks[0].Status)
	st.drain()
	require.Equal(t, workflow.StatusCompleted, st.status(id), "the decided task must wake the instance")
}

// lockWaiters counts the sessions of the test database waiting for a lock.
func lockWaiters(t *testing.T, ctx context.Context, db *pgxpool.Pool) int {
	t.Helper()
	var n int
	require.NoError(t, db.QueryRow(ctx, `SELECT count(*) FROM pg_stat_activity WHERE datname = current_database() AND wait_event_type = 'Lock'`).Scan(&n))
	return n
}

// waitLockWaiters waits until at least n sessions wait for a lock (the
// advisory gate, or the row lock behind it).
func waitLockWaiters(t *testing.T, ctx context.Context, db *pgxpool.Pool, n int) {
	t.Helper()
	deadline := time.Now().Add(20 * time.Second)
	for lockWaiters(t, ctx, db) < n {
		require.True(t, time.Now().Before(deadline), "expected %d session(s) waiting for a lock", n)
		time.Sleep(20 * time.Millisecond)
	}
}

// A database error inside a wf.* host call is an infrastructure failure: the
// run is abandoned and retried with backoff, not recorded as the workflow's
// failure.
func TestInfrastructureErrorInHostCallRetriesTheRun(t *testing.T) {
	env := hosttest.New(t)
	im := newImpls()
	publishFlow(t, env, im, "acme.a", "1.0.0", `
export const flow = async (wf, input) => {
  const at = await wf.now("boom");
  return { at };
};`)
	im.Activate(flowID, "acme.a", "1.0.0")
	env.Subjects.Grant(7)
	st := newStack(t, env, im)
	ctx := context.Background()
	_, err := env.DB.Exec(ctx, `CREATE TABLE scratch_outage (x int)`)
	require.NoError(t, err)
	_, err = env.DB.Exec(ctx, `INSERT INTO scratch_outage VALUES (1)`)
	require.NoError(t, err)
	_, err = env.DB.Exec(ctx, `CREATE FUNCTION scratch_outage_fn() RETURNS trigger AS $$
		BEGIN IF EXISTS (SELECT 1 FROM scratch_outage) THEN RAISE EXCEPTION 'simulated outage' USING ERRCODE = '53300'; END IF; RETURN NEW; END $$ LANGUAGE plpgsql`)
	require.NoError(t, err)
	_, err = env.DB.Exec(ctx, `CREATE TRIGGER scratch_outage BEFORE INSERT ON extruntime.workflow_occurrences FOR EACH ROW EXECUTE FUNCTION scratch_outage_fn()`)
	require.NoError(t, err)

	id := st.start(7, "k", `{"who":"w"}`)
	st.drain()
	inst := st.instance(id)
	require.Equal(t, workflow.StatusRunning, inst.Status, "the failed run is retried, the instance does not fail: %s", inst.Error)
	require.Equal(t, 1, inst.Failures)
	_, err = env.DB.Exec(ctx, `DELETE FROM scratch_outage`)
	require.NoError(t, err)
	st.until(5*time.Second, func() bool { return st.status(id) == workflow.StatusCompleted }, "instance recovers once the database is healthy again (status "+st.status(id)+")")
}

// outageOn makes inserts into table fail with a "too many connections"
// error while the returned switch is on.
func outageOn(t *testing.T, env *hosttest.Env, table string) (off func()) {
	t.Helper()
	ctx := context.Background()
	for _, q := range []string{
		`CREATE TABLE outage_switch (x int)`,
		`INSERT INTO outage_switch VALUES (1)`,
		`CREATE FUNCTION outage_fn() RETURNS trigger AS $$
			BEGIN IF EXISTS (SELECT 1 FROM outage_switch) THEN RAISE EXCEPTION 'simulated outage' USING ERRCODE = '53300'; END IF; RETURN NEW; END $$ LANGUAGE plpgsql`,
		`CREATE TRIGGER outage BEFORE INSERT ON ` + table + ` FOR EACH ROW EXECUTE FUNCTION outage_fn()`,
	} {
		_, err := env.DB.Exec(ctx, q)
		require.NoError(t, err, q)
	}
	return func() {
		_, err := env.DB.Exec(ctx, `DELETE FROM outage_switch`)
		require.NoError(t, err)
	}
}

// An infrastructure failure inside an ordinary command called by wf.call is
// not the command's result: it is not journaled as a failed call and does not
// fail the instance. The run is retried and the call, whose outcome is
// unknown, is resolved by the operator like any ambiguous command.
func TestInfrastructureErrorInWfCallIsNotJournaledAsAFailure(t *testing.T) {
	env := hosttest.New(t)
	im := newImpls()
	env.Artifacts.Add(t, "acme.impl", "1.0.0", `
export const add = async (ctx) => {
  await ctx.transaction(async (tx) => tx.insert({ id: "acme.test.item" }, { label: "x" }));
  return { ok: true };
};`)
	env.Catalog.Add(spi.ResolvedResource{Package: "acme.a", Version: "1.0.0", Resource: hosttest.Command("acme.test.add", false, nil, hosttest.Obj("ok", hosttest.Bool()))})
	_, err := env.Bindings.Activate(context.Background(), "acme.test.add", "acme.impl", "1.0.0", "add", nil)
	require.NoError(t, err)
	publishFlow(t, env, im, "acme.a", "1.0.0", `
export const flow = async (wf) => {
  try { return await wf.call("c", { id: "acme.test.add" }, null); } catch (e) { return { caught: e.code }; }
};`)
	im.Activate(flowID, "acme.a", "1.0.0")
	env.Subjects.Grant(7)
	st := newStack(t, env, im)
	off := outageOn(t, env, "host_items")

	id := st.start(7, "k", `{"who":"w"}`)
	st.drain()
	inst := st.instance(id)
	require.NotEqual(t, workflow.StatusFailed, inst.Status, "the outage must not fail the instance: %s", inst.Error)
	require.NotEqual(t, workflow.StatusCompleted, inst.Status, "the workflow must not observe the outage: %s", inst.Result)
	o, ok := occ(st.history(id), "c", 0)
	require.True(t, ok)
	require.NotEqual(t, workflow.OccFailed, o.Status, "an infrastructure failure is not a recorded result of the command")

	off()
	st.until(10*time.Second, func() bool { return st.status(id) == workflow.StatusBlocked }, "the call has an unknown outcome and waits for the operator")
	require.Equal(t, workflow.BlockAmbiguous, st.instance(id).BlockedReason)
	o, _ = occ(st.history(id), "c", 0)
	require.Equal(t, workflow.OccStarted, o.Status)
	require.Empty(t, env.Items())

	// The operator decides (nothing was committed): the call is dispatched again.
	require.NoError(t, st.engine.Retry(context.Background(), env.Tenant, id, "c", 0))
	require.NoError(t, st.engine.Resume(context.Background(), env.Tenant, id))
	st.until(10*time.Second, func() bool { return st.status(id) == workflow.StatusCompleted }, "retried call completes")
	require.Equal(t, []string{"x"}, env.Items())
}

// securedFlow registers a second workflow whose contract demands a permission.
func securedFlow(t *testing.T, env *hosttest.Env, im *impls) string {
	t.Helper()
	const id = "acme.test.secured"
	env.Catalog.Add(spi.ResolvedResource{Package: "acme.a", Version: "1.0.0", Resource: manifest.Resource{
		ID: id, Kind: manifest.ResWorkflow, Permission: "flows.start",
		Input: hosttest.Obj("who", hosttest.Str(), "url", hosttest.Optional(hosttest.Str())),
	}})
	im.mu.Lock()
	im.at[id+"|acme.a|1.0.0"] = workflow.Implementation{Package: "acme.a", Version: "1.0.0", Export: "secured", Artifact: im.at[flowID+"|acme.a|1.0.0"].Artifact}
	im.mu.Unlock()
	im.Activate(id, "acme.a", "1.0.0")
	return id
}

// tx.start / wf.start check the workflow contract's permission against the
// subject, as signals do.
func TestStartChecksTheWorkflowContractPermission(t *testing.T) {
	env := hosttest.New(t)
	im := newImpls()
	publishFlow(t, env, im, "acme.a", "1.0.0", `
export const secured = async () => "ok";
export const flow = async (wf, input) => {
  if (input.who === "parent") {
    try { await wf.start("kid", { id: "acme.test.secured" }, { who: "kid" }); } catch (e) { return e.code; }
  }
  return "done";
};`)
	im.Activate(flowID, "acme.a", "1.0.0")
	secured := securedFlow(t, env, im)
	env.Subjects.Grant(7)
	env.Subjects.Grant(8, "flows.start")
	st := newStack(t, env, im)
	ctx := context.Background()

	startAs := func(user int64, key string) error {
		tx, err := env.DB.Begin(ctx)
		require.NoError(t, err)
		defer func() { _ = tx.Rollback(ctx) }()
		_, err = st.engine.Start(ctx, tx, spi.Scope{Subject: env.Subject(user)}, spi.StartIntent{Workflow: secured, Input: json.RawMessage(`{"who":"x"}`), Key: key})
		return err
	}
	err := startAs(7, "k7")
	var we *pool.WireError
	require.ErrorAs(t, err, &we)
	require.Equal(t, pool.ErrKindDenied, we.Kind)
	require.Equal(t, workflow.CodeStartDenied, we.Code)
	require.NoError(t, startAs(8, "k8"), "a holder of the permission may start it")

	// The same rule applies to a workflow starting a child as its subject.
	id := st.start(7, "p", `{"who":"parent"}`)
	st.drain()
	inst := st.instance(id)
	require.Equal(t, workflow.StatusBlocked, inst.Status)
	require.Equal(t, workflow.BlockDenied, inst.BlockedReason)
	require.Contains(t, inst.BlockedDetail, "flows.start")
	list, err := st.engine.List(ctx, workflow.ListFilter{TenantID: env.Tenant, Workflow: secured})
	require.NoError(t, err)
	require.Empty(t, list, "no child was started")
}

// gated adds the package gate to the Implementations double.
type gated struct {
	*impls
	mu     sync.Mutex
	active map[string]bool
}

func (g *gated) PackageActive(_ context.Context, pkg string) (bool, error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.active[pkg], nil
}

func (g *gated) set(pkg string, active bool) {
	g.mu.Lock()
	g.active[pkg] = active
	g.mu.Unlock()
}

// A live instance of a deactivated package does not execute its code: its
// next run blocks with a diagnostic, and Resume continues it after the
// package is activated again.
func TestInstanceOfADeactivatedPackageBlocksAndResumesAfterReactivation(t *testing.T) {
	env := hosttest.New(t)
	im := newImpls()
	publishFlow(t, env, im, "acme.a", "1.0.0", `
export const flow = async (wf) => {
  await wf.waitFor("w", go);
  return await wf.step("after", async () => "ran");
};`)
	im.Activate(flowID, "acme.a", "1.0.0")
	env.Subjects.Grant(7)
	g := &gated{impls: im, active: map[string]bool{"acme.a": true}}
	st := newStack(t, env, im, func(c *workflow.Config, _ *host.Config) { c.Impl = g })

	id := st.start(7, "k", `{"who":"w"}`)
	st.drain()
	require.Equal(t, workflow.StatusWaiting, st.status(id))

	g.set("acme.a", false)
	st.calls.reset()
	require.NoError(t, st.signal(id, "go", `{"note":"n"}`, ""))
	st.drain()
	inst := st.instance(id)
	require.Equal(t, workflow.StatusBlocked, inst.Status)
	require.Equal(t, workflow.BlockPackageDeactivated, inst.BlockedReason)
	require.Contains(t, inst.BlockedDetail, "acme.a")
	require.Zero(t, st.calls.count("wf.step.begin"), "the disabled code did not run")

	// Resuming while the package is still off blocks it again.
	require.NoError(t, st.engine.Resume(context.Background(), env.Tenant, id))
	st.drain()
	require.Equal(t, workflow.BlockPackageDeactivated, st.instance(id).BlockedReason)

	g.set("acme.a", true)
	require.NoError(t, st.engine.Resume(context.Background(), env.Tenant, id))
	st.drain()
	require.Equal(t, workflow.StatusCompleted, st.status(id))
	require.JSONEq(t, `"ran"`, string(st.instance(id).Result))
}

// Resolve and Retry read the status under the instance row lock: a Resume
// that commits first is seen, and the journal of a running instance is not
// edited.
func TestResolveAndRetryRecheckBlockedStatusUnderTheInstanceLock(t *testing.T) {
	for _, action := range []string{"resolve", "retry"} {
		t.Run(action, func(t *testing.T) {
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
export const flow = async (wf) => wf.call("c", { id: "acme.test.crashy" }, null);`)
			im.Activate(flowID, "acme.a", "1.0.0")
			env.Subjects.Grant(7)
			st := newStack(t, env, im)
			id := st.start(7, "k", `{"who":"w"}`)
			st.until(10*time.Second, func() bool { return st.status(id) == workflow.StatusBlocked }, "ambiguous call blocks the instance")
			ctx := context.Background()

			// A concurrent Resume holds the row and commits `running`.
			other, err := env.DB.Begin(ctx)
			require.NoError(t, err)
			defer func() { _ = other.Rollback(ctx) }()
			_, err = other.Exec(ctx, `SELECT 1 FROM extruntime.workflow_instances WHERE id = $1 FOR UPDATE`, id)
			require.NoError(t, err)

			errc := make(chan error, 1)
			go func() {
				if action == "resolve" {
					errc <- st.engine.Resolve(ctx, env.Tenant, id, "c", 0, json.RawMessage(`{"v":7}`))
				} else {
					errc <- st.engine.Retry(ctx, env.Tenant, id, "c", 0)
				}
			}()
			select {
			case err := <-errc:
				require.Failf(t, "operator action did not wait for the instance lock", "returned %v", err)
			case <-time.After(300 * time.Millisecond):
			}
			_, err = other.Exec(ctx, `UPDATE extruntime.workflow_instances SET status = 'running', blocked_reason = '', updated_at = now() WHERE id = $1`, id)
			require.NoError(t, err)
			require.NoError(t, other.Commit(ctx))

			err = <-errc
			require.ErrorIs(t, err, workflow.ErrNotAccepting)
			o, ok := occ(st.history(id), "c", 0)
			require.True(t, ok, "the occurrence of a running instance was not deleted")
			require.Equal(t, workflow.OccStarted, o.Status, "nor completed")
		})
	}
}
