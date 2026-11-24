package telegram

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/iota-uz/iota-sdk/pkg/automation/catalog"
	"github.com/iota-uz/iota-sdk/pkg/automation/connectors"
	"github.com/iota-uz/iota-sdk/pkg/automation/engine"
	"github.com/iota-uz/iota-sdk/pkg/automation/expr"
	"github.com/iota-uz/iota-sdk/pkg/automation/graph"
)

// Node types of the connector.
const (
	NodeSend = "telegram.send"
	NodeAsk  = "telegram.ask"
	NodeRead = "telegram.read"
)

const (
	outcomeSent     = "sent"
	outcomeFailed   = "failed"
	outcomeTimeout  = "timeout"
	outcomeReceived = "received"

	// defaultAsk is how long a question waits for an answer.
	defaultAsk = 24 * time.Hour
)

// chatSchema is a chat of a node: a number (a chat id) or a string, which is an expression: CEL that gives
// the id (event.chatId) or a quoted @username.
const chatSchema = `{"oneOf":[{"type":"integer"},{"type":"string","minLength":1,"maxLength":1000}],"x-options-source":"telegram.chats",
 "description":"A chat id, or an expression that gives one ({{ }} is not needed: event.chatId). A channel or a group by @username: \"@name\" in quotes."}`

// readChatSchema is the chat of telegram.read: as chatSchema, but the wait is keyed by the numeric id of the
// chat, so a chat by @username is not offered.
const readChatSchema = `{"oneOf":[{"type":"integer"},{"type":"string","minLength":1,"maxLength":1000}],"x-options-source":"telegram.chats",
 "description":"A chat id, or an expression that gives one ({{ }} is not needed: event.chatId). The wait is in the chat by its numeric id: a chat by @username is not accepted."}`

// threadSchema is a forum topic: its number, or an expression that gives it. A topic is not a chat, so there is
// no chat picker for it.
const threadSchema = `{"oneOf":[{"type":"integer","minimum":1},{"type":"string","minLength":1,"maxLength":1000}],
 "description":"The number of a topic of a forum supergroup, or an expression that gives it; left out: the general chat."}`

const durationSchema = `{"type":"string","pattern":"^[1-9][0-9]{0,5}(s|m|h|d)$","x-max-ms":2592000000,"description":"30s, 20m, 2h or 7d; at most 30 days"}`

const sendSchema = `{"type":"object","additionalProperties":false,"required":["chat_id","text"],"properties":{
 "chat_id":` + chatSchema + `,
 "thread_id":` + threadSchema + `,
 "text":{"type":"string","maxLength":10000},
 "parse_mode":{"type":"string","enum":["","HTML"],"default":""},
 "silent":{"type":"boolean","default":false},
 "url_buttons":{"type":"array","maxItems":10,"items":{"type":"object","additionalProperties":false,"required":["text","url"],"properties":{
   "text":{"type":"string","minLength":1,"maxLength":200},"url":{"type":"string","minLength":1,"maxLength":2000}}}},
 "files":{"type":"array","maxItems":10,"items":{"oneOf":[{"type":"string","minLength":1,"maxLength":100},{"type":"integer"}]},
   "description":"Uploads (their ids) to send as the message's attachments; the text becomes the caption."}}}`

const askSchema = `{"type":"object","additionalProperties":false,"required":["chat_id","text","options"],"properties":{
 "chat_id":` + chatSchema + `,
 "thread_id":` + threadSchema + `,
 "text":{"type":"string","minLength":1,"maxLength":10000},
 "parse_mode":{"type":"string","enum":["","HTML"],"default":""},
 "options":{"type":"array","minItems":2,"maxItems":8,"x-unique":"outcome","items":{"type":"object","additionalProperties":false,"required":["outcome","label"],"properties":{
   "outcome":{"type":"string","pattern":"^[a-z][a-z0-9_]{0,39}$","not":{"enum":["timeout","failed"]}},
   "label":{"type":"string","minLength":1,"maxLength":200}}}},
 "columns":{"type":"integer","minimum":1,"maximum":4,"default":1},
 "timeout":` + durationSchema + `}}`

