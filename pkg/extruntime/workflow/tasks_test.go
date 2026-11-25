package workflow_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"

	"github.com/iota-uz/iota-sdk/modules/core/infrastructure/persistence"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/host"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/host/hosttest"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/manifest"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/spi"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/workflow"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/workflow/audience"
)

// fakeAudience is a TaskAudience whose memberships tests change at will.
type fakeAudience struct {
	mu      sync.Mutex
	members map[workflow.TaskAssignee]map[int64]bool
	tenant  uuid.UUID
	// slow, when set, runs before every IsMember answer (outside the lock).
	slow func()
	// fail are the assignees whose lookups fail.
	fail map[workflow.TaskAssignee]error
}

func newFakeAudience(tenant uuid.UUID) *fakeAudience {
	return &fakeAudience{members: map[workflow.TaskAssignee]map[int64]bool{}, tenant: tenant}
}

func (f *fakeAudience) set(a workflow.TaskAssignee, user int64, member bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.members[a] == nil {
		f.members[a] = map[int64]bool{}
	}
	f.members[a][user] = member
}

func (f *fakeAudience) IsMember(_ context.Context, tenant uuid.UUID, user int64, a workflow.TaskAssignee) (bool, error) {
	f.mu.Lock()
	slow := f.slow
	f.mu.Unlock()
	if slow != nil {
		slow()
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.fail[a]; err != nil {
		return false, err
	}
	return tenant == f.tenant && f.members[a][user], nil
}

func (f *fakeAudience) Users(_ context.Context, tenant uuid.UUID, a workflow.TaskAssignee, limit int) ([]int64, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []int64
	for u, ok := range f.members[a] {
		if ok && tenant == f.tenant && len(out) < limit {
			out = append(out, u)
		}
	}
	return out, nil
}

// changeLog records the task changes the observers saw.
type changeLog struct {
	mu    sync.Mutex
	kinds []string
	fail  map[string]int // kind -> failures left
}

func (c *changeLog) TaskChanged(_ context.Context, _ pgx.Tx, _ workflow.Task, ch workflow.TaskChange) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.fail[ch.Kind] > 0 {
		c.fail[ch.Kind]--
		return errors.New("observer down")
	}
	c.kinds = append(c.kinds, ch.Kind)
	return nil
}

func (c *changeLog) seen() []string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]string(nil), c.kinds...)
}

// taskFlow is a native workflow that opens one task and completes with its
// result.
func taskFlow(spec workflow.TaskSpec) workflow.NativeRunner {
	return workflow.NativeRunnerFunc(func(ctx context.Context, run workflow.NativeRun) (json.RawMessage, error) {
		r, err := run.Task(ctx, "review", 0, spec)
		if err != nil {
			return nil, err
		}
		return json.Marshal(r)
	})
}

func userAssignee(id int64) workflow.TaskAssignee {
	return workflow.TaskAssignee{Kind: workflow.TaskAssignUser, UserID: &id}
}

var reviewers = workflow.TaskAssignee{Kind: workflow.TaskAssignGroup, Ref: "6f1c2c5e-6a4e-4b7e-9d43-3c1f0b8a1e01"}

type taskStack struct {
	env      *nativeEnv
	eng      *workflow.Engine
	audience *fakeAudience
	log      *changeLog
}

func newTaskStack(t *testing.T, spec workflow.TaskSpec) *taskStack {
	t.Helper()
	env := newNativeEnv(t)
	s := &taskStack{env: env, audience: newFakeAudience(env.tenant), log: &changeLog{fail: map[string]int{}}}
	s.eng = env.engine(map[string]workflow.NativeContract{"test.review": {Runner: taskFlow(spec)}}, func(c *workflow.Config) {
		c.TaskAudience = s.audience
		c.TaskObservers = []workflow.TaskObserver{s.log}
		c.TaskAdminPermission = "tasks.admin"
	})
	return s
}

func (s *taskStack) startWaiting(key string) (uuid.UUID, workflow.Task) {
	s.env.t.Helper()
	id := s.env.start(s.eng, "test.review", key, `{}`)
	s.env.drainUntil(s.eng, func() bool { return s.env.inst(s.eng, id).Status == workflow.StatusWaiting }, "the task is open")
	return id, s.task(id)
}

func (s *taskStack) task(id uuid.UUID) workflow.Task {
	s.env.t.Helper()
	h, err := s.eng.Get(context.Background(), s.env.tenant, id)
	require.NoError(s.env.t, err)
	require.Len(s.env.t, h.Tasks, 1)
	return h.Tasks[0]
}

func (s *taskStack) decide(user int64, task uuid.UUID, doc string) error {
	return s.eng.DecideTask(context.Background(), s.env.subject(user), task, json.RawMessage(doc))
}

func (s *taskStack) inbox(user int64) []uuid.UUID {
	s.env.t.Helper()
	tasks, err := s.eng.Tasks(context.Background(), s.env.subject(user), workflow.TaskFilter{})
	require.NoError(s.env.t, err)
	ids := make([]uuid.UUID, len(tasks))
	for i, t := range tasks {
		ids[i] = t.ID
	}
	return ids
}

func (s *taskStack) result(id uuid.UUID) workflow.TaskResult {
	s.env.t.Helper()
	s.env.drainUntil(s.eng, func() bool { return s.env.inst(s.eng, id).Status == workflow.StatusCompleted }, "the workflow completes")
	var r workflow.TaskResult
	require.NoError(s.env.t, json.Unmarshal(s.env.inst(s.eng, id).Result, &r))
	return r
}

func approval() workflow.TaskSpec {
	return workflow.TaskSpec{
		Assignees: []workflow.TaskAssignee{reviewers}, Required: 2,
		Outcomes: []string{"approve", "reject"}, Veto: []string{"reject"},
	}
}

// A group task with a quorum of two closes on the second agreeing decision of
// two different members; a user decides once.
func TestTasks_GroupQuorum(t *testing.T) {
	s := newTaskStack(t, approval())
	for _, u := range []int64{11, 12, 13} {
		s.audience.set(reviewers, u, true)
	}
	id, task := s.startWaiting("q1")
	require.Equal(t, 2, task.Required)
	require.Equal(t, []workflow.TaskAssignee{reviewers}, task.Assignees)
	require.Equal(t, workflow.TaskAssignGroup, task.AssigneeKind, "one assignee: the compatibility kind is set")
	require.Equal(t, []uuid.UUID{task.ID}, s.inbox(11))

	require.ErrorIs(t, s.decide(99, task.ID, `{"outcome":"approve"}`), workflow.ErrForbidden, "not a member")
	require.ErrorIs(t, s.decide(11, task.ID, `{"outcome":"maybe"}`), workflow.ErrInvalid, "not an outcome")
	require.NoError(t, s.decide(11, task.ID, `{"outcome":"approve"}`))
	require.ErrorIs(t, s.decide(11, task.ID, `{"outcome":"approve"}`), workflow.ErrAlreadyDecided)
	require.Equal(t, "open", s.task(id).Status, "one approval is not a quorum")
	require.Empty(t, s.inbox(11), "a member who decided no longer sees it")
	require.Equal(t, []uuid.UUID{task.ID}, s.inbox(12))

	require.NoError(t, s.decide(12, task.ID, `{"outcome":"approve"}`))
	require.ErrorIs(t, s.decide(13, task.ID, `{"outcome":"approve"}`), workflow.ErrTaskClosed)
	r := s.result(id)
	require.Equal(t, workflow.TaskDecided, r.Outcome)
	require.Equal(t, "approve", r.Choice)
	require.EqualValues(t, 12, *r.By)
	require.Len(t, r.Decisions, 2)
	require.EqualValues(t, 11, r.Decisions[0].UserID)
	require.Equal(t, "approve", r.Decisions[1].Choice)
	require.Equal(t, []string{"created", "decision", "decision", "closed"}, s.log.seen())
}

