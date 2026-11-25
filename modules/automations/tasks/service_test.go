package tasks

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/a-h/templ"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/user"
	"github.com/iota-uz/iota-sdk/modules/core/domain/value_objects/internet"
	"github.com/iota-uz/iota-sdk/pkg/automation/engine"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/host"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/spi"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/workflow"
)

const me = int64(7)

var tenant = uuid.MustParse("0b6f2a4c-6a51-4a39-9d3e-2f1d1f6c0a11")

// fakeSubstrate serves fixed tasks.
type fakeSubstrate struct {
	tasks   []workflow.Task
	decided atomic.Int32
	last    json.RawMessage
}

func (f *fakeSubstrate) Tasks(context.Context, spi.Subject, workflow.TaskFilter) ([]workflow.Task, error) {
	return f.tasks, nil
}

func (f *fakeSubstrate) TasksByID(_ context.Context, _ uuid.UUID, ids []uuid.UUID) ([]workflow.Task, error) {
	var out []workflow.Task
	for _, id := range ids {
		for _, t := range f.tasks {
			if t.ID == id {
				out = append(out, t)
			}
		}
	}
	return out, nil
}

func (f *fakeSubstrate) DecideTask(_ context.Context, _ spi.Subject, _ uuid.UUID, d json.RawMessage) error {
	f.decided.Add(1)
	f.last = d
	return nil
}

func (f *fakeSubstrate) DelegateTask(context.Context, spi.Subject, uuid.UUID, int64, string) error {
	return nil
}

// fakeStore gives every task the same node params.
type fakeStore struct{ params string }

func (f fakeStore) Metas(_ context.Context, _ uuid.UUID, keys []TaskKey) (map[uuid.UUID]TaskMeta, error) {
	out := map[uuid.UUID]TaskMeta{}
	for _, k := range keys {
		out[k.TaskID] = TaskMeta{RunID: uuid.New(), NodeID: "ask", Input: json.RawMessage(f.params), Params: json.RawMessage(f.params)}
	}
	return out, nil
}

func (fakeStore) Decided(context.Context, uuid.UUID, int64, string, *DecidedPosition, int) ([]DecidedPosition, error) {
	return nil, nil
}

type audienceFunc func(a workflow.TaskAssignee) (bool, error)

func (f audienceFunc) IsMember(_ context.Context, _ uuid.UUID, _ int64, a workflow.TaskAssignee) (bool, error) {
	return f(a)
}

func (audienceFunc) Users(context.Context, uuid.UUID, workflow.TaskAssignee, int) ([]int64, error) {
	return nil, nil
}

type resolverFunc func() (string, string, error)

func (f resolverFunc) Subject(context.Context, spi.Subject, string, string) (string, string, error) {
	return f()
}

func subjects(perms ...string) host.SubjectLoader {
	return host.SubjectLoaderFunc(func(_ context.Context, tenantID uuid.UUID, userID int64, kind spi.SubjectKind) (spi.Subject, error) {
		return spi.Subject{TenantID: tenantID, UserID: userID, Kind: kind, Permissions: perms}, nil
	})
}

const buttons = `{"title":"Approve","buttons":[{"outcome":"approve","label":"Approve"},{"outcome":"reject","label":"Reject","require_comment":true}]%s}`

func openTask(assignees ...workflow.TaskAssignee) workflow.Task {
	return workflow.Task{
		ID: uuid.New(), InstanceID: uuid.New(), Workflow: engine.WorkflowID, Name: "task:w1", Status: "open",
		Required: 1, Assignees: assignees, CreatedAt: time.Now(),
	}
}

func userAssignee(id int64) workflow.TaskAssignee {
	return workflow.TaskAssignee{Kind: workflow.TaskAssignUser, UserID: &id}
}

func newSvc(t *testing.T, cfg Config) *Service {
	t.Helper()
	if cfg.Subjects == nil {
		cfg.Subjects = subjects()
	}
	if cfg.Logger == nil {
		cfg.Logger = slog.New(slog.DiscardHandler)
	}
	if cfg.Store == nil {
		cfg.Store = fakeStore{params: strings.Replace(buttons, "%s", "", 1)}
	}
	svc, err := NewService(cfg)
	require.NoError(t, err)
	return svc
}

