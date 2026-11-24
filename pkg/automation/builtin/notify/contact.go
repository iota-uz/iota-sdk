package notify

import (
	"context"
	"errors"
	"fmt"
	"slices"

	"github.com/jackc/pgx/v5"

	"github.com/iota-uz/iota-sdk/modules/core/notifications"
	"github.com/iota-uz/iota-sdk/pkg/automation/catalog"
	"github.com/iota-uz/iota-sdk/pkg/automation/engine"
	"github.com/iota-uz/iota-sdk/pkg/automation/expr"
	"github.com/iota-uz/iota-sdk/pkg/automation/graph"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/host"
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
	_ engine.EffectPreparer    = (*contactNode)(nil)
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
			{Name: "skipped", Field: skippedField},
			// Only a dry run fills it: the records the audience resolves to (never their addresses).
			{Name: "resolved", Field: expr.OptionalOf(expr.ObjectOf(expr.Fields{
				{Name: "records", Field: expr.ListOf(expr.StringField)},
			}))},
		},
	}
}

// ApplyEffect is ApplyEffectResult without the outcome.
func (n *contactNode) ApplyEffect(ctx context.Context, tx pgx.Tx, run engine.StepContext, node graph.Node, e graph.Effect) (map[string]any, error) {
	res, err := n.ApplyEffectResult(ctx, tx, run, node, e)
	return res.Output, err
}

// contactPrep is what PrepareEffect found out about the record, outside the step's transaction.
type contactPrep struct {
	record   notifications.ObjectRef
	channel  notifications.Channel
	resolved notifications.ResolvedAudience
}

// PrepareEffect checks the record's contact before the step's transaction opens, so a resolver that takes its
// own connection (a package's) never waits for one the transaction holds: the delivery layer judges the
// request in one order (the channel, then the audience, then the record's contact) with CheckContacts, exactly
// as a dry run does, and nothing is queued yet.
func (n *contactNode) PrepareEffect(ctx context.Context, run engine.StepContext, node graph.Node, e graph.Effect) (any, error) {
	req, channel, err := n.request(run, node, e.Params)
	if err != nil {
		return nil, err
	}
	resolved, err := n.cfg.Sender.Resolve(n.readCtx(ctx, run), req)
	if err != nil {
		return nil, failure(err)
	}
	return &contactPrep{record: req.Audience.Records[0], channel: channel, resolved: resolved}, nil
}

// ApplyEffectResult queues the message in the step's transaction when the record can be reached. The contact
// was checked before the transaction (PrepareEffect), so the send takes no second connection; a node applied
// without the engine's preparation (a test of the node) lets the delivery layer check it in the transaction
// (CheckContacts). A channel that is not available fails the node whether or not the record has an address:
// the delivery layer checks it first, in both.
func (n *contactNode) ApplyEffectResult(ctx context.Context, tx pgx.Tx, run engine.StepContext, node graph.Node, e graph.Effect) (engine.EffectResult, error) {
	req, channel, err := n.request(run, node, e.Params)
	if err != nil {
		return engine.EffectResult{}, err
	}
	out := map[string]any{"notice_id": "", "channel": string(channel), "skipped": []any{}}
	if p := run.Prepared; p != nil {
		if p.Err != nil {
			return engine.EffectResult{}, p.Err
		}
		// The engine applies the effect it prepared, and retries the step when no preparation fits. The node
		// never queues a record whose contact was not checked: a preparation that is exhausted, or made for
		// another record or channel, stops the step with a platform error, and the engine tries it again.
		prep, ok := p.Value.(*contactPrep)
		if p.Exhausted || !ok || prep.record != req.Audience.Records[0] || prep.channel != channel {
			return engine.EffectResult{}, &host.InfraError{Err: errors.New("send_to_contact: no preparation fits the step; it is tried again later")}
		}
		out["skipped"] = skippedOf(prep.resolved.Skipped)
		if prep.resolved.Total() == 0 && !slices.ContainsFunc(prep.resolved.Skipped, func(s notifications.SkippedRecipient) bool {
			return s.Reason == notifications.SkipNoContactResolver
		}) {
			return result(OutcomeNoContact, out)
		}
		req.CheckContacts = false // checked above, outside the transaction
	}
	receipt, err := n.cfg.Sender.SendTx(tenantCtx(ctx, run), tx, req)
	if err != nil {
		return engine.EffectResult{}, failure(err)
	}
	if len(receipt.Skipped) > 0 {
		out["skipped"] = skippedOf(receipt.Skipped)
	}
	if receipt.Recipients == 0 {
		return result(OutcomeNoContact, out)
	}
	out["notice_id"] = receipt.NoticeID.String()
	return result(OutcomeSent, out)
}

// PreviewEffect is what a dry run shows: the audience the delivery layer resolves, with the record's contact
// checked as a send would, without a send. The trace names the records and the reasons recipients were
// skipped, never the addresses of a contact.
func (n *contactNode) PreviewEffect(ctx context.Context, run engine.StepContext, node graph.Node, e graph.Effect) (engine.EffectResult, error) {
	req, channel, err := n.request(run, node, e.Params)
	if err != nil {
		return engine.EffectResult{}, err
	}
	out := map[string]any{"notice_id": "", "channel": string(channel), "skipped": []any{}}
	resolved, err := n.cfg.Sender.Resolve(n.readCtx(ctx, run), req)
	if err != nil {
		return engine.EffectResult{}, failure(err)
	}
	for _, s := range resolved.Skipped {
		if s.Reason == notifications.SkipNoContactResolver {
			// A send refuses it as invalid input, so the preview does too.
			return engine.EffectResult{}, &engine.StepError{Code: "invalid", Message: "no contact resolver is registered for " + s.Ref}
		}
	}
	records := make([]any, 0, len(resolved.Addressees))
	for _, a := range resolved.Addressees {
		if a.Record != nil {
			records = append(records, a.Record.Entity+"/"+a.Record.Key)
		}
	}
	out["skipped"] = skippedOf(resolved.Skipped)
	out["resolved"] = map[string]any{"records": records}
	if resolved.Total() == 0 {
		return result(OutcomeNoContact, out)
	}
	return result(OutcomeSent, out)
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
		// The record is queued only when its contact has an address on the channel: no_contact otherwise.
		CheckContacts: true,
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