// A veto outcome closes the task with the first decision.
func TestTasks_VetoClosesAtOnce(t *testing.T) {
	s := newTaskStack(t, approval())
	s.audience.set(reviewers, 11, true)
	id, task := s.startWaiting("v1")
	require.NoError(t, s.decide(11, task.ID, `{"outcome":"reject"}`))
	r := s.result(id)
	require.Equal(t, workflow.TaskDecided, r.Outcome)
	require.Equal(t, "reject", r.Choice)
	require.JSONEq(t, `{"outcome":"reject"}`, string(r.Decision))
}

// Membership is judged when the decision comes: a user removed from the
// group after the task was created can no longer decide or see it.
func TestTasks_RemovedMemberLosesTheTask(t *testing.T) {
	s := newTaskStack(t, approval())
	s.audience.set(reviewers, 11, true)
	s.audience.set(reviewers, 13, true)
	_, task := s.startWaiting("r1")
	require.Equal(t, []uuid.UUID{task.ID}, s.inbox(13))

	s.audience.set(reviewers, 13, false)
	require.ErrorIs(t, s.decide(13, task.ID, `{"outcome":"approve"}`), workflow.ErrForbidden)
	require.Empty(t, s.inbox(13))
	require.Equal(t, []uuid.UUID{task.ID}, s.inbox(11), "the others still see it")

	// Another tenant's actor sees nothing and cannot address the task.
	other := spi.Subject{TenantID: uuid.New(), UserID: 11}
	require.ErrorIs(t, s.eng.DecideTask(context.Background(), other, task.ID, json.RawMessage(`{"outcome":"approve"}`)), workflow.ErrNotFound)
	tasks, err := s.eng.Tasks(context.Background(), other, workflow.TaskFilter{})
	require.NoError(t, err)
	require.Empty(t, tasks)
}

// A delegates to B: A loses the task, B decides it for themselves with
// DelegatedFrom = A; B cannot delegate back to A.
func TestTasks_Delegation(t *testing.T) {
	const a, b, c = int64(31), int64(32), int64(33)
	s := newTaskStack(t, workflow.TaskSpec{Assignees: []workflow.TaskAssignee{userAssignee(a)}, AllowDelegation: true})
	id, task := s.startWaiting("d1")
	ctx := context.Background()

	require.ErrorIs(t, s.eng.DelegateTask(ctx, s.env.subject(b), task.ID, c, ""), workflow.ErrForbidden, "B may not decide yet")
	require.ErrorIs(t, s.eng.DelegateTask(ctx, s.env.subject(a), task.ID, a, ""), workflow.ErrDelegation, "not to oneself")
	require.NoError(t, s.eng.DelegateTask(ctx, s.env.subject(a), task.ID, b, "on leave"))
	require.Empty(t, s.inbox(a), "A no longer sees the task")
	require.Equal(t, []uuid.UUID{task.ID}, s.inbox(b))
	require.ErrorIs(t, s.decide(a, task.ID, `{"ok":true}`), workflow.ErrForbidden, "A lost the right")
	require.ErrorIs(t, s.eng.DelegateTask(ctx, s.env.subject(b), task.ID, a, ""), workflow.ErrDelegation, "no cycle back to A")

	got := s.task(id)
	require.Len(t, got.Delegations, 1)
	require.Equal(t, workflow.TaskDelegation{From: a, To: b, Comment: "on leave", At: got.Delegations[0].At}, got.Delegations[0])
	require.Equal(t, []workflow.TaskAssignee{userAssignee(a)}, got.Assignees, "a delegation is not an assignee")
	require.Equal(t, a, got.AssigneeUserID)

	require.NoError(t, s.decide(b, task.ID, `{"ok":true}`))
	r := s.result(id)
	require.Equal(t, b, *r.By)
	require.Len(t, r.Decisions, 1)
	require.NotNil(t, r.Decisions[0].DelegatedFrom)
	require.Equal(t, a, *r.Decisions[0].DelegatedFrom)
	require.Equal(t, []string{"created", "delegated", "decision", "closed"}, s.log.seen())
}

func TestTasks_DelegationNeedsAllowDelegation(t *testing.T) {
	s := newTaskStack(t, workflow.TaskSpec{Assignees: []workflow.TaskAssignee{userAssignee(31)}})
	_, task := s.startWaiting("d2")
	require.ErrorIs(t, s.eng.DelegateTask(context.Background(), s.env.subject(31), task.ID, 32, ""), workflow.ErrDelegation)
}

// A delegation chain A → B → C is allowed; C decides on behalf of B.
func TestTasks_DelegationChain(t *testing.T) {
	s := newTaskStack(t, workflow.TaskSpec{Assignees: []workflow.TaskAssignee{userAssignee(31)}, AllowDelegation: true})
	id, task := s.startWaiting("d3")
	ctx := context.Background()
	require.NoError(t, s.eng.DelegateTask(ctx, s.env.subject(31), task.ID, 32, ""))
	require.NoError(t, s.eng.DelegateTask(ctx, s.env.subject(32), task.ID, 33, ""))
	require.ErrorIs(t, s.eng.DelegateTask(ctx, s.env.subject(33), task.ID, 31, ""), workflow.ErrDelegation, "not back to the start of the chain")
	require.Empty(t, s.inbox(32))
	require.NoError(t, s.decide(33, task.ID, `{}`))
	r := s.result(id)
	require.EqualValues(t, 32, *r.Decisions[0].DelegatedFrom)
}

// Two escalation steps: the first due time adds an assignee and extends the
// due time, the second adds one more and times the task out with the
// decisions collected so far. A crash while escalating (the observer fails,
// rolling the run's transaction back) does not apply the step twice.
func TestTasks_EscalationAndTimeout(t *testing.T) {
	const first, lead, head = int64(41), int64(42), int64(43)
	s := newTaskStack(t, workflow.TaskSpec{
		Assignees: []workflow.TaskAssignee{userAssignee(first)}, Required: 2, Outcomes: []string{"approve", "reject"},
		Timeout: 300 * time.Millisecond,
		Escalation: []workflow.EscalationStep{
			{Add: []workflow.TaskAssignee{userAssignee(lead)}, Extend: 600 * time.Millisecond},
			{Add: []workflow.TaskAssignee{userAssignee(head)}},
		},
	})
	s.log.fail[workflow.TaskChangeEscalated] = 1
	id, task := s.startWaiting("e1")
	firstDue := *task.DueAt
	require.Empty(t, s.inbox(lead))
	require.NoError(t, s.decide(first, task.ID, `{"outcome":"approve"}`))

	s.env.drainUntil(s.eng, func() bool { return s.task(id).EscalationLevel == 1 }, "the first step is applied")
	got := s.task(id)
	require.Equal(t, "open", got.Status)
	require.Equal(t, []workflow.TaskAssignee{userAssignee(first), userAssignee(lead)}, got.Assignees, "the lead was added once")
	require.True(t, got.DueAt.After(firstDue), "the due time was extended")
	require.Equal(t, []uuid.UUID{task.ID}, s.inbox(lead))

	r := s.result(id)
	require.Equal(t, workflow.TaskTimeout, r.Outcome)
	require.Len(t, r.Decisions, 1, "the decisions collected until the timeout")
	require.Equal(t, first, r.Decisions[0].UserID)
	got = s.task(id)
	require.Equal(t, "expired", got.Status)
	require.Equal(t, 2, got.EscalationLevel)
	require.Equal(t, []workflow.TaskAssignee{userAssignee(first), userAssignee(lead), userAssignee(head)}, got.Assignees)

	var steps int
	require.NoError(t, s.env.db.QueryRow(context.Background(),
		`SELECT count(*) FROM extruntime.human_task_escalations WHERE task_id = $1`, task.ID).Scan(&steps))
	require.Equal(t, 2, steps, "one record per applied step")
	require.Equal(t, []string{"created", "decision", "escalated", "escalated", "closed"}, s.log.seen(), "the failed escalation left no trace")
}

