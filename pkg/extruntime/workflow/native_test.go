package workflow_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"sort"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/data"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/host"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/host/hosttest"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/manifest"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/native"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/pool"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/spi"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/workflow"
	"github.com/iota-uz/iota-sdk/pkg/itf"
)

// nativeEnv is a database without a Node pool, a package catalog or
// artifacts: the engine runs native contracts alone.
type nativeEnv struct {
	t        *testing.T
	db       *pgxpool.Pool
	tenant   uuid.UUID
	subjects *hosttest.Subjects
	logger   *slog.Logger
}

func newNativeEnv(t *testing.T) *nativeEnv {
	t.Helper()
	h := itf.NewHarness(t, itf.HarnessConfig{
		Name:      "extruntime_native",
		Database:  itf.DatabaseConfig{Provisioning: itf.ProvisioningPerTestDatabase},
		Isolation: itf.IsolationConfig{Mode: itf.IsolationCommitted},
	})
	scope := h.Scope(t)
	_, err := scope.Pool.Exec(context.Background(), `CREATE TABLE native_state (instance_id uuid NOT NULL, step text NOT NULL, PRIMARY KEY (instance_id, step))`)
	require.NoError(t, err)
	return &nativeEnv{
		t: t, db: scope.Pool, tenant: scope.Tenant.ID, subjects: hosttest.NewSubjects(),
		logger: slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError})),
	}
}

func (e *nativeEnv) engine(natives map[string]workflow.NativeContract, tune ...func(*workflow.Config)) *workflow.Engine {
	e.t.Helper()
	cfg := workflow.Config{
		DB: e.db, Subjects: e.subjects, Natives: natives,
		LeaseTTL: 5 * time.Second, Heartbeat: time.Second, PollInterval: 50 * time.Millisecond, RetryBackoff: 20 * time.Millisecond,
		Logger: e.logger,
	}
	for _, f := range tune {
		f(&cfg)
	}
	eng, err := workflow.New(cfg)
	require.NoError(e.t, err)
	e.t.Cleanup(eng.Shutdown)
	return eng
}

func (e *nativeEnv) subject(user int64) spi.Subject {
	return spi.Subject{TenantID: e.tenant, UserID: user, Kind: spi.SubjectInitiator}
}

func (e *nativeEnv) start(eng *workflow.Engine, wf, key, input string) uuid.UUID {
	e.t.Helper()
	ctx := context.Background()
	tx, err := e.db.Begin(ctx)
	require.NoError(e.t, err)
	defer func() { _ = tx.Rollback(ctx) }()
	id, err := eng.Start(ctx, tx, spi.Scope{Subject: e.subject(1)}, spi.StartIntent{Workflow: wf, Input: json.RawMessage(input), Key: key})
	require.NoError(e.t, err)
	require.NoError(e.t, tx.Commit(ctx))
	return uuid.MustParse(id)
}

// eventuallyStatus waits for the instance to reach status; read errors are
// reported through the collector, never from the polling goroutine.
func (e *nativeEnv) eventuallyStatus(eng *workflow.Engine, id uuid.UUID, status, msg string) {
	e.t.Helper()
	require.EventuallyWithT(e.t, func(c *assert.CollectT) {
		h, err := eng.Get(context.Background(), e.tenant, id)
		if assert.NoError(c, err) {
			assert.Equal(c, status, h.Instance.Status)
		}
	}, 10*time.Second, 20*time.Millisecond, msg)
}

func (e *nativeEnv) inst(eng *workflow.Engine, id uuid.UUID) workflow.Instance {
	e.t.Helper()
	h, err := eng.Get(context.Background(), e.tenant, id)
	require.NoError(e.t, err)
	return h.Instance
}

