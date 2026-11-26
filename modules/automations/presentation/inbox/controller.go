// Package inbox is the page «My tasks» of the automations section: /automations/tasks lists the
// person's open, overdue and decided tasks, /automations/tasks/{id} shows one task with its decision
// form. HTMX and templ over tasks.TaskInbox; a decision and a delegation take only an interactive
// session of a person.
package inbox

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/a-h/templ"
	"github.com/google/uuid"
	"github.com/gorilla/mux"
	icons "github.com/iota-uz/icons/phosphor"

	"github.com/iota-uz/iota-sdk/modules/automations/permissions"
	"github.com/iota-uz/iota-sdk/modules/automations/tasks"
	"github.com/iota-uz/iota-sdk/pkg/application"
	"github.com/iota-uz/iota-sdk/pkg/automation/engine"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/htmx"
	"github.com/iota-uz/iota-sdk/pkg/intl"
	"github.com/iota-uz/iota-sdk/pkg/markdown"
	"github.com/iota-uz/iota-sdk/pkg/middleware"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
)

// ControllerID is the descriptor ID of the inbox.
const ControllerID = "automations.tasks"

// EventTasksChanged is the HTMX event a decision or a delegation triggers: the navigation counter
// reads the counts again.
const EventTasksChanged = "automations:tasks-changed"

// Tabs of the list.
const (
	TabMine    = "mine"
	TabOverdue = "overdue"
	TabDecided = "decided"
)

// Inbox is what the pages call: the TaskInbox and the users a task may be passed to.
type Inbox interface {
	tasks.TaskInbox
	Delegates(ctx context.Context, id uuid.UUID, query string, limit int) ([]tasks.UserOption, error)
	// DisplayName is the name of a user of the tenant ("" when unknown).
	DisplayName(ctx context.Context, id uint) string
}

// Config configures the controller.
type Config struct {
	Inbox Inbox
	Forms tasks.FormLookup
	// Path is the inbox (default tasks.DefaultPath).
	Path string
	// AutomationsPath is the automations section, for the link to a task's run (default /automations).
	AutomationsPath string
	// Origin is who acts in a request (default: human for an interactive browser session).
	Origin func(r *http.Request) engine.Origin
	Logger *slog.Logger
}

// Controller serves the inbox.
type Controller struct {
	cfg Config
	md  markdown.Renderer
	log *slog.Logger
}

var _ application.Controller = (*Controller)(nil)

// NewController builds the controller.
func NewController(cfg Config) (*Controller, error) {
	if cfg.Inbox == nil {
		return nil, serrors.New(serrors.Invalid, "automations inbox: Inbox is required").WithOp("automations.inbox.NewController")
	}
	if cfg.Path == "" {
		cfg.Path = tasks.DefaultPath
	}
	if cfg.AutomationsPath == "" {
		cfg.AutomationsPath = "/automations"
	}
	if cfg.Origin == nil {
		cfg.Origin = func(*http.Request) engine.Origin { return engine.OriginAgent }
	}
	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}
	return &Controller{cfg: cfg, md: markdown.NewRenderer(), log: cfg.Logger}, nil
}

const idPattern = `{id:[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}}`

// Descriptor implements application.Controller.
func (c *Controller) Descriptor() application.ControllerDescriptor {
	p := c.cfg.Path
	page := application.Authenticated()
	return application.Descriptor(ControllerID, 0,
		application.Get(p, page),
		application.Get(p+"/badge", page),
		application.Get(p+"/"+idPattern, page),
		application.Get(p+"/"+idPattern+"/delegates", page),
		application.Post(p+"/"+idPattern+"/decide", page),
		application.Post(p+"/"+idPattern+"/delegate", page),
	).WithNav(c.NavNode())
}

// NavNode is the inbox's entry in the navigation: every user sees their own tasks, with the counter of
// open ones.
func (c *Controller) NavNode() application.NavNode {
	return application.NavNode{
		ID: "automations.tasks", TitleKey: "NavigationLinks.AutomationTasks", Path: c.cfg.Path, Order: 89,
		Icon:     icons.CheckSquareOffset(icons.Props{Size: "24"}),
		Keywords: []string{"tasks", "inbox", "approval", "задачи", "vazifalar"},
		Badge:    templ.ComponentFunc(c.renderBadge),
		// An assignee needs no permission to see their tasks.
		ForEveryone: true,
	}
}