// A task without escalation steps times out at its due time.
func TestTasks_TimeoutWithoutEscalation(t *testing.T) {
	s := newTaskStack(t, workflow.TaskSpec{Assignees: []workflow.TaskAssignee{userAssignee(41)}, Timeout: 200 * time.Millisecond})
	id, _ := s.startWaiting("t1")
	require.Equal(t, workflow.TaskTimeout, s.result(id).Outcome)
}

// Cancel closes the task as cancelled and rejects later decisions; the
// result keeps the decisions made before, read in the run that cancelled and
// on the replay of a later run.
func TestTasks_CancelRejectsDecisions(t *testing.T) {
	env := newNativeEnv(t)
	log := &changeLog{fail: map[string]int{}}
	spec := workflow.TaskSpec{Assignees: []workflow.TaskAssignee{userAssignee(51), userAssignee(52)}, Required: 2, Outcomes: []string{"yes", "no"}}
	var (
		mu   sync.Mutex
		seen []workflow.TaskResult
	)
	runner := workflow.NativeRunnerFunc(func(ctx context.Context, run workflow.NativeRun) (json.RawMessage, error) {
		if _, err := run.Task(ctx, "review", 0, spec); err != nil && !errors.Is(err, workflow.ErrSuspend) {
			return nil, err
		}
		if _, err := run.WaitFor(ctx, "go", 0, "go", 0); err != nil {
			return nil, err
		}
		if err := run.Cancel(ctx, "review", 0); err != nil {
			return nil, err
		}
		r, err := run.Task(ctx, "review", 0, spec)
		if err != nil {
			return nil, err
		}
		mu.Lock()
		seen = append(seen, r)
		mu.Unlock()
		if _, err := run.WaitFor(ctx, "go", 1, "go", 0); err != nil {
			return nil, err
		}
		return json.Marshal(r)
	})
	eng := env.engine(map[string]workflow.NativeContract{
		"test.cancel": {Runner: runner, Signals: map[string]manifest.Type{"go": anyPayload()}},
	}, func(c *workflow.Config) { c.TaskObservers = []workflow.TaskObserver{log} })
	id := env.start(eng, "test.cancel", "c1", `{}`)
	env.drainUntil(eng, func() bool { return env.inst(eng, id).Status == workflow.StatusWaiting }, "waits")
	h, err := eng.Get(context.Background(), env.tenant, id)
	require.NoError(t, err)
	taskID := h.Tasks[0].ID
	require.NoError(t, eng.DecideTask(context.Background(), env.subject(51), taskID, json.RawMessage(`{"outcome":"yes"}`)))
	require.NoError(t, eng.Signal(context.Background(), env.tenant, 1, id, "go", json.RawMessage(`{}`), ""))
	env.drainUntil(eng, func() bool {
		mu.Lock()
		defer mu.Unlock()
		return len(seen) == 1 && env.inst(eng, id).Status == workflow.StatusWaiting
	}, "the run that cancelled waits again")
	require.NoError(t, eng.Signal(context.Background(), env.tenant, 1, id, "go", json.RawMessage(`{}`), ""))
	env.drainUntil(eng, func() bool { return env.inst(eng, id).Status == workflow.StatusCompleted }, "completes")

	mu.Lock()
	require.Len(t, seen, 2, "read in the cancelling run and replayed in the next")
	for _, r := range seen {
		require.Equal(t, workflow.TaskCancelled, r.Outcome)
		require.Len(t, r.Decisions, 1)
		require.EqualValues(t, 51, r.Decisions[0].UserID)
		require.Equal(t, "yes", r.Decisions[0].Choice)
	}
	mu.Unlock()
	err = eng.DecideTask(context.Background(), env.subject(52), taskID, json.RawMessage(`{"outcome":"yes"}`))
	require.ErrorIs(t, err, workflow.ErrTaskClosed)
	require.Equal(t, []string{"created", "decision", "closed"}, log.seen())
}

// A decision and a delegation of the same user racing on one task: exactly
// one of them wins, never both (the delegator's vote is never counted).
// Two users delegating to each other at once: exactly one delegation wins.
func TestTasks_ConcurrentDecideAndDelegate(t *testing.T) {
	const a, b = int64(61), int64(62)
	s := newTaskStack(t, workflow.TaskSpec{
		Assignees: []workflow.TaskAssignee{userAssignee(a), userAssignee(b)}, Required: 2, Outcomes: []string{"yes", "no"}, AllowDelegation: true,
	})
	ctx := context.Background()
	race := func(fns ...func() error) []error {
		errs := make([]error, len(fns))
		var start, done sync.WaitGroup
		start.Add(1)
		for i, fn := range fns {
			done.Add(1)
			go func() {
				defer done.Done()
				start.Wait()
				errs[i] = fn()
			}()
		}
		start.Done()
		done.Wait()
		return errs
	}
	for i := range 8 {
		id, task := s.startWaiting(fmt.Sprintf("race-%d", i))
		errs := race(
			func() error { return s.decide(a, task.ID, `{"outcome":"yes"}`) },
			func() error { return s.eng.DelegateTask(ctx, s.env.subject(a), task.ID, b, "") },
		)
		got := s.task(id)
		switch {
		case errs[0] == nil:
			require.ErrorIs(t, errs[1], workflow.ErrAlreadyDecided, "round %d", i)
			require.Empty(t, got.Delegations)
			require.Len(t, got.Decisions, 1)
		case errs[1] == nil:
			require.ErrorIs(t, errs[0], workflow.ErrForbidden, "round %d", i)
			require.Len(t, got.Delegations, 1)
			require.Empty(t, got.Decisions, "the delegator's vote was not taken")
		default:
			require.Failf(t, "neither won", "round %d: %v, %v", i, errs[0], errs[1])
		}

		id, task = s.startWaiting(fmt.Sprintf("cross-%d", i))
		errs = race(
			func() error { return s.eng.DelegateTask(ctx, s.env.subject(a), task.ID, b, "") },
			func() error { return s.eng.DelegateTask(ctx, s.env.subject(b), task.ID, a, "") },
		)
		require.NotEqual(t, errs[0] == nil, errs[1] == nil, "round %d: exactly one delegation wins: %v, %v", i, errs[0], errs[1])
		require.Len(t, s.task(id).Delegations, 1)
	}
}

