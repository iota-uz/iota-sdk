package notify

import (
	"context"
	"fmt"
	"strconv"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/iota-uz/iota-sdk/modules/core/notifications"
	"github.com/iota-uz/iota-sdk/pkg/automation/catalog"
	"github.com/iota-uz/iota-sdk/pkg/automation/engine"
	"github.com/iota-uz/iota-sdk/pkg/automation/expr"
	"github.com/iota-uz/iota-sdk/pkg/automation/graph"
)

const notifyParamsSchema = `{
  "type": "object",
  "properties": {
    "recipients": {
      "type": "object",
      "minProperties": 1,
      "properties": {
        "users": {"type": "string", "minLength": 1, "maxLength": 2000, "description": "CEL: a list of user IDs"},
        "groups": {"type": "array", "maxItems": 50, "items": {"type": "string", "pattern": "^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$"}},
        "roles": {"type": "array", "maxItems": 50, "items": {"type": "string", "pattern": "^[1-9][0-9]{0,9}$"}},
        "participants": {"type": "array", "maxItems": 20, "items": {"type": "string", "minLength": 1, "maxLength": 100}}
      },
      "additionalProperties": false
    },
    "title": {"type": "string", "minLength": 1, "maxLength": 1000},
    "body": {"type": "string", "minLength": 1, "maxLength": 10000},
    "action_url": {"type": "string", "maxLength": 2000},
    "channels": {"type": "array", "maxItems": 10, "items": {"type": "string", "pattern": "^[a-z][a-z0-9_]{0,29}$"}},
    "level": {"enum": ["info", "warning", "critical"], "default": "info"}
  },
  "required": ["recipients", "title", "body"],
  "additionalProperties": false
}`

// notifyNode is the node type notify.
type notifyNode struct{ base }

// NewNotify is the node type notify: a notice to users, groups, roles or the participants of the trigger
// event, on the chosen channels and at the chosen level.
func NewNotify(cfg Config) catalog.NodeType { return &notifyNode{newBase(cfg)} }

var (
	_ engine.OutcomeEffectNode = (*notifyNode)(nil)
	_ engine.EffectPreviewer   = (*notifyNode)(nil)
	_ engine.GraphChecker      = (*notifyNode)(nil)
)

func (*notifyNode) Requires() []catalog.ConnectorKind { return nil }

func (*notifyNode) Spec() graph.NodeSpec {
	return graph.NodeSpec{
		Type: NodeNotify, Title: "Notify", Behavior: graph.BehaviorEffect, DryRun: graph.DryRunCapture,
		Description: "Sends a notice to users, groups, roles or the participants of the trigger event, on the chosen " +
			"channels. no_recipients when nobody is left after the audience is resolved.",
		Params:        graph.MustSchemaParams(notifyParamsSchema),
		Outcomes:      []string{OutcomeSent, OutcomeNoRecipients, OutcomeFailed},
		DoneOutcome:   OutcomeSent,
		FailedOutcome: OutcomeFailed,
		Expressions:   []graph.ExprParam{{Param: "recipients.users", Type: expr.TypeList}},
		Templates:     []string{"title", "body", "action_url"},
		Values:        []string{"recipients.users"},
		Output: expr.Fields{
			{Name: "notice_id", Field: expr.StringField},
			{Name: "recipients", Field: expr.IntField},
			{Name: "skipped", Field: skippedField},
			// Only a dry run fills it: who the audience resolves to.
			{Name: "resolved", Field: expr.OptionalOf(expr.ObjectOf(expr.Fields{
				{Name: "users", Field: expr.ListOf(expr.IntField)},
				{Name: "records", Field: expr.ListOf(expr.StringField)},
			}))},
		},
	}
}

// ApplyEffect is ApplyEffectResult without the outcome.
func (n *notifyNode) ApplyEffect(ctx context.Context, tx pgx.Tx, run engine.StepContext, node graph.Node, e graph.Effect) (map[string]any, error) {
	res, err := n.ApplyEffectResult(ctx, tx, run, node, e)
	return res.Output, err
}