func (e *nativeEnv) drainUntil(eng *workflow.Engine, cond func() bool, msg string) {
	e.t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		_, err := eng.RunOnce(context.Background())
		require.NoError(e.t, err)
		if cond() {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	require.Fail(e.t, "condition not met: "+msg)
}

func (e *nativeEnv) stateRows(id uuid.UUID) []string {
	e.t.Helper()
	rows, err := e.db.Query(context.Background(), `SELECT step FROM native_state WHERE instance_id = $1 ORDER BY step`, id)
	require.NoError(e.t, err)
	steps, err := pgx.CollectRows(rows, pgx.RowTo[string])
	require.NoError(e.t, err)
	return steps
}

func writeState(ctx context.Context, tx pgx.Tx, id uuid.UUID, step string) error {
	_, err := tx.Exec(ctx, `INSERT INTO native_state (instance_id, step) VALUES ($1, $2)`, id, step)
	return err
}

func anyPayload() manifest.Type { return manifest.Type{Kind: manifest.KindJSON} }

// A parent writes state, starts a child and waits; the child signals the
// parent from inside its own transaction; the parent completes with the
// payload. A failing transaction leaves neither state nor receipt.
func TestNative_ParentStartsChildThatSignalsBack(t *testing.T) {
	env := newNativeEnv(t)
	var eng *workflow.Engine
	var rolledBack sync.Once
	var lookupAfterRollback bool
	parent := workflow.NativeRunnerFunc(func(ctx context.Context, run workflow.NativeRun) (json.RawMessage, error) {
		id := run.Instance().ID
		rolledBack.Do(func() {
			_, err := run.Transaction(ctx, "broken", 0, func(ctx context.Context, tx pgx.Tx) (json.RawMessage, error) {
				if err := writeState(ctx, tx, id, "broken"); err != nil {
					return nil, err
				}
				return nil, errors.New("domain says no")
			})
			require.EqualError(t, err, "domain says no")
			_, found, err := run.Lookup(ctx, "broken", 0)
			require.NoError(t, err)
			lookupAfterRollback = found
		})
		if _, err := run.Transaction(ctx, "init", 0, func(ctx context.Context, tx pgx.Tx) (json.RawMessage, error) {
			return json.RawMessage(`{"ok":1}`), writeState(ctx, tx, id, "init")
		}); err != nil {
			return nil, err
		}
		child, err := run.Start(ctx, "child", 0, "test.child", json.RawMessage(fmt.Sprintf(`{"parent":%q}`, id)), workflow.StartOptions{})
		if err != nil {
			return nil, err
		}
		w, err := run.WaitFor(ctx, "done", 0, "childDone", 0)
		if err != nil {
			return nil, err
		}
		return json.Marshal(map[string]any{"child": child, "payload": w.Payload})
	})
	child := workflow.NativeRunnerFunc(func(ctx context.Context, run workflow.NativeRun) (json.RawMessage, error) {
		var in struct {
			Parent string `json:"parent"`
		}
		if err := json.Unmarshal(run.Instance().Input, &in); err != nil {
			return nil, err
		}
		return run.Transaction(ctx, "notify", 0, func(ctx context.Context, tx pgx.Tx) (json.RawMessage, error) {
			if err := writeState(ctx, tx, run.Instance().ID, "notify"); err != nil {
				return nil, err
			}
			return nil, eng.SignalInTx(ctx, tx, spi.Scope{Subject: run.Subject()}, host.SignalIntent{
				Workflow: "test.parent", Instance: in.Parent, Name: "childDone", Payload: json.RawMessage(`{"from":"child"}`), Key: "child-done",
			})
		})
	})
	eng = env.engine(map[string]workflow.NativeContract{
		"test.parent": {Runner: parent, Signals: map[string]manifest.Type{"childDone": anyPayload()}},
		"test.child":  {Runner: child},
	})
	pid := env.start(eng, "test.parent", "p1", `{}`)
	env.drainUntil(eng, func() bool { return env.inst(eng, pid).Status == workflow.StatusCompleted }, "parent completes")

	p := env.inst(eng, pid)
	require.True(t, p.Native)
	require.Empty(t, p.Package)
	require.Empty(t, p.Version)
	var res struct {
		Child   uuid.UUID       `json:"child"`
		Payload json.RawMessage `json:"payload"`
	}
	require.NoError(t, json.Unmarshal(p.Result, &res))
	require.JSONEq(t, `{"from":"child"}`, string(res.Payload))
	c := env.inst(eng, res.Child)
	require.Equal(t, workflow.StatusCompleted, c.Status)
	require.NotNil(t, c.ParentID)
	require.Equal(t, pid, *c.ParentID)
	require.Equal(t, []string{"init"}, env.stateRows(pid), "the rolled back transaction left no state")
	require.Equal(t, []string{"notify"}, env.stateRows(res.Child))
	require.False(t, lookupAfterRollback, "a rolled back transaction leaves no receipt")

	h, err := eng.Get(context.Background(), env.tenant, pid)
	require.NoError(t, err)
	_, ok := occ(h, "broken", 0)
	require.False(t, ok)
	init, ok := occ(h, "init", 0)
	require.True(t, ok)
	require.JSONEq(t, `{"ok":1}`, string(init.Result))

	kids, err := eng.List(context.Background(), workflow.ListFilter{TenantID: env.tenant, ParentID: &pid})
	require.NoError(t, err)
	require.Len(t, kids, 1)
	native := false
	pkgs, err := eng.List(context.Background(), workflow.ListFilter{TenantID: env.tenant, Native: &native})
	require.NoError(t, err)
	require.Empty(t, pkgs)
}

// A run whose lease was taken over gets a fencing error from Transaction and
// writes nothing, its own table included.
func TestNative_TransactionAfterLostLeaseIsFenced(t *testing.T) {
	env := newNativeEnv(t)
	var fenced error
	runner := workflow.NativeRunnerFunc(func(ctx context.Context, run workflow.NativeRun) (json.RawMessage, error) {
		// Another runner claims the instance meanwhile.
		_, err := env.db.Exec(ctx, `UPDATE extruntime.workflow_instances SET lease_epoch = lease_epoch + 1 WHERE id = $1`, run.Instance().ID)
		require.NoError(t, err)
		_, fenced = run.Transaction(ctx, "write", 0, func(ctx context.Context, tx pgx.Tx) (json.RawMessage, error) {
			return nil, writeState(ctx, tx, run.Instance().ID, "write")
		})
		return nil, fenced
	})
	eng := env.engine(map[string]workflow.NativeContract{"test.fenced": {Runner: runner}})
	id := env.start(eng, "test.fenced", "f1", `{}`)
	n, err := eng.RunOnce(context.Background())
	require.NoError(t, err)
	require.Equal(t, 1, n)
	require.ErrorIs(t, fenced, workflow.ErrFenced)
	require.Empty(t, env.stateRows(id))
	h, err := eng.Get(context.Background(), env.tenant, id)
	require.NoError(t, err)
	require.Empty(t, h.Occurrences)
	require.Equal(t, workflow.StatusRunning, h.Instance.Status, "the abandoned run records no outcome")
}

// An Effect whose process died after it was recorded as started runs again
// on the next run with Attempt 2 and the same Key.
func TestNative_EffectInterruptedAfterStartRetriesWithTheSameKey(t *testing.T) {
	env := newNativeEnv(t)
	type attempt struct {
		n   int
		key string
	}
	attempts := make(chan attempt, 4)
	release := make(chan struct{})
	runner := workflow.NativeRunnerFunc(func(ctx context.Context, run workflow.NativeRun) (json.RawMessage, error) {
		return run.Effect(ctx, "pay", 0, func(ctx context.Context, a workflow.EffectAttempt) (json.RawMessage, error) {
			attempts <- attempt{a.Attempt, a.Key}
			if a.Attempt == 1 {
				<-release // the process "dies" here
				return json.RawMessage(`"first"`), nil
			}
			return json.RawMessage(`"second"`), nil
		})
	})
	natives := map[string]workflow.NativeContract{"test.effect": {Runner: runner}}
	slow := func(c *workflow.Config) { c.LeaseTTL, c.Heartbeat = 30*time.Second, time.Hour }
	a := env.engine(natives, slow)
	b := env.engine(natives, slow)
	id := env.start(a, "test.effect", "e1", `{}`)

	doneA := make(chan error, 1)
	go func() { _, err := a.RunOnce(context.Background()); doneA <- err }()
	var first attempt
	select {
	case first = <-attempts:
	case err := <-doneA:
		require.FailNow(t, "the first run ended without its effect", "%v %+v", err, env.inst(a, id))
	case <-time.After(10 * time.Second):
		require.FailNow(t, "the first attempt did not start")
	}
	require.Equal(t, 1, first.n)
	_, err := env.db.Exec(context.Background(), `UPDATE extruntime.workflow_instances SET lease_until = now() - interval '1 second' WHERE id = $1`, id)
	require.NoError(t, err)
	_, err = b.RunOnce(context.Background())
	require.NoError(t, err)
	var second attempt
	select {
	case second = <-attempts:
	case <-time.After(10 * time.Second):
		require.FailNow(t, "the second run did not retry the effect", "%+v", env.inst(b, id))
	}
	require.Equal(t, 2, second.n)
	require.Equal(t, first.key, second.key)
	require.Equal(t, id.String()+":pay:0", second.key)
	close(release)
	require.NoError(t, <-doneA)

	inst := env.inst(b, id)
	require.Equal(t, workflow.StatusCompleted, inst.Status)
	require.JSONEq(t, `"second"`, string(inst.Result), "the stale run could not record its outcome")
	h, err := b.Get(context.Background(), env.tenant, id)
	require.NoError(t, err)
	pay, ok := occ(h, "pay", 0)
	require.True(t, ok)
	require.Equal(t, 2, pay.Attempts)
	require.Equal(t, workflow.OccCompleted, pay.Status)
}

// joinAny waits for a signal, a timer and a task at once; the first that
// arrives completes the instance.
func joinAny(user int64) workflow.NativeRunner {
	return workflow.NativeRunnerFunc(func(ctx context.Context, run workflow.NativeRun) (json.RawMessage, error) {
		w, err := run.WaitFor(ctx, "go", 0, "go", 0)
		if err == nil {
			return w.Payload, nil
		}
		if !errors.Is(err, workflow.ErrSuspend) {
			return nil, err
		}
		if err := run.Sleep(ctx, "nap", 0, time.Hour); !errors.Is(err, workflow.ErrSuspend) {
			return nil, fmt.Errorf("sleep: %w", err)
		}
		uid := user
		if _, err := run.Task(ctx, "approve", 0, workflow.TaskSpec{
			Assignees: []workflow.TaskAssignee{{Kind: workflow.TaskAssignUser, UserID: &uid}},
			Title:     manifest.Label{"en": "Approve"},
		}); !errors.Is(err, workflow.ErrSuspend) {
			return nil, fmt.Errorf("task: %w", err)
		}
		return nil, workflow.ErrSuspend
	})
}

func openTasks(t *testing.T, db *pgxpool.Pool, user int64) int {
	t.Helper()
	var n int
	require.NoError(t, db.QueryRow(context.Background(), `SELECT count(*) FROM extruntime.human_tasks WHERE assignee_user_id = $1 AND status = 'open'`, user).Scan(&n))
	return n
}

// Completion cancels the waits and tasks still open (CancelPending): the
// task leaves the inbox.
func TestNative_CompletionCancelsPendingWaitsAndTasks(t *testing.T) {
	env := newNativeEnv(t)
	eng := env.engine(map[string]workflow.NativeContract{
		"test.any": {Runner: joinAny(7), Signals: map[string]manifest.Type{"go": anyPayload()}},
	})
	id := env.start(eng, "test.any", "a1", `{}`)
	env.drainUntil(eng, func() bool { return env.inst(eng, id).Status == workflow.StatusWaiting }, "suspends")
	require.Equal(t, 1, openTasks(t, env.db, 7))
	h, err := eng.Get(context.Background(), env.tenant, id)
	require.NoError(t, err)
	require.Len(t, h.Tasks, 1)
	require.Equal(t, manifest.Label{"en": "Approve"}, h.Tasks[0].Title)

	require.NoError(t, eng.Signal(context.Background(), env.tenant, 1, id, "go", json.RawMessage(`{"winner":"signal"}`), ""))
	env.drainUntil(eng, func() bool { return env.inst(eng, id).Status == workflow.StatusCompleted }, "completes")

	h, err = eng.Get(context.Background(), env.tenant, id)
	require.NoError(t, err)
	for _, name := range []string{"nap", "approve"} {
		o, ok := occ(h, name, 0)
		require.True(t, ok, name)
		require.Equal(t, workflow.OccCancelled, o.Status, name)
	}
	require.Equal(t, "cancelled", h.Tasks[0].Status)
	require.Zero(t, openTasks(t, env.db, 7), "the task left the inbox")

	// An undeclared signal is rejected.
	err = eng.Signal(context.Background(), env.tenant, 1, id, "nope", json.RawMessage(`{}`), "")
	require.ErrorIs(t, err, workflow.ErrInvalid)
}

// KeepOpen keeps package-workflow behaviour: open tasks survive completion.
func TestNative_KeepOpenLeavesTasks(t *testing.T) {
	env := newNativeEnv(t)
	eng := env.engine(map[string]workflow.NativeContract{
		"test.keep": {Runner: joinAny(8), Signals: map[string]manifest.Type{"go": anyPayload()}, Completion: workflow.KeepOpen},
	})
	id := env.start(eng, "test.keep", "k1", `{}`)
	env.drainUntil(eng, func() bool { return env.inst(eng, id).Status == workflow.StatusWaiting }, "suspends")
	require.NoError(t, eng.Signal(context.Background(), env.tenant, 1, id, "go", json.RawMessage(`{}`), ""))
	env.drainUntil(eng, func() bool { return env.inst(eng, id).Status == workflow.StatusCompleted }, "completes")
	require.Equal(t, 1, openTasks(t, env.db, 8))
}

// Cancel cancels one waiting task; Task then reports it cancelled, and a
// decided task reports its decision.
func TestNative_CancelAndDecideTasks(t *testing.T) {
	env := newNativeEnv(t)
	var gotCancelled, gotDecided workflow.TaskResult
	uid := int64(9)
	spec := workflow.TaskSpec{Assignees: []workflow.TaskAssignee{{Kind: workflow.TaskAssignUser, UserID: &uid}}}
	runner := workflow.NativeRunnerFunc(func(ctx context.Context, run workflow.NativeRun) (json.RawMessage, error) {
		if _, err := run.Task(ctx, "a", 0, spec); err != nil && !errors.Is(err, workflow.ErrSuspend) {
			return nil, err
		}
		if err := run.Cancel(ctx, "a", 0); err != nil {
			return nil, err
		}
		if err := run.Cancel(ctx, "a", 0); err != nil { // a repeat is a no-op
			return nil, err
		}
		r, err := run.Task(ctx, "a", 0, spec)
		if err != nil {
			return nil, err
		}
		gotCancelled = r
		r, err = run.Task(ctx, "b", 0, spec)
		if err != nil {
			return nil, err
		}
		gotDecided = r
		return nil, nil
	})
	eng := env.engine(map[string]workflow.NativeContract{"test.tasks": {Runner: runner}})
	id := env.start(eng, "test.tasks", "t1", `{}`)
	env.drainUntil(eng, func() bool { return env.inst(eng, id).Status == workflow.StatusWaiting }, "waits for b")
	require.Equal(t, workflow.TaskCancelled, gotCancelled.Outcome)
	h, err := eng.Get(context.Background(), env.tenant, id)
	require.NoError(t, err)
	var b workflow.Task
	for _, tk := range h.Tasks {
		if tk.Name == "b" {
			b = tk
		}
	}
	require.Equal(t, "open", b.Status)
	env.subjects.Grant(uid)
	require.NoError(t, eng.DecideTask(context.Background(), env.subject(uid), b.ID, json.RawMessage(`{"ok":true}`)))
	env.drainUntil(eng, func() bool { return env.inst(eng, id).Status == workflow.StatusCompleted }, "completes")
	require.Equal(t, workflow.TaskDecided, gotDecided.Outcome)
	require.JSONEq(t, `{"ok":true}`, string(gotDecided.Decision))
	require.Equal(t, uid, *gotDecided.By)
	require.Len(t, gotDecided.Decisions, 1)
	require.Equal(t, uid, gotDecided.Decisions[0].UserID)
}

// waitForever starts a child (when the input says so) and waits.
func waitForever() workflow.NativeRunner {
	return workflow.NativeRunnerFunc(func(ctx context.Context, run workflow.NativeRun) (json.RawMessage, error) {
		var in struct {
			Depth int `json:"depth"`
		}
		_ = json.Unmarshal(run.Instance().Input, &in)
		if in.Depth > 0 {
			if _, err := run.Start(ctx, "child", 0, "test.tree", json.RawMessage(fmt.Sprintf(`{"depth":%d}`, in.Depth-1)), workflow.StartOptions{}); err != nil {
				return nil, err
			}
		}
		if _, err := run.WaitFor(ctx, "never", 0, "never", 0); err != nil {
			return nil, err
		}
		return nil, nil
	})
}

func childrenOf(t *testing.T, eng *workflow.Engine, tenant, id uuid.UUID) []workflow.Instance {
	t.Helper()
	kids, err := eng.List(context.Background(), workflow.ListFilter{TenantID: tenant, ParentID: &id})
	require.NoError(t, err)
	return kids
}

// TerminateTree stops a three-level tree; Terminate leaves the children.
func TestNative_TerminateTreeStopsDescendantsAndTerminateDoesNot(t *testing.T) {
	env := newNativeEnv(t)
	eng := env.engine(map[string]workflow.NativeContract{
		"test.tree": {Runner: waitForever(), Signals: map[string]manifest.Type{"never": anyPayload()}},
	})
	root := env.start(eng, "test.tree", "root", `{"depth":2}`)
	other := env.start(eng, "test.tree", "other", `{"depth":1}`)
	waiting := func(id uuid.UUID) bool { return env.inst(eng, id).Status == workflow.StatusWaiting }
	env.drainUntil(eng, func() bool {
		kids := childrenOf(t, eng, env.tenant, root)
		if len(kids) != 1 || len(childrenOf(t, eng, env.tenant, kids[0].ID)) != 1 {
			return false
		}
		return waiting(root) && waiting(other) && len(childrenOf(t, eng, env.tenant, other)) == 1
	}, "the trees are built")
	child := childrenOf(t, eng, env.tenant, root)[0].ID
	grandchild := childrenOf(t, eng, env.tenant, child)[0].ID
	otherChild := childrenOf(t, eng, env.tenant, other)[0].ID
	env.drainUntil(eng, func() bool { return waiting(grandchild) && waiting(otherChild) && waiting(child) }, "all wait")

	require.NoError(t, eng.Terminate(context.Background(), env.tenant, other))
	require.Equal(t, workflow.StatusTerminated, env.inst(eng, other).Status)
	require.Equal(t, workflow.StatusWaiting, env.inst(eng, otherChild).Status, "Terminate does not cascade")

	require.NoError(t, eng.TerminateTree(context.Background(), env.tenant, root))
	for _, id := range []uuid.UUID{root, child, grandchild} {
		require.Equal(t, workflow.StatusTerminated, env.inst(eng, id).Status)
	}
	require.ErrorIs(t, eng.TerminateTree(context.Background(), env.tenant, root), workflow.ErrNotAccepting)
}

// observer records the transitions of its contract and fails on demand.
type observer struct {
	mu    sync.Mutex
	seen  []string
	fail  bool
	calls int
}

func (o *observer) InstanceStatusChanged(ctx context.Context, tx pgx.Tx, inst workflow.Instance, from, to string) error {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.calls++
	if o.fail {
		return errors.New("observer is down")
	}
	// The observer runs in the transaction of the transition.
	var status string
	if err := tx.QueryRow(ctx, `SELECT status FROM extruntime.workflow_instances WHERE id = $1`, inst.ID).Scan(&status); err != nil {
		return err
	}
	o.seen = append(o.seen, from+">"+to+"="+status)
	return nil
}

func (o *observer) snapshot() ([]string, int) {
	o.mu.Lock()
	defer o.mu.Unlock()
	return append([]string(nil), o.seen...), o.calls
}

func (o *observer) setFail(f bool) {
	o.mu.Lock()
	o.fail = f
	o.mu.Unlock()
}

// The StatusObserver sees completed, failed and terminated in the
// transaction; its failure rolls the transition back and it is retried.
func TestNative_StatusObserver(t *testing.T) {
	env := newNativeEnv(t)
	obs := &observer{}
	runner := workflow.NativeRunnerFunc(func(ctx context.Context, run workflow.NativeRun) (json.RawMessage, error) {
		switch string(run.Instance().Input) {
		case `"fail"`:
			return nil, errors.New("business failure")
		case `"wait"`:
			_, err := run.WaitFor(ctx, "w", 0, "go", 0)
			return nil, err
		}
		return json.RawMessage(`1`), nil
	})
	eng := env.engine(map[string]workflow.NativeContract{
		"test.obs": {Runner: runner, StatusObserver: obs, Signals: map[string]manifest.Type{"go": anyPayload()}},
	})
	obs.setFail(true)
	done := env.start(eng, "test.obs", "o1", `"done"`)
	_, err := eng.RunOnce(context.Background())
	require.NoError(t, err)
	require.Equal(t, workflow.StatusRunning, env.inst(eng, done).Status, "the failing observer rolled the transition back")
	_, calls := obs.snapshot()
	require.Equal(t, 1, calls)
	obs.setFail(false)
	env.drainUntil(eng, func() bool { return env.inst(eng, done).Status == workflow.StatusCompleted }, "completes on retry")

	failed := env.start(eng, "test.obs", "o2", `"fail"`)
	env.drainUntil(eng, func() bool { return env.inst(eng, failed).Status == workflow.StatusFailed }, "fails")
	var we struct {
		Message string `json:"message"`
	}
	require.NoError(t, json.Unmarshal(env.inst(eng, failed).Error, &we))
	require.Contains(t, we.Message, "business failure")

	waiting := env.start(eng, "test.obs", "o3", `"wait"`)
	env.drainUntil(eng, func() bool { return env.inst(eng, waiting).Status == workflow.StatusWaiting }, "waits")
	require.NoError(t, eng.Terminate(context.Background(), env.tenant, waiting))

	seen, _ := obs.snapshot()
	require.Equal(t, []string{"running>completed=completed", "running>failed=failed", "waiting>terminated=terminated"}, seen)
}

// A package instance in an engine without the package runtime blocks as
// unavailable; a native instance needs no Node, catalog or artifacts.
func TestNative_PackageInstanceWithoutRuntimeBlocks(t *testing.T) {
	env := newNativeEnv(t)
	eng := env.engine(nil)
	id := uuid.New()
	_, err := env.db.Exec(context.Background(), `
		INSERT INTO extruntime.workflow_instances (id, tenant_id, workflow, package, version, export, input, subject_user_id, subject_kind)
		VALUES ($1, $2, 'acme.flow', 'acme', '1.0.0', 'flow', '{}', 1, 'initiator')`, id, env.tenant)
	require.NoError(t, err)
	_, err = eng.RunOnce(context.Background())
	require.NoError(t, err)
	inst := env.inst(eng, id)
	require.Equal(t, workflow.StatusBlocked, inst.Status)
	require.Equal(t, workflow.BlockUnavailable, inst.BlockedReason)
}

// Waking a native instance costs what the run reads, not the journal size.
func TestNative_WakeTimeDoesNotDependOnJournalSize(t *testing.T) {
	env := newNativeEnv(t)
	runner := workflow.NativeRunnerFunc(func(ctx context.Context, run workflow.NativeRun) (json.RawMessage, error) {
		if _, _, err := run.Lookup(ctx, "seed", 0); err != nil {
			return nil, err
		}
		_, err := run.WaitFor(ctx, "tick", 0, "tick", 0)
		return nil, err
	})
	eng := env.engine(map[string]workflow.NativeContract{"test.big": {Runner: runner, Signals: map[string]manifest.Type{"tick": anyPayload()}}})
	seed := func(id uuid.UUID, n int) {
		_, err := env.db.Exec(context.Background(), `
			INSERT INTO extruntime.workflow_occurrences (instance_id, tenant_id, name, n, kind, status, result, attempts)
			SELECT $1, $2, 'seed', g, 'now', 'completed', to_jsonb(repeat('x', 200)), 1 FROM generate_series(0, $3 - 1) g`, id, env.tenant, n)
		require.NoError(t, err)
	}
	measure := func(id uuid.UUID) time.Duration {
		var samples []time.Duration
		for range 7 {
			_, err := env.db.Exec(context.Background(), `UPDATE extruntime.workflow_instances SET status = 'waiting', wake_at = now() WHERE id = $1`, id)
			require.NoError(t, err)
			start := time.Now()
			n, err := eng.RunOnce(context.Background())
			require.NoError(t, err)
			require.Equal(t, 1, n)
			samples = append(samples, time.Since(start))
		}
		sort.Slice(samples, func(i, j int) bool { return samples[i] < samples[j] })
		return samples[len(samples)/2]
	}
	small := env.start(eng, "test.big", "small", `{}`)
	big := env.start(eng, "test.big", "big", `{}`)
	seed(small, 40)
	seed(big, 4000)
	_, err := eng.RunOnce(context.Background())
	require.NoError(t, err)
	tSmall, tBig := measure(small), measure(big)
	t.Logf("wake with 40 occurrences: %v, with 4000: %v", tSmall, tBig)
	require.Less(t, tBig, 2*tSmall+15*time.Millisecond, "the wake of a big journal stays within the noise of a small one")
}

// Serve runs native contracts in their own slot pool, without a package
// runner, and wakes on a signal.
func TestNative_ServeRunsWithoutPackageRuntime(t *testing.T) {
	env := newNativeEnv(t)
	eng := env.engine(map[string]workflow.NativeContract{
		"test.serve": {Runner: joinAny(11), Signals: map[string]manifest.Type{"go": anyPayload()}, Concurrency: 1},
	})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	eng.Serve(ctx)
	id := env.start(eng, "test.serve", "s1", `{}`)
	env.eventuallyStatus(eng, id, workflow.StatusWaiting, "suspends")
	require.NoError(t, eng.Signal(context.Background(), env.tenant, 1, id, "go", json.RawMessage(`{"v":1}`), ""))
	env.eventuallyStatus(eng, id, workflow.StatusCompleted, "completes")
	require.JSONEq(t, `{"v":1}`, string(env.inst(eng, id).Result))
}

// A native contract whose ID a package declares as its workflow is a
// composition error; so is an ID registered twice.
func TestNative_RegistrationConflicts(t *testing.T) {
	env := newNativeEnv(t)
	nc := workflow.NativeContract{Runner: joinAny(1)}
	cat := hosttest.NewCatalog()
	cat.Add(spi.ResolvedResource{Package: "acme", Version: "1.0.0", Resource: manifest.Resource{ID: "test.clash", Kind: manifest.ResWorkflow}})
	eng := env.engine(map[string]workflow.NativeContract{"test.clash": nc})
	err := eng.AttachPackages(context.Background(), workflow.Packages{Catalog: cat, Bindings: noBindings{}, Artifacts: &hosttest.Artifacts{}, Impl: newImpls(), Runner: noRunner{}})
	require.ErrorContains(t, err, "also declared as a package workflow")

	dup := env.engine(map[string]workflow.NativeContract{"test.a": nc}, func(c *workflow.Config) {
		c.NativeSource = func() (map[string]workflow.NativeContract, error) {
			return map[string]workflow.NativeContract{"test.a": nc}, nil
		}
	})
	require.ErrorContains(t, dup.Ready(context.Background()), "registered twice")
	_, ok := dup.Native("test.a")
	require.False(t, ok)
}

type noBindings struct{}

func (noBindings) Resolve(context.Context, string) (host.Binding, error) {
	return host.Binding{}, host.ErrNoBinding
}

type noRunner struct{}

func (noRunner) Run(context.Context, host.RunRequest) (host.Result, error) {
	return host.Result{}, errors.New("no runner")
}

type stepIn struct {
	Step string `json:"step"`
}

type stepOut struct {
	Rows int `json:"rows"`
}

// callNatives are native commands a native workflow calls: an atomic one
// whose receipt commits in its transaction, and an ordinary one.
func callNatives(t *testing.T) (*native.Set, *atomic.Int32) {
	t.Helper()
	var ordinaryRuns atomic.Int32
	insert := func(ctx context.Context, tx pgx.Tx, in stepIn) (stepOut, error) {
		inst, err := uuid.Parse(in.Step[:36])
		if err != nil {
			return stepOut{}, err
		}
		if _, err := tx.Exec(ctx, `INSERT INTO native_state (instance_id, step) VALUES ($1, $2)`, inst, in.Step[37:]); err != nil {
			return stepOut{}, err
		}
		var n int
		err = tx.QueryRow(ctx, `SELECT count(*) FROM native_state WHERE instance_id = $1`, inst).Scan(&n)
		return stepOut{Rows: n}, err
	}
	atomicCmd := native.AtomicCommand("test.cmd.atomic", native.Spec{}, func(native.Deps) (native.OperationFunc[stepIn, stepOut], error) {
		return func(ctx context.Context, tx native.Tx, in stepIn) (stepOut, error) {
			rtx, err := composables.UseTx(ctx)
			if err != nil {
				return stepOut{}, err
			}
			return insert(ctx, rtx.(pgx.Tx), in)
		}, nil
	})
	ordinary := native.Command("test.cmd.ordinary", native.Spec{}, func(native.Deps) (native.CommandFunc[stepIn, stepOut], error) {
		return func(ctx context.Context, c native.CommandContext, in stepIn) (stepOut, error) {
			ordinaryRuns.Add(1)
			var out stepOut
			err := c.Transaction(ctx, func(ctx context.Context, _ native.Tx) error {
				rtx, err := composables.UseTx(ctx)
				if err != nil {
					return err
				}
				out, err = insert(ctx, rtx.(pgx.Tx), in)
				return err
			})
			return out, err
		}, nil
	})
	set := native.NewSet()
	require.NoError(t, set.Add(atomicCmd, ordinary))
	return set, &ordinaryRuns
}

// NativeRun.Call dispatches native commands through a host without a Node
// pool; a recorded call is not dispatched again.
func TestNative_CallRunsNativeCommandsWithoutTheRuntime(t *testing.T) {
	env := newNativeEnv(t)
	set, ordinaryRuns := callNatives(t)
	cat := hosttest.NewCatalog()
	for _, r := range set.Resources() {
		cat.Add(spi.ResolvedResource{Resource: r, Native: true})
	}
	runner := workflow.NativeRunnerFunc(func(ctx context.Context, run workflow.NativeRun) (json.RawMessage, error) {
		id := run.Instance().ID.String()
		a, err := run.Call(ctx, "a", 0, "test.cmd.atomic", json.RawMessage(fmt.Sprintf(`{"step":%q}`, id+":atomic")))
		if err != nil {
			return nil, err
		}
		o, err := run.Call(ctx, "o", 0, "test.cmd.ordinary", json.RawMessage(fmt.Sprintf(`{"step":%q}`, id+":ordinary")))
		if err != nil {
			return nil, err
		}
		// A second run replays both from their receipts.
		if err := run.Sleep(ctx, "pause", 0, 100*time.Millisecond); err != nil {
			return nil, err
		}
		return json.Marshal(map[string]json.RawMessage{"a": a, "o": o})
	})
	eng := env.engine(map[string]workflow.NativeContract{"test.calls": {Runner: runner}}, func(c *workflow.Config) { c.Catalog = cat })
	h, err := host.New(host.Config{DB: env.db, Bindings: noBindings{}, Catalog: cat, Data: data.NewStore(cat, nil, nil), Natives: native.Bind(set, nil), Logger: env.logger})
	require.NoError(t, err)
	eng.SetCommands(h)

	id := env.start(eng, "test.calls", "c1", `{}`)
	env.drainUntil(eng, func() bool { return env.inst(eng, id).Status == workflow.StatusCompleted }, "completes")
	require.JSONEq(t, `{"a":{"rows":1},"o":{"rows":2}}`, string(env.inst(eng, id).Result))
	require.Equal(t, []string{"atomic", "ordinary"}, env.stateRows(id))
	require.EqualValues(t, 1, ordinaryRuns.Load())
	hist, err := eng.Get(context.Background(), env.tenant, id)
	require.NoError(t, err)
	for _, name := range []string{"a", "o"} {
		o, ok := occ(hist, name, 0)
		require.True(t, ok)
		require.Equal(t, workflow.OccCompleted, o.Status)
		require.NotEmpty(t, o.InvocationID)
	}
	require.NotEmpty(t, hist.Invocations, "the calls are recorded as invocations of the instance")
}

// An expired lease that nobody took over yet fences the final write (checked
// on the database clock, without a heartbeat in between), and the heartbeat
// does not resurrect it.
func TestNative_ExpiredLeaseWithoutTakeoverIsFenced(t *testing.T) {
	for _, tc := range []struct {
		name      string
		heartbeat time.Duration
	}{{"final fence", time.Hour}, {"heartbeat", 30 * time.Millisecond}} {
		t.Run(tc.name, func(t *testing.T) {
			env := newNativeEnv(t)
			var fenced error
			var renewed bool
			runs := 0
			runner := workflow.NativeRunnerFunc(func(ctx context.Context, run workflow.NativeRun) (json.RawMessage, error) {
				id := run.Instance().ID
				bg := context.Background()
				if runs++; runs > 1 {
					// The next claim of the expired instance runs normally.
					return json.RawMessage(`"retried"`), nil
				}
				_, err := env.db.Exec(bg, `UPDATE extruntime.workflow_instances SET lease_until = clock_timestamp() - interval '1 second' WHERE id = $1`, id)
				require.NoError(t, err)
				time.Sleep(200 * time.Millisecond)
				require.NoError(t, env.db.QueryRow(bg, `SELECT lease_until > clock_timestamp() FROM extruntime.workflow_instances WHERE id = $1`, id).Scan(&renewed))
				_, fenced = run.Transaction(context.WithoutCancel(ctx), "write", 0, func(ctx context.Context, tx pgx.Tx) (json.RawMessage, error) {
					return nil, writeState(ctx, tx, id, "write")
				})
				return nil, fenced
			})
			eng := env.engine(map[string]workflow.NativeContract{"test.expired": {Runner: runner}}, func(c *workflow.Config) {
				c.LeaseTTL, c.Heartbeat = 30*time.Second, tc.heartbeat
			})
			id := env.start(eng, "test.expired", "x1", `{}`)
			_, err := eng.RunOnce(context.Background())
			require.NoError(t, err)
			require.False(t, renewed, "the expired lease was not renewed")
			require.ErrorIs(t, fenced, workflow.ErrFenced)
			require.Empty(t, env.stateRows(id))
			h, err := eng.Get(context.Background(), env.tenant, id)
			require.NoError(t, err)
			require.Empty(t, h.Occurrences)
			require.Equal(t, 2, runs, "the expired instance was claimed again")
			require.JSONEq(t, `"retried"`, string(h.Instance.Result))
		})
	}
}

func insertNativeInstance(t *testing.T, env *nativeEnv, workflowID string, parent *uuid.UUID, status string) uuid.UUID {
	t.Helper()
	id := uuid.New()
	_, err := env.db.Exec(context.Background(), `
		INSERT INTO extruntime.workflow_instances (id, tenant_id, workflow, package, version, export, input, subject_user_id, subject_kind, parent_instance_id, status, native)
		VALUES ($1, $2, $3, '', '', '', '{}', 1, 'initiator', $4, $5, true)`, id, env.tenant, workflowID, parent, status)
	require.NoError(t, err)
	return id
}

// A grandchild inserted while TerminateTree waits for its parent's row is
// still terminated.
func TestNative_TerminateTreeSeesChildrenCreatedDuringTraversal(t *testing.T) {
	env := newNativeEnv(t)
	eng := env.engine(nil)
	ctx := context.Background()
	root := insertNativeInstance(t, env, "test.tree", nil, workflow.StatusWaiting)
	child := insertNativeInstance(t, env, "test.tree", &root, workflow.StatusWaiting)

	tx, err := env.db.Begin(ctx)
	require.NoError(t, err)
	defer func() { _ = tx.Rollback(ctx) }()
	_, err = tx.Exec(ctx, `SELECT 1 FROM extruntime.workflow_instances WHERE id = $1 FOR UPDATE`, child)
	require.NoError(t, err)
	done := make(chan error, 1)
	go func() { done <- eng.TerminateTree(ctx, env.tenant, root) }()
	require.EventuallyWithT(t, func(c *assert.CollectT) {
		var n int
		if assert.NoError(c, env.db.QueryRow(ctx, `SELECT count(*) FROM pg_stat_activity WHERE datname = current_database() AND wait_event_type = 'Lock'`).Scan(&n)) {
			assert.Positive(c, n)
		}
	}, 10*time.Second, 10*time.Millisecond, "TerminateTree waits for the child row")
	grandchild := uuid.New()
	_, err = tx.Exec(ctx, `
		INSERT INTO extruntime.workflow_instances (id, tenant_id, workflow, package, version, export, input, subject_user_id, subject_kind, parent_instance_id, native)
		VALUES ($1, $2, 'test.tree', '', '', '', '{}', 1, 'initiator', $3, true)`, grandchild, env.tenant, child)
	require.NoError(t, err)
	require.NoError(t, tx.Commit(ctx))
	require.NoError(t, <-done)
	for _, id := range []uuid.UUID{root, child, grandchild} {
		require.Equal(t, workflow.StatusTerminated, env.inst(eng, id).Status)
	}
}

// After TerminateTree a descendant's in-flight effect is not cancelled by the
// heartbeat; its outcome is fenced.
func TestNative_TerminateTreeLetsTheDescendantStepFinish(t *testing.T) {
	env := newNativeEnv(t)
	started := make(chan struct{})
	release := make(chan struct{})
	var stepErr, recordErr error
	root := workflow.NativeRunnerFunc(func(ctx context.Context, run workflow.NativeRun) (json.RawMessage, error) {
		if _, err := run.Start(ctx, "child", 0, "test.slow", nil, workflow.StartOptions{}); err != nil {
			return nil, err
		}
		_, err := run.WaitFor(ctx, "never", 0, "never", 0)
		return nil, err
	})
	slow := workflow.NativeRunnerFunc(func(ctx context.Context, run workflow.NativeRun) (json.RawMessage, error) {
		_, recordErr = run.Effect(ctx, "work", 0, func(ctx context.Context, _ workflow.EffectAttempt) (json.RawMessage, error) {
			close(started)
			<-release
			stepErr = ctx.Err()
			return json.RawMessage(`1`), nil
		})
		return nil, recordErr
	})
	eng := env.engine(map[string]workflow.NativeContract{
		"test.root": {Runner: root, Signals: map[string]manifest.Type{"never": anyPayload()}},
		"test.slow": {Runner: slow},
	}, func(c *workflow.Config) { c.Heartbeat = 40 * time.Millisecond })
	rid := env.start(eng, "test.root", "r1", `{}`)
	done := make(chan struct{})
	go func() { defer close(done); _, _ = eng.RunOnce(context.Background()) }()
	select {
	case <-started:
	case <-time.After(10 * time.Second):
		require.FailNow(t, "the child effect did not start")
	}
	require.NoError(t, eng.TerminateTree(context.Background(), env.tenant, rid))
	time.Sleep(200 * time.Millisecond) // several heartbeats
	close(release)
	<-done
	require.NoError(t, stepErr, "the running step was not cancelled")
	require.ErrorIs(t, recordErr, workflow.ErrFenced)
	kids := childrenOf(t, eng, env.tenant, rid)
	require.Len(t, kids, 1)
	require.Equal(t, workflow.StatusTerminated, kids[0].Status)
}

// Serve in a process without the package executor blocks a ready package
// instance as unavailable instead of leaving it running.
func TestNative_ServeBlocksPackageInstancesWithoutRuntime(t *testing.T) {
	env := newNativeEnv(t)
	eng := env.engine(nil, func(c *workflow.Config) { c.Concurrency = 2 })
	require.Equal(t, 2, eng.PackageConcurrency())
	id := uuid.New()
	_, err := env.db.Exec(context.Background(), `
		INSERT INTO extruntime.workflow_instances (id, tenant_id, workflow, package, version, export, input, subject_user_id, subject_kind)
		VALUES ($1, $2, 'acme.flow', 'acme', '1.0.0', 'flow', '{}', 1, 'initiator')`, id, env.tenant)
	require.NoError(t, err)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	eng.Serve(ctx)
	env.eventuallyStatus(eng, id, workflow.StatusBlocked, "blocks")
	require.Equal(t, workflow.BlockUnavailable, env.inst(eng, id).BlockedReason)
}

// holdRow locks the instance row in its own transaction without changing it,
// waits until a statement of the engine is waiting for it and the lease has
// expired, then releases it.
func holdRowUntilExpired(t *testing.T, env *nativeEnv, id uuid.UUID, locked chan<- struct{}) {
	t.Helper()
	ctx := context.Background()
	tx, err := env.db.Begin(ctx)
	require.NoError(t, err)
	defer func() { _ = tx.Rollback(ctx) }()
	_, err = tx.Exec(ctx, `SELECT 1 FROM extruntime.workflow_instances WHERE id = $1 FOR UPDATE`, id)
	require.NoError(t, err)
	if locked != nil {
		close(locked)
	}
	require.EventuallyWithT(t, func(c *assert.CollectT) {
		var waiting int
		var expired bool
		if assert.NoError(c, env.db.QueryRow(ctx, `SELECT count(*) FROM pg_stat_activity WHERE datname = current_database() AND wait_event_type = 'Lock'`).Scan(&waiting)) &&
			assert.NoError(c, env.db.QueryRow(ctx, `SELECT lease_until < clock_timestamp() FROM extruntime.workflow_instances WHERE id = $1`, id).Scan(&expired)) {
			assert.Positive(c, waiting)
			assert.True(c, expired)
		}
	}, 10*time.Second, 10*time.Millisecond, "the engine waits for the row while the lease expires")
	require.NoError(t, tx.Rollback(ctx))
}

// A lease that expires while the final fence waits for a row lock (held by a
// transaction that leaves the row unchanged) fences the write.
func TestNative_LeaseExpiringDuringTheFenceLockWaitIsFenced(t *testing.T) {
	env := newNativeEnv(t)
	inFn := make(chan uuid.UUID, 1)
	locked := make(chan struct{})
	var fenced error
	runs := 0
	runner := workflow.NativeRunnerFunc(func(ctx context.Context, run workflow.NativeRun) (json.RawMessage, error) {
		if runs++; runs > 1 {
			return json.RawMessage(`"retried"`), nil
		}
		_, fenced = run.Transaction(ctx, "write", 0, func(ctx context.Context, tx pgx.Tx) (json.RawMessage, error) {
			inFn <- run.Instance().ID
			<-locked
			return nil, writeState(ctx, tx, run.Instance().ID, "write")
		})
		return nil, fenced
	})
	eng := env.engine(map[string]workflow.NativeContract{"test.lockwait": {Runner: runner, RunTimeout: time.Minute}}, func(c *workflow.Config) {
		c.LeaseTTL, c.Heartbeat = 500*time.Millisecond, time.Hour
	})
	env.start(eng, "test.lockwait", "l1", `{}`)
	done := make(chan error, 1)
	go func() { _, err := eng.RunOnce(context.Background()); done <- err }()
	var id uuid.UUID
	select {
	case id = <-inFn:
	case err := <-done:
		require.FailNow(t, "the run ended before its transaction", "%v", err)
	case <-time.After(10 * time.Second):
		require.FailNow(t, "the run did not reach its transaction")
	}
	holdRowUntilExpired(t, env, id, locked)
	require.NoError(t, <-done)
	require.ErrorIs(t, fenced, workflow.ErrFenced)
	require.Empty(t, env.stateRows(id), "the consumer row was not committed")
	require.JSONEq(t, `"retried"`, string(env.inst(eng, id).Result))
}

// A heartbeat that waits for the row lock while the lease expires does not
// renew it: the run is aborted.
func TestNative_HeartbeatDoesNotRenewALeaseThatExpiredDuringItsLockWait(t *testing.T) {
	env := newNativeEnv(t)
	started := make(chan uuid.UUID, 1)
	var runErr error
	runs := 0
	runner := workflow.NativeRunnerFunc(func(ctx context.Context, run workflow.NativeRun) (json.RawMessage, error) {
		if runs++; runs > 1 {
			return json.RawMessage(`"retried"`), nil
		}
		started <- run.Instance().ID
		<-ctx.Done()
		runErr = ctx.Err()
		return nil, runErr
	})
	eng := env.engine(map[string]workflow.NativeContract{"test.hb": {Runner: runner, RunTimeout: time.Minute}}, func(c *workflow.Config) {
		c.LeaseTTL, c.Heartbeat = 500*time.Millisecond, 50*time.Millisecond
	})
	env.start(eng, "test.hb", "h1", `{}`)
	done := make(chan error, 1)
	go func() { _, err := eng.RunOnce(context.Background()); done <- err }()
	var id uuid.UUID
	select {
	case id = <-started:
	case err := <-done:
		require.FailNow(t, "the run ended before it started", "%v", err)
	case <-time.After(10 * time.Second):
		require.FailNow(t, "the run did not start")
	}
	holdRowUntilExpired(t, env, id, nil)
	require.NoError(t, <-done)
	require.ErrorIs(t, runErr, context.Canceled, "the expired run was aborted, not renewed")
	require.JSONEq(t, `"retried"`, string(env.inst(eng, id).Result))
}

// An owner whose lease expired before any takeover cannot complete the
// instance, cancel its waits or call the observer: the next owner does.
func TestNative_ExpiredOwnerCannotComplete(t *testing.T) {
	env := newNativeEnv(t)
	obs := &observer{}
	runs := 0
	runner := workflow.NativeRunnerFunc(func(ctx context.Context, run workflow.NativeRun) (json.RawMessage, error) {
		if runs++; runs > 1 {
			return json.RawMessage(`"second"`), nil
		}
		_, err := env.db.Exec(context.Background(), `UPDATE extruntime.workflow_instances SET lease_until = clock_timestamp() - interval '1 second' WHERE id = $1`, run.Instance().ID)
		require.NoError(t, err)
		return json.RawMessage(`"first"`), nil
	})
	eng := env.engine(map[string]workflow.NativeContract{"test.late": {Runner: runner, StatusObserver: obs}}, func(c *workflow.Config) {
		c.Heartbeat = time.Hour
	})
	id := env.start(eng, "test.late", "e1", `{}`)
	_, err := eng.RunOnce(context.Background())
	require.NoError(t, err)
	inst := env.inst(eng, id)
	require.Equal(t, workflow.StatusCompleted, inst.Status)
	require.JSONEq(t, `"second"`, string(inst.Result), "the expired owner's output was not stored")
	seen, calls := obs.snapshot()
	require.Equal(t, 1, calls)
	require.Equal(t, []string{"running>completed=completed"}, seen)
}

// Sub-millisecond durations reach PostgreSQL as intervals it accepts.
func TestNative_MicrosecondSleepAndTimeouts(t *testing.T) {
	env := newNativeEnv(t)
	uid := int64(3)
	runner := workflow.NativeRunnerFunc(func(ctx context.Context, run workflow.NativeRun) (json.RawMessage, error) {
		if err := run.Sleep(ctx, "tiny", 0, 1500*time.Nanosecond); err != nil {
			return nil, err
		}
		w, err := run.WaitFor(ctx, "w", 0, "go", time.Microsecond)
		if err != nil {
			return nil, err
		}
		tr, err := run.Task(ctx, "t", 0, workflow.TaskSpec{Assignees: []workflow.TaskAssignee{{Kind: workflow.TaskAssignUser, UserID: &uid}}, Timeout: time.Microsecond})
		if err != nil {
			return nil, err
		}
		return json.Marshal(map[string]any{"timedOut": w.TimedOut, "task": tr.Outcome})
	})
	eng := env.engine(map[string]workflow.NativeContract{"test.tiny": {Runner: runner, Signals: map[string]manifest.Type{"go": anyPayload()}}})
	id := env.start(eng, "test.tiny", "t1", `{}`)
	env.drainUntil(eng, func() bool { return env.inst(eng, id).Status == workflow.StatusCompleted }, "completes")
	require.JSONEq(t, `{"timedOut":true,"task":"timeout"}`, string(env.inst(eng, id).Result))
}

// A replay that waits for another signal at a recorded wait is
// nondeterminism: the instance blocks instead of parking on the wrong name.
func TestNative_WaitForWithAnotherSignalBlocks(t *testing.T) {
	env := newNativeEnv(t)
	runs := 0
	runner := workflow.NativeRunnerFunc(func(ctx context.Context, run workflow.NativeRun) (json.RawMessage, error) {
		runs++
		signal := "a"
		if runs > 1 {
			signal = "b"
		}
		_, err := run.WaitFor(ctx, "w", 0, signal, 0)
		return nil, err
	})
	eng := env.engine(map[string]workflow.NativeContract{"test.sig": {Runner: runner, Signals: map[string]manifest.Type{"a": anyPayload(), "b": anyPayload()}}})
	id := env.start(eng, "test.sig", "s1", `{}`)
	env.drainUntil(eng, func() bool { return env.inst(eng, id).Status == workflow.StatusWaiting }, "waits for a")
	require.NoError(t, eng.Signal(context.Background(), env.tenant, 1, id, "b", json.RawMessage(`{}`), ""))
	env.drainUntil(eng, func() bool { return env.inst(eng, id).Status == workflow.StatusBlocked }, "blocks")
	require.Equal(t, workflow.BlockNondeterminism, env.inst(eng, id).BlockedReason)
}

// StartV2 of a native instance validates the saved input against the
// contract as registered now.
func TestNative_StartV2ValidatesInputAgainstTheCurrentContract(t *testing.T) {
	env := newNativeEnv(t)
	done := workflow.NativeRunnerFunc(func(context.Context, workflow.NativeRun) (json.RawMessage, error) { return nil, nil })
	eng := env.engine(map[string]workflow.NativeContract{"test.v2": {Runner: done}})
	id := env.start(eng, "test.v2", "v1", `{}`)
	env.drainUntil(eng, func() bool { return env.inst(eng, id).Status == workflow.StatusCompleted }, "completes")

	strict := env.engine(map[string]workflow.NativeContract{"test.v2": {Runner: done, Input: hosttest.Obj("who", hosttest.Str())}})
	_, err := strict.StartV2(context.Background(), env.subject(1), id, "")
	require.ErrorIs(t, err, workflow.ErrInvalid)
	next, err := eng.StartV2(context.Background(), env.subject(1), id, "")
	require.NoError(t, err, "a contract without an input schema accepts it")
	require.Equal(t, id, *env.inst(eng, next).PredecessorID)
}

// A runner that returns after its run timeout is not accepted: the run is
// retried.
func TestNative_RunAfterItsDeadlineIsNotAccepted(t *testing.T) {
	env := newNativeEnv(t)
	runs := 0
	runner := workflow.NativeRunnerFunc(func(ctx context.Context, run workflow.NativeRun) (json.RawMessage, error) {
		if runs++; runs == 1 {
			<-ctx.Done()
			return json.RawMessage(`"late"`), nil
		}
		return json.RawMessage(`"on time"`), nil
	})
	eng := env.engine(map[string]workflow.NativeContract{"test.deadline": {Runner: runner, RunTimeout: 100 * time.Millisecond}})
	id := env.start(eng, "test.deadline", "d1", `{}`)
	env.drainUntil(eng, func() bool { return env.inst(eng, id).Status == workflow.StatusCompleted }, "completes")
	require.JSONEq(t, `"on time"`, string(env.inst(eng, id).Result))
	require.Equal(t, 2, runs)
}

// A NativeSource that fails is not a missing contract: a start is refused
// (retryable) instead of routed, and Serve starts the native pools once the
// source loads.
func TestNative_NativeSourceFailureIsRetried(t *testing.T) {
	env := newNativeEnv(t)
	var failing atomic.Bool
	failing.Store(true)
	done := workflow.NativeRunnerFunc(func(context.Context, workflow.NativeRun) (json.RawMessage, error) {
		return json.RawMessage(`"ok"`), nil
	})
	eng := env.engine(nil, func(c *workflow.Config) {
		c.NativeSource = func() (map[string]workflow.NativeContract, error) {
			if failing.Load() {
				return nil, errors.New("provider not ready")
			}
			return map[string]workflow.NativeContract{"test.src": {Runner: done}}, nil
		}
	})
	ctx := context.Background()
	tx, err := env.db.Begin(ctx)
	require.NoError(t, err)
	_, err = eng.Start(ctx, tx, spi.Scope{Subject: env.subject(1)}, spi.StartIntent{Workflow: "test.src", Input: json.RawMessage(`{}`), Key: "s1"})
	require.NoError(t, tx.Rollback(ctx))
	require.ErrorContains(t, err, "provider not ready")
	require.True(t, host.IsInfrastructure(err), "the start is retried, not failed or routed to a package")

	sctx, cancel := context.WithCancel(ctx)
	defer cancel()
	eng.Serve(sctx)
	failing.Store(false)
	id := env.start(eng, "test.src", "s2", `{}`)
	env.eventuallyStatus(eng, id, workflow.StatusCompleted, "the native pool starts after the source loads")
}

// Registrations a NativeSource supplies are checked against the package
// catalog when they load, also after AttachPackages.
func TestNative_SourcedRegistrationConflictsWithAPackage(t *testing.T) {
	env := newNativeEnv(t)
	cat := hosttest.NewCatalog()
	cat.Add(spi.ResolvedResource{Package: "acme", Version: "1.0.0", Resource: manifest.Resource{ID: "test.clash", Kind: manifest.ResWorkflow}})
	eng := env.engine(nil, func(c *workflow.Config) {
		c.NativeSource = func() (map[string]workflow.NativeContract, error) {
			return map[string]workflow.NativeContract{"test.clash": {Runner: joinAny(1)}}, nil
		}
	})
	require.NoError(t, eng.AttachPackages(context.Background(), workflow.Packages{Catalog: cat, Bindings: noBindings{}, Artifacts: &hosttest.Artifacts{}, Impl: newImpls(), Runner: noRunner{}}))
	require.ErrorContains(t, eng.Ready(context.Background()), "also declared as a package workflow")
	tx, err := env.db.Begin(context.Background())
	require.NoError(t, err)
	defer func() { _ = tx.Rollback(context.Background()) }()
	_, err = eng.Start(context.Background(), tx, spi.Scope{Subject: env.subject(1)}, spi.StartIntent{Workflow: "test.clash", Input: json.RawMessage(`{}`), Key: "c1"})
	require.ErrorContains(t, err, "also declared as a package workflow")
}

// A start produced by a terminated instance (an ordinary command it called
// before it noticed) adds no child to the stopped tree.
func TestNative_TerminatedProducerStartsNoChild(t *testing.T) {
	env := newNativeEnv(t)
	done := workflow.NativeRunnerFunc(func(context.Context, workflow.NativeRun) (json.RawMessage, error) { return nil, nil })
	eng := env.engine(map[string]workflow.NativeContract{"test.kid": {Runner: done}})
	parent := insertNativeInstance(t, env, "test.kid", nil, workflow.StatusWaiting)
	require.NoError(t, eng.Terminate(context.Background(), env.tenant, parent))
	ctx := context.Background()
	tx, err := env.db.Begin(ctx)
	require.NoError(t, err)
	defer func() { _ = tx.Rollback(ctx) }()
	_, err = eng.Start(ctx, tx, spi.Scope{Subject: env.subject(1)}, spi.StartIntent{
		Workflow: "test.kid", Input: json.RawMessage(`{}`), Key: "k1", Producer: spi.Producer{InstanceID: parent.String()},
	})
	we, ok := pool.AsWireError(err)
	require.True(t, ok, "%v", err)
	require.Equal(t, workflow.CodeInstanceNotAccepting, we.Code)
}

// resultObserver keeps the instance a completed transition reported.
type resultObserver struct {
	mu   sync.Mutex
	last workflow.Instance
}

func (o *resultObserver) InstanceStatusChanged(_ context.Context, _ pgx.Tx, inst workflow.Instance, _, _ string) error {
	o.mu.Lock()
	o.last = inst
	o.mu.Unlock()
	return nil
}

// The observer receives the row as the transition wrote it.
func TestNative_StatusObserverSeesTheWrittenInstance(t *testing.T) {
	env := newNativeEnv(t)
	obs := &resultObserver{}
	done := workflow.NativeRunnerFunc(func(context.Context, workflow.NativeRun) (json.RawMessage, error) {
		return json.RawMessage(`{"n":1}`), nil
	})
	eng := env.engine(map[string]workflow.NativeContract{"test.seen": {Runner: done, StatusObserver: obs}})
	id := env.start(eng, "test.seen", "o1", `{}`)
	env.drainUntil(eng, func() bool { return env.inst(eng, id).Status == workflow.StatusCompleted }, "completes")
	obs.mu.Lock()
	defer obs.mu.Unlock()
	require.Equal(t, workflow.StatusCompleted, obs.last.Status)
	require.JSONEq(t, `{"n":1}`, string(obs.last.Result))
	require.NotNil(t, obs.last.FinishedAt)
}

// A panicking runner fails its instance instead of the process.
func TestNative_RunnerPanicFailsTheInstance(t *testing.T) {
	env := newNativeEnv(t)
	boom := workflow.NativeRunnerFunc(func(context.Context, workflow.NativeRun) (json.RawMessage, error) { panic("bug") })
	eng := env.engine(map[string]workflow.NativeContract{"test.panic": {Runner: boom}})
	id := env.start(eng, "test.panic", "p1", `{}`)
	env.drainUntil(eng, func() bool { return env.inst(eng, id).Status == workflow.StatusFailed }, "fails")
	var we struct {
		Code string `json:"code"`
	}
	require.NoError(t, json.Unmarshal(env.inst(eng, id).Error, &we))
	require.Equal(t, "workflow.runnerPanic", we.Code)
}
