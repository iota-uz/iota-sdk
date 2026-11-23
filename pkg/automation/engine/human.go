package engine

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/iota-uz/iota-sdk/pkg/automation/catalog"
	"github.com/iota-uz/iota-sdk/pkg/automation/expr"
	"github.com/iota-uz/iota-sdk/pkg/automation/graph"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/events"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/manifest"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/native"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/spi"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/workflow"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
)

const opHuman = serrors.Op("automation.engine.human")

// HumanNodeType is the node type of a human task.
const HumanNodeType = "human"

// HumanParams are the params of a human node: a task of #1327 whose decision buttons are the node's
// outcomes (and timeout).
type HumanParams struct {
	Title, Body   string                    // templates
	Assignees     []workflow.TaskAssignee   // user | role | group | permission; 1..20
	Required      int                       // quorum n-of-N, default 1
	Buttons       []HumanButton             // each button is an outcome → TaskSpec.Outcomes
	Veto          []string                  // outcomes that close the task on the first decision
	Form          json.RawMessage           // optional JSON Schema of extra decision fields
	Timeout       string                    // duration, ≤ WaitMax
	Escalation    []workflow.EscalationStep // ≤ 5, #1327
	AllowDelegate bool
}

// HumanButton is one decision button of a human node.
type HumanButton struct {
	Outcome, Label string
	Style          string // primary|danger|default
	RequireComment bool
}

const humanParamsSchema = `{
  "type": "object",
  "properties": {
    "title": {"type": "string", "minLength": 1, "maxLength": 500},
    "body": {"type": "string", "maxLength": 10000},
    "assignees": {
      "type": "array", "minItems": 1, "maxItems": 20,
      "items": {
        "type": "object",
        "properties": {
          "kind": {"enum": ["user", "role", "group", "permission"]},
          "user_id": {"type": "integer", "minimum": 1},
          "ref": {"type": "string", "minLength": 1, "maxLength": 200}
        },
        "required": ["kind"],
        "additionalProperties": false
      }
    },
    "required": {"type": "integer", "minimum": 1, "maximum": 100},
    "buttons": {
      "type": "array", "minItems": 1, "maxItems": 20,
      "items": {
        "type": "object",
        "properties": {
          "outcome": {"type": "string", "pattern": "^[a-z][a-z0-9_]{0,39}$"},
          "label": {"type": "string", "minLength": 1, "maxLength": 80},
          "style": {"enum": ["primary", "danger", "default"]},
          "require_comment": {"type": "boolean"}
        },
        "required": ["outcome", "label"],
        "additionalProperties": false
      }
    },
    "veto": {"type": "array", "maxItems": 20, "items": {"type": "string"}},
    "form": {"type": "object", "description": "JSON Schema of extra decision fields"},
    "timeout": {"type": "string", "pattern": "^[1-9][0-9]{0,5}(s|m|h|d)$"},
    "escalation": {
      "type": "array", "maxItems": 5,
      "items": {
        "type": "object",
        "properties": {
          "add": {"type": "array", "maxItems": 20, "items": {"$ref": "#/properties/assignees/items"}},
          "extend": {"type": "string", "pattern": "^[1-9][0-9]{0,5}(s|m|h|d)$"}
        },
        "additionalProperties": false
      }
    },
    "allow_delegate": {"type": "boolean"}
  },
  "required": ["title", "assignees", "buttons"],
  "additionalProperties": false
}`

// humanNode is the human node type: behavior call routed to the TaskBridge (HumanNode).
type humanNode struct{}

// NewHumanNode is the node type "human" to register in the catalog: a task with a quorum, delegation and
// escalation; its buttons are its outcomes, and timeout when it has a timeout.
func NewHumanNode() catalog.NodeType { return humanNode{} }

func (humanNode) Human() bool                       { return true }
func (humanNode) Requires() []catalog.ConnectorKind { return nil }