// Escalation records have keys of their own: a workflow occurrence named
// like the task's escalation, written before or after the escalation, takes
// nothing from it.
func TestTasks_EscalationKeysAreApartFromOccurrences(t *testing.T) {
	for _, before := range []bool{true, false} {
		t.Run(fmt.Sprintf("before=%v", before), func(t *testing.T) {
			env := newNativeEnv(t)
			spec := workflow.TaskSpec{
				Assignees: []workflow.TaskAssignee{userAssignee(71)}, Timeout: 200 * time.Millisecond,
				Escalation: []workflow.EscalationStep{{Add: []workflow.TaskAssignee{userAssignee(72)}, Extend: 200 * time.Millisecond}},
			}
			write := func(ctx context.Context, run workflow.NativeRun) error {
				_, err := run.Transaction(ctx, "review#0.escalation", 0, func(context.Context, pgx.Tx) (json.RawMessage, error) {
					return json.RawMessage(`"mine"`), nil
				})
				return err
			}
			runner := workflow.NativeRunnerFunc(func(ctx context.Context, run workflow.NativeRun) (json.RawMessage, error) {
				if before {
					if err := write(ctx, run); err != nil {
						return nil, err
					}
				}
				r, err := run.Task(ctx, "review", 0, spec)
				if err != nil {
					return nil, err
				}
				if !before {
					if err := write(ctx, run); err != nil {
						return nil, err
					}
				}
				return json.Marshal(r)
			})
			eng := env.engine(map[string]workflow.NativeContract{"test.keys": {Runner: runner}})
			id := env.start(eng, "test.keys", "k", `{}`)
			env.drainUntil(eng, func() bool { return env.inst(eng, id).Status == workflow.StatusCompleted }, "completes after the escalation and the timeout")
			require.JSONEq(t, `{"outcome":"timeout"}`, string(env.inst(eng, id).Result))
			h, err := eng.Get(context.Background(), env.tenant, id)
			require.NoError(t, err)
			require.Equal(t, []workflow.TaskAssignee{userAssignee(71), userAssignee(72)}, h.Tasks[0].Assignees, "the step added its assignee")
			require.Equal(t, 1, h.Tasks[0].EscalationLevel)
			o, ok := occ(h, "review#0.escalation", 0)
			require.True(t, ok)
			require.Equal(t, workflow.KindTransaction, o.Kind)
			require.JSONEq(t, `"mine"`, string(o.Result))
		})
	}
}

// A package task's explicit timeout "0s" is due at once; an absent timeout
// leaves the task open.
func TestTasks_PackageZeroTimeoutIsDue(t *testing.T) {
	env := hosttest.New(t)
	im := newImpls()
	publishFlow(t, env, im, "acme.a", "1.0.0", `
export const flow = async (wf, input) => wf.task("approve", {
  subject: { entity: "acme.test.item", key: "1" }, assignee: { userId: "7" },
  decision: { type: { kind: "object" } }, timeout: input.who === "zero" ? "0s" : undefined });`)
	im.Activate(flowID, "acme.a", "1.0.0")
	env.Subjects.Grant(7)
	st := newStack(t, env, im)
	zero := st.start(7, "zero", `{"who":"zero"}`)
	none := st.start(7, "none", `{"who":"none"}`)
	st.until(10*time.Second, func() bool { return st.status(zero) == workflow.StatusCompleted }, "the zero timeout expires the task")
	require.JSONEq(t, `{"outcome":"timeout"}`, string(st.instance(zero).Result))
	require.Equal(t, workflow.StatusWaiting, st.status(none))
	require.Nil(t, st.history(none).Tasks[0].DueAt, "no timeout: no due time")
	require.Equal(t, "open", st.history(none).Tasks[0].Status)
}

// Invalid specs fail the Task call and create nothing.
func TestTasks_SpecValidation(t *testing.T) {
	env := newNativeEnv(t)
	bad := []workflow.TaskSpec{
		{},
		{Assignees: []workflow.TaskAssignee{userAssignee(1)}, Required: 2},
		{Assignees: []workflow.TaskAssignee{userAssignee(1)}, Outcomes: []string{"a"}, Veto: []string{"b"}},
		{Assignees: []workflow.TaskAssignee{reviewers}}, // no TaskAudience
		{Assignees: []workflow.TaskAssignee{userAssignee(1)}, Escalation: []workflow.EscalationStep{{}}},
		{Assignees: []workflow.TaskAssignee{userAssignee(1)}, Timeout: time.Hour, Escalation: []workflow.EscalationStep{{Extend: time.Duration(1<<63 - 1)}}},
		{Assignees: []workflow.TaskAssignee{userAssignee(1)}, Timeout: workflow.MaxTaskDuration + 1},
		{Assignees: []workflow.TaskAssignee{{Kind: workflow.TaskAssignRole, Ref: "admins"}}},
		{Assignees: []workflow.TaskAssignee{userAssignee(1)}, Outcomes: []string{"a", "b"}, Decision: &manifest.Type{Kind: manifest.KindObject}},
		{Assignees: []workflow.TaskAssignee{userAssignee(1)}, Outcomes: []string{"approve", "reject"}, Veto: []string{"reject"},
			Decision: &manifest.Type{Kind: manifest.KindObject, Properties: []manifest.Property{
				{Name: "outcome", Type: manifest.Type{Kind: manifest.KindEnum, Values: []string{"approve", "approve"}}}}}}, // reject unreachable
	}
	var (
		mu   sync.Mutex
		errs = map[int]error{}
	)
	runner := workflow.NativeRunnerFunc(func(ctx context.Context, run workflow.NativeRun) (json.RawMessage, error) {
		var in struct {
			I int `json:"i"`
		}
		_ = json.Unmarshal(run.Instance().Input, &in)
		_, err := run.Task(ctx, "review", 0, bad[in.I])
		mu.Lock()
		errs[in.I] = err
		mu.Unlock()
		return nil, nil
	})
	eng := env.engine(map[string]workflow.NativeContract{"test.bad": {Runner: runner}})
	for i := range bad {
		id := env.start(eng, "test.bad", fmt.Sprintf("b%d", i), fmt.Sprintf(`{"i":%d}`, i))
		env.drainUntil(eng, func() bool { return env.inst(eng, id).Status == workflow.StatusCompleted }, "completes")
		h, err := eng.Get(context.Background(), env.tenant, id)
		require.NoError(t, err)
		require.Empty(t, h.Tasks, "spec %d created no task", i)
	}
	for i := range bad {
		require.Error(t, errs[i], "spec %d", i)
	}
}

// A package workflow passes the quorum, outcomes, escalation and delegation
// of wf.task through the host call; the result keeps its old shape and adds
// the winning choice and the decisions.
func TestTasks_PackageQuorumOverTheWire(t *testing.T) {
	env := hosttest.New(t)
	im := newImpls()
	publishFlow(t, env, im, "acme.a", "1.0.0", `
export const flow = async (wf) => wf.task("approve", {
  subject: { entity: "acme.test.item", key: "1" },
  assignees: [{ userId: "7" }, { userId: "8" }],
  required: 2, outcomes: ["approve", "reject"], veto: ["reject"], allowDelegation: true,
  decision: { type: { kind: "object", properties: [{ name: "outcome", type: { kind: "enum", values: ["approve", "reject"] } }] } },
  timeout: "1h", escalation: [{ add: [{ userId: "10" }], extend: "1h" }],
});`)
	im.Activate(flowID, "acme.a", "1.0.0")
	env.Subjects.Grant(7)
	env.Subjects.Grant(8)
	env.Subjects.Grant(9)
	st := newStack(t, env, im)
	id := st.start(7, "k", `{"who":"w"}`)
	require.Equal(t, 1, st.drain())
	task := st.history(id).Tasks[0]
	require.Equal(t, 2, task.Required)
	require.Equal(t, []string{"approve", "reject"}, task.Outcomes)
	require.Len(t, task.Escalation, 1)
	require.Equal(t, time.Hour, task.Escalation[0].Extend)
	require.Empty(t, task.AssigneeKind, "two assignees: no compatibility assignee")

	ctx := context.Background()
	require.NoError(t, st.engine.DecideTask(ctx, env.Subject(7), task.ID, json.RawMessage(`{"outcome":"approve"}`)))
	require.NoError(t, st.engine.DelegateTask(ctx, env.Subject(8), task.ID, 9, ""))
	require.NoError(t, st.engine.DecideTask(ctx, env.Subject(9), task.ID, json.RawMessage(`{"outcome":"approve"}`)))
	require.Equal(t, 1, st.drain())
	var got map[string]any
	require.NoError(t, json.Unmarshal(st.instance(id).Result, &got))
	require.Equal(t, "decided", got["outcome"])
	require.Equal(t, "approve", got["choice"])
	require.Equal(t, "9", got["by"])
	decisions := got["decisions"].([]any)
	require.Len(t, decisions, 2)
	require.Equal(t, "8", decisions[1].(map[string]any)["delegatedFrom"])
}