// Output of the nodes.
var (
	sentOutput = expr.Fields{
		{Name: "chat_id", Field: expr.StringField},
		{Name: "message_id", Field: expr.StringField},
	}
	askOutput = expr.Fields{
		{Name: "choice", Field: expr.StringField},
		{Name: "label", Field: expr.StringField},
		{Name: "from", Field: expr.ObjectOf(expr.Fields{
			{Name: "user_id", Field: expr.StringField},
			{Name: "username", Field: optString},
			{Name: "first_name", Field: expr.StringField},
		})},
		{Name: "answered_at", Field: expr.TimestampField},
		{Name: "message_id", Field: expr.StringField},
	}
)

// nodeBase is what the three nodes share.
type nodeBase struct{ c *Connector }

func (nodeBase) Requires() []catalog.ConnectorKind { return nil }

func (c *Connector) Nodes() []catalog.NodeType {
	b := nodeBase{c}
	return []catalog.NodeType{&sendNode{b}, &askNode{b}, &readNode{b, newReadParams()}}
}

func (c *Connector) Triggers() []catalog.TriggerSource { return []catalog.TriggerSource{triggers{}} }

// ---------------------------------------------------------------------------------------------
// telegram.send

type sendNode struct{ nodeBase }

func (n *sendNode) Spec() graph.NodeSpec {
	return graph.NodeSpec{
		Type: NodeSend, Title: "Telegram: send a message", Behavior: graph.BehaviorCall, DryRun: graph.DryRunCapture,
		Description: "Sends a text, with attachments and link buttons, to a chat of the bot.",
		Params:      sendParams{graph.MustSchemaParams(sendSchema)},
		Templates:   []string{"text", "url_buttons[].text", "url_buttons[].url"},
		Values:      []string{"chat_id", "thread_id"},
		Outcomes:    []string{outcomeSent, outcomeFailed}, DoneOutcome: outcomeSent, FailedOutcome: outcomeFailed,
		Output: sentOutput,
	}
}

// Idempotent is false: a send whose answer was lost may have been delivered; it is never repeated blindly.
func (n *sendNode) Idempotent() bool { return false }

func (n *sendNode) Execute(ctx context.Context, in catalog.CallInput) (catalog.CallResult, error) {
	chat, thread, err := target(in.Params)
	if err != nil {
		return catalog.CallResult{}, err
	}
	text, _ := in.Params["text"].(string)
	files, err := n.c.attachments(ctx, in.TenantID, in.Params["files"])
	if err != nil {
		return catalog.CallResult{}, err
	}
	m := outgoing{Text: text, ParseMode: str(in.Params["parse_mode"]), Thread: thread, Silent: flag(in.Params["silent"]), Files: files}
	rows, _ := in.Params["url_buttons"].([]any)
	for _, r := range rows {
		b, _ := r.(map[string]any)
		m.Buttons = append(m.Buttons, []Button{{Text: str(b["text"]), URL: str(b["url"])}})
	}
	ref, sent, err := n.c.send(ctx, in.Credentials["token"], chat, m)
	if res, err := n.c.settle(ctx, in, err); err != nil || res != nil {
		return deref(res), err
	}
	n.c.recordUsage(ctx, in, sent)
	return catalog.CallResult{Outcome: outcomeSent, Output: map[string]any{
		"chat_id": strconv.FormatInt(ref.ChatID, 10), "message_id": strconv.FormatInt(ref.MessageID, 10),
	}}, nil
}

// sendParams is the params of telegram.send: the schema, and the one rule across fields: an album of
// files (two or more) has no keyboard.
type sendParams struct{ inner graph.ParamsSpec }

func (p sendParams) JSONSchema() json.RawMessage { return p.inner.JSONSchema() }

func (p sendParams) Parse(raw map[string]any) (map[string]any, []graph.Issue) {
	out, issues := p.inner.Parse(raw)
	if len(issues) > 0 {
		return nil, issues
	}
	files, _ := out["files"].([]any)
	buttons, _ := out["url_buttons"].([]any)
	if len(files) > 1 && len(buttons) > 0 {
		return nil, []graph.Issue{{Path: []any{"url_buttons"}, Message: "an album of several files takes no buttons: send one file with them, or the buttons with a text"}}
	}
	return out, nil
}

