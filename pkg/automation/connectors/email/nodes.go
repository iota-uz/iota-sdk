package email

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/mail"
	"regexp"
	"strconv"
	"strings"

	"github.com/emersion/go-imap/v2"
	"github.com/jackc/pgx/v5"

	"github.com/iota-uz/iota-sdk/pkg/automation/catalog"
	"github.com/iota-uz/iota-sdk/pkg/automation/engine"
	"github.com/iota-uz/iota-sdk/pkg/automation/expr"
	"github.com/iota-uz/iota-sdk/pkg/automation/graph"
)

// Node types of the connectors.
const (
	NodeSend      = "email.send"
	NodeWaitReply = "email.wait_reply"
	NodeLabel     = "email.label"
)

const (
	outcomeSent    = "sent"
	outcomeFailed  = "failed"
	outcomeReplied = "replied"
	outcomeTimeout = "timeout"
	outcomeDone    = "done"
	outcomeMissing = "not_found"

	// maxRecipients is how many addresses one list of a message takes.
	maxRecipients = 50
)

const addressList = `{"type":"array","minItems":1,"maxItems":50,"items":{"type":"string","minLength":1,"maxLength":500}}`

const sendSchema = `{"type":"object","additionalProperties":false,"required":["to","subject","text"],"properties":{
 "to":` + addressList + `,
 "cc":{"type":"array","maxItems":50,"items":{"type":"string","minLength":1,"maxLength":500}},
 "bcc":{"type":"array","maxItems":50,"items":{"type":"string","minLength":1,"maxLength":500}},
 "subject":{"type":"string","maxLength":998},
 "text":{"type":"string","maxLength":1000000},
 "html":{"type":"string","maxLength":1000000},
 "reply_to":{"type":"string","minLength":1,"maxLength":500},
 "in_reply_to":{"type":"string","minLength":1,"maxLength":2000,"description":"CEL that gives the Message-ID of the message to answer (event.messageId): the new message is threaded under it."}}}`

// ---------------------------------------------------------------------------------------------
// email.send

type sendNode struct{ *hub }

func (n *sendNode) Requires() []catalog.ConnectorKind { return nil }

func (n *sendNode) Spec() graph.NodeSpec {
	return graph.NodeSpec{
		Type: NodeSend, Title: "Email: send a message", Behavior: graph.BehaviorCall, DryRun: graph.DryRunCapture,
		Description: "Sends a message over SMTP. Its Message-ID is made before sending, so a reply can be waited for even when the answer of the server was lost.",
		Params:      graph.MustSchemaParams(sendSchema),
		Templates:   []string{"to[]", "cc[]", "bcc[]", "subject", "text", "html", "reply_to"},
		Values:      []string{"in_reply_to"},
		Outcomes:    []string{outcomeSent, outcomeFailed}, DoneOutcome: outcomeSent, FailedOutcome: outcomeFailed,
		Output: expr.Fields{
			{Name: "message_id", Field: expr.StringField},
			{Name: "accepted", Field: expr.ListOf(expr.StringField)},
			{Name: "rejected", Field: expr.ListOf(expr.StringField)},
		},
	}
}

// Idempotent is false: a message whose answer was lost may have been delivered; it is never repeated
// blindly (4xx is unavailable, and a drop after the message was written is outcome_unknown).
func (n *sendNode) Idempotent() bool { return false }