// holdTask locks the task row in a transaction of its own until release is
// called, and waitBlocked waits until another session queues behind a lock.
func holdTask(t *testing.T, s *taskStack, task uuid.UUID) (pgx.Tx, func()) {
	t.Helper()
	ctx := context.Background()
	tx, err := s.env.db.Begin(ctx)
	require.NoError(t, err)
	// A failing test releases the lock before the harness closes the pool.
	t.Cleanup(func() { _ = tx.Rollback(ctx) })
	_, err = tx.Exec(ctx, `SELECT 1 FROM extruntime.human_tasks WHERE id = $1 FOR UPDATE`, task)
	require.NoError(t, err)
	return tx, func() { require.NoError(t, tx.Commit(ctx)) }
}

func waitBlocked(t *testing.T, s *taskStack) {
	t.Helper()
	require.Eventually(t, func() bool {
		var n int
		err := s.env.db.QueryRow(context.Background(),
			`SELECT count(*) FROM pg_stat_activity WHERE datname = current_database() AND wait_event_type = 'Lock'`).Scan(&n)
		return err == nil && n > 0
	}, 10*time.Second, 10*time.Millisecond, "the call waits for the task lock")
}

func async(fn func() error) <-chan error {
	ch := make(chan error, 1)
	go func() { ch <- fn() }()
	return ch
}

// Rights are judged after the task lock is taken: a membership revoked, or a
// target blocked, while the call waited for the lock refuses it, and a
// delegation committed meanwhile is the origin of the vote.
func TestTasks_RightsAreJudgedUnderTheLock(t *testing.T) {
	s := newTaskStack(t, workflow.TaskSpec{
		Assignees: []workflow.TaskAssignee{reviewers, userAssignee(82)}, Required: 2, Outcomes: []string{"yes", "no"}, AllowDelegation: true,
	})
	ctx := context.Background()
	s.audience.set(reviewers, 81, true)
	s.audience.set(reviewers, 83, true)
	_, task := s.startWaiting("lock-1")

	// A member removed while their decision waits is refused.
	_, release := holdTask(t, s, task.ID)
	got := async(func() error { return s.decide(81, task.ID, `{"outcome":"yes"}`) })
	waitBlocked(t, s)
	s.audience.set(reviewers, 81, false)
	release()
	require.ErrorIs(t, <-got, workflow.ErrForbidden)

	// A member removed while their delegation waits is refused, and so is a
	// delegation to a user blocked meanwhile.
	_, release = holdTask(t, s, task.ID)
	got = async(func() error { return s.eng.DelegateTask(ctx, s.env.subject(83), task.ID, 84, "") })
	waitBlocked(t, s)
	s.audience.set(reviewers, 83, false)
	release()
	require.ErrorIs(t, <-got, workflow.ErrForbidden)
	s.audience.set(reviewers, 83, true)
	_, release = holdTask(t, s, task.ID)
	got = async(func() error { return s.eng.DelegateTask(ctx, s.env.subject(83), task.ID, 84, "") })
	waitBlocked(t, s)
	s.env.subjects.Block(84, "left")
	release()
	require.ErrorIs(t, <-got, workflow.ErrDelegation)
	require.Empty(t, s.task(task.InstanceID).Delegations)

	// User 82 is assigned directly; a delegation 83 → 82 committed while
	// 82's decision waits is recorded as its origin.
	hold, release := holdTask(t, s, task.ID)
	got = async(func() error { return s.decide(82, task.ID, `{"outcome":"yes"}`) })
	waitBlocked(t, s)
	_, err := hold.Exec(ctx, `
		INSERT INTO extruntime.human_task_assignees (task_id, tenant_id, kind, user_id, delegated_from, added_at)
		VALUES ($1, $2, 'user', 82, 83, clock_timestamp())`, task.ID, s.env.tenant)
	require.NoError(t, err)
	release()
	require.NoError(t, <-got)
	decisions := s.task(task.InstanceID).Decisions
	require.Len(t, decisions, 1)
	require.NotNil(t, decisions[0].DelegatedFrom)
	require.EqualValues(t, 83, *decisions[0].DelegatedFrom)
}

// With a pool of one connection, deciding and delegating a group task still
// work: the core's subject loader and audience read through the transaction
// that holds the task lock instead of waiting for a second connection.
func TestTasks_ChecksUnderTheLockNeedNoSecondConnection(t *testing.T) {
	env := newNativeEnv(t)
	ctx := context.Background()
	newUser := func(name string) int64 {
		var id int64
		require.NoError(t, env.db.QueryRow(ctx, `
			INSERT INTO users (first_name, last_name, email, ui_language, type, tenant_id) VALUES ($1, 'T', $2, 'en', 'user', $3) RETURNING id`,
			name, name+"-"+uuid.NewString()[:8]+"@x.test", env.tenant).Scan(&id))
		return id
	}
	a, b, c := newUser("a"), newUser("b"), newUser("c")
	_, err := env.db.Exec(ctx, `INSERT INTO user_groups (id, name, type, tenant_id) VALUES ($1, 'reviewers', 'user', $2)`, reviewers.Ref, env.tenant)
	require.NoError(t, err)
	_, err = env.db.Exec(ctx, `INSERT INTO group_users (group_id, user_id) VALUES ($1, $2), ($1, $3)`, reviewers.Ref, a, b)
	require.NoError(t, err)

	spec := workflow.TaskSpec{Assignees: []workflow.TaskAssignee{reviewers}, Required: 2, Outcomes: []string{"yes", "no"}, AllowDelegation: true}
	creator := env.engine(map[string]workflow.NativeContract{"test.review": {Runner: taskFlow(spec)}}, func(cfg *workflow.Config) {
		cfg.TaskAudience = audience.New(env.db)
	})
	id := env.start(creator, "test.review", "one-conn", `{}`)
	env.drainUntil(creator, func() bool { return env.inst(creator, id).Status == workflow.StatusWaiting }, "the task is open")
	h, err := creator.Get(ctx, env.tenant, id)
	require.NoError(t, err)
	taskID := h.Tasks[0].ID
	creator.Shutdown()

	pcfg, err := pgxpool.ParseConfig(env.db.Config().ConnString())
	require.NoError(t, err)
	pcfg.MaxConns, pcfg.MinConns = 1, 0
	one, err := pgxpool.NewWithConfig(ctx, pcfg)
	require.NoError(t, err)
	t.Cleanup(one.Close)
	eng, err := workflow.New(workflow.Config{
		DB: one, Logger: env.logger, Natives: map[string]workflow.NativeContract{"test.review": {Runner: taskFlow(spec)}},
		Subjects:     &host.UserSubjects{Users: persistence.NewUserRepository(persistence.NewUploadRepository()), Pool: one},
		TaskAudience: audience.New(one),
	})
	require.NoError(t, err)
	subject := func(u int64) spi.Subject {
		return spi.Subject{TenantID: env.tenant, UserID: u, Kind: spi.SubjectInitiator}
	}
	call := func(fn func(ctx context.Context) error) error {
		cctx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
		return fn(cctx)
	}

	require.NoError(t, call(func(ctx context.Context) error {
		inbox, err := eng.Tasks(ctx, subject(a), workflow.TaskFilter{})
		if err == nil && len(inbox) != 1 {
			return fmt.Errorf("inbox of a: %d tasks", len(inbox))
		}
		return err
	}))
	require.NoError(t, call(func(ctx context.Context) error { return eng.DelegateTask(ctx, subject(b), taskID, c, "") }))
	require.NoError(t, call(func(ctx context.Context) error {
		return eng.DecideTask(ctx, subject(a), taskID, json.RawMessage(`{"outcome":"yes"}`))
	}))
	require.NoError(t, call(func(ctx context.Context) error {
		return eng.DecideTask(ctx, subject(c), taskID, json.RawMessage(`{"outcome":"yes"}`))
	}))
	h, err = eng.Get(ctx, env.tenant, id)
	require.NoError(t, err)
	require.Equal(t, "decided", h.Tasks[0].Status)
	require.Equal(t, b, *h.Tasks[0].Decisions[1].DelegatedFrom)
}