// ---------------------------------------------------------------------------------------------
// telegram.ask

type askNode struct{ nodeBase }

var (
	_ engine.Asker         = (*askNode)(nil)
	_ engine.AnswerEventer = (*askNode)(nil)
	_ engine.ParamAnswerer = (*askNode)(nil)
)

func (n *askNode) Spec() graph.NodeSpec {
	return graph.NodeSpec{
		Type: NodeAsk, Title: "Telegram: ask with buttons", Behavior: graph.BehaviorCall, DryRun: graph.DryRunCapture,
		Description:   "Sends a question with buttons and waits for a press: the pressed option is the outcome.",
		Params:        graph.MustSchemaParams(askSchema),
		Templates:     []string{"text", "options[].label"},
		Values:        []string{"chat_id", "thread_id"},
		Outcomes:      []string{outcomeTimeout, outcomeFailed},
		OutcomesFrom:  &graph.ParamRef{Param: "options", Field: "outcome"},
		FailedOutcome: outcomeFailed,
		Output:        askOutput,
	}
}

func (n *askNode) Idempotent() bool { return false }

func (n *askNode) Execute(ctx context.Context, in catalog.CallInput) (catalog.CallResult, error) {
	cor, ok := engine.CorrelationFrom(ctx)
	if !ok {
		return catalog.CallResult{}, &catalog.CallError{Code: catalog.CallInvalidRequest, Message: "a question needs its correlation token"}
	}
	chat, thread, err := target(in.Params)
	if err != nil {
		return catalog.CallResult{}, err
	}
	options, _ := in.Params["options"].([]any)
	columns := 1
	if v, ok := intOf(in.Params["columns"]); ok && v >= 1 && v <= 4 {
		columns = int(v)
	}
	var rows [][]Button
	for i, o := range options {
		m, _ := o.(map[string]any)
		b := Button{Text: str(m["label"]), CallbackData: CallbackData(cor.Token, i)}
		if len(rows) == 0 || len(rows[len(rows)-1]) == columns {
			rows = append(rows, nil)
		}
		rows[len(rows)-1] = append(rows[len(rows)-1], b)
	}
	ref, sent, err := n.c.send(ctx, in.Credentials["token"], chat, outgoing{
		Text: str(in.Params["text"]), ParseMode: str(in.Params["parse_mode"]), Thread: thread, Buttons: rows,
	})
	if res, err := n.c.settle(ctx, in, err); err != nil || res != nil {
		return deref(res), err
	}
	n.c.recordUsage(ctx, in, sent)
	// The outcome is the answer's: Await is what the engine waits on.
	return catalog.CallResult{Output: map[string]any{
		"chat_id": strconv.FormatInt(ref.ChatID, 10), "message_id": strconv.FormatInt(ref.MessageID, 10),
	}}, nil
}

func (n *askNode) Await(in catalog.CallInput, _ catalog.CallResult) (engine.Await, error) {
	timeout := defaultAsk
	if ms, ok := graph.DurationMs(in.Params["timeout"]); ok {
		timeout = time.Duration(ms) * time.Millisecond
	}
	return engine.Await{Event: EventCallback, Timeout: timeout, Single: true}, nil
}

// AnswerEvent names the answer before the question is sent: the press of a button, the first one closes.
func (n *askNode) AnswerEvent(catalog.CallInput) (string, bool) { return EventCallback, true }

// Answer is unused: AnswerFor reads the options of the node.
func (n *askNode) Answer(json.RawMessage) (string, json.RawMessage, error) {
	return "", nil, fmt.Errorf("telegram.ask maps a press through the options of its node")
}

