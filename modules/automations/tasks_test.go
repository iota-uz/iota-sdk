package automations_test

import (
	"context"
	"embed"
	"encoding/json"
	stderrors "errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/a-h/templ"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/iota-uz/iota-sdk/modules"
	"github.com/iota-uz/iota-sdk/modules/automations"
	"github.com/iota-uz/iota-sdk/modules/automations/presentation/graphs"
	"github.com/iota-uz/iota-sdk/modules/automations/presentation/inbox"
	"github.com/iota-uz/iota-sdk/modules/automations/tasks"
	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/user"
	"github.com/iota-uz/iota-sdk/modules/core/domain/value_objects/internet"
	"github.com/iota-uz/iota-sdk/modules/core/notifications"
	"github.com/iota-uz/iota-sdk/modules/core/services"
	"github.com/iota-uz/iota-sdk/pkg/automation/engine"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/composition"
	"github.com/iota-uz/iota-sdk/pkg/config"
	"github.com/iota-uz/iota-sdk/pkg/config/providers/static"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/events"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/spi"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/wiring"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/workflow"
	"github.com/iota-uz/iota-sdk/pkg/itf"
)

// flakyForm is a TaskForm whose Decide fails while fail is set (a signature the provider refused).
type flakyForm struct {
	fail  atomic.Bool
	calls atomic.Int32
}

func (*flakyForm) Kind() string { return "acme.sign" }
func (*flakyForm) Render(context.Context, tasks.TaskView) templ.Component {
	return templ.Raw(`<input type="hidden" name="form" value='{"signature":"s"}'><button type="submit" data-testid="acme-sign">Sign</button>`)
}
func (f *flakyForm) Decide(_ context.Context, _ tasks.TaskView, _ user.User, form json.RawMessage) (tasks.FormDecision, error) {
	f.calls.Add(1)
	if f.fail.Load() {
		return tasks.FormDecision{}, &tasks.FormError{Message: "The signature was refused", Err: stderrors.New("provider: 502")}
	}
	return tasks.FormDecision{Outcome: "approve", Fields: json.RawMessage(`{"signed":true}`)}, nil
}

// Intent implements tasks.FormIntent: {"reject":true} asks to reject, anything else to approve.
func (*flakyForm) Intent(_ context.Context, _ tasks.TaskView, form json.RawMessage) (string, error) {
	var f struct {
		Reject bool `json:"reject"`
	}
	_ = json.Unmarshal(form, &f)
	if f.Reject {
		return "reject", nil
	}
	return "approve", nil
}

// formComponent contributes a task form.
type formComponent struct{ form tasks.TaskForm }

func (formComponent) Descriptor() composition.Descriptor {
	return composition.Descriptor{Name: "acme-forms"}
}
func (formComponent) LocaleFS() []*embed.FS { return nil }
func (f formComponent) Build(b *composition.Builder) error {
	tasks.ContributeTaskForm(b, f.form)
	return nil
}

// tasksApp is the SDK composition with the extension core and the given extra components.
func tasksApp(t *testing.T, extra ...composition.Component) (*itf.Suite, *composition.Container) {
	t.Helper()
	return tasksAppWith(t, nil, extra...)
}

// tasksAppWith is tasksApp with more configuration.
func tasksAppWith(t *testing.T, cfg map[string]any, extra ...composition.Component) (*itf.Suite, *composition.Container) {
	t.Helper()
	values := map[string]any{"automation.connections.keys": "k1:" + strings.Repeat("ab", 32)}
	for k, v := range cfg {
		values[k] = v
	}
	src, err := config.Build(static.New(values))
	require.NoError(t, err)
	components := []composition.Component{}
	for _, c := range modules.Components() {
		if c.Descriptor().Name != "automations" {
			components = append(components, c)
		}
	}
	components = append(components, automations.NewComponent(automations.WithoutBackground()), wiring.NewComponent(wiring.Options{Natives: modules.Natives()}))
	components = append(components, extra...)
	s := itf.NewSuiteBuilder(t).WithComponents(components...).WithSource(src).AsUser(allAutomations...).Build()
	s = s.AsUser(sameTenant(s, allAutomations...))
	found := false
	for _, c := range s.Env().App.Controllers() {
		switch c.Descriptor().ID {
		case graphs.ControllerID, inbox.ControllerID:
			s.Register(c)
			found = found || c.Descriptor().ID == inbox.ControllerID
		}
	}
	require.True(t, found, "the inbox is composed")
	return s, s.Env().Container
}

// people are committed users of the suite's tenant (the engine reads them on its own connections).
type people struct {
	t      *testing.T
	db     *pgxpool.Pool
	tenant uuid.UUID
}

func (p people) user(name string) user.User {
	p.t.Helper()
	var id uint
	email := strings.ToLower(name) + "-" + uuid.NewString()[:8] + "@example.test"
	require.NoError(p.t, p.db.QueryRow(context.Background(), `
		INSERT INTO users (first_name, last_name, email, ui_language, type, tenant_id) VALUES ($1, 'Tester', $2, 'en', 'user', $3) RETURNING id`,
		name, email, p.tenant).Scan(&id))
	return user.New(name, "Tester", internet.MustParseEmail(email), "en", user.WithID(id), user.WithTenantID(p.tenant))
}