// ApplyEffectResult queues the notice in the step's transaction.
func (n *notifyNode) ApplyEffectResult(ctx context.Context, tx pgx.Tx, run engine.StepContext, node graph.Node, e graph.Effect) (engine.EffectResult, error) {
	req, err := n.request(run, node, e.Params)
	if err != nil {
		return engine.EffectResult{}, err
	}
	out := map[string]any{"notice_id": "", "recipients": 0, "skipped": []any{}}
	if req.Audience.IsZero() {
		return result(OutcomeNoRecipients, out)
	}
	// The delivery layer judges the request first: what a caller can act on (an unknown participant, a
	// channel that is not available) is its error, and an audience that resolves to nobody queues nothing.
	receipt, err := n.cfg.Sender.SendTx(tenantCtx(ctx, run), tx, req)
	if err != nil {
		return engine.EffectResult{}, failure(err)
	}
	// Who was left out is told by the same resolution. It only explains the outcome, so it reads outside
	// the step's transaction (a failed statement there would abort the step) and a resolution that fails
	// now leaves the list empty rather than failing a notice that is queued.
	if resolved, err := n.cfg.Sender.Resolve(n.readCtx(ctx, run), req); err == nil {
		out["skipped"] = skippedOf(resolved.Skipped)
	}
	if receipt.Recipients == 0 {
		return result(OutcomeNoRecipients, out)
	}
	out["notice_id"] = receipt.NoticeID.String()
	out["recipients"] = receipt.Recipients
	return result(OutcomeSent, out)
}

// PreviewEffect is what a dry run shows: the audience as the delivery layer resolves it, without a send.
func (n *notifyNode) PreviewEffect(ctx context.Context, run engine.StepContext, node graph.Node, e graph.Effect) (engine.EffectResult, error) {
	req, err := n.request(run, node, e.Params)
	if err != nil {
		return engine.EffectResult{}, err
	}
	out := map[string]any{"notice_id": "", "recipients": 0, "skipped": []any{}}
	if req.Audience.IsZero() {
		return result(OutcomeNoRecipients, out)
	}
	resolved, err := n.cfg.Sender.Resolve(n.readCtx(ctx, run), req)
	if err != nil {
		return engine.EffectResult{}, failure(err)
	}
	users := make([]any, 0, len(resolved.UserIDs))
	for _, id := range resolved.UserIDs {
		users = append(users, int64(id))
	}
	records := make([]any, 0, len(resolved.Addressees))
	for _, a := range resolved.Addressees {
		records = append(records, a.Key())
	}
	out["recipients"] = resolved.Total()
	out["skipped"] = skippedOf(resolved.Skipped)
	out["resolved"] = map[string]any{"users": users, "records": records}
	outcome := OutcomeSent
	if resolved.Total() == 0 {
		outcome = OutcomeNoRecipients
	}
	return result(outcome, out)
}

// request is the audience request of a rendered effect.
func (n *notifyNode) request(run engine.StepContext, node graph.Node, params map[string]any) (notifications.AudienceRequest, error) {
	rec, _ := params["recipients"].(map[string]any)
	audience := notifications.Audience{}
	users, err := userIDs(rec["users"])
	if err != nil {
		return notifications.AudienceRequest{}, err
	}
	audience.Users = users
	for _, s := range strs(rec["groups"]) {
		id, err := uuid.Parse(s)
		if err != nil {
			return notifications.AudienceRequest{}, invalid("group %q is not a UUID", s)
		}
		audience.Groups = append(audience.Groups, id)
	}
	for _, s := range strs(rec["roles"]) {
		id, err := strconv.ParseUint(s, 10, 32)
		if err != nil || id == 0 {
			return notifications.AudienceRequest{}, invalid("role %q is not a role ID", s)
		}
		audience.Roles = append(audience.Roles, uint(id))
	}
	audience.Participants = strs(rec["participants"])

	title, body := text(params, "title"), text(params, "body")
	switch {
	case title == "":
		return notifications.AudienceRequest{}, invalid("the title is empty")
	case len(title) > maxTitle:
		return notifications.AudienceRequest{}, invalid("the title is longer than %d bytes", maxTitle)
	case body == "":
		return notifications.AudienceRequest{}, invalid("the text is empty")
	case len(body) > maxBody:
		return notifications.AudienceRequest{}, invalid("the text is longer than %d bytes", maxBody)
	}
	action, err := resolveActionURL(text(params, "action_url"), n.host)
	if err != nil {
		return notifications.AudienceRequest{}, err
	}
	channels, err := channelsOf(params)
	if err != nil {
		return notifications.AudienceRequest{}, err
	}
	level, err := levelOf(params)
	if err != nil {
		return notifications.AudienceRequest{}, err
	}
	req := notifications.AudienceRequest{
		Event: notice(run, title, body, action), Audience: audience, Channels: channels, Level: level,
		Producer: producer(run, node),
	}
	if len(audience.Participants) > 0 {
		if req.Source, err = n.source(run); err != nil {
			return notifications.AudienceRequest{}, err
		}
	}
	return req, nil
}