func (humanNode) Spec() graph.NodeSpec {
	return graph.NodeSpec{
		Type: HumanNodeType, Title: "Human task", Behavior: graph.BehaviorCall, DryRun: graph.DryRunCapture,
		Description:  "Asks people to decide: the first decisions that reach the quorum choose the outcome.",
		Params:       graph.MustSchemaParams(humanParamsSchema),
		OutcomesFrom: &graph.ParamRef{Param: "buttons", Field: "outcome"},
		Optional:     &graph.OptionalOutcome{Param: "timeout", Outcome: CodeTimeout},
		Templates:    []string{"title", "body"},
		Output: expr.Fields{
			{Name: "outcome", Field: expr.StringField},
			{Name: "decisions", Field: expr.ListOf(expr.ObjectOf(expr.Fields{
				{Name: "user_id", Field: expr.IntField},
				{Name: "outcome", Field: expr.StringField},
				{Name: "comment", Field: expr.StringField},
				{Name: "fields", Field: expr.MapOf(expr.DynField)},
				{Name: "at", Field: expr.StringField},
			}))},
			{Name: "delegated_from", Field: expr.OptionalOf(expr.IntField)},
		},
	}
}

// humanBridge is the engine's TaskBridge and TaskResolver of human nodes.
type humanBridge struct{ waitMax time.Duration }

// NewHumanBridge is the TaskBridge of the human node (Config.Tasks); waitMax bounds its timeout.
func NewHumanBridge(waitMax time.Duration) TaskBridge {
	if waitMax <= 0 {
		waitMax = 30 * 24 * time.Hour
	}
	return humanBridge{waitMax: waitMax}
}

var (
	_ TaskBridge   = humanBridge{}
	_ TaskResolver = humanBridge{}
)

// ParseHumanParams reads the rendered params of a human node.
func ParseHumanParams(raw json.RawMessage) (HumanParams, error) {
	var p struct {
		Title     string         `json:"title"`
		Body      string         `json:"body"`
		Assignees []assigneeJSON `json:"assignees"`
		Required  int            `json:"required"`
		Buttons   []struct {
			Outcome        string `json:"outcome"`
			Label          string `json:"label"`
			Style          string `json:"style"`
			RequireComment bool   `json:"require_comment"`
		} `json:"buttons"`
		Veto       []string        `json:"veto"`
		Form       json.RawMessage `json:"form"`
		Timeout    string          `json:"timeout"`
		Escalation []struct {
			Add    []assigneeJSON `json:"add"`
			Extend string         `json:"extend"`
		} `json:"escalation"`
		AllowDelegate bool `json:"allow_delegate"`
	}
	if err := json.Unmarshal(raw, &p); err != nil {
		return HumanParams{}, fmt.Errorf("the params of a human node: %w", err)
	}
	out := HumanParams{Title: p.Title, Body: p.Body, Required: p.Required, Veto: p.Veto, Form: p.Form, Timeout: p.Timeout, AllowDelegate: p.AllowDelegate}
	for _, a := range p.Assignees {
		out.Assignees = append(out.Assignees, a.assignee())
	}
	for _, b := range p.Buttons {
		out.Buttons = append(out.Buttons, HumanButton{Outcome: b.Outcome, Label: b.Label, Style: b.Style, RequireComment: b.RequireComment})
	}
	for _, e := range p.Escalation {
		step := workflow.EscalationStep{}
		for _, a := range e.Add {
			step.Add = append(step.Add, a.assignee())
		}
		if ms, ok := graph.DurationMs(e.Extend); ok {
			step.Extend = time.Duration(ms) * time.Millisecond
		}
		out.Escalation = append(out.Escalation, step)
	}
	return out, nil
}

type assigneeJSON struct {
	Kind   string `json:"kind"`
	UserID *int64 `json:"user_id"`
	Ref    string `json:"ref"`
}

func (a assigneeJSON) assignee() workflow.TaskAssignee {
	return workflow.TaskAssignee{Kind: a.Kind, UserID: a.UserID, Ref: a.Ref}
}