func (p people) role(name string, members ...user.User) int64 {
	p.t.Helper()
	var id int64
	require.NoError(p.t, p.db.QueryRow(context.Background(), `INSERT INTO roles (name, type, tenant_id) VALUES ($1, 'user', $2) RETURNING id`, name+"-"+uuid.NewString()[:6], p.tenant).Scan(&id))
	for _, m := range members {
		p.exec(`INSERT INTO user_roles (user_id, role_id) VALUES ($1, $2)`, m.ID(), id)
	}
	return id
}

func (p people) group(name string, members ...user.User) uuid.UUID {
	p.t.Helper()
	var id uuid.UUID
	require.NoError(p.t, p.db.QueryRow(context.Background(), `INSERT INTO user_groups (name, type, tenant_id) VALUES ($1, 'user', $2) RETURNING id`, name+"-"+uuid.NewString()[:6], p.tenant).Scan(&id))
	for _, m := range members {
		p.exec(`INSERT INTO group_users (group_id, user_id) VALUES ($1, $2)`, id, m.ID())
	}
	return id
}

func (p people) exec(sql string, args ...any) {
	p.t.Helper()
	_, err := p.db.Exec(context.Background(), sql, args...)
	require.NoError(p.t, err)
}

// humanGraph starts manually and asks the assignees with a human node.
func humanTaskGraph(title string, params map[string]any) map[string]any {
	p := map[string]any{
		"title": title, "body": "Check the **amount** before approving.",
		"buttons": []any{
			map[string]any{"outcome": "approve", "label": "Approve", "style": "primary"},
			map[string]any{"outcome": "reject", "label": "Reject", "style": "danger", "require_comment": true},
		},
	}
	for k, v := range params {
		p[k] = v
	}
	edges := []any{
		map[string]any{"from": map[string]any{"node": "start", "outcome": "fired"}, "to": "ask"},
		map[string]any{"from": map[string]any{"node": "ask", "outcome": "approve"}, "to": "fin"},
		map[string]any{"from": map[string]any{"node": "ask", "outcome": "reject"}, "to": "fin"},
	}
	if _, ok := p["timeout"]; ok {
		edges = append(edges, map[string]any{"from": map[string]any{"node": "ask", "outcome": "timeout"}, "to": "fin"})
	}
	return map[string]any{
		"name": title, "run_as": "owner",
		"triggers": []any{map[string]any{"id": "start", "event": engine.EventManual}},
		"nodes": []any{
			map[string]any{"id": "ask", "type": engine.HumanNodeType, "params": p},
			map[string]any{"id": "fin", "type": "end", "params": map[string]any{}},
		},
		"edges": edges,
	}
}

// openTask saves, enables and starts the graph, and runs the engine until the human node's task is
// open; it returns the task and the run.
func openTask(t *testing.T, s *itf.Suite, container *composition.Container, g map[string]any) (uuid.UUID, uuid.UUID) {
	t.Helper()
	return openTaskAbout(t, s, container, g, nil)
}

// openTaskAbout is openTask with the run's subject.
func openTaskAbout(t *testing.T, s *itf.Suite, container *composition.Container, g map[string]any, subject map[string]any) (uuid.UUID, uuid.UUID) {
	t.Helper()
	author := s.Env().User
	var created graphs.SaveResult
	require.Nil(t, rpc(t, s, graphs.MethodCreate, map[string]any{"graph": g, "layout": map[string]any{}}, &created).Error)
	require.True(t, created.OK, "%+v", created)
	var review graphs.ReviewResponse
	require.Nil(t, rpc(t, s, graphs.MethodEnableReview, map[string]any{"id": created.ID, "version": created.Version}, &review).Error)
	var enabled graphs.EnableResponse
	require.Nil(t, rpc(t, s, graphs.MethodEnable, map[string]any{"id": created.ID, "version": created.Version, "code_digest": review.CodeDigest}, &enabled).Error)
	require.True(t, enabled.OK, "%+v", enabled)
	var started graphs.RunManualResponse
	start := map[string]any{"id": created.ID, "trigger_id": "start", "input": map[string]any{}}
	if subject != nil {
		start["subject"] = subject
	}
	require.Nil(t, rpc(t, s, graphs.MethodRunManual, start, &started).Error)
	require.True(t, started.OK, "%+v", started)
	s.AsUser(author)
	runID, err := uuid.Parse(started.RunID)
	require.NoError(t, err)
	var task uuid.UUID
	waitFor(t, container, func(db *pgxpool.Pool) bool {
		err := db.QueryRow(context.Background(), `
			SELECT t.id FROM extruntime.human_tasks t JOIN automation.runs r ON r.instance_id = t.instance_id
			WHERE r.id = $1 AND t.status = 'open'`, runID).Scan(&task)
		return err == nil
	}, "the task opens")
	return task, runID
}

// waitFor runs the workflow engine until cond holds.
func waitFor(t *testing.T, container *composition.Container, cond func(db *pgxpool.Pool) bool, what string) {
	t.Helper()
	wf, err := composition.Resolve[*workflow.Engine](container)
	require.NoError(t, err)
	db, err := composition.Resolve[*pgxpool.Pool](container)
	require.NoError(t, err)
	deadline := time.Now().Add(30 * time.Second)
	for !cond(db) {
		if time.Now().After(deadline) {
			t.Fatalf("timed out: %s", what)
		}
		_, err := wf.RunOnce(context.Background())
		require.NoError(t, err)
		time.Sleep(20 * time.Millisecond)
	}
}