// AnswerFor maps the press (the number of the button) to the option's outcome.
func (n *askNode) AnswerFor(params map[string]any, payload json.RawMessage) (string, json.RawMessage, error) {
	var p struct {
		MessageID  string `json:"messageId"`
		Choice     string `json:"choice"`
		Label      string `json:"label"`
		AnsweredAt string `json:"answeredAt"`
		From       struct {
			UserID    string `json:"userId"`
			Username  string `json:"username"`
			FirstName string `json:"firstName"`
		} `json:"from"`
	}
	if err := json.Unmarshal(payload, &p); err != nil {
		return "", nil, fmt.Errorf("the answer is not a press of a button")
	}
	i, err := strconv.Atoi(p.Choice)
	options, _ := params["options"].([]any)
	if err != nil || i < 0 || i >= len(options) {
		return "", nil, fmt.Errorf("the answer names a button the question does not have")
	}
	opt, _ := options[i].(map[string]any)
	outcome := str(opt["outcome"])
	label := p.Label
	if label == "" {
		label = str(opt["label"])
	}
	from := map[string]any{"user_id": p.From.UserID, "first_name": p.From.FirstName}
	if p.From.Username != "" {
		from["username"] = p.From.Username
	}
	out, err := json.Marshal(map[string]any{
		"choice": outcome, "label": label, "from": from, "answered_at": p.AnsweredAt, "message_id": p.MessageID,
	})
	return outcome, out, err
}

// CallbackData is the data of the i-th button of the question with the engine's correlation token:
// "a:<token>:<i>", at most 64 bytes. The token is what finds the question; there is no table.
func CallbackData(token string, i int) string { return "a:" + token + ":" + strconv.Itoa(i) }

// parseCallback reads "a:<token>:<i>".
func parseCallback(data string) (string, int, bool) {
	parts := strings.Split(data, ":")
	if len(parts) != 3 || parts[0] != "a" || len(parts[1]) != engine.CorrelationTokenLen {
		return "", 0, false
	}
	n, err := strconv.Atoi(parts[2])
	if err != nil || n < 0 || n > 99 {
		return "", 0, false
	}
	return parts[1], n, true
}

// ---------------------------------------------------------------------------------------------
// telegram.read

// readSchema is the params the author writes; the filter is derived from them.
const readSchema = `{"type":"object","additionalProperties":false,"required":["chat_id"],"properties":{
 "chat_id":` + readChatSchema + `,
 "from_user_id":{"oneOf":[{"type":"integer"},{"type":"string","minLength":1,"maxLength":1000}],
   "description":"Wait for this user only: an id, or an expression read when the message arrives (nodes.<id>.output...), where event is the incoming message."},
 "timeout":{"type":"string","pattern":"^[1-9][0-9]{0,5}(s|m|h|d)$","x-max-ms":2592000000,"default":"24h","description":"30s, 20m, 2h or 7d; at most 30 days"}}}`

type readNode struct {
	nodeBase
	params graph.ParamsSpec
}

var (
	_ engine.WaitKeyer  = (*readNode)(nil)
	_ engine.EffectNode = (*readNode)(nil)
)

func (n *readNode) Spec() graph.NodeSpec {
	return graph.NodeSpec{
		Type: NodeRead, Title: "Telegram: wait for a message", Behavior: graph.BehaviorWait, DryRun: graph.DryRunCapture,
		Description: "Waits for a text message in a chat; the message is the node's output.",
		Params:      n.params,
		Values:      []string{"chat_id", "from_user_id"},
		Wait: &graph.WaitSpec{
			Event: graph.ValueOrParam{Value: EventMessage}, Filter: graph.ValueOrParam{Param: "filter"}, TimeoutParam: "timeout",
			Matched: outcomeReceived, TimeoutOutcome: outcomeTimeout,
			Effect: true, // the params are rendered when the wait opens: the chat is an expression
		},
	}
}

// TenantConnectionOnly: only a tenant's own bot receives messages, so a read never works with the platform
// credential.
func (n *readNode) TenantConnectionOnly() bool { return true }

// WaitSubject is the conversation of the chat: "<connection_id>:<chat_id>". Without the connection named
// in the params the run's own conversation is the default, and the filter narrows by chat.
func (n *readNode) WaitSubject(params map[string]any) (engine.SubjectRef, bool) {
	conn, _ := params[catalog.ConnectionParam].(string)
	chat, err := chatArg(params["chat_id"])
	if _, parse := uuid.Parse(conn); parse != nil || err != nil || strings.HasPrefix(chat, "@") {
		return engine.SubjectRef{}, false
	}
	return engine.SubjectRef{Entity: graph.SubjectConversation, Key: conn + ":" + chat}, true
}