// TaskSpec is the task of a human node for its rendered params.
func (p HumanParams) TaskSpec(waitMax time.Duration, subject SubjectRef) (workflow.TaskSpec, error) {
	if len(p.Assignees) == 0 || len(p.Assignees) > workflow.MaxTaskAssignees {
		return workflow.TaskSpec{}, fmt.Errorf("a human task has 1..%d assignees", workflow.MaxTaskAssignees)
	}
	if len(p.Buttons) == 0 {
		return workflow.TaskSpec{}, errors.New("a human task has buttons")
	}
	spec := workflow.TaskSpec{Assignees: p.Assignees, Required: p.Required, Veto: p.Veto, Escalation: p.Escalation, AllowDelegation: p.AllowDelegate}
	for _, b := range p.Buttons {
		if b.Outcome == CodeTimeout {
			return workflow.TaskSpec{}, errors.New("timeout is the outcome of the task's timeout, not a button")
		}
		spec.Outcomes = append(spec.Outcomes, b.Outcome)
	}
	for _, v := range p.Veto {
		if !slices.Contains(spec.Outcomes, v) {
			return workflow.TaskSpec{}, fmt.Errorf("veto %s is not a button", v)
		}
	}
	if p.Timeout != "" {
		ms, ok := graph.DurationMs(p.Timeout)
		if !ok {
			return workflow.TaskSpec{}, fmt.Errorf("timeout %q is not a duration", p.Timeout)
		}
		spec.Timeout = time.Duration(ms) * time.Millisecond
		if spec.Timeout > waitMax {
			spec.Timeout = waitMax
		}
	}
	if len(spec.Escalation) > 0 && spec.Timeout == 0 {
		return workflow.TaskSpec{}, errors.New("escalation needs a timeout")
	}
	// A decision: the button, a comment and the fields of the form.
	outcome := manifest.Type{Kind: manifest.KindEnum, Values: spec.Outcomes}
	spec.Decision = &manifest.Type{Kind: manifest.KindObject, Properties: []manifest.Property{
		{Name: "outcome", Type: outcome},
		{Name: "comment", Type: manifest.Type{Kind: manifest.KindString, Optional: true, MaxLength: 4000}},
		{Name: "fields", Type: manifest.Type{Kind: manifest.KindJSON, Optional: true}},
	}}
	title := p.Title
	spec.Title = manifest.Label{"en": title, "ru": title, "uz": title, "uz-Cyrl": title}
	if s := normSubject(subject); s.Entity != graph.SubjectNone {
		spec.Subject = &spi.ObjectRef{Entity: s.Entity, Key: s.Key}
	}
	return spec, nil
}

func (b humanBridge) Begin(_ context.Context, _ pgx.Tx, run StepContext, _ graph.Node, input json.RawMessage) (WaitSpec, error) {
	p, err := ParseHumanParams(input)
	if err != nil {
		return WaitSpec{}, &StepError{Code: CodeInvalidInput, Message: err.Error()}
	}
	spec, err := p.TaskSpec(b.waitMax, run.Subject)
	if err != nil {
		return WaitSpec{}, &StepError{Code: CodeInvalidInput, Message: err.Error()}
	}
	return WaitSpec{Kind: waitTask, Task: &spec}, nil
}

// humanDecision is a decision of a human task as the node's output shows it.
type humanDecision struct {
	UserID  int64           `json:"user_id"`
	Outcome string          `json:"outcome"`
	Comment string          `json:"comment"`
	Fields  json.RawMessage `json:"fields"`
	At      string          `json:"at"`
}

// Resolve maps the task's result: its choice is the outcome, a timeout the timeout outcome; the output
// is {outcome, decisions, delegated_from?}.
func (humanBridge) Resolve(_ graph.Node, res workflow.TaskResult) WaitResult {
	outcome := res.Choice
	if res.Outcome == workflow.TaskTimeout {
		outcome = CodeTimeout
	}
	out := map[string]any{"outcome": outcome}
	decisions := make([]humanDecision, 0, len(res.Decisions))
	var delegatedFrom *int64
	for _, d := range res.Decisions {
		var body struct {
			Comment string          `json:"comment"`
			Fields  json.RawMessage `json:"fields"`
		}
		_ = json.Unmarshal(d.Decision, &body)
		fields := body.Fields
		if len(fields) == 0 || string(fields) == "null" {
			fields = json.RawMessage(`{}`)
		}
		decisions = append(decisions, humanDecision{UserID: d.UserID, Outcome: d.Choice, Comment: body.Comment, Fields: fields, At: d.DecidedAt.UTC().Format(time.RFC3339)})
		if res.By != nil && d.UserID == *res.By && d.DelegatedFrom != nil {
			delegatedFrom = d.DelegatedFrom
		}
	}
	out["decisions"] = decisions
	if delegatedFrom != nil {
		out["delegated_from"] = *delegatedFrom
	}
	b, err := json.Marshal(out)
	if err != nil {
		return WaitResult{Error: &StepError{Code: graph.FailInvalidAnswer, Message: "the decision is not JSON"}}
	}
	return WaitResult{Outcome: outcome, Output: b}
}

// EventTaskAssigned is published when a human node's task is created, delegated or escalated.
const EventTaskAssigned = "automation.task.assigned.v1"

// SubjectTask is the entity of EventTaskAssigned: a task, keyed by its ID.
const SubjectTask = "automation.task"

// MaxAssignedUsers bounds UserIDs of one TaskAssigned.
const MaxAssignedUsers = 200