func runStatus(t *testing.T, container *composition.Container, run uuid.UUID) string {
	t.Helper()
	db, err := composition.Resolve[*pgxpool.Pool](container)
	require.NoError(t, err)
	var status string
	require.NoError(t, db.QueryRow(context.Background(), `
		SELECT i.status FROM automation.runs r JOIN extruntime.workflow_instances i ON i.id = r.instance_id WHERE r.id = $1`, run).Scan(&status))
	return status
}

func decide(s *itf.Suite, task uuid.UUID, fields url.Values) *itf.Request {
	return s.POST("/automations/tasks/" + task.String() + "/decide").HTMX().Form(fields)
}

func decisions(t *testing.T, container *composition.Container, task uuid.UUID) int {
	t.Helper()
	db, err := composition.Resolve[*pgxpool.Pool](container)
	require.NoError(t, err)
	var n int
	require.NoError(t, db.QueryRow(context.Background(), `SELECT count(*) FROM extruntime.human_task_decisions WHERE task_id = $1`, task).Scan(&n))
	return n
}

// A task on a role with a quorum of 2 of 3: after the first decision the two others see «1 of 2», the
// one who decided finds it under «Decided by me»; the second decision closes it for the third, who sees
// who decided it. A double submit records one decision; a token is refused with HUMAN_REQUIRED.
func TestInboxQuorumOfARole(t *testing.T) {
	s, container := tasksApp(t)
	env := s.Env()
	pp := people{t: t, db: env.Pool, tenant: env.Tenant.ID}
	anna, boris, vera := pp.user("Anna"), pp.user("Boris"), pp.user("Vera")
	approvers := pp.role("Approvers", anna, boris, vera)
	task, run := openTask(t, s, container, humanTaskGraph("Approve the invoice", map[string]any{
		"assignees": []any{map[string]any{"kind": "role", "ref": strconv.FormatInt(approvers, 10)}}, "required": 2,
	}))

	for _, u := range []user.User{anna, boris, vera} {
		s.AsUser(u)
		body := s.GET("/automations/tasks").Expect(t).Status(http.StatusOK).Contains("Approve the invoice").Body()
		assert.Contains(t, body, "0 of 2")
		assert.Contains(t, body, "Your role")
	}
	s.AsUser(anna)
	page := s.GET("/automations/tasks/" + task.String()).Expect(t).Status(http.StatusOK).Body()
	assert.Contains(t, page, "<strong>amount</strong>", "the instructions are markdown")
	assert.Contains(t, page, `data-testid="automations-task-outcome-approve"`)
	assert.Contains(t, page, `data-testid="automations-task-confirm-reject"`, "the danger outcome is confirmed inline")
	assert.Contains(t, page, "(required for Reject)", "the required comment is marked before sending")

	// A rejection without its comment is refused and nothing is recorded.
	decide(s, task, url.Values{"outcome": {"reject"}}).Expect(t).Status(http.StatusOK).Contains(`data-testid="automations-task-error"`)
	assert.Equal(t, 0, decisions(t, container, task))

	// A token is not a person: refused with the code, before the task is read.
	for _, target := range []uuid.UUID{task, uuid.New()} {
		refused := s.POST("/automations/tasks/"+target.String()+"/decide").Header("Authorization", "Bearer agent").Form(url.Values{"outcome": {"approve"}}).
			Expect(t).Status(http.StatusForbidden)
		assert.Equal(t, inbox.CodeHumanRequired, refused.Header("X-Error-Code"))
		assert.Contains(t, refused.Body(), `"code":"HUMAN_REQUIRED"`)
	}
	s.POST("/automations/tasks/"+task.String()+"/delegate").Header("Authorization", "Bearer agent").Form(url.Values{"to": {"1"}}).
		Expect(t).Status(http.StatusForbidden).Contains("HUMAN_REQUIRED")
	assert.Equal(t, 0, decisions(t, container, task))

	// A double click records one decision.
	first := decide(s, task, url.Values{"outcome": {"approve"}}).Expect(t).Status(http.StatusOK)
	assert.Contains(t, first.Header("Hx-Trigger"), inbox.EventTasksChanged)
	assert.Contains(t, first.Header("Hx-Trigger"), "notify")
	decide(s, task, url.Values{"outcome": {"approve"}}).Expect(t).Status(http.StatusOK)
	assert.Equal(t, 1, decisions(t, container, task))
	s.GET("/automations/tasks").Expect(t).NotContains("Approve the invoice")
	s.GET("/automations/tasks?tab=decided").Expect(t).Contains("Approve the invoice").Contains("You decided: Approve")

	for _, u := range []user.User{boris, vera} {
		s.AsUser(u)
		s.GET("/automations/tasks").Expect(t).Contains("Approve the invoice").Contains("1 of 2")
	}
	// The vote that closes the quorum, submitted twice at once: both answers are that decision, also
	// the one that finds the task closed by its twin.
	svc, ok, err := automations.TaskInboxOf(container)
	require.NoError(t, err)
	require.True(t, ok)
	bctx := tasks.WithOrigin(composables.WithTenantID(composables.WithUser(context.Background(), boris), env.Tenant.ID), engine.OriginHuman)
	var wg sync.WaitGroup
	errs := make([]error, 4)
	for i := range errs {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			errs[i] = svc.Decide(bctx, task, tasks.DecideInput{Outcome: "approve"})
		}(i)
	}
	wg.Wait()
	for _, err := range errs {
		require.NoError(t, err)
	}
	require.ErrorIs(t, svc.Decide(bctx, task, tasks.DecideInput{Outcome: "reject", Comment: "x"}), tasks.ErrAlreadyDecided)
	require.ErrorIs(t, svc.Decide(bctx, task, tasks.DecideInput{}), tasks.ErrAlreadyDecided, "no button is not a confirmed repeat")
	assert.Equal(t, 2, decisions(t, container, task))
	waitFor(t, container, func(*pgxpool.Pool) bool { return runStatus(t, container, run) == "completed" }, "the quorum wakes the run")

	s.AsUser(vera)
	s.GET("/automations/tasks").Expect(t).NotContains("Approve the invoice")
	closed := decide(s, task, url.Values{"outcome": {"reject"}, "comment": {"too late"}}).Expect(t).Status(http.StatusOK).Body()
	assert.Contains(t, closed, `data-testid="automations-task-closed"`)
	assert.Contains(t, closed, "Already decided by Boris Tester", "who closed the task")
	assert.Equal(t, 2, decisions(t, container, task))

	// Anna leaves the role: the task, its decisions and her own vote are hidden from her now.
	pp.exec(`DELETE FROM user_roles WHERE user_id = $1 AND role_id = $2`, anna.ID(), approvers)
	s.AsUser(anna)
	s.GET("/automations/tasks?tab=decided").Expect(t).NotContains("Approve the invoice")
	s.GET("/automations/tasks/" + task.String()).Expect(t).Status(http.StatusNotFound).NotContains("Boris Tester")
	// The click is answered in place: «not available», not a silent 404 htmx would not swap.
	lost := decide(s, task, url.Values{"outcome": {"approve"}}).Expect(t).Status(http.StatusOK).Contains(`data-testid="automations-task-missing"`)
	assert.Equal(t, "#automations-tasks", lost.Header("Hx-Retarget"))
}

