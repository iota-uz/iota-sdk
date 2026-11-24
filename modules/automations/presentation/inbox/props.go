package inbox

import (
	"context"
	"encoding/json"
	"fmt"
	"html/template"
	"math"
	"strings"
	"time"

	"github.com/a-h/templ"
	"github.com/google/uuid"
	"github.com/iota-uz/go-i18n/v2/i18n"

	"github.com/iota-uz/iota-sdk/modules/automations/tasks"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/intl"
)

// screenID is the element both screens of the inbox (the list and a task) render in: a decision swaps
// the next task or the list into it.
const screenID = "automations-tasks"

// ListProps are the list of a tab.
type ListProps struct {
	Tab, Query string
	Items      []tasks.TaskView
	NextCursor string
	Counts     tasks.TaskCounts
	// Failed: the list could not be read.
	Failed bool
	// Done: the list follows a decision that left the tab empty.
	Done bool
	base *Controller
}

// TaskProps are one task.
type TaskProps struct {
	Task         tasks.TaskView
	Tab, Query   string
	Instructions template.HTML
	Fields       []Field
	CustomForm   templ.Component
	RunURL       string
	CanReadRun   bool
	// Comment is the comment entered before a refused decision.
	Comment string
	// Error is the reason a decision was refused, shown next to the buttons; ErrorOutcome the button.
	Error, ErrorOutcome string
	// Notice "closed": the task closed before the decision.
	Notice string

	DelegateOpen    bool
	DelegateComment string
	DelegateError   string
	base            *Controller
}

// MissingProps are the screen of a task the user cannot open.
type MissingProps struct {
	Tab    string
	Failed bool
	Retry  string
	base   *Controller
}

// t localizes a key of the module with template data; a count picks the plural form.
func t(ctx context.Context, key string, data map[string]any, count ...int) string {
	l, ok := intl.UseLocalizer(ctx)
	if !ok {
		return key
	}
	cfg := &i18n.LocalizeConfig{MessageID: key, TemplateData: data}
	if len(count) > 0 {
		cfg.PluralCount = count[0]
		if data == nil {
			cfg.TemplateData = map[string]any{"Count": count[0]}
		}
	}
	s, err := l.Localize(cfg)
	if err != nil {
		return key
	}
	return s
}

func tr(ctx context.Context, key string) string { return t(ctx, "Automations.Tasks."+key, nil) }

func trf(ctx context.Context, key string, data map[string]any) string {
	return t(ctx, "Automations.Tasks."+key, data)
}

func trn(ctx context.Context, key string, n int) string {
	return t(ctx, "Automations.Tasks."+key, map[string]any{"Count": n}, n)
}

func location(ctx context.Context) *time.Location {
	if loc, ok := composables.UseLocation(ctx); ok && loc != nil {
		return loc
	}
	return time.Local
}

// dateTime is a moment as the reader's locale writes it, in the application's time zone.
func dateTime(ctx context.Context, at time.Time) string {
	at = at.In(location(ctx))
	months := strings.Split(tr(ctx, "Time.Months"), ",")
	month := at.Format("Jan")
	if len(months) == 12 {
		month = strings.TrimSpace(months[at.Month()-1])
	}
	day := fmt.Sprintf("%d %s", at.Day(), month)
	if now := time.Now().In(location(ctx)); now.Year() != at.Year() {
		day += fmt.Sprintf(" %d", at.Year())
	}
	return day + ", " + at.Format("15:04")
}

// relative is the distance to a moment in words: "in 2 h", "3 days ago".
func relative(ctx context.Context, at time.Time) string {
	d := time.Until(at)
	past := d < 0
	if past {
		d = -d
	}
	var phrase string
	switch {
	case d < time.Minute:
		return tr(ctx, "Time.Now")
	case d < time.Hour:
		phrase = trn(ctx, "Time.Minutes", int(math.Round(d.Minutes())))
	case d < 48*time.Hour:
		phrase = trn(ctx, "Time.Hours", int(math.Round(d.Hours())))
	default:
		phrase = trn(ctx, "Time.Days", int(math.Round(d.Hours()/24)))
	}
	if past {
		return trf(ctx, "Time.Ago", map[string]any{"Span": phrase})
	}
	return trf(ctx, "Time.In", map[string]any{"Span": phrase})
}

