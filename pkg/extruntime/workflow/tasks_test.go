package workflow_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/require"

	"github.com/iota-uz/iota-sdk/pkg/extruntime/host/hosttest"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/manifest"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/spi"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/workflow"
)

// fakeAudience is a TaskAudience whose memberships tests change at will.
type fakeAudience struct {
	mu      sync.Mutex
	members map[workflow.TaskAssignee]map[int64]bool
	tenant  uuid.UUID
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
	defer f.mu.Unlock()
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
	require.EqualValues(t, a, got.AssigneeUserID)

	require.NoError(t, s.decide(b, task.ID, `{"ok":true}`))
	r := s.result(id)
	require.EqualValues(t, b, *r.By)
	require.Len(t, r.Decisions, 1)
	require.NotNil(t, r.Decisions[0].DelegatedFrom)
	require.EqualValues(t, a, *r.Decisions[0].DelegatedFrom)
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
	require.EqualValues(t, first, r.Decisions[0].UserID)
	got = s.task(id)
	require.Equal(t, "expired", got.Status)
	require.Equal(t, 2, got.EscalationLevel)
	require.Equal(t, []workflow.TaskAssignee{userAssignee(first), userAssignee(lead), userAssignee(head)}, got.Assignees)

	var steps int
	require.NoError(t, s.env.db.QueryRow(context.Background(),
		`SELECT count(*) FROM extruntime.workflow_occurrences WHERE instance_id = $1 AND kind = 'escalation'`, id).Scan(&steps))
	require.Equal(t, 2, steps, "one occurrence per applied step")
	require.Equal(t, []string{"created", "decision", "escalated", "escalated", "closed"}, s.log.seen(), "the failed escalation left no trace")
}

// A task without escalation steps times out at its due time.
func TestTasks_TimeoutWithoutEscalation(t *testing.T) {
	s := newTaskStack(t, workflow.TaskSpec{Assignees: []workflow.TaskAssignee{userAssignee(41)}, Timeout: 200 * time.Millisecond})
	id, _ := s.startWaiting("t1")
	require.Equal(t, workflow.TaskTimeout, s.result(id).Outcome)
}

// Cancel closes the task as cancelled and rejects later decisions.
func TestTasks_CancelRejectsDecisions(t *testing.T) {
	env := newNativeEnv(t)
	log := &changeLog{fail: map[string]int{}}
	spec := workflow.TaskSpec{Assignees: []workflow.TaskAssignee{userAssignee(51)}}
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
	require.NoError(t, eng.Signal(context.Background(), env.tenant, 1, id, "go", json.RawMessage(`{}`), ""))
	env.drainUntil(eng, func() bool { return env.inst(eng, id).Status == workflow.StatusCompleted }, "completes")
	require.JSONEq(t, `{"outcome":"cancelled"}`, string(env.inst(eng, id).Result))
	err = eng.DecideTask(context.Background(), env.subject(51), taskID, json.RawMessage(`{}`))
	require.ErrorIs(t, err, workflow.ErrTaskClosed)
	require.Equal(t, []string{"created", "closed"}, log.seen())
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
		{Assignees: []workflow.TaskAssignee{{Kind: workflow.TaskAssignRole, Ref: "admins"}}},
		{Assignees: []workflow.TaskAssignee{userAssignee(1)}, Outcomes: []string{"a", "b"}, Decision: &manifest.Type{Kind: manifest.KindObject}},
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