func (n *sendNode) Execute(ctx context.Context, in catalog.CallInput) (catalog.CallResult, error) {
	a, err := n.parseSMTP(in.Credentials)
	if err != nil {
		return catalog.CallResult{}, &catalog.CallError{Code: catalog.CallInvalidRequest, Message: err.Error()}
	}
	to, err := recipients(in.Params["to"], "to", true)
	if err != nil {
		return catalog.CallResult{}, err
	}
	cc, err := recipients(in.Params["cc"], "cc", false)
	if err != nil {
		return catalog.CallResult{}, err
	}
	bcc, err := recipients(in.Params["bcc"], "bcc", false)
	if err != nil {
		return catalog.CallResult{}, err
	}
	var replyTo string
	if s := str(in.Params["reply_to"]); s != "" {
		if replyTo = mailboxAddress(s); replyTo == "" {
			return catalog.CallResult{}, &catalog.CallError{Code: catalog.CallInvalidRequest, Message: "reply_to is not an email address"}
		}
	}
	inReplyTo, err := messageIDArg(in.Params["in_reply_to"], "in_reply_to")
	if err != nil {
		return catalog.CallResult{}, err
	}
	msg := outgoing{
		From: mail.Address{Name: a.fromName, Address: mailboxAddress(a.fromAddress)}, To: to, Cc: cc, ReplyTo: replyTo,
		Subject: str(in.Params["subject"]), Text: str(in.Params["text"]), HTML: str(in.Params["html"]),
		MessageID: messageIDFor(in.IdempotencyKey, a.fromAddress), InReplyTo: inReplyTo, Date: n.now(),
	}
	raw, err := buildMessage(msg)
	if err != nil {
		return catalog.CallResult{}, &catalog.CallError{Code: catalog.CallInvalidRequest, Message: "the message cannot be built"}
	}
	token, err := n.callToken(ctx, in, a)
	if err != nil {
		return catalog.CallResult{}, err
	}
	all := dedupe(append(append(append([]string(nil), to...), cc...), bcc...))
	n.seedBaselines(ctx, in.TenantID, msg.From.Address)
	res, err := n.deliver(ctx, a, token, msg.From.Address, all, raw)
	if err != nil {
		var ce *catalog.CallError
		if a.oauth() && errors.As(err, &ce) && ce.Code == catalog.CallUnauthorized && in.Connection != nil {
			n.refused(ctx, in)
		}
		return catalog.CallResult{}, err
	}
	n.record(ctx, in)
	return catalog.CallResult{Outcome: outcomeSent, Output: map[string]any{
		"message_id": msg.MessageID, "accepted": strs(res.accepted), "rejected": strs(res.rejected),
	}}, nil
}

// refused marks the other connection of the sign-in the server refused the token of.
func (n *sendNode) refused(ctx context.Context, in catalog.CallInput) {
	if id, err := uuidOf(in.Connection.ID); err == nil {
		n.markLogin(ctx, in.TenantID, id, in.Credentials)
	}
}

// record counts a sent message (one, however many recipients).
func (n *sendNode) record(ctx context.Context, in catalog.CallInput) {
	if in.Usage == nil || in.Connection == nil {
		return
	}
	run := in.RunID
	err := in.Usage.Record(ctx, catalog.UsageEntry{
		TenantID: in.TenantID, Kind: KindSMTP, ConnectionID: in.Connection.ID, Unit: "message", Quantity: 1, RunID: &run, At: n.now(),
	})
	if err != nil {
		n.log().Warn("email: a sent message could not be counted", "error", err.Error())
	}
}

// recipients are the bare addresses of a list param; a value that is not an address is refused.
func recipients(v any, name string, required bool) ([]string, error) {
	list, _ := v.([]any)
	if len(list) == 0 {
		if required {
			return nil, &catalog.CallError{Code: catalog.CallInvalidRequest, Message: name + " has no address"}
		}
		return nil, nil
	}
	if len(list) > maxRecipients {
		return nil, &catalog.CallError{Code: catalog.CallInvalidRequest, Message: name + " has more than 50 addresses"}
	}
	out := make([]string, 0, len(list))
	for _, item := range list {
		addr := mailboxAddress(str(item))
		if addr == "" {
			return nil, &catalog.CallError{Code: catalog.CallInvalidRequest, Message: name + " has a value that is not an email address"}
		}
		out = append(out, addr)
	}
	return out, nil
}

// messageIDArg is a Message-ID param (a value of CEL), in angle brackets; empty when it is not set.
func messageIDArg(v any, name string) (string, error) {
	s, ok := v.(string)
	if !ok || strings.TrimSpace(s) == "" {
		if v != nil && !ok {
			return "", &catalog.CallError{Code: catalog.CallInvalidRequest, Message: name + " is not a Message-ID"}
		}
		return "", nil
	}
	id := bracket(s)
	if strings.ContainsAny(id, " \t\r\n") || !strings.Contains(id, "@") {
		return "", &catalog.CallError{Code: catalog.CallInvalidRequest, Message: name + " is not a Message-ID"}
	}
	return id, nil
}