func userCtx() context.Context {
	u := user.New("Dina", "D", internet.MustParseEmail("dina@example.com"), user.UILanguageEN, user.WithID(uint(me)), user.WithTenantID(tenant))
	return WithOrigin(composables.WithUser(context.Background(), u), engine.OriginHuman)
}

// A transient failure to load the reader is an error (the delivery is retried); only a blocked subject
// is a denial.
func TestCanReadDeniesOnlyBlockedSubjects(t *testing.T) {
	t.Parallel()
	u := user.New("Dina", "D", internet.MustParseEmail("dina@example.com"), user.UILanguageEN, user.WithID(uint(me)), user.WithTenantID(tenant))
	down := errors.New("connection refused")
	svc := newSvc(t, Config{Tasks: &fakeSubstrate{}, Subjects: host.SubjectLoaderFunc(func(context.Context, uuid.UUID, int64, spi.SubjectKind) (spi.Subject, error) {
		return spi.Subject{}, down
	})})
	ok, err := svc.CanRead(context.Background(), tenant, u, uuid.New())
	require.ErrorIs(t, err, down)
	assert.False(t, ok)

	svc = newSvc(t, Config{Tasks: &fakeSubstrate{}, Subjects: host.SubjectLoaderFunc(func(context.Context, uuid.UUID, int64, spi.SubjectKind) (spi.Subject, error) {
		return spi.Subject{}, &host.SubjectBlockedError{TenantID: tenant, UserID: me, Reason: "user is blocked"}
	})})
	ok, err = svc.CanRead(context.Background(), tenant, u, uuid.New())
	require.NoError(t, err)
	assert.False(t, ok)
}

// A permission the user holds covers them although a role lookup fails; without one, the failure is an
// error.
func TestViaKeepsAPermissionWhenARoleLookupFails(t *testing.T) {
	t.Parallel()
	down := errors.New("roles: timeout")
	failing := audienceFunc(func(workflow.TaskAssignee) (bool, error) { return false, down })
	role := workflow.TaskAssignee{Kind: workflow.TaskAssignRole, Ref: "1"}
	perm := workflow.TaskAssignee{Kind: workflow.TaskAssignPermission, Ref: "Invoices.Approve"}

	withPerm := openTask(perm, role)
	svc := newSvc(t, Config{Tasks: &fakeSubstrate{tasks: []workflow.Task{withPerm}}, Audience: failing, Subjects: subjects("Invoices.Approve")})
	v, err := svc.Get(userCtx(), withPerm.ID)
	require.NoError(t, err)
	assert.Equal(t, ViaPermission, v.AssignedVia)
	assert.True(t, v.CanDecide)

	onlyRole := openTask(role)
	svc = newSvc(t, Config{Tasks: &fakeSubstrate{tasks: []workflow.Task{onlyRole}}, Audience: failing})
	_, err = svc.Get(userCtx(), onlyRole.ID)
	require.ErrorIs(t, err, down)

	// A second role that covers the user still counts after the first lookup failed.
	group := workflow.TaskAssignee{Kind: workflow.TaskAssignGroup, Ref: "g"}
	both := openTask(role, group)
	svc = newSvc(t, Config{Tasks: &fakeSubstrate{tasks: []workflow.Task{both}}, Audience: audienceFunc(func(a workflow.TaskAssignee) (bool, error) {
		if a.Kind == workflow.TaskAssignRole {
			return false, down
		}
		return true, nil
	})})
	v, err = svc.Get(userCtx(), both.ID)
	require.NoError(t, err)
	assert.Equal(t, ViaGroup, v.AssignedVia)
}

// Quorum progress counts the votes that agree, as the substrate closes a task: a split vote does not
// read as complete.
func TestQuorumCountsAgreeingVotes(t *testing.T) {
	t.Parallel()
	split := openTask(userAssignee(me))
	split.Required = 2
	split.Decisions = []workflow.TaskDecision{{UserID: 1, Choice: "approve"}, {UserID: 2, Choice: "reject"}}
	agree := openTask(userAssignee(me))
	agree.Required = 2
	agree.Decisions = []workflow.TaskDecision{{UserID: 1, Choice: "approve"}, {UserID: 2, Choice: "reject"}, {UserID: 3, Choice: "approve"}}
	svc := newSvc(t, Config{Tasks: &fakeSubstrate{tasks: []workflow.Task{split, agree}}})
	v, err := svc.Get(userCtx(), split.ID)
	require.NoError(t, err)
	assert.Equal(t, Quorum{Required: 2, Decided: 1}, v.Quorum)
	v, err = svc.Get(userCtx(), agree.ID)
	require.NoError(t, err)
	assert.Equal(t, Quorum{Required: 2, Decided: 2}, v.Quorum)
}