// A member removed from the group after the task was created no longer sees it and a direct decision
// is «not found»; a delegation passes the task to a colleague with «Passed from», and it leaves the
// delegator's list. Another tenant's task and an unknown one look the same.
func TestInboxMembershipAndDelegation(t *testing.T) {
	s, container := tasksApp(t)
	env := s.Env()
	pp := people{t: t, db: env.Pool, tenant: env.Tenant.ID}
	gleb, dina, oleg := pp.user("Gleb"), pp.user("Dina"), pp.user("Oleg")
	team := pp.group("Reviewers", gleb, dina)
	task, _ := openTask(t, s, container, humanTaskGraph("Review the contract", map[string]any{
		"assignees": []any{map[string]any{"kind": "group", "ref": team.String()}}, "allow_delegate": true,
	}))

	s.AsUser(gleb)
	s.GET("/automations/tasks").Expect(t).Contains("Review the contract").Contains("Your group")
	pp.exec(`DELETE FROM group_users WHERE group_id = $1 AND user_id = $2`, team, gleb.ID())
	s.GET("/automations/tasks").Expect(t).NotContains("Review the contract")
	decide(s, task, url.Values{"outcome": {"approve"}}).Expect(t).Status(http.StatusOK).Contains(`data-testid="automations-task-missing"`)
	assert.Equal(t, 0, decisions(t, container, task))
	s.GET("/automations/tasks/" + uuid.NewString()).Expect(t).Status(http.StatusNotFound).Contains(`data-testid="automations-task-missing"`)

	s.AsUser(dina)
	// "Tester" is the last name of every user: the search matches Dina too, so only the self-exclusion
	// keeps her out.
	opts := s.GET("/automations/tasks/" + task.String() + "/delegates?q=Tester").Expect(t).Status(http.StatusOK).Body()
	assert.Contains(t, opts, fmt.Sprintf(`data-value="%d"`, oleg.ID()))
	assert.Contains(t, opts, fmt.Sprintf(`data-value="%d"`, gleb.ID()))
	assert.NotContains(t, opts, fmt.Sprintf(`data-value="%d"`, dina.ID()), "not to oneself")
	s.POST("/automations/tasks/" + task.String() + "/delegate").HTMX().Form(url.Values{"to": {fmt.Sprint(dina.ID())}}).
		Expect(t).Status(http.StatusOK).Contains(`data-testid="automations-task-delegate-error"`)
	done := s.POST("/automations/tasks/" + task.String() + "/delegate").HTMX().Form(url.Values{"to": {fmt.Sprint(oleg.ID())}, "delegate_comment": {"you know this client"}}).
		Expect(t).Status(http.StatusOK)
	assert.Contains(t, done.Header("Hx-Trigger"), "Passed to Oleg Tester")
	s.GET("/automations/tasks").Expect(t).NotContains("Review the contract")

	s.AsUser(oleg)
	s.GET("/automations/tasks").Expect(t).Contains("Review the contract").Contains("Passed from Dina Tester")
	decide(s, task, url.Values{"outcome": {"approve"}}).Expect(t).Status(http.StatusOK)
	assert.Equal(t, 1, decisions(t, container, task))

	other, _ := tasksApp(t)
	other.GET("/automations/tasks/" + task.String()).Expect(t).Status(http.StatusNotFound).Contains(`data-testid="automations-task-missing"`)
}