func dedupe(list []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(list))
	for _, a := range list {
		if k := strings.ToLower(a); !seen[k] {
			seen[k] = true
			out = append(out, a)
		}
	}
	return out
}

func strs(l []string) []any {
	out := make([]any, len(l))
	for i, s := range l {
		out[i] = s
	}
	return out
}

func str(v any) string {
	s, _ := v.(string)
	return s
}

// ---------------------------------------------------------------------------------------------
// email.wait_reply

const waitSchema = `{"type":"object","additionalProperties":false,"required":["message_id"],"properties":{
 "message_id":{"type":"string","minLength":1,"maxLength":2000,"description":"CEL that gives the Message-ID of the message a reply is waited for: nodes.<send>.output.message_id."},
 "thread_id":{"type":"string","minLength":1,"maxLength":2000,"description":"CEL that gives the Message-ID of the first message of the thread, when the message was sent as an answer to another (event.threadId of the incoming message): a reply names the first message of its thread, not the one it answers."},
 "from":{"type":"string","minLength":1,"maxLength":1000,"description":"Wait for a reply from this address only; a template ({{ nodes.x.output.address }}). Case does not matter."},
 "timeout":{"type":"string","pattern":"^[1-9][0-9]{0,5}(s|m|h|d)$","x-max-ms":2592000000,"default":"3d","description":"30s, 20m, 2h or 7d; at most 30 days"}}}`

type waitReplyNode struct {
	*hub
	params graph.ParamsSpec
}

var (
	_ engine.WaitKeyer  = (*waitReplyNode)(nil)
	_ engine.EffectNode = (*waitReplyNode)(nil)
)

func newWaitReplyNode(h *hub) *waitReplyNode {
	return &waitReplyNode{hub: h, params: waitParams{inner: graph.MustSchemaParams(waitSchema)}}
}

func (n *waitReplyNode) Requires() []catalog.ConnectorKind { return nil }

func (n *waitReplyNode) Spec() graph.NodeSpec {
	return graph.NodeSpec{
		Type: NodeWaitReply, Title: "Email: wait for a reply", Behavior: graph.BehaviorWait, DryRun: graph.DryRunCapture,
		Description: "Waits for a reply to a message the mailbox sent: the reply's headers (References, In-Reply-To) name it. The reply is the node's output.",
		Params:      n.params,
		Templates:   []string{"from"},
		Values:      []string{"message_id", "thread_id"},
		Wait: &graph.WaitSpec{
			Event: graph.ValueOrParam{Value: EventReceived}, Filter: graph.ValueOrParam{Param: "filter"}, TimeoutParam: "timeout",
			Matched: outcomeReplied, TimeoutOutcome: outcomeTimeout,
			Effect: true, // the params are rendered when the wait opens: the message id is an expression
		},
		Output: expr.Fields{{Name: "event", Field: expr.ObjectOf(receivedPayload())}},
	}
}

// WaitSubject is the conversation of the message: "<imap connection id>:<Message-ID>". The reply names
// its thread's first message, which is the message itself unless it was sent as an answer (thread_id).
func (n *waitReplyNode) WaitSubject(params map[string]any) (engine.SubjectRef, bool) {
	conn, _ := params[catalog.ConnectionParam].(string)
	id := str(params["thread_id"])
	if id == "" {
		id = str(params["message_id"])
	}
	if _, err := uuidOf(conn); err != nil || strings.TrimSpace(id) == "" {
		return engine.SubjectRef{}, false
	}
	return engine.SubjectRef{Entity: graph.SubjectConversation, Key: ConversationKey(conn, id)}, true
}

// ApplyEffect has nothing to do: the node only waits; its effect is the rendering of its params.
func (n *waitReplyNode) ApplyEffect(context.Context, pgx.Tx, engine.StepContext, graph.Node, graph.Effect) (map[string]any, error) {
	return map[string]any{}, nil
}

// waitParams are the params the author writes and the filter derived from them: the incoming message is in
// the thread, answers the message, and comes from the address when one is named.
type waitParams struct{ inner graph.ParamsSpec }

func (p waitParams) JSONSchema() json.RawMessage { return p.inner.JSONSchema() }

var mentionsEvent = regexp.MustCompile(`\bevent\b`)

