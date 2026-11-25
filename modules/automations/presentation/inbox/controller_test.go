package inbox

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/iota-uz/iota-sdk/modules/automations/tasks"
	"github.com/iota-uz/iota-sdk/pkg/automation/engine"
)

// fakeInbox answers what a test sets; any other call panics on the nil embedded Inbox.
type fakeInbox struct {
	Inbox
	counts  tasks.TaskCounts
	view    tasks.TaskView
	decided []tasks.DecideInput
}

func (f *fakeInbox) Counts(context.Context) (tasks.TaskCounts, error) { return f.counts, nil }
func (f *fakeInbox) Get(context.Context, uuid.UUID) (tasks.TaskView, error) {
	return f.view, nil
}
func (f *fakeInbox) Decide(_ context.Context, _ uuid.UUID, in tasks.DecideInput) error {
	f.decided = append(f.decided, in)
	return nil
}
func (f *fakeInbox) List(context.Context, tasks.TaskFilter) (tasks.TaskPage, error) {
	return tasks.TaskPage{}, nil
}

func newTestController(t *testing.T, in *fakeInbox) *Controller {
	t.Helper()
	c, err := NewController(Config{Inbox: in, Origin: func(*http.Request) engine.Origin { return engine.OriginHuman }})
	require.NoError(t, err)
	return c
}

// The counter answers 204 only while what it shows is still right: its count and its colour (an
// overdue task turns it red while the count stays).
func TestBadgeRefreshesWhenOverdueChanges(t *testing.T) {
	t.Parallel()
	in := &fakeInbox{counts: tasks.TaskCounts{Open: 1}}
	c := newTestController(t, in)
	get := func(n string) *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		c.badge(w, httptest.NewRequest(http.MethodGet, "/automations/tasks/badge?n="+url.QueryEscape(n), nil))
		return w
	}
	assert.Equal(t, http.StatusNoContent, get("1").Code)

	in.counts.Overdue = 1
	w := get("1")
	require.Equal(t, http.StatusOK, w.Code, "the only open task fell overdue: the counter repaints")
	assert.Contains(t, w.Body.String(), "bg-red-500")
	assert.Contains(t, w.Body.String(), `value="1!"`)
	assert.Equal(t, http.StatusNoContent, get("1!").Code)

	in.counts = tasks.TaskCounts{Open: 2, Overdue: 1}
	assert.Equal(t, http.StatusOK, get("1!").Code)
	in.counts = tasks.TaskCounts{}
	assert.Equal(t, http.StatusOK, get("2!").Code)
	assert.Equal(t, http.StatusNoContent, get("0").Code)
}

func dangerTask() tasks.TaskView {
	return tasks.TaskView{
		ID: uuid.New(), Title: "Pay", State: tasks.StateOpen, CanDecide: true,
		Outcomes: []tasks.OutcomeOption{
			{Value: "approve", Label: "Approve", Style: tasks.StylePrimary},
			{Value: "reject", Label: "Reject", Style: tasks.StyleDanger},
		},
	}
}

func TestUnconfirmedDangerOutcome(t *testing.T) {
	t.Parallel()
	v := dangerTask()
	assert.True(t, unconfirmed(v, tasks.DecideInput{Outcome: "reject"}, ""))
	assert.True(t, unconfirmed(v, tasks.DecideInput{Outcome: "reject"}, "approve"))
	assert.False(t, unconfirmed(v, tasks.DecideInput{Outcome: "reject"}, "reject"))
	assert.False(t, unconfirmed(v, tasks.DecideInput{Outcome: "approve"}, ""), "a normal outcome needs no confirmation")
	assert.False(t, unconfirmed(v, tasks.DecideInput{Outcome: "nope"}, ""), "an unknown outcome is the service's to refuse")
	assert.True(t, unconfirmed(v, tasks.DecideInput{Outcome: "reject", Form: []byte(`{}`)}, ""), "a form parameter does not confirm a default-form task")
	custom := v
	custom.FormKind = "signature"
	assert.False(t, unconfirmed(custom, tasks.DecideInput{Outcome: "reject"}, ""), "a custom form confirms on its own")
	closed := v
	closed.State = tasks.StateDecided
	assert.False(t, unconfirmed(closed, tasks.DecideInput{Outcome: "reject"}, ""), "a closed task goes to the service")
}