// A task form: its error records nothing and the run keeps waiting; the retry goes through with the
// form's outcome and fields. A form annotation without a registered form is refused on save.
func TestInboxTaskForm(t *testing.T) {
	form := &flakyForm{}
	form.fail.Store(true)
	s, container := tasksApp(t, formComponent{form: form})
	env := s.Env()
	pp := people{t: t, db: env.Pool, tenant: env.Tenant.ID}
	signer := pp.user("Sara")

	var refused graphs.SaveResult
	require.Nil(t, rpc(t, s, graphs.MethodCreate, map[string]any{"graph": humanTaskGraph("Unknown form", map[string]any{
		"assignees": []any{map[string]any{"kind": "user", "user_id": signer.ID()}},
		"form":      map[string]any{"type": "object", "x-iota-form": "acme.missing"},
	}), "layout": map[string]any{}}, &refused).Error)
	assert.False(t, refused.OK, "an unregistered form is an issue of the graph")

	task, run := openTask(t, s, container, humanTaskGraph("Sign the act", map[string]any{
		"assignees": []any{map[string]any{"kind": "user", "user_id": signer.ID()}},
		"form":      map[string]any{"type": "object", "x-iota-form": "acme.sign"},
	}))
	s.AsUser(signer)
	s.GET("/automations/tasks/" + task.String()).Expect(t).Status(http.StatusOK).Contains(`data-testid="acme-sign"`).NotContains(`data-testid="automations-task-outcomes"`)
	refusedBody := decide(s, task, url.Values{"form": {`{"signature":"s"}`}}).Expect(t).Status(http.StatusOK).Body()
	assert.Contains(t, refusedBody, "The signature was refused")
	assert.Equal(t, 0, decisions(t, container, task))
	assert.Equal(t, "waiting", runStatus(t, container, run))

	form.fail.Store(false)
	decide(s, task, url.Values{"form": {`{"signature":"s"}`}}).Expect(t).Status(http.StatusOK)
	assert.Equal(t, 1, decisions(t, container, task))
	db, err := composition.Resolve[*pgxpool.Pool](container)
	require.NoError(t, err)
	var stored string
	require.NoError(t, db.QueryRow(context.Background(), `SELECT decision::text FROM extruntime.human_task_decisions WHERE task_id = $1`, task).Scan(&stored))
	assert.JSONEq(t, `{"outcome":"approve","fields":{"signed":true}}`, stored)

	// The form sent again asking the same is that decision, without its action; asking another is
	// refused; a default-form submit with no button is not a repeat either.
	calls := form.calls.Load()
	svc, ok, err := automations.TaskInboxOf(container)
	require.NoError(t, err)
	require.True(t, ok)
	sctx := tasks.WithOrigin(composables.WithTenantID(composables.WithUser(context.Background(), signer), env.Tenant.ID), engine.OriginHuman)
	require.NoError(t, svc.Decide(sctx, task, tasks.DecideInput{Form: json.RawMessage(`{"signature":"s"}`)}))
	require.ErrorIs(t, svc.Decide(sctx, task, tasks.DecideInput{Form: json.RawMessage(`{"reject":true}`)}), tasks.ErrAlreadyDecided)
	require.ErrorIs(t, svc.Decide(sctx, task, tasks.DecideInput{Outcome: "approve", Form: json.RawMessage(`{"reject":true}`)}), tasks.ErrAlreadyDecided,
		"the form's data chooses the outcome, not an outcome field sent with it")
	assert.Equal(t, calls, form.calls.Load(), "the signature is not sent again")
	assert.Equal(t, 1, decisions(t, container, task))
}