func (p waitParams) Parse(raw map[string]any) (map[string]any, []graph.Issue) {
	out, issues := p.inner.Parse(raw)
	if len(issues) > 0 {
		return nil, issues
	}
	thread := str(out["thread_id"])
	if thread == "" {
		thread = str(out["message_id"])
	}
	// In the wait's filter, event is the incoming message, not the run's trigger: an id has to come from a
	// node's output.
	for _, name := range []string{"message_id", "thread_id"} {
		if mentionsEvent.MatchString(str(out[name])) {
			return nil, []graph.Issue{{Path: []any{name}, Message: "in the wait's filter, event is the incoming message: read the id from nodes.<id>.output (the output of email.send), or from the trigger's event.threadId through an earlier node"}}
		}
	}
	// The thread routes the wait; the reply must also answer the message itself: the direct parent a client
	// names in In-Reply-To, and, when it names none, the last of References (the message it answers). An
	// older message in References is only history: a reply to B in a conversation of A and B (References
	// root, A, B) is B's reply, not A's. Two runs that send in one conversation are woken each by its own.
	msg := "string(" + str(out["message_id"]) + ")"
	terms := []string{
		"event.threadId == string(" + thread + ")",
		`(event.inReplyTo != "" ? event.inReplyTo == ` + msg + ` : (size(event.references) > 0 && event.references[size(event.references) - 1] == ` + msg + `))`,
	}
	if from := str(out["from"]); from != "" {
		cel, err := templateToCEL(from)
		if err != nil {
			return nil, []graph.Issue{{Path: []any{"from"}, Message: err.Error()}}
		}
		if mentionsEvent.MatchString(cel) {
			return nil, []graph.Issue{{Path: []any{"from"}, Message: "in the wait's filter, event is the incoming message: read the address from nodes.<id>.output"}}
		}
		terms = append(terms, "event.from.address.lowerAscii() == string("+cel+").lowerAscii()")
	}
	out["filter"] = strings.Join(terms, " && ")
	return out, nil
}

// templateToCEL is a template as the CEL expression of its text: the literal parts quoted, each {{ }}
// part as string(…).
func templateToCEL(text string) (string, error) {
	var parts []string
	at := 0
	for _, e := range expr.TemplateExpressions(text) {
		if e.Start > at {
			parts = append(parts, strconv.Quote(text[at:e.Start]))
		}
		if strings.TrimSpace(e.Source) == "" {
			return "", errors.New("an empty {{ }}")
		}
		parts = append(parts, "string("+e.Source+")")
		at = e.End
	}
	if at < len(text) {
		parts = append(parts, strconv.Quote(text[at:]))
	}
	if len(parts) == 0 {
		return `""`, nil
	}
	return "(" + strings.Join(parts, " + ") + ")", nil
}

// ---------------------------------------------------------------------------------------------
// email.label

const labelSchema = `{"type":"object","additionalProperties":false,"required":["message_id"],"properties":{
 "message_id":{"type":"string","minLength":1,"maxLength":2000,"description":"CEL that gives the Message-ID of the message (event.messageId)."},
 "mark_seen":{"type":"boolean","description":"Mark the message as read."},
 "keywords":{"type":"array","maxItems":20,"items":{"type":"string","pattern":"^[A-Za-z0-9_.$-]{1,50}$"},"x-unique-ci":true,"description":"IMAP keywords (labels) to put on the message."},
 "move_to":{"type":"string","minLength":1,"maxLength":200,"description":"A mailbox to move the message to."}}}`

type labelNode struct{ *hub }

func (n *labelNode) Requires() []catalog.ConnectorKind { return nil }

func (n *labelNode) Spec() graph.NodeSpec {
	return graph.NodeSpec{
		Type: NodeLabel, Title: "Email: mark or move a message", Behavior: graph.BehaviorCall, DryRun: graph.DryRunCapture,
		Description: "Marks a message of the mailbox as read, puts keywords on it and moves it to another mailbox. not_found when the mailbox has no such message.",
		Params:      graph.MustSchemaParams(labelSchema),
		Values:      []string{"message_id"},
		Outcomes:    []string{outcomeDone, outcomeMissing, outcomeFailed}, DoneOutcome: outcomeDone, FailedOutcome: outcomeFailed,
	}
}