// source is the event that started the run, in the form the delivery layer resolves participants from.
func (b base) source(run engine.StepContext) (*notifications.Event, error) {
	t := run.Trigger
	if t == nil || t.Key == "" {
		return nil, &engine.StepError{Code: CodeNoTriggerEvent, Message: "participants are the recipients of the event that started the run, and this run has none"}
	}
	ev := notifications.Event{Key: t.Key, ID: t.ID, TenantID: run.TenantID, Payload: t.Payload, Subject: subjectRef(run)}
	if ev.ID == "" {
		ev.ID = run.RunID.String()
	}
	if b.cfg.Catalog != nil {
		if def, ok := b.cfg.Catalog.Get(t.Key); ok {
			if data, err := def.DataOf(t.Payload); err == nil {
				ev.Data = data
			}
			if def.DedupeKey != nil {
				ev.DedupeKey = def.DedupeKey(ev)
			}
		}
	}
	return &ev, nil
}

// CheckNode checks what a graph that uses the node says about it while the graph is saved: the channels are
// known, the link stays in the application, and participants are offered by every trigger event that can
// reach the node.
func (n *notifyNode) CheckNode(g graph.Graph, index int, _ catalog.View) []graph.GraphError {
	node := g.Nodes[index]
	at := fmt.Sprintf("nodes[%d].params", index)
	var out []graph.GraphError
	out = append(out, checkChannels(node.Params, at)...)
	if s, ok := node.Params["action_url"].(string); ok {
		if reason := checkActionURL(s, n.host); reason != "" {
			out = append(out, graph.GraphError{Path: at + ".action_url", Code: graph.CodeParams, Message: "action_url " + reason})
		}
	}
	rec, _ := node.Params["recipients"].(map[string]any)
	keys := strs(rec["participants"])
	if len(keys) == 0 {
		return out
	}
	for _, trigger := range triggersReaching(g, node.ID) {
		if reason := n.participantsRefused(trigger.Event, keys); reason != "" {
			out = append(out, graph.GraphError{Path: at + ".recipients.participants", Code: graph.CodeParams,
				Message: fmt.Sprintf("participants are not available: trigger %s (%s) %s", trigger.ID, trigger.Event, reason)})
		}
	}
	return out
}

// participantsRefused says why an event cannot name these participants: it has no notification definition,
// or the definition has no such recipient key. "" when it can.
func (b base) participantsRefused(event string, keys []string) string {
	if b.cfg.Catalog == nil {
		return "has no notification definition"
	}
	def, ok := b.cfg.Catalog.Get(event)
	if !ok || len(def.RecipientKeys) == 0 {
		return "has no notification definition with recipients"
	}
	known := make(map[string]bool, len(def.RecipientKeys))
	for _, r := range def.RecipientKeys {
		known[r.Key] = true
	}
	for _, k := range keys {
		if !known[k] {
			return fmt.Sprintf("has no recipient %q", k)
		}
	}
	return ""
}

// triggersReaching are the triggers of a graph from which the node can be reached.
func triggersReaching(g graph.Graph, nodeID string) []graph.Trigger {
	next := map[string][]string{}
	for _, e := range g.Edges {
		next[e.From.Node] = append(next[e.From.Node], e.To)
	}
	var out []graph.Trigger
	for _, t := range g.Triggers {
		seen := map[string]bool{t.ID: true}
		queue := []string{t.ID}
		for len(queue) > 0 {
			cur := queue[0]
			queue = queue[1:]
			if cur == nodeID {
				out = append(out, t)
				break
			}
			for _, to := range next[cur] {
				if !seen[to] {
					seen[to] = true
					queue = append(queue, to)
				}
			}
		}
	}
	return out
}

// checkChannels refuses a channel the SDK does not know.
func checkChannels(params map[string]any, at string) []graph.GraphError {
	var out []graph.GraphError
	list, _ := params["channels"].([]any)
	for i, v := range list {
		s, _ := v.(string)
		if !notifications.Channel(s).Valid() {
			out = append(out, graph.GraphError{Path: fmt.Sprintf("%s.channels[%d]", at, i), Code: graph.CodeParams,
				Message: fmt.Sprintf("%q is not a notification channel", s)})
		}
	}
	return out
}
