package telegram

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/iota-uz/iota-sdk/pkg/automation/catalog"
	"github.com/iota-uz/iota-sdk/pkg/automation/expr"
	"github.com/iota-uz/iota-sdk/pkg/automation/graph"
)

// Events of the connector.
const (
	// EventMessage is a message a tenant's own bot received.
	EventMessage = "telegram.message_received.v1"
	// EventCallback is a press of an inline button of an ask: it only wakes the ask.
	EventCallback = "telegram.callback_answered.v1"
)

var (
	optString = expr.OptionalOf(expr.StringField)

	fromField = expr.ObjectOf(expr.Fields{
		{Name: "userId", Field: expr.StringField},
		{Name: "username", Field: optString},
		{Name: "firstName", Field: expr.StringField},
		{Name: "lastName", Field: optString},
	})

	// fileField is an attachment: stored in the uploads of the SDK, or the reason it was not.
	fileField = expr.ObjectOf(expr.Fields{
		{Name: "upload_id", Field: optString},
		{Name: "name", Field: expr.StringField},
		{Name: "mime", Field: optString},
		{Name: "size", Field: expr.IntField},
		{Name: "url", Field: optString},
		{Name: "error", Field: optString},
	})
)

func messagePayload() expr.Fields {
	return expr.Fields{
		{Name: "connectionId", Field: expr.StringField},
		{Name: "chatId", Field: expr.StringField},
		{Name: "threadId", Field: optString},
		{Name: "chatType", Field: expr.EnumOf("private", "group", "supergroup", "channel")},
		{Name: "chatTitle", Field: optString},
		{Name: "messageId", Field: expr.StringField},
		{Name: "from", Field: fromField},
		{Name: "text", Field: optString},
		{Name: "command", Field: optString},
		{Name: "args", Field: optString},
		{Name: "isCommand", Field: expr.BoolField},
		// route is "text" for a message that is not a command and "command:<name>" for one: what the
		// command options of a trigger are matched by (the matches of a trigger are independent, and
		// "plain messages, and these commands" is one of them).
		{Name: "route", Field: expr.StringField},
		{Name: "hasMedia", Field: expr.BoolField},
		{Name: "files", Field: expr.OptionalOf(expr.ListOf(fileField))},
		{Name: "date", Field: expr.TimestampField},
	}
}

func callbackPayload() expr.Fields {
	return expr.Fields{
		{Name: "chatId", Field: expr.StringField},
		{Name: "messageId", Field: expr.StringField},
		// choice is the number of the pressed button among the options of the ask ("0", "1", …); the ask
		// node maps it to the outcome it declared.
		{Name: "choice", Field: expr.StringField},
		{Name: "label", Field: expr.StringField},
		{Name: "from", Field: fromField},
		{Name: "answeredAt", Field: expr.TimestampField},
	}
}

// Match fields of the message trigger. The trigger's params (triggerParams) are mapped onto them.
func messageMatch() map[string]graph.MatchField {
	return map[string]graph.MatchField{
		"chat_ids":         {Payload: "chatId", Type: graph.MatchStrings},
		"route_filter":     {Payload: "route", Type: graph.MatchStrings, Fold: graph.FoldLower},
		"command_mode":     {Payload: "isCommand", Type: graph.MatchStrings},
		"chat_type_filter": {Payload: "chatType", Type: graph.MatchStrings},
	}
}

const triggerSchema = `{"type":"object","additionalProperties":false,"properties":{
 "commands":{"type":"string","enum":["ignore","only","include"],"default":"ignore",
   "description":"ignore: plain messages only; only: commands only; include: both."},
 "command_names":{"type":"array","maxItems":50,"items":{"type":"string","pattern":"^[A-Za-z0-9_]{1,32}$"},
   "description":"With only: start only on these commands; with include: on plain messages and these commands (without the slash)."},
 "chat_ids":{"type":"array","maxItems":100,"items":{"type":"string","pattern":"^-?[0-9]{1,18}$"},
   "description":"Start only on messages of these chats."},
 "private_only":{"type":"boolean","description":"Start only on messages in private chats."}}}`

// triggerParams is the params of the message trigger: the author's four options, and the match fields the
// event's payload is matched by, derived from them.
type triggerParams struct{ inner graph.ParamsSpec }

func newTriggerParams() graph.ParamsSpec {
	return triggerParams{inner: graph.MustSchemaParams(triggerSchema)}
}

func (p triggerParams) JSONSchema() json.RawMessage { return p.inner.JSONSchema() }

func (p triggerParams) Parse(raw map[string]any) (map[string]any, []graph.Issue) {
	out, issues := p.inner.Parse(raw)
	if len(issues) > 0 {
		return nil, issues
	}
	// A chat id is matched as the webhook writes it: in canonical decimal form ("00042" is "42").
	if ids, _ := out["chat_ids"].([]any); len(ids) > 0 {
		canon := make([]any, 0, len(ids))
		for i, v := range ids {
			n, err := strconv.ParseInt(fmt.Sprint(v), 10, 64)
			if err != nil || n == 0 {
				return nil, []graph.Issue{{Path: []any{"chat_ids", i}, Message: "a chat id is a non-zero integer"}}
			}
			canon = append(canon, strconv.FormatInt(n, 10))
		}
		out["chat_ids"] = canon
	}
	var routes []any
	if names, _ := out["command_names"].([]any); len(names) > 0 {
		for _, n := range names {
			routes = append(routes, "command:"+strings.ToLower(fmt.Sprint(n)))
		}
	}
	switch out["commands"] {
	case "ignore":
		// Plain messages only: the names of commands have nothing to narrow.
		out["command_mode"] = []any{"false"}
	case "only":
		out["command_mode"] = []any{"true"}
		if len(routes) > 0 {
			out["route_filter"] = routes // these commands
		}
	case "include":
		if len(routes) > 0 {
			out["route_filter"] = append([]any{"text"}, routes...) // plain messages, and these commands
		}
	}
	if private, _ := out["private_only"].(bool); private {
		out["chat_type_filter"] = []any{"private"}
	}
	return out, nil
}

type triggers struct{}

func (triggers) Requires() []catalog.ConnectorKind { return nil }

// TenantConnectionOnly: a message to the platform bot belongs to no tenant, so only a tenant's own bot
// delivers the message trigger.
func (triggers) TenantConnectionOnly(key string) bool { return key == EventMessage }

func (triggers) Events() []graph.EventSpec {
	return []graph.EventSpec{
		{
			Key: EventMessage, Title: "Telegram: message received", Group: "telegram", Subject: graph.SubjectConversation,
			Description:   "A message to a bot of the tenant's own. Commands (/start) are left out unless the trigger takes them.",
			Payload:       messagePayload(),
			TriggerParams: newTriggerParams(),
			Match:         messageMatch(),
		},
		{
			Key: EventCallback, Title: "Telegram: button pressed", Group: "telegram", Subject: graph.SubjectConversation,
			Description: "A press of a button of a question; it wakes the question and starts nothing.",
			Payload:     callbackPayload(), NoTrigger: true,
		},
	}
}