// A human node on a role without a notify node: automation.task.assigned.v1 names every member, and
// the module's notification delivers it to them by default with a link to the task, also to a member
// without Automations.Read; after a delegation only the delegate is named.
func TestInboxNotification(t *testing.T) {
	s, container := tasksApp(t)
	env := s.Env()
	pp := people{t: t, db: env.Pool, tenant: env.Tenant.ID}
	ira, ivan, lev := pp.user("Ira"), pp.user("Ivan"), pp.user("Lev")
	role := pp.role("Signers", ira, ivan)
	task, _ := openTask(t, s, container, humanTaskGraph("Pay the supplier", map[string]any{
		"assignees": []any{map[string]any{"kind": "role", "ref": strconv.FormatInt(role, 10)}}, "allow_delegate": true,
	}))

	cat, err := composition.Resolve[*notifications.Catalog](container)
	require.NoError(t, err)
	def, ok := cat.Get(engine.EventTaskAssigned)
	require.True(t, ok, "the module registers the notification of the event")
	assert.Equal(t, []string{tasks.RecipientAssignees}, def.DefaultRecipientKeys)

	db := env.Pool
	assigned := func() []notifications.Event {
		rows, err := db.Query(context.Background(), `SELECT id::text, payload FROM extruntime.outbox_events WHERE event = $1 AND tenant_id = $2 ORDER BY seq`, engine.EventTaskAssigned, env.Tenant.ID)
		require.NoError(t, err)
		defer rows.Close()
		var out []notifications.Event
		for rows.Next() {
			var id string
			var payload json.RawMessage
			require.NoError(t, rows.Scan(&id, &payload))
			data, err := def.DataOf(payload)
			require.NoError(t, err)
			out = append(out, notifications.Event{Key: engine.EventTaskAssigned, ID: id, TenantID: env.Tenant.ID, Payload: payload, Data: data})
		}
		return out
	}
	recipients := func(e notifications.Event) []uint {
		for _, r := range def.RecipientKeys {
			if r.Key == tasks.RecipientAssignees {
				ids, err := r.Resolve(context.Background(), e)
				require.NoError(t, err)
				return ids
			}
		}
		t.Fatal("no assignees recipient")
		return nil
	}
	// deliver hands the events to the core's outbox bridge and runs the delivery: the notices are
	// written as the core writes them, with the recipients checked at delivery.
	router, err := composition.Resolve[*services.NotificationRoutingService](container)
	require.NoError(t, err)
	dispatch, err := composition.Resolve[*services.NotificationDispatchService](container)
	require.NoError(t, err)
	bridge := services.NewNotificationOutboxBridge(router.Catalog(), dispatch)
	dctx := composables.WithTenantID(composables.WithPool(context.Background(), db), env.Tenant.ID)
	deliver := func(list []notifications.Event) {
		for _, e := range list {
			require.NoError(t, composables.InTx(dctx, func(txCtx context.Context) error {
				tx, err := composables.UseTx(txCtx)
				require.NoError(t, err)
				return bridge.Deliver(txCtx, tx.(pgx.Tx), events.Envelope{
					ID: e.ID, TenantID: e.TenantID, Event: e.Key, Payload: e.Payload, OccurredAt: time.Now(),
					Subject: &spi.ObjectRef{Entity: engine.SubjectTask, Key: task.String()},
				})
			}))
		}
		for {
			more, err := dispatch.Process(dctx)
			require.NoError(t, err)
			if !more {
				break
			}
		}
	}
	inApp := func(u user.User) []string {
		rows, err := db.Query(context.Background(), `SELECT title || ' ' || coalesce(action_url, '') FROM core.notifications WHERE tenant_id = $1 AND user_id = $2 AND event_key = $3`,
			env.Tenant.ID, u.ID(), engine.EventTaskAssigned)
		require.NoError(t, err)
		out, err := pgx.CollectRows(rows, pgx.RowTo[string])
		require.NoError(t, err)
		return out
	}
	list := assigned()
	require.Len(t, list, 1)
	assert.ElementsMatch(t, []uint{ira.ID(), ivan.ID()}, recipients(list[0]))
	content, err := def.Render(list[0], "ru")
	require.NoError(t, err)
	assert.Equal(t, "Вам назначена задача: Pay the supplier", content.Title)
	assert.Equal(t, "/automations/tasks/"+task.String(), content.ActionURL)
	ctx := context.Background()
	allowed, err := def.RecipientGuard(ctx, list[0], ira)
	require.NoError(t, err)
	assert.True(t, allowed, "an assignee without Automations.Read gets it")
	allowed, err = def.RecipientGuard(ctx, list[0], lev)
	require.NoError(t, err)
	assert.False(t, allowed, "a stranger does not")

	// The notices are written in-app, to each member of the role, none of whom holds Automations.Read.
	link := "/automations/tasks/" + task.String()
	deliver(list)
	assert.Equal(t, []string{"You have a new task: Pay the supplier " + link}, inApp(ira))
	assert.Equal(t, []string{"You have a new task: Pay the supplier " + link}, inApp(ivan))

	s.AsUser(ira)
	s.POST("/automations/tasks/" + task.String() + "/delegate").HTMX().Form(url.Values{"to": {fmt.Sprint(lev.ID())}}).Expect(t).Status(http.StatusOK)
	list = assigned()
	require.Len(t, list, 2)
	assert.Equal(t, []uint{lev.ID()}, recipients(list[1]), "only the delegate")
	content, err = def.Render(list[1], "en")
	require.NoError(t, err)
	assert.Equal(t, "Ira Tester passed a task to you: Pay the supplier", content.Title)
	allowed, err = def.RecipientGuard(ctx, list[1], lev)
	require.NoError(t, err)
	assert.True(t, allowed, "the delegate reads the task now")

	// After the delegation only the delegate is notified.
	deliver(list[1:])
	assert.Equal(t, []string{"Ira Tester passed a task to you: Pay the supplier " + link}, inApp(lev))
	assert.Len(t, inApp(ira), 1)
	assert.Len(t, inApp(ivan), 1)
}