// TaskAssignedPayload is the payload of automation.task.assigned.v1.
type TaskAssignedPayload struct {
	TaskID       uuid.UUID  `json:"task_id"`
	RunID        uuid.UUID  `json:"run_id"`
	AutomationID uuid.UUID  `json:"automation_id"`
	NodeID       string     `json:"node_id"`
	Title        string     `json:"title"`    // rendered HumanParams.Title
	Reason       string     `json:"reason"`   // "created" | "delegated" | "escalated"
	UserIDs      []int64    `json:"user_ids"` // newly added assignees: TaskAudience.Users of #1327 at this moment, ≤ 200
	DelegatedBy  *int64     `json:"delegated_by,omitempty"`
	DueAt        *time.Time `json:"due_at,omitempty"`
}

// TaskAssigned is declared once with Label/Descriptions in en, ru, uz, uz-Cyrl (EventSpec of #1325).
// Subject "automation.task", key = task id. ReadPermission "Automations.Read". Category "automations".
var TaskAssigned = native.Event[TaskAssignedPayload](EventTaskAssigned, native.EventSpec{
	Subject: SubjectTask,
	Label: manifest.Label{
		"en": "Automation task assigned", "ru": "Назначена задача автоматизации",
		"uz": "Avtomatlashtirish vazifasi tayinlandi", "uz-Cyrl": "Автоматлаштириш вазифаси тайинланди",
	},
	Descriptions: manifest.Label{
		"en":      "A human step of an automation was assigned to people: on creation, delegation or escalation.",
		"ru":      "Шаг автоматизации назначен людям: при создании, делегировании или эскалации.",
		"uz":      "Avtomatlashtirish qadami odamlarga tayinlandi: yaratilganda, topshirilganda yoki eskalatsiyada.",
		"uz-Cyrl": "Автоматлаштириш қадами одамларга тайинланди: яратилганда, топширилганда ёки эскалацияда.",
	},
	Payload: &manifest.Type{Kind: manifest.KindObject, Properties: []manifest.Property{
		{Name: "task_id", Type: manifest.Type{Kind: manifest.KindUUID}},
		{Name: "run_id", Type: manifest.Type{Kind: manifest.KindUUID}},
		{Name: "automation_id", Type: manifest.Type{Kind: manifest.KindUUID}},
		{Name: "node_id", Type: manifest.Type{Kind: manifest.KindString}},
		{Name: "title", Type: manifest.Type{Kind: manifest.KindString}},
		{Name: "reason", Type: manifest.Type{Kind: manifest.KindEnum, Values: []string{workflow.TaskChangeCreated, workflow.TaskChangeDelegated, workflow.TaskChangeEscalated}}},
		{Name: "user_ids", Type: manifest.Type{Kind: manifest.KindArray, Items: &manifest.Type{Kind: manifest.KindInt}}},
		{Name: "delegated_by", Type: manifest.Type{Kind: manifest.KindInt, Optional: true}},
		{Name: "due_at", Type: manifest.Type{Kind: manifest.KindDateTime, Optional: true}},
	}},
	ReadPermission: PermRead,
	Category:       "automations",
})

// taskAssignedObserver publishes TaskAssigned for the tasks of human nodes.
type taskAssignedObserver struct {
	audience workflow.TaskAudience
	logger   *slog.Logger
}

// NewTaskAssignedObserver is the workflow.TaskObserver that publishes automation.task.assigned.v1 in the
// transaction of a task's created, delegated and escalated changes (a rolled-back change leaves no
// event; a retried step does not publish twice). audience expands roles, groups and permissions.
func NewTaskAssignedObserver(audience workflow.TaskAudience, logger *slog.Logger) workflow.TaskObserver {
	if logger == nil {
		logger = slog.Default()
	}
	return &taskAssignedObserver{audience: audience, logger: logger}
}