// The inbox cursor carries the sort key the page returned: a task whose due
// time moves later after its page moves no other task across pages.
func TestTasks_InboxCursorKeepsThePageOrder(t *testing.T) {
	s := newTaskStack(t, workflow.TaskSpec{Assignees: []workflow.TaskAssignee{userAssignee(91)}, Timeout: time.Hour})
	ctx := context.Background()
	_, a := s.startWaiting("page-a")
	_, b := s.startWaiting("page-b")
	page, err := s.eng.Tasks(ctx, s.env.subject(91), workflow.TaskFilter{Limit: 1})
	require.NoError(t, err)
	require.Len(t, page, 1)
	require.Equal(t, a.ID, page[0].ID, "a is due first")
	cursor := workflow.CursorAfter(page[0])
	parsed, err := workflow.ParseTaskCursor(cursor.String())
	require.NoError(t, err)
	require.Equal(t, cursor.ID, parsed.ID)
	require.True(t, cursor.DueAt.Equal(*parsed.DueAt))

	_, err = s.env.db.Exec(ctx, `UPDATE extruntime.human_tasks SET due_at = due_at + interval '2 hours' WHERE id = $1`, a.ID)
	require.NoError(t, err)
	page, err = s.eng.Tasks(ctx, s.env.subject(91), workflow.TaskFilter{Limit: 1, Cursor: &parsed})
	require.NoError(t, err)
	require.Len(t, page, 1)
	require.Equal(t, b.ID, page[0].ID, "b is not skipped")

	// The contract's After (a task ID) pages by the cursor task's current
	// sort key: after b comes a, now due last.
	page, err = s.eng.Tasks(ctx, s.env.subject(91), workflow.TaskFilter{Limit: 1, After: &b.ID})
	require.NoError(t, err)
	require.Len(t, page, 1)
	require.Equal(t, a.ID, page[0].ID)
	_, err = s.eng.Tasks(ctx, s.env.subject(91), workflow.TaskFilter{After: new(uuid.UUID)})
	require.ErrorIs(t, err, workflow.ErrNotFound, "an unknown cursor task")
}

// A task an escalation moves later after a page held it is not listed again
// on a later page of the same traversal: the pages order tasks by their due
// time as the first page's snapshot saw it. That holds also for an escalation
// whose transaction began (and set applied_at) before the first page but
// committed after it. A new traversal sees the current order.
func TestTasks_InboxCursorSurvivesAnEscalation(t *testing.T) {
	for _, overlap := range []bool{false, true} {
		t.Run(fmt.Sprintf("overlapping=%v", overlap), func(t *testing.T) {
			s := newTaskStack(t, workflow.TaskSpec{Assignees: []workflow.TaskAssignee{userAssignee(92)}, Timeout: time.Hour})
			ctx := context.Background()
			_, a := s.startWaiting("esc-a")
			_, b := s.startWaiting("esc-b")

			// a escalates as the run does it: the step records the due time it
			// replaces, its transaction (by default) and applied_at.
			escalate := func() pgx.Tx {
				tx, err := s.env.db.Begin(ctx)
				require.NoError(t, err)
				_, err = tx.Exec(ctx, `
					INSERT INTO extruntime.human_task_escalations (task_id, tenant_id, step, added, applied_at, due_before)
					SELECT id, tenant_id, 0, '[]', clock_timestamp(), due_at FROM extruntime.human_tasks WHERE id = $1`, a.ID)
				require.NoError(t, err)
				_, err = tx.Exec(ctx, `UPDATE extruntime.human_tasks SET due_at = due_at + interval '2 hours', escalation_level = 1 WHERE id = $1`, a.ID)
				require.NoError(t, err)
				return tx
			}
			var pending pgx.Tx
			if overlap {
				pending = escalate()
			}
			page, err := s.eng.Tasks(ctx, s.env.subject(92), workflow.TaskFilter{Limit: 1})
			require.NoError(t, err)
			require.Len(t, page, 1)
			require.Equal(t, a.ID, page[0].ID, "a is due first")
			cursor, err := workflow.ParseTaskCursor(workflow.CursorAfter(page[0]).String())
			require.NoError(t, err)
			require.NotEmpty(t, cursor.Snapshot, "the cursor carries the first page's snapshot")
			if overlap {
				require.NoError(t, pending.Commit(ctx))
			} else {
				require.NoError(t, escalate().Commit(ctx))
			}

			var got []uuid.UUID
			for range 3 {
				page, err = s.eng.Tasks(ctx, s.env.subject(92), workflow.TaskFilter{Limit: 1, Cursor: &cursor})
				require.NoError(t, err)
				if len(page) == 0 {
					break
				}
				got = append(got, page[0].ID)
				cursor, err = workflow.ParseTaskCursor(workflow.CursorAfter(page[0]).String())
				require.NoError(t, err)
			}
			require.Equal(t, []uuid.UUID{b.ID}, got, "b once, a not again")
			require.Equal(t, []uuid.UUID{b.ID, a.ID}, s.inbox(92), "a fresh inbox orders by the current due times")
		})
	}
}

func TestTasks_ParseTaskCursorSnapshot(t *testing.T) {
	id := uuid.New()
	c, err := workflow.ParseTaskCursor("/" + id.String() + "/100:105:101,103")
	require.NoError(t, err)
	require.Equal(t, "100:105:101,103", c.Snapshot)
	require.Equal(t, "/"+id.String()+"/100:105:101,103", c.String())
	_, err = workflow.ParseTaskCursor("/" + id.String() + "/100:105:x');--")
	require.ErrorIs(t, err, workflow.ErrInvalid)
	// Shaped like a snapshot but refused by PostgreSQL: an invalid cursor, not a database error.
	for _, bad := range []string{"200:100:", "0:5:", "100:200:250", "100:200:99", "100:200:160,150", "1:99999999999999999999:"} {
		_, err = workflow.ParseTaskCursor("/" + id.String() + "/" + bad)
		require.ErrorIs(t, err, workflow.ErrInvalid, bad)
	}
	_, err = workflow.ParseTaskCursor("/" + id.String() + "/100:100:")
	require.NoError(t, err, "no transaction in progress")

	// A snapshot of many concurrent transactions is read whole, however long.
	xips := make([]string, 0, 600)
	for i := range 600 {
		xips = append(xips, fmt.Sprint(1_000_000_000+i))
	}
	long := "1000000000:1000000600:" + strings.Join(xips, ",")
	require.Greater(t, len(long), 4096)
	c, err = workflow.ParseTaskCursor("/" + id.String() + "/" + long)
	require.NoError(t, err)
	require.Equal(t, long, c.Snapshot)
}

