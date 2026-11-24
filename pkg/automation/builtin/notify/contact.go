package notify

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"

	"github.com/iota-uz/iota-sdk/modules/core/notifications"
	"github.com/iota-uz/iota-sdk/pkg/automation/catalog"
	"github.com/iota-uz/iota-sdk/pkg/automation/engine"
	"github.com/iota-uz/iota-sdk/pkg/automation/expr"
	"github.com/iota-uz/iota-sdk/pkg/automation/graph"
)

const contactParamsSchema = `{
  "type": "object",
  "properties": {
    "record": {"type": "string", "minLength": 1, "maxLength": 2000, "description": "CEL: {entity, key}; the subject of the run by default"},
    "channel": {"type": "string", "pattern": "^[a-z][a-z0-9_]{0,29}$"},
    "subject": {"type": "string", "maxLength": 1000},
    "body": {"type": "string", "minLength": 1, "maxLength": 10000}
  },
  "required": ["channel", "body"],
  "additionalProperties": false
}`

// contactNode is the node type send_to_contact.
type contactNode struct{ base }

// NewSendToContact is the node type send_to_contact: a message to the phone or the email of a record (a
// customer, an employee, a user) on one channel, through the contact resolver of the record's entity.
func NewSendToContact(cfg Config) catalog.NodeType { return &contactNode{newBase(cfg)} }

var (
	_ engine.OutcomeEffectNode = (*contactNode)(nil)
	_ engine.EffectPreviewer   = (*contactNode)(nil)
	_ engine.GraphChecker      = (*contactNode)(nil)
)

func (*contactNode) Requires() []catalog.ConnectorKind { return nil }

func (*contactNode) Spec() graph.NodeSpec {
	return graph.NodeSpec{
		Type: NodeSendToContact, Title: "Send to the contact of a record", Behavior: graph.BehaviorEffect, DryRun: graph.DryRunCapture,
		Description: "Sends a message to the phone or the email of a record: the subject of the run, or another record. " +
			"no_contact when the record has no address on the channel.",
		Params:        graph.MustSchemaParams(contactParamsSchema),
		Outcomes:      []string{OutcomeSent, OutcomeNoContact, OutcomeFailed},
		DoneOutcome:   OutcomeSent,
		FailedOutcome: OutcomeFailed,
		Expressions:   []graph.ExprParam{{Param: "record", Type: expr.TypeMap}},
		Templates:     []string{"subject", "body"},
		Values:        []string{"record"},
		Output: expr.Fields{
			{Name: "notice_id", Field: expr.StringField},
			{Name: "channel", Field: expr.StringField},
		},
	}
}

// ApplyEffect is ApplyEffectResult without the outcome.
func (n *contactNode) ApplyEffect(ctx context.Context, tx pgx.Tx, run engine.StepContext, node graph.Node, e graph.Effect) (map[string]any, error) {
	res, err := n.ApplyEffectResult(ctx, tx, run, node, e)
	return res.Output, err
}

// ApplyEffectResult queues the message in the step's transaction when the record can be reached.
func (n *contactNode) ApplyEffectResult(ctx context.Context, tx pgx.Tx, run engine.StepContext, node graph.Node, e graph.Effect) (engine.EffectResult, error) {
	req, channel, err := n.request(run, node, e.Params)
	if err != nil {
		return engine.EffectResult{}, err
	}
	out := map[string]any{"notice_id": "", "channel": string(channel)}
	reachable, err := n.reachable(ctx, run, req.Audience.Records[0], channel)
	if err != nil {
		return engine.EffectResult{}, err
	}
	if !reachable {
		return result(OutcomeNoContact, out)
	}
	receipt, err := n.cfg.Sender.SendTx(tenantCtx(ctx, run), tx, req)
	if err != nil {
		return engine.EffectResult{}, failure(err)
	}
	if receipt.Recipients == 0 {
		return result(OutcomeNoContact, out)
	}
	out["notice_id"] = receipt.NoticeID.String()
	return result(OutcomeSent, out)
}