// ApplyEffect has nothing to do: the node only waits; its effect is the rendering of its params.
func (n *readNode) ApplyEffect(context.Context, pgx.Tx, engine.StepContext, graph.Node, graph.Effect) (map[string]any, error) {
	return map[string]any{}, nil
}

// readParams is the params of telegram.read: what the author writes, and the filter derived from it —
// event.chatId == chat_id [&& event.from.userId == from_user_id].
type readParams struct{ inner graph.ParamsSpec }

func newReadParams() graph.ParamsSpec { return readParams{inner: graph.MustSchemaParams(readSchema)} }

func (p readParams) JSONSchema() json.RawMessage { return p.inner.JSONSchema() }

// refEnv reads which names an expression refers to; it checks nothing, so it needs no scope.
var refEnv = sync.OnceValue(func() *expr.Env {
	env, err := expr.NewEnv(expr.Scope{}, expr.DefaultLimits())
	if err != nil {
		return nil
	}
	return env
})

// readsEvent reports whether an expression refers to the name event (the run's trigger), as a name and not as a
// word inside a string literal.
func readsEvent(src string) bool {
	env := refEnv()
	if env == nil {
		return true // cannot tell: the safe answer is the refusal
	}
	for _, chain := range env.References(src) {
		if len(chain) > 0 && chain[0] == "event" {
			return true
		}
	}
	return false
}

func (p readParams) Parse(raw map[string]any) (map[string]any, []graph.Issue) {
	out, issues := p.inner.Parse(raw)
	if len(issues) > 0 {
		return nil, issues
	}
	// The wait is keyed by the numeric chat: a chat given by @username cannot name the conversation it waits in.
	if lit, ok := out["chat_id"].(string); ok && usernameRe.MatchString(strings.Trim(strings.TrimSpace(lit), "\"'")) {
		return nil, []graph.Issue{{Path: []any{"chat_id"}, Message: "telegram.read waits in a chat given by its numeric id: a chat by @username has none until it is resolved, so give the id"}}
	}
	// A message without text (a photo without a caption) is an event for triggers, not an answer to a read.
	// The chat is not part of the filter: the keyed subject of the wait names the connection and the chat in
	// canonical form, which is all a comparison of the message's chat would add (and "00042" would not match).
	terms := []string{`event.text != ""`}
	if v, set := out["from_user_id"]; set {
		if s, isExpr := v.(string); isExpr && readsEvent(s) {
			return nil, []graph.Issue{{Path: []any{"from_user_id"},
				Message: "in the wait's filter, event is the incoming message: read the user from nodes.<id>.output (the question's from.user_id), or give an id"}}
		}
		if t, ok := filterTerm("event.from.userId", v); ok {
			terms = append(terms, t)
		}
	}
	out["filter"] = strings.Join(terms, " && ")
	return out, nil
}

// filterTerm is `field == "<id>"` for an id, `field == string(<expression>)` for an expression.
func filterTerm(field string, v any) (string, bool) {
	switch x := v.(type) {
	case string:
		return field + " == string(" + x + ")", true
	case nil:
		return "", false
	}
	id, ok := intOf(v)
	if !ok {
		return "", false
	}
	return field + " == " + strconv.Quote(strconv.FormatInt(id, 10)), true
}

// ---------------------------------------------------------------------------------------------
// helpers

// target is the chat and the topic of a node's params.
func target(params map[string]any) (string, int64, error) {
	chat, err := chatArg(params["chat_id"])
	if err != nil {
		return "", 0, &catalog.CallError{Code: catalog.CallInvalidRequest, Message: "chat_id: " + err.Error()}
	}
	var thread int64
	if v, set := params["thread_id"]; set && v != nil {
		id, ok := intOf(v)
		if !ok || id <= 0 {
			return "", 0, &catalog.CallError{Code: catalog.CallInvalidRequest, Message: "thread_id is the number of a topic"}
		}
		thread = id
	}
	return chat, thread, nil
}