// Idempotent: setting flags and moving to a mailbox give the same state when repeated.
func (n *labelNode) Idempotent() bool { return true }

func (n *labelNode) Execute(ctx context.Context, in catalog.CallInput) (catalog.CallResult, error) {
	a, err := n.parseIMAP(in.Credentials)
	if err != nil {
		return catalog.CallResult{}, &catalog.CallError{Code: catalog.CallInvalidRequest, Message: err.Error()}
	}
	id, err := messageIDArg(in.Params["message_id"], "message_id")
	if err != nil || id == "" {
		return catalog.CallResult{}, &catalog.CallError{Code: catalog.CallInvalidRequest, Message: "message_id is not a Message-ID"}
	}
	var flags []imap.Flag
	if seen, _ := in.Params["mark_seen"].(bool); seen {
		flags = append(flags, imap.FlagSeen)
	}
	for _, k := range strsOf(in.Params["keywords"]) {
		if strings.HasPrefix(k, `\`) {
			return catalog.CallResult{}, &catalog.CallError{Code: catalog.CallInvalidRequest, Message: "a keyword is not a system flag"}
		}
		flags = append(flags, imap.Flag(k))
	}
	dest := strings.TrimSpace(str(in.Params["move_to"]))
	token, err := n.callToken(ctx, in, a)
	if err != nil {
		return catalog.CallResult{}, err
	}
	s, err := n.openIMAP(ctx, a, token)
	if err != nil {
		return catalog.CallResult{}, n.callError(ctx, in, a, err)
	}
	defer s.close()
	if _, err := s.selectMailbox(a.mailbox, false); err != nil {
		return catalog.CallResult{}, n.callError(ctx, in, a, err)
	}
	found, err := s.c.UIDSearch(&imap.SearchCriteria{Header: []imap.SearchCriteriaHeaderField{{Key: "Message-ID", Value: id}}, NotFlag: []imap.Flag{imap.FlagDeleted}}, nil).Wait()
	if err != nil {
		return catalog.CallResult{}, n.callError(ctx, in, a, err)
	}
	uids := found.AllUIDs()
	if len(uids) == 0 {
		return catalog.CallResult{Outcome: outcomeMissing, Output: map[string]any{}}, nil
	}
	set := imap.UIDSetNum(uids...)
	if len(flags) > 0 {
		if err := s.c.Store(set, &imap.StoreFlags{Op: imap.StoreFlagsAdd, Silent: true, Flags: flags}, nil).Close(); err != nil {
			return catalog.CallResult{}, n.callError(ctx, in, a, err)
		}
	}
	if dest != "" && dest != a.mailbox {
		if err := s.moveTo(set, a.mailbox, dest, id); err != nil {
			return catalog.CallResult{}, n.callError(ctx, in, a, err)
		}
	}
	return catalog.CallResult{Outcome: outcomeDone, Output: map[string]any{}}, nil
}

// callError is the CallError of a failure of an IMAP call: a refusal of the login is unauthorized, a
// mailbox the server does not have is invalid_request, anything else is the server being unavailable.
func (n *labelNode) callError(ctx context.Context, in catalog.CallInput, a account, err error) error {
	var ce *catalog.CallError
	if errors.As(err, &ce) {
		if a.oauth() && ce.Code == catalog.CallUnauthorized && in.Connection != nil {
			if id, perr := uuidOf(in.Connection.ID); perr == nil {
				n.markLogin(ctx, in.TenantID, id, in.Credentials)
			}
		}
		return ce
	}
	var ie *imap.Error
	if errors.As(err, &ie) && ie.Type == imap.StatusResponseTypeNo {
		return &catalog.CallError{Code: catalog.CallInvalidRequest, Message: "the server refused the command"}
	}
	return &catalog.CallError{Code: catalog.CallUnavailable, Message: fmt.Sprint("the IMAP server could not be reached"), Retryable: true}
}

func strsOf(v any) []string {
	list, _ := v.([]any)
	out := make([]string, 0, len(list))
	for _, item := range list {
		if s := strings.TrimSpace(str(item)); s != "" {
			out = append(out, s)
		}
	}
	return out
}