func (p *ListProps) tabURL(tab string) string { return p.base.listURL(tab, p.Query, "") }
func (p *ListProps) moreURL() string          { return p.base.listURL(p.Tab, p.Query, p.NextCursor) }
func (p *ListProps) resetURL() string         { return p.base.listURL(p.Tab, "", "") }
func (p *ListProps) path() string             { return p.base.cfg.Path }
func (p *ListProps) open(id uuid.UUID) string { return p.base.taskURL(id, p.Tab, p.Query) }
func (p *TaskProps) backURL() string          { return p.base.listURL(p.Tab, p.Query, "") }
func (p *MissingProps) backURL() string       { return p.base.listURL(p.Tab, "", "") }
func (p *TaskProps) delegatesURL() string {
	return tasks.TaskURL(p.base.cfg.Path, p.Task.ID.String()) + "/delegates"
}

// jsString is a JavaScript string literal of s for an Alpine expression.
func jsString(s string) string {
	b, err := json.Marshal(s)
	if err != nil {
		return "''"
	}
	return string(b)
}

// jsList is a JavaScript array literal of strings.
func jsList(list []string) string {
	b, err := json.Marshal(list)
	if err != nil {
		return "[]"
	}
	return string(b)
}

// postURL is the URL a form of the task posts to (the tab and search ride along for the next task).
func (p *TaskProps) postURL(verb string) string {
	u := tasks.TaskURL(p.base.cfg.Path, p.Task.ID.String()) + "/" + verb
	q := p.base.taskURL(p.Task.ID, p.Tab, p.Query)
	if i := strings.IndexByte(q, '?'); i >= 0 {
		u += q[i:]
	}
	return u
}

// commentRequiredFor are the labels of the buttons that need a comment.
func (p *TaskProps) commentRequiredFor() []string {
	var out []string
	for _, o := range p.Task.Outcomes {
		if o.CommentRequired {
			out = append(out, o.Label)
		}
	}
	return out
}

func (p *TaskProps) allCommentRequired() bool {
	if len(p.Task.Outcomes) == 0 {
		return false
	}
	for _, o := range p.Task.Outcomes {
		if !o.CommentRequired {
			return false
		}
	}
	return true
}

// readOnly: the screen shows the task without its form.
func (p *TaskProps) readOnly() bool {
	return !p.Task.CanDecide || p.Notice == "closed"
}

// via is where the task came from in words.
func via(ctx context.Context, v tasks.TaskView) string {
	switch v.AssignedVia {
	case tasks.ViaDelegated:
		if v.DelegatedBy != nil {
			return trf(ctx, "Via.DelegatedBy", map[string]any{"Name": v.DelegatedBy.Name})
		}
		return tr(ctx, "Via.Delegated")
	case tasks.ViaRole:
		return tr(ctx, "Via.Role")
	case tasks.ViaGroup:
		return tr(ctx, "Via.Group")
	case tasks.ViaPermission:
		return tr(ctx, "Via.Permission")
	}
	return tr(ctx, "Via.User")
}

func quorum(ctx context.Context, q tasks.Quorum) string {
	return trf(ctx, "Quorum", map[string]any{"Decided": q.Decided, "Required": q.Required})
}

// closedLine is how a closed task ended.
func closedLine(ctx context.Context, v tasks.TaskView) string {
	switch v.State {
	case tasks.StateDecided:
		data := map[string]any{"Outcome": v.OutcomeLabel(v.Outcome), "Name": "", "At": ""}
		if v.ClosedBy != nil {
			data["Name"] = v.ClosedBy.Name
		}
		if v.ClosedAt != nil {
			data["At"] = dateTime(ctx, *v.ClosedAt)
		}
		if v.ClosedBy != nil && v.ClosedAt != nil {
			return trf(ctx, "Closed.DecidedBy", data)
		}
		return trf(ctx, "Closed.Decided", data)
	case tasks.StateExpired:
		return tr(ctx, "Closed.Expired")
	case tasks.StateCancelled:
		return tr(ctx, "Closed.Cancelled")
	}
	return ""
}

func myDecisionLine(ctx context.Context, v tasks.TaskView) string {
	if v.MyDecision == nil {
		return ""
	}
	return trf(ctx, "MyDecision", map[string]any{"Outcome": v.OutcomeLabel(*v.MyDecision)})
}

func badgeText(n int) string {
	if n > 99 {
		return "99+"
	}
	return fmt.Sprint(n)
}

func isoTime(at time.Time) string { return at.UTC().Format(time.RFC3339) }

func outcomeClass(style string) string {
	switch style {
	case tasks.StylePrimary:
		return "btn btn-primary btn-normal"
	case tasks.StyleDanger:
		return "btn btn-danger btn-normal"
	}
	return "btn btn-secondary btn-normal"
}