// AuthorizeTask judges the actor by their current rights and the task as it
// is now, as DecideTask does, and records nothing.
func TestTasks_AuthorizeTask(t *testing.T) {
	perm := workflow.TaskAssignee{Kind: workflow.TaskAssignPermission, Ref: "invoices.approve"}
	s := newTaskStack(t, workflow.TaskSpec{Assignees: []workflow.TaskAssignee{perm}, Timeout: time.Hour})
	ctx := context.Background()
	s.env.subjects.Grant(97, "invoices.approve")
	inst, task := s.startWaiting("auth")

	got, err := s.eng.AuthorizeTask(ctx, s.env.subject(97), task.ID)
	require.NoError(t, err)
	require.Equal(t, task.ID, got.ID)
	require.Empty(t, s.task(inst).Decisions, "nothing is recorded")
	_, err = s.eng.AuthorizeTask(ctx, s.env.subject(98), task.ID)
	require.ErrorIs(t, err, workflow.ErrForbidden, "not an assignee")

	s.env.subjects.Grant(97)
	_, err = s.eng.AuthorizeTask(ctx, s.env.subject(97), task.ID)
	require.ErrorIs(t, err, workflow.ErrForbidden, "the permission was revoked")
	s.env.subjects.Grant(97, "invoices.approve")
	s.env.subjects.Block(97, "left")
	_, err = s.eng.AuthorizeTask(ctx, s.env.subject(97), task.ID)
	require.ErrorIs(t, err, workflow.ErrForbidden, "the user was blocked")

	s.env.subjects.Grant(97, "invoices.approve")
	_, err = s.env.db.Exec(ctx, `UPDATE extruntime.human_tasks SET due_at = now() - interval '1 second' WHERE id = $1`, task.ID)
	require.NoError(t, err)
	_, err = s.eng.AuthorizeTask(ctx, s.env.subject(97), task.ID)
	require.ErrorIs(t, err, workflow.ErrTaskClosed, "past the final deadline")
}

// lingeringSubjects delays LoadSubject while delay is set: a rights lookup
// that takes its time.
type lingeringSubjects struct {
	*hosttest.Subjects
	delay atomic.Int64
}

func (s *lingeringSubjects) LoadSubject(ctx context.Context, tenant uuid.UUID, user int64, kind spi.SubjectKind) (spi.Subject, error) {
	if d := time.Duration(s.delay.Load()); d > 0 {
		time.Sleep(d)
	}
	return s.Subjects.LoadSubject(ctx, tenant, user, kind)
}

// A final deadline that passes while AuthorizeTask looks the actor's rights
// up is judged after the lookup: the answer is ErrTaskClosed.
func TestTasks_AuthorizeTaskJudgesTheDeadlineAfterTheRightsLookup(t *testing.T) {
	env := newNativeEnv(t)
	subjects := &lingeringSubjects{Subjects: env.subjects}
	eng := env.engine(map[string]workflow.NativeContract{
		"test.review": {Runner: taskFlow(workflow.TaskSpec{Assignees: []workflow.TaskAssignee{userAssignee(99)}, Timeout: time.Hour})},
	}, func(c *workflow.Config) { c.Subjects = subjects })
	ctx := context.Background()
	id := env.start(eng, "test.review", "late-auth", `{}`)
	env.drainUntil(eng, func() bool { return env.inst(eng, id).Status == workflow.StatusWaiting }, "the task is open")
	h, err := eng.Get(ctx, env.tenant, id)
	require.NoError(t, err)
	require.Len(t, h.Tasks, 1)
	task := h.Tasks[0]

	_, err = eng.AuthorizeTask(ctx, env.subject(99), task.ID)
	require.NoError(t, err)
	_, err = env.db.Exec(ctx, `UPDATE extruntime.human_tasks SET due_at = clock_timestamp() + interval '300 milliseconds' WHERE id = $1`, task.ID)
	require.NoError(t, err)
	subjects.delay.Store(int64(700 * time.Millisecond))
	_, err = eng.AuthorizeTask(ctx, env.subject(99), task.ID)
	require.ErrorIs(t, err, workflow.ErrTaskClosed, "the deadline passed during the rights lookup")
}

// A failed membership lookup of one assignee does not hide another that
// covers the user, in the inbox and for a decision; with none covering, the
// failure is the answer.
func TestTasks_MembershipLookupFailureFallsThrough(t *testing.T) {
	broken := workflow.TaskAssignee{Kind: workflow.TaskAssignRole, Ref: "7"}
	down := errors.New("roles: timeout")
	s := newTaskStack(t, workflow.TaskSpec{Assignees: []workflow.TaskAssignee{broken, reviewers}, Timeout: time.Hour})
	s.audience.set(reviewers, 31, true)
	s.audience.mu.Lock()
	s.audience.fail = map[workflow.TaskAssignee]error{broken: down}
	s.audience.mu.Unlock()
	ctx := context.Background()
	_, task := s.startWaiting("fallthrough")

	require.Equal(t, []uuid.UUID{task.ID}, s.inbox(31), "the group covers the user")
	_, err := s.eng.AuthorizeTask(ctx, s.env.subject(31), task.ID)
	require.NoError(t, err)

	_, err = s.eng.AuthorizeTask(ctx, s.env.subject(32), task.ID)
	require.ErrorIs(t, err, down, "no assignee covers the user: the lookup failure is the answer")
	_, err = s.eng.Tasks(ctx, s.env.subject(32), workflow.TaskFilter{})
	require.ErrorIs(t, err, down)
}

// The run records the due time an escalation step replaced.
func TestTasks_EscalationRecordsTheReplacedDueTime(t *testing.T) {
	s := newTaskStack(t, workflow.TaskSpec{
		Assignees: []workflow.TaskAssignee{userAssignee(93)}, Timeout: 200 * time.Millisecond,
		Escalation: []workflow.EscalationStep{{Add: []workflow.TaskAssignee{userAssignee(94)}, Extend: time.Hour}},
	})
	id, task := s.startWaiting("esc-due")
	s.env.drainUntil(s.eng, func() bool { return s.task(id).EscalationLevel == 1 }, "the step is applied")
	var (
		before *time.Time
		xact   *string
	)
	require.NoError(t, s.env.db.QueryRow(context.Background(),
		`SELECT due_before, applied_xact::text FROM extruntime.human_task_escalations WHERE task_id = $1 AND step = 0`, task.ID).Scan(&before, &xact))
	require.NotNil(t, before)
	require.NotNil(t, xact, "the step records its transaction")
	require.WithinDuration(t, *task.DueAt, *before, time.Millisecond)
}