// A danger outcome sent without its confirmation (Enter in a field) records nothing: the task shows
// again with the confirmation open; sent from the panel it is recorded.
func TestDecideRefusesAnUnconfirmedDangerOutcome(t *testing.T) {
	t.Parallel()
	in := &fakeInbox{view: dangerTask()}
	c := newTestController(t, in)
	post := func(form url.Values) *httptest.ResponseRecorder {
		r := httptest.NewRequest(http.MethodPost, "/automations/tasks/"+in.view.ID.String()+"/decide", strings.NewReader(form.Encode()))
		r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		r.Header.Set("Hx-Request", "true")
		r = mux.SetURLVars(r, map[string]string{"id": in.view.ID.String()})
		w := httptest.NewRecorder()
		c.decide(w, r)
		return w
	}
	w := post(url.Values{"outcome": {"reject"}, "comment": {"no"}})
	assert.Empty(t, in.decided, "nothing is recorded")
	body := w.Body.String()
	assert.Contains(t, body, `data-testid="automations-task-error"`)
	assert.Contains(t, body, `confirm: &#34;reject&#34;`, "the confirmation panel opens")
	assert.Contains(t, body, ">no</textarea>", "the comment stays")

	post(url.Values{"outcome": {"reject"}, "confirm": {"reject"}})
	require.Len(t, in.decided, 1)
	assert.Equal(t, "reject", in.decided[0].Outcome)
	post(url.Values{"outcome": {"approve"}})
	require.Len(t, in.decided, 2)
}

// A number JSON cannot write is refused as an invalid decision, nothing is recorded and what was
// typed stays.
func TestDecideRefusesANonFiniteNumber(t *testing.T) {
	t.Parallel()
	v := dangerTask()
	v.DecisionSchema = json.RawMessage(`{"type":"object","properties":{"amount":{"type":"number"}}}`)
	in := &fakeInbox{view: v}
	c := newTestController(t, in)
	form := url.Values{"outcome": {"approve"}, "field.amount": {"NaN"}}
	r := httptest.NewRequest(http.MethodPost, "/automations/tasks/"+v.ID.String()+"/decide", strings.NewReader(form.Encode()))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	r.Header.Set("Hx-Request", "true")
	r = mux.SetURLVars(r, map[string]string{"id": v.ID.String()})
	w := httptest.NewRecorder()
	c.decide(w, r)
	assert.Empty(t, in.decided)
	assert.Contains(t, w.Body.String(), `data-testid="automations-task-error"`)
	assert.Contains(t, w.Body.String(), `value="NaN"`)
}

func render(t *testing.T, p *TaskProps) string {
	t.Helper()
	var buf bytes.Buffer
	require.NoError(t, TaskScreen(p).Render(context.Background(), &buf))
	return buf.String()
}

// The default form: Enter in a one-line field submits nothing, the confirmation rides along, a danger
// outcome is known to the guard; an optional boolean is a choice of unset, yes and no.
func TestDecisionFormMarkup(t *testing.T) {
	t.Parallel()
	in := &fakeInbox{}
	c := newTestController(t, in)
	v := dangerTask()
	v.AllowDelegation = true
	v.DecisionSchema = json.RawMessage(`{"type":"object","required":["agree"],"properties":{"urgent":{"type":"boolean"},"agree":{"type":"boolean"}}}`)
	p := c.taskProps(httptest.NewRequest(http.MethodGet, "/", nil), v)
	html := render(t, p)
	assert.Contains(t, html, `name="confirm"`)
	assert.Contains(t, html, `danger: [&#34;reject&#34;]`)
	assert.Contains(t, html, `@keydown.enter="if ($event.target.tagName === &#39;INPUT&#39;`)
	assert.Contains(t, html, `<select id="automations-task-field-urgent" name="field.urgent"`)
	assert.Contains(t, html, `<option value="false"`)
	assert.Contains(t, html, `type="checkbox" name="field.agree"`)
	assert.Contains(t, html, `data-testid="automations-task-copy-id"`)
	assert.Contains(t, html, `data-testid="automations-task-id-manual"`)
	assert.Contains(t, html, `data-retry-focus`, "the delegate button tells Retry what to ask for")

	v.DecisionSchema = json.RawMessage(`{"type":"object","allOf":[{"required":["reason"]}]}`)
	html = render(t, c.taskProps(httptest.NewRequest(http.MethodGet, "/", nil), v))
	assert.Contains(t, html, `data-testid="automations-task-field-document"`)
	assert.Contains(t, html, `name="fields"`)
}