// A task past its final deadline, still open until the run closes it, takes no decision: the form's
// effect never runs and the page says decisions are no longer taken; it is still open and overdue. A task whose escalation is still pending past its
// due time is overdue: it stays in «Mine» and is listed under «Overdue».
func TestInboxDeadlines(t *testing.T) {
	form := &flakyForm{}
	// Only the test runs the workflow engine: the tasks stay open past their due time.
	s, container := tasksAppWith(t, map[string]any{"extruntime.disablebackground": true}, formComponent{form: form})
	env := s.Env()
	pp := people{t: t, db: env.Pool, tenant: env.Tenant.ID}
	nina, pavel := pp.user("Nina"), pp.user("Pavel")

	final, _ := openTask(t, s, container, humanTaskGraph("Sign before noon", map[string]any{
		"assignees": []any{map[string]any{"kind": "user", "user_id": nina.ID()}}, "timeout": "1s",
		"form": map[string]any{"type": "object", "x-iota-form": "acme.sign"},
	}))
	late, _ := openTask(t, s, container, humanTaskGraph("Approve the late invoice", map[string]any{
		"assignees": []any{map[string]any{"kind": "user", "user_id": nina.ID()}}, "timeout": "1s",
		"escalation": []any{map[string]any{"add": []any{map[string]any{"kind": "user", "user_id": pavel.ID()}}, "extend": "1d"}},
	}))
	time.Sleep(1500 * time.Millisecond) // the engine does not run: both stay open past their due time

	s.AsUser(nina)
	decide(s, final, url.Values{"form": {`{"signature":"s"}`}}).Expect(t).Status(http.StatusOK).Contains(`data-testid="automations-task-closed"`)
	assert.Equal(t, int32(0), form.calls.Load(), "no signature is sent after the final deadline")
	assert.Equal(t, 0, decisions(t, container, final))
	s.GET("/automations/tasks/" + final.String()).Expect(t).Status(http.StatusOK).Contains("The due time passed")

	// Both are open past their due time, so both are counted open and overdue, and both are listed;
	// only the one with a pending escalation takes a decision.
	svc, ok, err := automations.TaskInboxOf(container)
	require.NoError(t, err)
	require.True(t, ok)
	counts, err := svc.Counts(composables.WithTenantID(composables.WithUser(context.Background(), nina), env.Tenant.ID))
	require.NoError(t, err)
	assert.Equal(t, tasks.TaskCounts{Open: 2, Overdue: 2}, counts)
	s.GET("/automations/tasks").Expect(t).Contains("Sign before noon")
	s.GET("/automations/tasks?tab=overdue").Expect(t).Contains("Approve the late invoice").Contains("Sign before noon")
	s.GET("/automations/tasks").Expect(t).Contains("Approve the late invoice").Contains(`data-testid="automations-task-due"`)
	decide(s, late, url.Values{"outcome": {"approve"}}).Expect(t).Status(http.StatusOK)
	assert.Equal(t, 1, decisions(t, container, late), "an overdue task with a pending escalation still takes the decision")
}

// The record a task is about is named by the record itself: its name, searchable, and the link to its
// card only for a reader allowed to open it.
func TestInboxSubjectNames(t *testing.T) {
	s, container := tasksApp(t)
	env := s.Env()
	admin := env.User
	pp := people{t: t, db: env.Pool, tenant: env.Tenant.ID}
	olga, rita, client := pp.user("Olga"), pp.user("Rita"), pp.user("Zarina")
	reviewers := pp.role("Reviewers", olga, rita)
	var perm uuid.UUID
	require.NoError(t, env.Pool.QueryRow(context.Background(), `
		INSERT INTO permissions (id, name, resource, action, modifier) VALUES ($1, 'User.Read', 'user', 'read', 'all')
		ON CONFLICT (name) DO UPDATE SET name = EXCLUDED.name RETURNING id`, uuid.New()).Scan(&perm))
	readers := pp.role("User readers", rita)
	pp.exec(`INSERT INTO role_permissions (role_id, permission_id) VALUES ($1, $2)`, readers, perm)
	_, _ = openTaskAbout(t, s, container, humanTaskGraph("Check the profile", map[string]any{
		"assignees": []any{map[string]any{"kind": "role", "ref": strconv.FormatInt(reviewers, 10)}},
	}), map[string]any{"entity": "core.user", "key": strconv.FormatUint(uint64(client.ID()), 10)})

	s.AsUser(olga)
	body := s.GET("/automations/tasks?q=zarina").Expect(t).Status(http.StatusOK).Contains("Check the profile").Body()
	assert.Contains(t, body, "Zarina Tester", "the record's own name")
	assert.NotContains(t, body, fmt.Sprintf(`href="/users/%d"`, client.ID()), "no link without User.Read")
	s.AsUser(rita)
	s.GET("/automations/tasks").Expect(t).Contains(fmt.Sprintf(`href="/users/%d"`, client.ID()))

	// An employee, named by the HRM module: by name, linked for a reader with Employee.Read.
	var employee int64
	require.NoError(t, env.Pool.QueryRow(context.Background(), `
		INSERT INTO employees (first_name, last_name, email, salary, hourly_rate, coefficient, tenant_id)
		VALUES ('Timur', 'Karimov', $1, 0, 0, 1, $2) RETURNING id`, "timur-"+uuid.NewString()[:8]+"@example.test", env.Tenant.ID).Scan(&employee))
	require.NoError(t, env.Pool.QueryRow(context.Background(), `
		INSERT INTO permissions (id, name, resource, action, modifier) VALUES ($1, 'Employee.Read', 'employee', 'read', 'all')
		ON CONFLICT (name) DO UPDATE SET name = EXCLUDED.name RETURNING id`, uuid.New()).Scan(&perm))
	pp.exec(`INSERT INTO role_permissions (role_id, permission_id) VALUES ($1, $2)`, readers, perm)
	s.AsUser(admin)
	_, _ = openTaskAbout(t, s, container, humanTaskGraph("Check the employee", map[string]any{
		"assignees": []any{map[string]any{"kind": "role", "ref": strconv.FormatInt(reviewers, 10)}},
	}), map[string]any{"entity": "hrm.employee", "key": strconv.FormatInt(employee, 10)})
	s.AsUser(olga)
	body = s.GET("/automations/tasks?q=karimov").Expect(t).Status(http.StatusOK).Contains("Check the employee").Contains("Timur Karimov").Body()
	assert.NotContains(t, body, fmt.Sprintf(`href="/hrm/employees/%d"`, employee))
	s.AsUser(rita)
	s.GET("/automations/tasks?q=karimov").Expect(t).Contains(fmt.Sprintf(`href="/hrm/employees/%d"`, employee))
}