// A subject the resolver cannot name keeps the task listed and decidable, with a neutral label and no
// link.
func TestSubjectNameFailureKeepsTheTaskUsable(t *testing.T) {
	t.Parallel()
	task := openTask(userAssignee(me))
	task.SubjectEntity, task.SubjectKey = "invoice", "42"
	task.AllowDelegation = true
	sub := &fakeSubstrate{tasks: []workflow.Task{task}}
	svc := newSvc(t, Config{Tasks: sub, SubjectNames: resolverFunc(func() (string, string, error) {
		return "", "", errors.New("records: timeout")
	})})
	ctx := userCtx()
	page, err := svc.List(ctx, TaskFilter{})
	require.NoError(t, err)
	require.Len(t, page.Items, 1)
	require.NotNil(t, page.Items[0].Subject)
	assert.Equal(t, SubjectRef{Entity: "invoice", Key: "42", Label: "invoice"}, *page.Items[0].Subject)
	require.NoError(t, svc.Decide(ctx, task.ID, DecideInput{Outcome: "approve"}))
	assert.Equal(t, int32(1), sub.decided.Load())
	require.NoError(t, svc.Delegate(ctx, task.ID, 9, ""), "delegation is not refused for the subject's name")
}

// testForm counts its actions; Prepare/Intent are wired by the embedding types.
type testForm struct {
	acted   atomic.Int32
	outcome string
	fields  string
	ctxDue  atomic.Pointer[time.Time]
	// prepared are the fields Prepare returns when set (else fields); onPrepare runs inside Prepare.
	prepared  string
	onPrepare func()
}

func (*testForm) Kind() string                                     { return "acme.test" }
func (*testForm) Render(context.Context, TaskView) templ.Component { return templ.NopComponent }
func (f *testForm) Decide(ctx context.Context, _ TaskView, _ user.User, _ json.RawMessage) (FormDecision, error) {
	f.acted.Add(1)
	if d, ok := ctx.Deadline(); ok {
		f.ctxDue.Store(&d)
	}
	return FormDecision{Outcome: f.outcome, Fields: json.RawMessage(f.fields)}, nil
}

type preparedForm struct{ *testForm }

func (f preparedForm) Prepare(context.Context, TaskView, user.User, json.RawMessage) (FormDecision, error) {
	if f.onPrepare != nil {
		f.onPrepare()
	}
	fields := f.fields
	if f.prepared != "" {
		fields = f.prepared
	}
	return FormDecision{Outcome: f.outcome, Fields: json.RawMessage(fields)}, nil
}

type intentForm struct{ *testForm }

func (f intentForm) Intent(context.Context, TaskView, json.RawMessage) (string, error) {
	return f.outcome, nil
}

const formSchema = `,"form":{"type":"object","x-iota-form":"acme.test","properties":{"amount":{"type":"integer"}},"required":["amount"]}`

// openSchema names the form and constrains no field.
const openSchema = `,"form":{"type":"object","x-iota-form":"acme.test"}`

// A submission the task would refuse never reaches the form's action: a comment too long, an outcome
// that lacks its comment (FormIntent), fields that break the schema (FormPreparer).
func TestDecideValidatesBeforeTheFormActs(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name   string
		form   func(*testForm) TaskForm
		schema string
		in     DecideInput
		out    string
		flds   string
	}{
		{"comment too long, plain form", func(f *testForm) TaskForm { return f }, openSchema, DecideInput{Comment: strings.Repeat("я", MaxCommentLength+1)}, "approve", `{}`},
		{"reject without its comment, FormIntent", func(f *testForm) TaskForm { return intentForm{f} }, openSchema, DecideInput{}, "reject", `{}`},
		{"fields break the schema, FormPreparer", func(f *testForm) TaskForm { return preparedForm{f} }, formSchema, DecideInput{}, "approve", `{"amount":"many"}`},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			f := &testForm{outcome: c.out, fields: c.flds}
			task := openTask(userAssignee(me))
			sub := &fakeSubstrate{tasks: []workflow.Task{task}}
			svc := newSvc(t, Config{Tasks: sub, Store: fakeStore{params: strings.Replace(buttons, "%s", c.schema, 1)}, Forms: NewForms(c.form(f))})
			in := c.in
			in.Form = json.RawMessage(`{}`)
			err := svc.Decide(userCtx(), task.ID, in)
			require.ErrorIs(t, err, ErrInvalidDecision)
			assert.Zero(t, f.acted.Load(), "the form did not act")
			assert.Zero(t, sub.decided.Load())
		})
	}

	// A valid prepared decision acts once and is recorded.
	f := &testForm{outcome: "approve", fields: `{"amount":3}`}
	task := openTask(userAssignee(me))
	sub := &fakeSubstrate{tasks: []workflow.Task{task}}
	svc := newSvc(t, Config{Tasks: sub, Store: fakeStore{params: strings.Replace(buttons, "%s", formSchema, 1)}, Forms: NewForms(preparedForm{f})})
	require.NoError(t, svc.Decide(userCtx(), task.ID, DecideInput{Form: json.RawMessage(`{}`)}))
	assert.Equal(t, int32(1), f.acted.Load())
	assert.Equal(t, int32(1), sub.decided.Load())
}

