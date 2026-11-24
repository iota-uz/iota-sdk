package email

import (
	"encoding/json"
	"strings"

	"github.com/iota-uz/iota-sdk/pkg/automation/catalog"
	"github.com/iota-uz/iota-sdk/pkg/automation/engine"
	"github.com/iota-uz/iota-sdk/pkg/automation/expr"
	"github.com/iota-uz/iota-sdk/pkg/automation/graph"
)

// EventReceived is a message that arrived in a mailbox of the tenant.
const EventReceived = "email.received.v1"

// HookSubjectContains is the match hook of the trigger's subject_contains.
const HookSubjectContains = "email.subject_contains"

// MatchHooks are the trigger matching hooks the connector needs; the application merges them into
// engine.Config.MatchHooks (a graph with an email trigger is refused without them).
func MatchHooks() map[string]func(params map[string]any, event graph.Event) bool {
	return map[string]func(map[string]any, graph.Event) bool{
		HookSubjectContains: func(params map[string]any, ev graph.Event) bool {
			want, _ := params["subject_contains"].(string)
			if strings.TrimSpace(want) == "" {
				return true
			}
			subject, _ := ev.Payload()["subject"].(string)
			return strings.Contains(strings.ToLower(subject), strings.ToLower(strings.TrimSpace(want)))
		},
	}
}

// ConversationKey is the key of the conversation of a message: "<imap connection id>:<thread root
// Message-ID>". The wait of email.wait_reply listens on the key of the message it sent.
func ConversationKey(connectionID, messageID string) string {
	return connectionID + ":" + bracket(messageID)
}

var (
	optString = expr.OptionalOf(expr.StringField)

	personField = expr.ObjectOf(expr.Fields{
		{Name: "address", Field: expr.StringField},
		{Name: "name", Field: optString},
	})

	attachmentField = expr.ObjectOf(expr.Fields{
		{Name: "filename", Field: expr.StringField},
		{Name: "contentType", Field: expr.StringField},
		{Name: "size", Field: expr.IntField},
		{Name: "upload_id", Field: optString},
		{Name: "url", Field: optString},
		{Name: "error", Field: optString},
	})
)

func receivedPayload() expr.Fields {
	return expr.Fields{
		{Name: "connectionId", Field: expr.StringField},
		{Name: "mailbox", Field: expr.StringField},
		{Name: "messageId", Field: expr.StringField},
		{Name: "inReplyTo", Field: optString},
		// threadId is the Message-ID that names the conversation: the first of references, else inReplyTo,
		// else the message's own.
		{Name: "threadId", Field: expr.StringField},
		{Name: "references", Field: expr.ListOf(expr.StringField)},
		{Name: "from", Field: personField},
		{Name: "to", Field: expr.ListOf(personField)},
		{Name: "cc", Field: expr.ListOf(personField)},
		{Name: "subject", Field: expr.StringField},
		{Name: "text", Field: expr.StringField},
		{Name: "hasHtml", Field: expr.BoolField},
		// isReply and isAuto are what skip_replies and skip_auto test.
		{Name: "isReply", Field: expr.BoolField},
		{Name: "isAuto", Field: expr.BoolField},
		{Name: "attachments", Field: expr.ListOf(attachmentField)},
		{Name: "receivedAt", Field: expr.TimestampField},
	}
}

func receivedMatch() map[string]graph.MatchField {
	return map[string]graph.MatchField{
		"from":     {Payload: "from.address", Type: graph.MatchStrings, Fold: graph.FoldLower},
		"is_reply": {Payload: "isReply", Type: graph.MatchBool},
		"is_auto":  {Payload: "isAuto", Type: graph.MatchBool},
	}
}

const triggerSchema = `{"type":"object","additionalProperties":false,"properties":{
 "from":{"type":"array","maxItems":50,"x-unique-ci":true,"items":{"type":"string","minLength":3,"maxLength":254,"pattern":"^\\S(.*\\S)?$"},
   "description":"Start only on mail from these addresses (case does not matter)."},
 "subject_contains":{"type":"string","minLength":1,"maxLength":200,"description":"Start only when the subject contains this text (case does not matter)."},
 "skip_replies":{"type":"boolean","default":true,"description":"Skip messages in a thread: with an In-Reply-To or References header, or a subject that starts with Re: or Ответ:."},
 "skip_auto":{"type":"boolean","default":true,"description":"Skip automatic mail: Auto-Submitted, List-Id (a mailing list), Precedence: bulk."}}}`

// triggerParams are the options an author sets; skip_replies and skip_auto become the match fields
// is_reply and is_auto (false: only a message that is not one).
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
	if skip, _ := out["skip_replies"].(bool); skip {
		out["is_reply"] = false
	}
	if skip, _ := out["skip_auto"].(bool); skip {
		out["is_auto"] = false
	}
	return out, nil
}

// triggers is the trigger source of the imap connector; the registry adds the connector's kind to what it
// requires.
type triggers struct{}

func (triggers) Requires() []catalog.ConnectorKind { return nil }

func (triggers) Events() []graph.EventSpec {
	return []graph.EventSpec{{
		Key: EventReceived, Title: "Email: message received", Group: "email", Subject: graph.SubjectConversation,
		Description: "A message that arrived in the mailbox after it was connected. The conversation is the thread's first Message-ID.",
		Payload:     receivedPayload(), TriggerParams: newTriggerParams(), Match: receivedMatch(), MatchHook: HookSubjectContains,
	}}
}

var _ catalog.TriggerSource = triggers{}

// subjectOf is the subject of the event of a message.
func subjectOf(connectionID string, m received, fallback string) engine.SubjectRef {
	root := threadRoot(m)
	if root == "" {
		root = fallback
	}
	return engine.SubjectRef{Entity: graph.SubjectConversation, Key: connectionID + ":" + root}
}