// Register implements application.Controller.
func (c *Controller) Register(r *mux.Router) {
	// Decisions and delegations: an agent or an API token (an MCP credential with no browser session)
	// is refused with HUMAN_REQUIRED before the redirect to the login page a page request gets.
	actions := r.PathPrefix(c.cfg.Path).Methods(http.MethodPost).Subrouter()
	actions.Use(
		middleware.Authorize(),
		c.requireHuman,
		middleware.RedirectNotAuthenticated(),
		middleware.ProvideUser(),
		middleware.ProvideDynamicLogo(),
		middleware.WithPageContext(),
		middleware.NavItems(),
	)
	actions.HandleFunc("/"+idPattern+"/decide", c.decide)
	actions.HandleFunc("/"+idPattern+"/delegate", c.delegate)

	sub := r.PathPrefix(c.cfg.Path).Subrouter()
	sub.Use(
		middleware.Authorize(),
		middleware.RedirectNotAuthenticated(),
		middleware.ProvideUser(),
		middleware.ProvideDynamicLogo(),
		middleware.WithPageContext(),
		middleware.NavItems(),
	)
	sub.HandleFunc("", c.list).Methods(http.MethodGet)
	sub.HandleFunc("/badge", c.badge).Methods(http.MethodGet)
	sub.HandleFunc("/"+idPattern, c.task).Methods(http.MethodGet)
	sub.HandleFunc("/"+idPattern+"/delegates", c.delegates).Methods(http.MethodGet)
}