var usernameRe = regexp.MustCompile(`^@[A-Za-z][A-Za-z0-9_]{4,31}$`)

// chatArg is a chat as the Bot API takes it: an id, or @username.
func chatArg(v any) (string, error) {
	if id, ok := intOf(v); ok {
		if id == 0 {
			return "", fmt.Errorf("a chat id is not zero")
		}
		return strconv.FormatInt(id, 10), nil
	}
	s, ok := v.(string)
	if !ok {
		return "", fmt.Errorf("a chat id or @username")
	}
	s = strings.TrimSpace(s)
	if usernameRe.MatchString(s) {
		return s, nil
	}
	return "", fmt.Errorf("a chat id or @username")
}

// intOf reads an integer from a JSON number, or from a string of digits (an id an expression gave).
func intOf(v any) (int64, bool) {
	switch x := v.(type) {
	case json.Number:
		n, err := x.Int64()
		return n, err == nil
	case float64:
		if x != math.Trunc(x) || math.Abs(x) > 1<<53 {
			return 0, false
		}
		return int64(x), true
	case int:
		return int64(x), true
	case int64:
		return x, true
	case string:
		n, err := strconv.ParseInt(strings.TrimSpace(x), 10, 64)
		return n, err == nil
	}
	return 0, false
}

func str(v any) string { s, _ := v.(string); return s }

func flag(v any) bool { b, _ := v.(bool); return b }

func deref(r *catalog.CallResult) catalog.CallResult {
	if r == nil {
		return catalog.CallResult{}
	}
	return *r
}

// settle turns the failure of a send into what a call node returns: nil result and nil error when the
// send succeeded; a result with RetryAfter for 429; a CallError for a refusal. An error whose outcome is
// unknown is returned as it is: the engine records outcome_unknown, never a second message.
func (c *Connector) settle(ctx context.Context, in catalog.CallInput, err error) (*catalog.CallResult, error) {
	if err == nil {
		return nil, nil //nolint:nilnil // nothing to settle: the send succeeded
	}
	err = callFailure(err)
	var wait *RetryAfterError
	var ce *catalog.CallError
	switch {
	case errors.As(err, &wait):
		return &catalog.CallResult{RetryAfter: wait.After}, nil
	case errors.As(err, &ce) && ce.Code == catalog.CallUnauthorized:
		c.needsReauth(ctx, in)
	}
	return nil, err
}

// needsReauth marks the connection whose token Telegram refused; the platform bot has no connection. The mark
// is made only while the connection still holds the token that was refused: a save that replaced the token
// while the send was in flight must not have its working replacement marked by the old token's 401 (the
// version the credentials were read at gates the write).
func (c *Connector) needsReauth(ctx context.Context, in catalog.CallInput) {
	if c.cfg.Connections == nil || in.Connection == nil || in.Connection.Platform {
		return
	}
	id, err := uuid.Parse(in.Connection.ID)
	if err != nil {
		return
	}
	current, creds, err := c.cfg.Connections.Get(ctx, in.TenantID, id)
	if err != nil || creds["token"] != in.Credentials["token"] {
		return
	}
	// "reauth" is the reason the integrations page explains as «enter the token again».
	_, err = c.cfg.Connections.SetStatusIf(ctx, in.TenantID, id, connectors.StatusNeedsReauth, "reauth",
		func(_ json.RawMessage, version int) bool { return version == current.Version })
	if err != nil {
		c.log().Warn("telegram: a refused token was not recorded", "error", err.Error())
	}
}

// recordUsage counts the messages a send delivered (an album is several): for the platform bot and for the
// tenant's own, apart.
func (c *Connector) recordUsage(ctx context.Context, in catalog.CallInput, delivered int) {
	if in.Usage == nil || in.Connection == nil || delivered <= 0 {
		return
	}
	run := in.RunID
	err := in.Usage.Record(ctx, catalog.UsageEntry{
		TenantID: in.TenantID, Kind: Kind, ConnectionID: in.Connection.ID, Unit: "message", Quantity: int64(delivered), RunID: &run, At: time.Now(),
	})
	if err != nil {
		c.log().Warn("telegram: a message was not counted", "error", err.Error())
	}
}