func (o *taskAssignedObserver) TaskChanged(ctx context.Context, tx pgx.Tx, t workflow.Task, change workflow.TaskChange) error {
	if t.Workflow != WorkflowID || !strings.HasPrefix(t.Name, "task:") {
		return nil
	}
	switch change.Kind {
	case workflow.TaskChangeCreated, workflow.TaskChangeDelegated, workflow.TaskChangeEscalated:
	default:
		return nil
	}
	waitID := strings.TrimPrefix(t.Name, "task:")
	var (
		tenant, runID, automationID uuid.UUID
		nodeID                      string
		seq                         int
		tokenID                     uuid.UUID
	)
	err := tx.QueryRow(ctx, `
		SELECT r.tenant_id, r.id, r.automation_id, w.node_id, (w.spec->>'seq')::int, w.token_id
		FROM automation.runs r JOIN automation.waits w ON w.tenant_id = r.tenant_id AND w.run_id = r.id AND w.wait_id = $2
		WHERE r.instance_id = $1`, t.InstanceID, waitID).Scan(&tenant, &runID, &automationID, &nodeID, &seq, &tokenID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	if err != nil {
		return serrors.FromDB(opHuman, err)
	}
	users, cut, err := o.newUsers(ctx, tenant, t, change)
	if err != nil {
		return err
	}
	if cut {
		warning := fmt.Sprintf("the task of %s has more than %d new assignees; the notification names the first %d", nodeID, MaxAssignedUsers, MaxAssignedUsers)
		o.logger.Warn("automation: "+warning, "task_id", t.ID.String(), "run_id", runID.String())
		warn, err := json.Marshal([]string{warning})
		if err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `UPDATE automation.steps SET warnings = coalesce(warnings, '[]'::jsonb) || $5::jsonb
			WHERE tenant_id = $1 AND run_id = $2 AND token_id = $3 AND seq = $4`, tenant, runID, tokenID, seq, warn); err != nil {
			return serrors.FromDB(opHuman, err)
		}
	}
	if len(users) == 0 {
		return nil
	}
	payload := TaskAssignedPayload{
		TaskID: t.ID, RunID: runID, AutomationID: automationID, NodeID: nodeID, Title: labelOf(t.Title),
		Reason: change.Kind, UserIDs: users, DueAt: t.DueAt,
	}
	if change.Kind == workflow.TaskChangeDelegated {
		payload.DelegatedBy = change.ActorID
	}
	if _, err := events.Publish(ctx, tx, spi.Scope{Subject: spi.Subject{TenantID: tenant}}, TaskAssigned,
		&spi.ObjectRef{Entity: SubjectTask, Key: t.ID.String()}, payload); err != nil {
		return serrors.Wrap(opHuman, err)
	}
	return nil
}

// newUsers are the users the change added: its assignees expanded by the audience, without the users
// the earlier assignees already cover; cut reports a list cut to MaxAssignedUsers.
func (o *taskAssignedObserver) newUsers(ctx context.Context, tenant uuid.UUID, t workflow.Task, change workflow.TaskChange) ([]int64, bool, error) {
	earlier := map[int64]bool{}
	if change.Kind == workflow.TaskChangeEscalated {
		added := map[string]bool{}
		for _, a := range change.Assignees {
			added[a.String()] = true
		}
		for _, a := range t.Assignees {
			if added[a.String()] {
				continue
			}
			ids, err := o.expand(ctx, tenant, a, workflow.MaxTaskInbox*5)
			if err != nil {
				return nil, false, err
			}
			for _, id := range ids {
				earlier[id] = true
			}
		}
		for _, d := range t.Delegations {
			earlier[d.To] = true
		}
	}
	seen := map[int64]bool{}
	var out []int64
	cut := false
	for _, a := range change.Assignees {
		ids, err := o.expand(ctx, tenant, a, MaxAssignedUsers+1)
		if err != nil {
			return nil, false, err
		}
		for _, id := range ids {
			if earlier[id] || seen[id] {
				continue
			}
			if len(out) == MaxAssignedUsers {
				cut = true
				break
			}
			seen[id] = true
			out = append(out, id)
		}
	}
	slices.Sort(out)
	return out, cut, nil
}

func (o *taskAssignedObserver) expand(ctx context.Context, tenant uuid.UUID, a workflow.TaskAssignee, limit int) ([]int64, error) {
	if a.Kind == workflow.TaskAssignUser {
		if a.UserID == nil {
			return nil, nil
		}
		return []int64{*a.UserID}, nil
	}
	if o.audience == nil {
		return nil, nil
	}
	ids, err := o.audience.Users(ctx, tenant, a, limit)
	if err != nil {
		return nil, serrors.Wrap(opHuman, err)
	}
	return ids, nil
}

// labelOf is a title of one language: en, else any.
func labelOf(l manifest.Label) string {
	if s := l["en"]; s != "" {
		return s
	}
	keys := make([]string, 0, len(l))
	for k := range l {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	for _, k := range keys {
		if l[k] != "" {
			return l[k]
		}
	}
	return ""
}