// requireHuman refuses an action that carries an API credential (Authorization) or an agent's session.
// A request without any credential goes on to the login redirect.
func (c *Controller) requireHuman(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, sessErr := composables.UseSession(r.Context())
		hasCredential := strings.TrimSpace(r.Header.Get("Authorization")) != "" || sessErr == nil
		if hasCredential && c.cfg.Origin(r) != engine.OriginHuman {
			c.humanRequired(w, r)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// ---- list ----

func tabOf(r *http.Request) string {
	switch t := r.URL.Query().Get("tab"); t {
	case TabOverdue, TabDecided:
		return t
	}
	return TabMine
}

func scopeOf(tab string) string {
	switch tab {
	case TabOverdue:
		return tasks.ScopeOverdue
	case TabDecided:
		return tasks.ScopeDecidedByMe
	}
	return tasks.ScopeMine
}

// listURL is the list with a tab, a search and a cursor.
func (c *Controller) listURL(tab, q, cursor string) string {
	v := url.Values{}
	if tab != "" && tab != TabMine {
		v.Set("tab", tab)
	}
	if q != "" {
		v.Set("q", q)
	}
	if cursor != "" {
		v.Set("cursor", cursor)
	}
	if len(v) == 0 {
		return c.cfg.Path
	}
	return c.cfg.Path + "?" + v.Encode()
}

// taskURL is a task opened from a tab of the list.
func (c *Controller) taskURL(id uuid.UUID, tab, q string) string {
	v := url.Values{}
	if tab != "" && tab != TabMine {
		v.Set("tab", tab)
	}
	if q != "" {
		v.Set("q", q)
	}
	u := tasks.TaskURL(c.cfg.Path, id.String())
	if len(v) > 0 {
		u += "?" + v.Encode()
	}
	return u
}

func (c *Controller) list(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	tab, q := tabOf(r), strings.TrimSpace(r.URL.Query().Get("q"))
	cursor := r.URL.Query().Get("cursor")
	props := &ListProps{Tab: tab, Query: q, base: c}
	page, err := c.cfg.Inbox.List(ctx, tasks.TaskFilter{Scope: scopeOf(tab), Search: q, Cursor: cursor})
	switch {
	case errors.Is(err, tasks.ErrTaskNotFound):
		// A user who cannot act has no tasks.
	case err != nil:
		c.log.Warn("automations inbox: list", "error", err)
		props.Failed = true
	default:
		props.Items, props.NextCursor = page.Items, page.NextCursor
	}
	if counts, err := c.cfg.Inbox.Counts(ctx); err == nil {
		props.Counts = counts
	}
	// "Show more" appends the next page to the list; a page that cannot be read says so in its place,
	// with Retry.
	if htmx.IsHxRequest(r) && cursor != "" {
		if props.Failed {
			c.render(w, r, ListMoreFailed(c.listURL(tab, q, cursor)))
			return
		}
		c.render(w, r, ListMore(props))
		return
	}
	if htmx.IsHxRequest(r) && htmx.Target(r) == screenID {
		c.render(w, r, ListScreen(props))
		return
	}
	c.render(w, r, ListPage(props))
}

// ---- one task ----

func idOf(r *http.Request) (uuid.UUID, bool) {
	id, err := uuid.Parse(mux.Vars(r)["id"])
	return id, err == nil
}

func (c *Controller) task(w http.ResponseWriter, r *http.Request) {
	id, ok := idOf(r)
	if !ok {
		c.notFound(w, r)
		return
	}
	v, err := c.cfg.Inbox.Get(r.Context(), id)
	if err != nil {
		if errors.Is(err, tasks.ErrTaskNotFound) {
			c.notFound(w, r)
			return
		}
		c.log.Warn("automations inbox: get", "error", err)
		c.failed(w, r)
		return
	}
	props := c.taskProps(r, v)
	if htmx.IsHxRequest(r) && htmx.Target(r) == screenID {
		c.render(w, r, TaskScreen(props))
		return
	}
	c.render(w, r, TaskPage(props))
}

func (c *Controller) taskProps(r *http.Request, v tasks.TaskView) *TaskProps {
	tab, q := tabOf(r), strings.TrimSpace(r.URL.Query().Get("q"))
	if tab == "" {
		tab = TabMine
	}
	p := &TaskProps{Task: v, Tab: tab, Query: q, base: c, Fields: fieldsOf(v.DecisionSchema)}
	if v.Instructions != "" {
		if html, err := c.md.Render([]byte(v.Instructions)); err == nil {
			p.Instructions = html
		} else {
			p.Instructions = template.HTML(template.HTMLEscapeString(v.Instructions))
		}
	}
	if v.FormKind != "" && !v.FormUnavailable && c.cfg.Forms != nil {
		if form, ok, err := c.cfg.Forms.Form(v.FormKind); err == nil && ok {
			p.CustomForm = form.Render(r.Context(), v)
		}
	}
	p.RunURL = c.cfg.AutomationsPath + "/" + v.AutomationID.String() + "/runs/" + v.RunID.String()
	p.CanReadRun = canReadAutomations(r.Context())
	return p
}

func (c *Controller) notFound(w http.ResponseWriter, r *http.Request) {
	props := &MissingProps{base: c, Tab: tabOf(r)}
	// An HTMX action (a decision of a task the user just lost) shows the screen in place: htmx swaps
	// no 4xx answer, which would leave the click without a word.
	if htmx.IsHxRequest(r) {
		htmx.Retarget(w, "#"+screenID)
		htmx.Reswap(w, "outerHTML")
		htmx.SetTrigger(w, EventTasksChanged, "")
		c.render(w, r, MissingScreen(props))
		return
	}
	w.WriteHeader(http.StatusNotFound)
	c.render(w, r, MissingPage(props))
}

func (c *Controller) failed(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusInternalServerError)
	c.render(w, r, MissingPage(&MissingProps{base: c, Tab: tabOf(r), Failed: true, Retry: r.URL.RequestURI()}))
}

// ---- decide ----

func (c *Controller) decide(w http.ResponseWriter, r *http.Request) {
	// Only a person's interactive session acts on a task: anything else is refused before the task
	// is read.
	if c.cfg.Origin(r) != engine.OriginHuman {
		c.humanRequired(w, r)
		return
	}
	id, ok := idOf(r)
	if !ok {
		c.notFound(w, r)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}
	ctx := tasks.WithOrigin(r.Context(), c.cfg.Origin(r))
	before, err := c.cfg.Inbox.Get(ctx, id)
	if err != nil {
		if errors.Is(err, tasks.ErrTaskNotFound) {
			c.notFound(w, r)
			return
		}
		c.failed(w, r)
		return
	}
	fields, kept, ferr := fieldsFromForm(fieldsOf(before.DecisionSchema), r.PostForm)
	in := tasks.DecideInput{Outcome: r.PostForm.Get("outcome"), Comment: r.PostForm.Get("comment"), Fields: fields}
	if raw := strings.TrimSpace(r.PostForm.Get("form")); raw != "" {
		in.Form = []byte(raw)
	}
	if ferr != nil {
		c.decideFailed(w, r, id, in, kept, fmt.Errorf("%w: fields: %w", tasks.ErrInvalidDecision, ferr))
		return
	}
	if unconfirmed(before, in, r.PostForm.Get("confirm")) {
		c.decideFailed(w, r, id, in, kept, errUnconfirmed)
		return
	}
	err = c.cfg.Inbox.Decide(ctx, id, in)
	if err == nil {
		label := before.OutcomeLabel(in.Outcome)
		if in.Outcome == "" {
			label = before.Title
		}
		c.afterDone(w, r, id, intl.TWithDefault(r.Context(), "Automations.Tasks.Toast.Decided", "Decision recorded"), label)
		return
	}
	c.decideFailed(w, r, id, in, kept, err)
}

// errUnconfirmed: a danger outcome of the default form sent without its confirmation.
var errUnconfirmed = errors.New("danger outcome not confirmed")

// unconfirmed reports a danger outcome of the default decision form that did not come from its
// confirmation panel: the panel sets confirm to the outcome, so a submission without it (Enter in a
// field submitting the hidden button, a stale page) is shown again with the panel open. A custom
// form confirms on its own; a task that is no longer open goes to the service, which says why.
func unconfirmed(v tasks.TaskView, in tasks.DecideInput, confirm string) bool {
	if v.FormKind != "" || v.State != tasks.StateOpen || v.PastDeadline {
		return false
	}
	for _, o := range v.Outcomes {
		if o.Value == in.Outcome {
			return o.Style == tasks.StyleDanger && confirm != o.Value
		}
	}
	return false
}

// afterDone answers a decision or a delegation that went through: a toast, the counters refresh, and
// the next task of the tab opens (the tab itself when none is left).
func (c *Controller) afterDone(w http.ResponseWriter, r *http.Request, id uuid.UUID, title, message string) {
	ctx := r.Context()
	tab, q := tabOf(r), strings.TrimSpace(r.URL.Query().Get("q"))
	if tab == TabDecided {
		tab = TabMine
	}
	var next *tasks.TaskView
	if page, err := c.cfg.Inbox.List(ctx, tasks.TaskFilter{Scope: scopeOf(tab), Search: q, Limit: 2}); err == nil {
		for i := range page.Items {
			if page.Items[i].ID != id {
				next = &page.Items[i]
				break
			}
		}
	}
	target := c.listURL(tab, q, "")
	if next != nil {
		target = c.taskURL(next.ID, tab, q)
	}
	if !htmx.IsHxRequest(r) {
		http.Redirect(w, r, target, http.StatusSeeOther)
		return
	}
	triggerDone(w, title, message)
	htmx.PushURL(w, target)
	htmx.Retarget(w, "#"+screenID)
	htmx.Reswap(w, "outerHTML")
	if next != nil {
		props := c.taskProps(r, *next)
		props.Tab = tab
		c.render(w, r, TaskScreen(props))
		return
	}
	props := &ListProps{Tab: tab, Query: q, base: c, Done: true}
	if page, err := c.cfg.Inbox.List(ctx, tasks.TaskFilter{Scope: scopeOf(tab), Search: q}); err == nil {
		props.Items, props.NextCursor = page.Items, page.NextCursor
	} else {
		props.Failed = true
	}
	if counts, err := c.cfg.Inbox.Counts(ctx); err == nil {
		props.Counts = counts
	}
	c.render(w, r, ListScreen(props))
}

// decideFailed shows the task again with what went wrong: a closed task read only, a refused decision
// with the values entered and the reason next to the buttons.
func (c *Controller) decideFailed(w http.ResponseWriter, r *http.Request, id uuid.UUID, in tasks.DecideInput, kept []Field, err error) {
	ctx := r.Context()
	switch {
	case errors.Is(err, tasks.ErrTaskNotFound):
		c.notFound(w, r)
		return
	case errors.Is(err, tasks.ErrHumanRequired):
		c.humanRequired(w, r)
		return
	}
	v, gerr := c.cfg.Inbox.Get(ctx, id)
	if gerr != nil {
		c.notFound(w, r)
		return
	}
	props := c.taskProps(r, v)
	props.Comment = in.Comment
	if len(kept) > 0 {
		props.Fields = kept
	}
	switch {
	case errors.Is(err, tasks.ErrTaskClosed), errors.Is(err, tasks.ErrAlreadyDecided):
		props.Notice = "closed"
	case errors.Is(err, errUnconfirmed):
		props.Error = intl.TWithDefault(ctx, "Automations.Tasks.Unconfirmed", "Confirm the decision below: it was not sent.")
		props.ErrorOutcome = in.Outcome
		props.Confirm = in.Outcome
	case errors.Is(err, tasks.ErrInvalidDecision):
		props.Error = intl.TWithDefault(ctx, "Automations.Tasks.Errors.Invalid", "Check the decision: a required comment or field is missing or wrong.")
		props.ErrorOutcome = in.Outcome
	case errors.Is(err, tasks.ErrFormUnavailable):
		props.Error = intl.TWithDefault(ctx, "Automations.Tasks.Form.Unavailable", "Form unavailable")
	default:
		var fe *tasks.FormError
		if errors.As(err, &fe) && strings.TrimSpace(fe.Message) != "" {
			props.Error = fe.Message
		} else {
			c.log.Warn("automations inbox: decide", "task_id", id.String(), "error", err)
			props.Error = intl.TWithDefault(ctx, "Automations.Tasks.Errors.DecideFailed", "Could not record the decision. Try again in a minute.")
		}
		props.ErrorOutcome = in.Outcome
	}
	if htmx.IsHxRequest(r) {
		htmx.Retarget(w, "#"+screenID)
		htmx.Reswap(w, "outerHTML")
		htmx.SetTrigger(w, EventTasksChanged, "")
		c.render(w, r, TaskScreen(props))
		return
	}
	w.WriteHeader(http.StatusUnprocessableEntity)
	c.render(w, r, TaskPage(props))
}

// ---- delegate ----

func (c *Controller) delegate(w http.ResponseWriter, r *http.Request) {
	// Only a person's interactive session acts on a task: anything else is refused before the task
	// is read.
	if c.cfg.Origin(r) != engine.OriginHuman {
		c.humanRequired(w, r)
		return
	}
	id, ok := idOf(r)
	if !ok {
		c.notFound(w, r)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}
	ctx := tasks.WithOrigin(r.Context(), c.cfg.Origin(r))
	to, _ := strconv.ParseUint(r.PostForm.Get("to"), 10, 32)
	comment := r.PostForm.Get("delegate_comment")
	err := c.cfg.Inbox.Delegate(ctx, id, uint(to), comment)
	if err == nil {
		name := c.cfg.Inbox.DisplayName(ctx, uint(to))
		title := intl.TWithDefault(r.Context(), "Automations.Tasks.Toast.Delegated", "Passed on")
		if name != "" {
			title = trf(r.Context(), "Toast.DelegatedTo", map[string]any{"Name": name})
		}
		c.afterDone(w, r, id, title, "")
		return
	}
	switch {
	case errors.Is(err, tasks.ErrTaskNotFound):
		c.notFound(w, r)
		return
	case errors.Is(err, tasks.ErrHumanRequired):
		c.humanRequired(w, r)
		return
	}
	v, gerr := c.cfg.Inbox.Get(r.Context(), id)
	if gerr != nil {
		c.notFound(w, r)
		return
	}
	props := c.taskProps(r, v)
	props.DelegateOpen, props.DelegateComment = true, comment
	switch {
	case errors.Is(err, tasks.ErrTaskClosed), errors.Is(err, tasks.ErrAlreadyDecided):
		props.Notice = "closed"
	case errors.Is(err, tasks.ErrDelegationDenied):
		props.DelegateError = intl.TWithDefault(r.Context(), "Automations.Tasks.Errors.DelegationDenied", "This task cannot be passed to that person. Pick another colleague.")
	default:
		c.log.Warn("automations inbox: delegate", "task_id", id.String(), "error", err)
		props.DelegateError = intl.TWithDefault(r.Context(), "Automations.Tasks.Errors.DelegateFailed", "Could not pass the task on. Try again in a minute.")
	}
	if htmx.IsHxRequest(r) {
		htmx.Retarget(w, "#"+screenID)
		htmx.Reswap(w, "outerHTML")
		c.render(w, r, TaskScreen(props))
		return
	}
	w.WriteHeader(http.StatusUnprocessableEntity)
	c.render(w, r, TaskPage(props))
}

func (c *Controller) delegates(w http.ResponseWriter, r *http.Request) {
	id, ok := idOf(r)
	if !ok {
		http.NotFound(w, r)
		return
	}
	opts, err := c.cfg.Inbox.Delegates(r.Context(), id, r.URL.Query().Get("q"), 20)
	if err != nil {
		opts = nil
	}
	c.render(w, r, DelegateOptions(strings.TrimSpace(r.URL.Query().Get("q")), opts))
}

// ---- badge ----

// badge answers the counter's refresh: nothing (204, no swap and nothing announced) while the counter
// shown (n, its badgeToken: the count and whether it is red) is still right, else the new counter.
func (c *Controller) badge(w http.ResponseWriter, r *http.Request) {
	counts, err := c.cfg.Inbox.Counts(r.Context())
	if err != nil {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if r.URL.Query().Get("n") == badgeToken(counts) {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	c.render(w, r, BadgeCount(counts))
}

// renderBadge renders the counter with the page; a slow count never holds the page: it renders
// empty and the next refresh fills it.
func (c *Controller) renderBadge(ctx context.Context, w io.Writer) error {
	cctx, cancel := context.WithTimeout(ctx, 500*time.Millisecond)
	defer cancel()
	counts, err := c.cfg.Inbox.Counts(cctx)
	if err != nil {
		counts = tasks.TaskCounts{}
	}
	return Badge(c.cfg.Path+"/badge", counts).Render(ctx, w)
}

func (c *Controller) render(w http.ResponseWriter, r *http.Request, comp templ.Component) {
	if w.Header().Get("Content-Type") == "" {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
	}
	if err := comp.Render(r.Context(), w); err != nil {
		c.log.Error("automations inbox: render", "error", err)
	}
}

// triggerDone sets the one HX-Trigger of a done action: the success toast and EventTasksChanged (two
// headers would be joined into a value htmx cannot parse).
func triggerDone(w http.ResponseWriter, title, message string) {
	events := map[string]any{
		"notify":          map[string]any{"variant": htmx.ToastVariantSuccess, "title": title, "message": message},
		EventTasksChanged: nil,
	}
	b, err := json.Marshal(events)
	if err != nil {
		w.Header().Set("Hx-Trigger", EventTasksChanged)
		return
	}
	w.Header().Set("Hx-Trigger", asciiJSON(string(b)))
}

// asciiJSON escapes the non-ASCII characters of JSON text (\uXXXX), as header values must be.
func asciiJSON(s string) string {
	var b strings.Builder
	for _, r := range s {
		switch {
		case r < 0x80:
			b.WriteRune(r)
		case r <= 0xFFFF:
			fmt.Fprintf(&b, "\\u%04x", r)
		default:
			r -= 0x10000
			fmt.Fprintf(&b, "\\u%04x\\u%04x", 0xD800+(r>>10), 0xDC00+(r&0x3FF))
		}
	}
	return b.String()
}

// canReadAutomations reports whether the user of ctx holds Automations.Read (the run of a task is
// linked for them).
func canReadAutomations(ctx context.Context) bool {
	return composables.CanUserStrict(ctx, permissions.AutomationsRead) == nil
}

// CodeHumanRequired is the machine code of a decision or a delegation that did not come from a
// person's interactive session (an agent, an API token), as the automations API answers it.
const CodeHumanRequired = "HUMAN_REQUIRED"

// humanRequired refuses an action of an agent: 403 with {"error":{"code":"HUMAN_REQUIRED","message"}}
// and the code in the X-Error-Code header.
func (c *Controller) humanRequired(w http.ResponseWriter, r *http.Request) {
	body, err := json.Marshal(map[string]any{"error": map[string]string{
		"code":    CodeHumanRequired,
		"message": trOr(r.Context(), "Errors.HumanRequired", "Only a person decides a task, in their own session."),
	}})
	if err != nil {
		body = []byte(`{"error":{"code":"` + CodeHumanRequired + `"}}`)
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("X-Error-Code", CodeHumanRequired)
	w.WriteHeader(http.StatusForbidden)
	_, _ = w.Write(body)
}