// PreviewEffect is what a dry run shows: whether the record can be reached, and the audience the delivery
// layer resolves, without a send.
func (n *contactNode) PreviewEffect(ctx context.Context, run engine.StepContext, node graph.Node, e graph.Effect) (engine.EffectResult, error) {
	req, channel, err := n.request(run, node, e.Params)
	if err != nil {
		return engine.EffectResult{}, err
	}
	out := map[string]any{"notice_id": "", "channel": string(channel)}
	reachable, err := n.reachable(ctx, run, req.Audience.Records[0], channel)
	if err != nil {
		return engine.EffectResult{}, err
	}
	if !reachable {
		return result(OutcomeNoContact, out)
	}
	resolved, err := n.cfg.Sender.Resolve(n.readCtx(ctx, run), req)
	if err != nil {
		return engine.EffectResult{}, failure(err)
	}
	if resolved.Total() == 0 {
		return result(OutcomeNoContact, out)
	}
	return result(OutcomeSent, out)
}

// reachable asks the contact resolver of the record's entity whether the record has an address on the
// channel, outside the step's transaction as the delivery worker does. A record the resolver does not know,
// or one without an address on the channel, is not reachable; a resolver that fails is the node's failure.
func (n *contactNode) reachable(ctx context.Context, run engine.StepContext, ref notifications.ObjectRef, channel notifications.Channel) (bool, error) {
	if n.cfg.Contacts == nil {
		return true, nil // the delivery layer records a missing address when it delivers
	}
	resolver, ok := n.cfg.Contacts.Lookup(ref.Entity)
	if !ok {
		return false, &engine.StepError{Code: CodeNoContactResolver, Message: "no contact resolver is registered for " + ref.Entity}
	}
	contact, err := resolver.ResolveContact(n.readCtx(ctx, run), run.TenantID, ref)
	switch {
	case errors.Is(err, notifications.ErrContactNotFound):
		return false, nil
	case err != nil:
		return false, &engine.StepError{Code: CodeContactLookup, Message: "the contact of " + ref.Entity + " could not be read", Retryable: true}
	}
	for _, t := range contact.Targets {
		if t.Channel == channel && strings.TrimSpace(t.Address) != "" {
			return true, nil
		}
	}
	return false, nil
}

// request is the audience request of a rendered effect and its channel.
func (n *contactNode) request(run engine.StepContext, node graph.Node, params map[string]any) (notifications.AudienceRequest, notifications.Channel, error) {
	channel := notifications.Channel(text(params, "channel"))
	if !channel.External() {
		return notifications.AudienceRequest{}, "", invalid("channel %q is not an external channel of the application", string(channel))
	}
	ref, err := recordOf(run, params["record"])
	if err != nil {
		return notifications.AudienceRequest{}, "", err
	}
	subject, body := text(params, "subject"), text(params, "body")
	switch {
	case body == "":
		return notifications.AudienceRequest{}, "", invalid("the text is empty")
	case len(body) > maxBody:
		return notifications.AudienceRequest{}, "", invalid("the text is longer than %d bytes", maxBody)
	case len(subject) > maxTitle:
		return notifications.AudienceRequest{}, "", invalid("the subject is longer than %d bytes", maxTitle)
	}
	return notifications.AudienceRequest{
		Event:    notice(run, subject, body, ""),
		Audience: notifications.Audience{Records: []notifications.ObjectRef{ref}},
		Channels: []notifications.Channel{channel},
		Producer: producer(run, node),
	}, channel, nil
}

// recordOf is the record of the node: the {entity, key} its record expression gave, else the subject of the run.
func recordOf(run engine.StepContext, v any) (notifications.ObjectRef, error) {
	if m, ok := v.(map[string]any); ok {
		entity, _ := m["entity"].(string)
		key, _ := m["key"].(string)
		ref := notifications.ObjectRef{Entity: entity, Key: key}
		if err := ref.Validate(); err != nil {
			return notifications.ObjectRef{}, invalid("record is not an {entity, key}")
		}
		return ref, nil
	}
	if v != nil {
		return notifications.ObjectRef{}, invalid("record is not an {entity, key}")
	}
	s := subjectRef(run)
	if s.IsZero() {
		return notifications.ObjectRef{}, invalid("the run has no record: its subject is not a record, and the node names none")
	}
	return s.Ref(), nil
}

// CheckNode refuses a channel that is not an external channel of the SDK.
func (n *contactNode) CheckNode(g graph.Graph, index int, _ catalog.View) []graph.GraphError {
	node := g.Nodes[index]
	s, ok := node.Params["channel"].(string)
	if !ok {
		return nil
	}
	if !notifications.Channel(s).External() {
		return []graph.GraphError{{Path: fmt.Sprintf("nodes[%d].params.channel", index), Code: graph.CodeParams,
			Message: fmt.Sprintf("%q is not a channel a record can be reached on", s)}}
	}
	return nil
}