// After the final due time no decision is accepted, even before the run
// closes the task; with an escalation step pending, decisions still count.
func TestTasks_NoDecisionAfterTheFinalDeadline(t *testing.T) {
	env := newNativeEnv(t)
	final := workflow.TaskSpec{Assignees: []workflow.TaskAssignee{userAssignee(95)}, Timeout: 100 * time.Millisecond}
	pending := final
	pending.Escalation = []workflow.EscalationStep{{Add: []workflow.TaskAssignee{userAssignee(96)}, Extend: time.Hour}}
	eng := env.engine(map[string]workflow.NativeContract{"test.final": {Runner: taskFlow(final)}, "test.pending": {Runner: taskFlow(pending)}})
	ctx := context.Background()
	open := func(wf string) (uuid.UUID, uuid.UUID) {
		id := env.start(eng, wf, wf, `{}`)
		_, err := eng.RunOnce(ctx)
		require.NoError(t, err)
		h, err := eng.Get(ctx, env.tenant, id)
		require.NoError(t, err)
		require.Len(t, h.Tasks, 1)
		return id, h.Tasks[0].ID
	}
	finalID, finalTask := open("test.final")
	_, pendingTask := open("test.pending")
	_, err := env.db.Exec(ctx, `UPDATE extruntime.human_tasks SET due_at = now() - interval '1 second'`)
	require.NoError(t, err)

	require.ErrorIs(t, eng.DecideTask(ctx, env.subject(95), finalTask, json.RawMessage(`{}`)), workflow.ErrTaskClosed)
	inbox, err := eng.Tasks(ctx, env.subject(95), workflow.TaskFilter{})
	require.NoError(t, err)
	require.Len(t, inbox, 1, "only the task with a step pending")
	require.Equal(t, pendingTask, inbox[0].ID)
	require.NoError(t, eng.DecideTask(ctx, env.subject(95), pendingTask, json.RawMessage(`{}`)))
	env.drainUntil(eng, func() bool { return env.inst(eng, finalID).Status == workflow.StatusCompleted }, "the run times the task out")
	require.JSONEq(t, `{"outcome":"timeout"}`, string(env.inst(eng, finalID).Result))
}

// A positive extension below a millisecond survives the round trip.
func TestTasks_SubMillisecondExtendIsKept(t *testing.T) {
	s := newTaskStack(t, workflow.TaskSpec{
		Assignees: []workflow.TaskAssignee{userAssignee(97)}, Timeout: time.Hour,
		Escalation: []workflow.EscalationStep{{Extend: 500 * time.Microsecond}, {Extend: 1500 * time.Nanosecond}},
	})
	_, task := s.startWaiting("ext")
	require.Equal(t, 500*time.Microsecond, task.Escalation[0].Extend)
	require.Equal(t, 2*time.Microsecond, task.Escalation[1].Extend, "rounded up, never to zero")
}

// Cancelling a task whose closing decision committed before the cancel keeps
// the decision: the occurrence is not cancelled and Task returns it.
func TestTasks_CancelAfterTheDecisionKeepsIt(t *testing.T) {
	env := newNativeEnv(t)
	spec := workflow.TaskSpec{Assignees: []workflow.TaskAssignee{userAssignee(98)}}
	runner := workflow.NativeRunnerFunc(func(ctx context.Context, run workflow.NativeRun) (json.RawMessage, error) {
		if _, ok, err := run.Lookup(ctx, "review", 0); err != nil {
			return nil, err
		} else if !ok {
			if _, err := run.Task(ctx, "review", 0, spec); err != nil && !errors.Is(err, workflow.ErrSuspend) {
				return nil, err
			}
		}
		if _, err := run.WaitFor(ctx, "go", 0, "go", 0); err != nil {
			return nil, err
		}
		if err := run.Cancel(ctx, "review", 0); err != nil {
			return nil, err
		}
		r, err := run.Task(ctx, "review", 0, spec)
		if err != nil {
			return nil, err
		}
		return json.Marshal(r)
	})
	eng := env.engine(map[string]workflow.NativeContract{"test.late": {Runner: runner, Signals: map[string]manifest.Type{"go": anyPayload()}}})
	ctx := context.Background()
	id := env.start(eng, "test.late", "late", `{}`)
	env.drainUntil(eng, func() bool { return env.inst(eng, id).Status == workflow.StatusWaiting }, "waits")
	h, err := eng.Get(ctx, env.tenant, id)
	require.NoError(t, err)
	require.NoError(t, eng.DecideTask(ctx, env.subject(98), h.Tasks[0].ID, json.RawMessage(`{"ok":true}`)))
	require.NoError(t, eng.Signal(ctx, env.tenant, 1, id, "go", json.RawMessage(`{}`), ""))
	env.drainUntil(eng, func() bool { return env.inst(eng, id).Status == workflow.StatusCompleted }, "completes")
	var r workflow.TaskResult
	require.NoError(t, json.Unmarshal(env.inst(eng, id).Result, &r))
	require.Equal(t, workflow.TaskDecided, r.Outcome, "the decision, not a cancel")
	require.JSONEq(t, `{"ok":true}`, string(r.Decision))
	h, err = eng.Get(ctx, env.tenant, id)
	require.NoError(t, err)
	require.Equal(t, "decided", h.Tasks[0].Status)
}

// A package task's user assignee must reference core.user.
func TestTasks_PackageAssigneeMustBeACoreUser(t *testing.T) {
	env := hosttest.New(t)
	im := newImpls()
	publishFlow(t, env, im, "acme.a", "1.0.0", `
export const flow = async (wf) => {
  let code = "";
  try {
    await wf.task("approve", { subject: { entity: "acme.test.item", key: "1" }, assignee: { entity: "sales.customer", key: "42" }, decision: { type: { kind: "object" } } });
  } catch (e) { code = e.code; }
  return { code };
};`)
	im.Activate(flowID, "acme.a", "1.0.0")
	env.Subjects.Grant(7)
	st := newStack(t, env, im)
	id := st.start(7, "k", `{"who":"w"}`)
	require.Equal(t, 1, st.drain())
	require.JSONEq(t, `{"code":"workflow.invalidAssignee"}`, string(st.instance(id).Result))
	require.Empty(t, st.history(id).Tasks)
}

// The final deadline is judged when the vote (or delegation) is written: one
// that passes while the rights are being checked (after the deadline check
// under the lock, before the write) refuses it. Each operation has a task of
// its own, so each reaches its own write.
func TestTasks_DeadlinePassingDuringTheCheckRefusesTheWrite(t *testing.T) {
	ops := map[string]func(s *taskStack, task uuid.UUID) error{
		"decide": func(s *taskStack, task uuid.UUID) error { return s.decide(99, task, `{}`) },
		"delegate": func(s *taskStack, task uuid.UUID) error {
			return s.eng.DelegateTask(context.Background(), s.env.subject(99), task, 98, "")
		},
	}
	for name, op := range ops {
		t.Run(name, func(t *testing.T) {
			s := newTaskStack(t, workflow.TaskSpec{Assignees: []workflow.TaskAssignee{reviewers}, Timeout: 1500 * time.Millisecond, AllowDelegation: true})
			s.audience.set(reviewers, 99, true)
			id, task := s.startWaiting("late-" + name)
			due := *task.DueAt
			var started []time.Time
			s.audience.mu.Lock()
			s.audience.slow = func() {
				started = append(started, time.Now())
				time.Sleep(time.Until(due) + 100*time.Millisecond)
			}
			s.audience.mu.Unlock()
			require.ErrorIs(t, op(s, task.ID), workflow.ErrTaskClosed)
			require.Len(t, started, 1, "the membership check ran once")
			require.True(t, started[0].Before(due), "it started before the deadline, so the lock's deadline check passed")
			got := s.task(id)
			require.Empty(t, got.Decisions, "no vote after the deadline")
			require.Empty(t, got.Delegations)
		})
	}
}