// A form that acts before its fields are known (no FormPreparer) cannot decide a task whose schema
// constrains them: its fields could break the schema after its action. It still decides an open one.
func TestFormsWithoutPrepareDoNotActOnConstrainedSchemas(t *testing.T) {
	t.Parallel()
	for name, form := range map[string]func(*testForm) TaskForm{
		"plain":      func(f *testForm) TaskForm { return f },
		"FormIntent": func(f *testForm) TaskForm { return intentForm{f} },
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			f := &testForm{outcome: "approve", fields: `{"amount":"many"}`}
			task := openTask(userAssignee(me))
			sub := &fakeSubstrate{tasks: []workflow.Task{task}}
			svc := newSvc(t, Config{Tasks: sub, Store: fakeStore{params: strings.Replace(buttons, "%s", formSchema, 1)}, Forms: NewForms(form(f))})
			v, err := svc.Get(userCtx(), task.ID)
			require.NoError(t, err)
			assert.True(t, v.FormUnavailable)
			require.ErrorIs(t, svc.Decide(userCtx(), task.ID, DecideInput{Form: json.RawMessage(`{}`)}), ErrFormUnavailable)
			assert.Zero(t, f.acted.Load(), "the form did not act")
			assert.Zero(t, sub.decided.Load())

			f = &testForm{outcome: "approve", fields: `{"signed":true}`}
			sub = &fakeSubstrate{tasks: []workflow.Task{task}}
			svc = newSvc(t, Config{Tasks: sub, Store: fakeStore{params: strings.Replace(buttons, "%s", openSchema, 1)}, Forms: NewForms(form(f))})
			require.NoError(t, svc.Decide(userCtx(), task.ID, DecideInput{Form: json.RawMessage(`{}`)}))
			assert.Equal(t, int32(1), f.acted.Load())
			assert.JSONEq(t, `{"outcome":"approve","fields":{"signed":true}}`, string(sub.last))
		})
	}
	assert.False(t, ConstrainsFields(json.RawMessage(`{"type":"object","x-iota-form":"a","title":"T","additionalProperties":true}`)))
	assert.True(t, ConstrainsFields(json.RawMessage(`{"type":"object","x-iota-form":"a","minProperties":1}`)))
}

// Once a prepared decision has passed, the action's result is recorded only when it agrees with it and
// passes too; otherwise the prepared decision is, so an action that ran always has a recorded decision.
func TestPreparedDecisionIsRecordedWhenTheActionDisagrees(t *testing.T) {
	t.Parallel()
	f := &testForm{outcome: "approve", prepared: `{"amount":3}`, fields: `{"amount":"many"}`}
	task := openTask(userAssignee(me))
	sub := &fakeSubstrate{tasks: []workflow.Task{task}}
	svc := newSvc(t, Config{Tasks: sub, Store: fakeStore{params: strings.Replace(buttons, "%s", formSchema, 1)}, Forms: NewForms(preparedForm{f})})
	require.NoError(t, svc.Decide(userCtx(), task.ID, DecideInput{Form: json.RawMessage(`{}`)}))
	assert.Equal(t, int32(1), f.acted.Load())
	assert.JSONEq(t, `{"outcome":"approve","fields":{"amount":3}}`, string(sub.last))

	f = &testForm{outcome: "approve", prepared: `{"amount":3}`, fields: `{"amount":4}`}
	sub = &fakeSubstrate{tasks: []workflow.Task{task}}
	svc = newSvc(t, Config{Tasks: sub, Store: fakeStore{params: strings.Replace(buttons, "%s", formSchema, 1)}, Forms: NewForms(preparedForm{f})})
	require.NoError(t, svc.Decide(userCtx(), task.ID, DecideInput{Form: json.RawMessage(`{}`)}))
	assert.JSONEq(t, `{"outcome":"approve","fields":{"amount":4}}`, string(sub.last), "a valid result of the action")
}