// The module of a record finds the live run about it and cancels it: the run's open task closes as
// cancelled. A run about another record stays.
// Falsely green if: the lookup ignored the subject — the second run would be cancelled too and Live of
// the first subject would list two runs.
func TestSubjectRunsOfCancelsTheRunsOfARecord(t *testing.T) {
	s, container := tasksApp(t)
	env := s.Env()
	pp := people{t: t, db: env.Pool, tenant: env.Tenant.ID}
	anna, boris := pp.user("Anna"), pp.user("Boris")
	g := humanTaskGraph("Sign the contract", map[string]any{
		"assignees": []any{map[string]any{"kind": "user", "user_id": anna.ID()}},
	})
	about := engine.SubjectRef{Entity: "core.user", Key: strconv.FormatUint(uint64(anna.ID()), 10)}
	task, run := openTaskAbout(t, s, container, g, map[string]any{"entity": about.Entity, "key": about.Key})
	_, otherRun := openTaskAbout(t, s, container, g, map[string]any{"entity": "core.user", "key": strconv.FormatUint(uint64(boris.ID()), 10)})

	runs, ok, err := automations.SubjectRunsOf(container)
	require.NoError(t, err)
	require.True(t, ok)
	ctx := context.Background()
	live, err := runs.Live(ctx, env.Tenant.ID, about)
	require.NoError(t, err)
	require.Len(t, live, 1)
	assert.Equal(t, run, live[0].RunID)

	n, err := runs.Cancel(ctx, env.Tenant.ID, about)
	require.NoError(t, err)
	assert.Equal(t, 1, n)
	waitFor(t, container, func(db *pgxpool.Pool) bool {
		var status string
		return db.QueryRow(ctx, `SELECT status FROM extruntime.human_tasks WHERE id = $1`, task).Scan(&status) == nil && status == "cancelled"
	}, "the task of the cancelled run closes")
	live, err = runs.Live(ctx, env.Tenant.ID, about)
	require.NoError(t, err)
	assert.Empty(t, live)
	assert.NotEqual(t, "cancelled", runStatus(t, container, otherRun), "the run about another record stays")

	_, err = runs.Live(ctx, env.Tenant.ID, engine.SubjectRef{})
	assert.Equal(t, serrors.Invalid, serrors.CodeOf(err), "a subject of nothing is refused")
}

// Another module acts for a person through EngineOf: the service applies the person's rights — the
// author reads the graph the person created, and a person without Automations.Read is refused.
// Falsely green if: EngineOf returned a service that skips authorization — the refusal below holds.
func TestEngineOfActsForAPerson(t *testing.T) {
	s, container := tasksApp(t)
	env := s.Env()
	eng, ok, err := automations.EngineOf(container)
	require.NoError(t, err)
	require.True(t, ok)
	ctx := context.Background()
	author := engine.Actor{TenantID: env.Tenant.ID, UserID: int64(env.User.ID()), Origin: engine.OriginHuman, Subject: spi.Subject{
		TenantID: env.Tenant.ID, UserID: int64(env.User.ID()), Permissions: []string{engine.PermRead, engine.PermAuthor},
	}}
	raw, err := json.Marshal(humanTaskGraph("Approve", map[string]any{
		"assignees": []any{map[string]any{"kind": "user", "user_id": env.User.ID()}},
	}))
	require.NoError(t, err)
	_, err = eng.Authoring.Validate(ctx, author, raw)
	require.NoError(t, err)
	created, _, err := eng.Service.Create(ctx, author, engine.CreateInput{Graph: raw})
	require.NoError(t, err)
	got, err := eng.Service.Get(ctx, author, created.ID)
	require.NoError(t, err)
	assert.Equal(t, created.ID, got.ID)

	nobody := author
	nobody.Subject.Permissions = nil
	_, err = eng.Service.Get(ctx, nobody, created.ID)
	require.ErrorIs(t, err, engine.ErrForbidden)
}