// The action is authorized by the task as it is when it starts: an escalation that applies its last
// step while the submission is prepared, and whose new final deadline passes, stops it.
func TestDecideReauthorizesAfterPreparation(t *testing.T) {
	t.Parallel()
	due := time.Now().Add(time.Hour).UTC()
	task := openTask(userAssignee(me))
	task.DueAt = &due
	task.Escalation = []workflow.EscalationStep{{Extend: time.Minute}}
	sub := &fakeSubstrate{tasks: []workflow.Task{task}}
	f := &testForm{outcome: "approve", fields: `{"amount":1}`}
	f.onPrepare = func() {
		// The run applies the last step meanwhile, and its deadline passes.
		past := time.Now().Add(-time.Second).UTC()
		sub.tasks[0].EscalationLevel, sub.tasks[0].DueAt = 1, &past
	}
	svc := newSvc(t, Config{Tasks: sub, Store: fakeStore{params: strings.Replace(buttons, "%s", formSchema, 1)}, Forms: NewForms(preparedForm{f})})
	v, err := svc.Get(userCtx(), task.ID)
	require.NoError(t, err)
	require.Nil(t, v.Deadline, "no final deadline while the step is pending")
	require.ErrorIs(t, svc.Decide(userCtx(), task.ID, DecideInput{Form: json.RawMessage(`{}`)}), ErrTaskClosed)
	assert.Zero(t, f.acted.Load(), "the form did not act")
	assert.Zero(t, sub.decided.Load())
}

// The final deadline is judged afresh as the form's action starts, and the action's context ends at it.
func TestDecideEnforcesTheFinalDeadlineAtTheFormAction(t *testing.T) {
	t.Parallel()
	due := time.Now().Add(time.Hour).UTC()
	task := openTask(userAssignee(me))
	task.DueAt = &due
	params := strings.Replace(buttons, "%s", formSchema, 1)

	// The view is read before the deadline, the action would start after it.
	var calls atomic.Int32
	now := func() time.Time {
		if calls.Add(1) == 1 {
			return due.Add(-time.Second)
		}
		return due.Add(time.Second)
	}
	f := &testForm{outcome: "approve", fields: `{"amount":1}`}
	sub := &fakeSubstrate{tasks: []workflow.Task{task}}
	svc := newSvc(t, Config{Tasks: sub, Store: fakeStore{params: params}, Forms: NewForms(preparedForm{f}), Now: now})
	err := svc.Decide(userCtx(), task.ID, DecideInput{Form: json.RawMessage(`{}`)})
	require.ErrorIs(t, err, ErrTaskClosed)
	assert.Zero(t, f.acted.Load(), "the form did not act past the deadline")

	// Before it, the action runs with the deadline on its context.
	f = &testForm{outcome: "approve", fields: `{"amount":1}`}
	svc = newSvc(t, Config{Tasks: sub, Store: fakeStore{params: params}, Forms: NewForms(preparedForm{f})})
	require.NoError(t, svc.Decide(userCtx(), task.ID, DecideInput{Form: json.RawMessage(`{}`)}))
	require.Equal(t, int32(1), f.acted.Load())
	got := f.ctxDue.Load()
	require.NotNil(t, got)
	assert.True(t, got.Equal(due))

	// With an escalation step pending the due time is no final deadline.
	pending := task
	pending.ID = uuid.New()
	pending.Escalation = []workflow.EscalationStep{{Extend: time.Hour}}
	svc = newSvc(t, Config{Tasks: &fakeSubstrate{tasks: []workflow.Task{pending}}, Store: fakeStore{params: params}, Forms: NewForms(&testForm{})})
	v, err := svc.Get(userCtx(), pending.ID)
	require.NoError(t, err)
	assert.Nil(t, v.Deadline)
}
