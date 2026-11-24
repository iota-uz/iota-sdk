// Package sms is the SMS connectors of automation graphs: Eskiz and Twilio, each with a "Send SMS" node
// (eskiz.send_sms, twilio.send_sms) that works with a connection of its kind, the tenant's own or the
// platform's from the environment.
//
// The network layer is not rewritten: Eskiz goes through pkg/eskiz and Twilio through twilio-go. Neither
// sends an idempotency key, so the nodes are not idempotent: a send whose outcome is unknown (the request
// left and no answer came) ends as failed with outcome_unknown and is never repeated. The only repeat is
// after an answer that guarantees the SMS was not accepted: 429 or 408.
//
// Incoming SMS and delivery statuses are out of scope: the node reports that the provider accepted the
// message.
package sms

import (
	"context"
	"net/http"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/google/uuid"

	"github.com/iota-uz/iota-sdk/pkg/automation/catalog"
	"github.com/iota-uz/iota-sdk/pkg/automation/connectors"
	"github.com/iota-uz/iota-sdk/pkg/automation/expr"
	"github.com/iota-uz/iota-sdk/pkg/automation/graph"
	"github.com/iota-uz/iota-sdk/pkg/config/stdconfig/eskizconfig"
	"github.com/iota-uz/iota-sdk/pkg/config/stdconfig/twilioconfig"
)

// Connector kinds and node types.
const (
	KindEskiz  = catalog.KindEskiz
	KindTwilio = catalog.KindTwilio

	NodeEskizSend  = "eskiz.send_sms"
	NodeTwilioSend = "twilio.send_sms"
)

// Provider names in SendOutput.Provider.
const (
	ProviderEskiz  = "eskiz"
	ProviderTwilio = "twilio"
)

// UnitSMS is the unit of the usage a sent message records.
const UnitSMS = "sms"

// DefaultMaxMessageSize is the longest text, in characters, the Eskiz node sends.
const DefaultMaxMessageSize = 500

// SendParams are the params of both nodes (the reserved "connection" param is added by the catalog):
// the connection ID of the node's kind or "platform:<kind>" (left out: the only available one), the
// recipient (a template, normalized to E.164) and the text (a template, not empty once rendered).
type SendParams struct {
	Connection string
	To         string
	Text       string
}

// SendOutput is nodes.<id>.output of a node that took its sent outcome.
type SendOutput struct {
	Provider          string `json:"provider"`
	ProviderMessageID string `json:"provider_message_id"`
	To                string `json:"to"` // E.164
	Segments          int    `json:"segments,omitempty"`
}

// StatusSetter marks a connection whose credentials the provider refused as needs_reauth
// (connectors.ConnectionStore implements it).
type StatusSetter interface {
	SetStatus(ctx context.Context, tenantID, id uuid.UUID, s connectors.ConnectionStatus, reason string) error
}

// Option configures a connector.
type Option func(*options)

type options struct {
	countryCode string
	maxSize     int
	transport   http.RoundTripper
	trusted     []string
	statuses    StatusSetter
	now         func() time.Time
}

func newOptions(opts []Option) options {
	o := options{maxSize: DefaultMaxMessageSize, now: time.Now}
	for _, opt := range opts {
		opt(&o)
	}
	return o
}

// WithDefaultCountryCode sets the country code ("998" or "+998") a recipient without one gets. Without
// it such a number is invalid_request.
func WithDefaultCountryCode(code string) Option {
	return func(o *options) { o.countryCode = countryCode(code) }
}

// WithMaxMessageSize sets the longest text, in characters, the Eskiz node sends (default 500).
func WithMaxMessageSize(n int) Option {
	return func(o *options) {
		if n > 0 {
			o.maxSize = n
		}
	}
}

// WithTransport makes the connector send every provider request through rt (default: the default
// transport). Tests point it at a fake server.
func WithTransport(rt http.RoundTripper) Option {
	return func(o *options) { o.transport = rt }
}

// WithTrustedEskizURLs allows these API origins (scheme://host[:port]) of a tenant's Eskiz connection
// besides https://notify.eskiz.uz. A tenant's URL is otherwise refused: the connector would call any
// address it names. The platform credential's URL (from the environment) is not checked. Tests use it.
func WithTrustedEskizURLs(origins ...string) Option {
	return func(o *options) { o.trusted = append(o.trusted, origins...) }
}

// WithConnectionStatus lets the node mark a connection needs_reauth when the provider refuses its
// credentials. Without it the call only fails with unauthorized.
func WithConnectionStatus(s StatusSetter) Option {
	return func(o *options) { o.statuses = s }
}

// message is what a backend sends.
type message struct {
	to   string // E.164
	text string
	from string // the connection's sender, may be empty
}

// accepted is what the provider returned for a message it accepted.
type accepted struct {
	id       string
	segments int
}

// backend is a provider: its descriptor, its check of credentials and its send.
type backend interface {
	descriptor() connectors.Descriptor
	nodeTitle() string
	test(ctx context.Context, c connectors.Credentials) (connectors.Identity, error)
	// check refuses a message the provider would not take, before any request.
	check(c connectors.Credentials, m message, platform bool) *catalog.CallError
	send(ctx context.Context, in catalog.CallInput, m message) (accepted, *failure)
}

type connector struct {
	kind catalog.ConnectorKind
	be   backend
	node *sendNode
}

func newConnector(kind catalog.ConnectorKind, nodeType string, o options, be backend) *connector {
	c := &connector{kind: kind, be: be}
	c.node = &sendNode{typ: nodeType, kind: kind, be: be, o: o}
	return c
}

func (c *connector) Kind() catalog.ConnectorKind       { return c.kind }
func (c *connector) Nodes() []catalog.NodeType         { return []catalog.NodeType{c.node} }
func (c *connector) Triggers() []catalog.TriggerSource { return nil }
func (c *connector) Descriptor() connectors.Descriptor { return c.be.descriptor() }

func (c *connector) Test(ctx context.Context, creds connectors.Credentials) (connectors.Identity, error) {
	return c.be.test(ctx, creds)
}

var paramsSchema = graph.MustSchemaParams(`{
  "type": "object",
  "required": ["to", "text"],
  "properties": {
    "to":   {"type": "string", "minLength": 1, "maxLength": 200, "description": "recipient; normalized to E.164"},
    "text": {"type": "string", "minLength": 1, "maxLength": 1600, "description": "message text"}
  },
  "additionalProperties": false
}`)

// sendNode is the "Send SMS" node of a provider.
type sendNode struct {
	typ  string
	kind catalog.ConnectorKind
	be   backend
	o    options
}

func (n *sendNode) Spec() graph.NodeSpec {
	return graph.NodeSpec{
		Type:          n.typ,
		Title:         n.be.nodeTitle(),
		Description:   "Sends one SMS and reports that the provider accepted it. A send whose outcome is unknown ends as failed and is not repeated.",
		Behavior:      graph.BehaviorCall,
		Params:        paramsSchema,
		Templates:     []string{"to", "text"},
		Outcomes:      []string{"sent", "failed"},
		DoneOutcome:   "sent",
		FailedOutcome: "failed",
		DryRun:        graph.DryRunCapture,
		Output: expr.Fields{
			{Name: "provider", Field: expr.StringField},
			{Name: "provider_message_id", Field: expr.StringField},
			{Name: "to", Field: expr.StringField},
			{Name: "segments", Field: expr.OptionalOf(expr.IntField)},
		},
	}
}

func (n *sendNode) Requires() []catalog.ConnectorKind { return []catalog.ConnectorKind{n.kind} }

// Idempotent is false: neither provider takes an idempotency key, so a repeat may send a second SMS.
func (n *sendNode) Idempotent() bool { return false }

// UnknownIsTerminal: a send whose outcome is unknown ends as failed with outcome_unknown, whatever the
// graph connects, and no operator can retry it into a second SMS.
func (n *sendNode) UnknownIsTerminal() bool { return true }

func invalid(msg string) error {
	return &catalog.CallError{Code: catalog.CallInvalidRequest, Message: msg}
}

func (n *sendNode) Execute(ctx context.Context, in catalog.CallInput) (catalog.CallResult, error) {
	to, _ := in.Params["to"].(string)
	text, _ := in.Params["text"].(string)
	e164, ok := normalizePhone(to, n.o.countryCode)
	if !ok {
		return catalog.CallResult{}, invalid("the recipient number is not in E.164 format")
	}
	if strings.TrimSpace(text) == "" {
		return catalog.CallResult{}, invalid("the message text is empty")
	}
	if !utf8.ValidString(text) {
		return catalog.CallResult{}, invalid("the message text is not valid UTF-8")
	}
	if len(in.Credentials) == 0 {
		return catalog.CallResult{}, &catalog.CallError{Code: catalog.CallUnauthorized, Message: "the connection has no credentials"}
	}
	m := message{to: e164, text: text, from: strings.TrimSpace(in.Credentials["from"])}
	if ce := n.be.check(in.Credentials, m, in.Connection != nil && in.Connection.Platform); ce != nil {
		if ce.Code == catalog.CallUnauthorized {
			n.markNeedsReauth(ctx, in)
		}
		return catalog.CallResult{}, ce
	}
	acc, fail := n.be.send(ctx, in, m)
	if fail != nil {
		if fail.retry {
			// The provider certainly did not accept the message: the engine repeats within the deadline.
			return catalog.CallResult{RetryAfter: fail.retryAfter}, nil
		}
		if fail.err.Code == catalog.CallUnauthorized {
			n.markNeedsReauth(ctx, in)
		}
		return catalog.CallResult{}, fail.err
	}
	n.record(ctx, in, acc)
	out := SendOutput{Provider: string(n.kind), ProviderMessageID: acc.id, To: e164, Segments: acc.segments}
	res := map[string]any{"provider": out.Provider, "provider_message_id": out.ProviderMessageID, "to": out.To}
	if out.Segments > 0 {
		res["segments"] = int64(out.Segments)
	}
	return catalog.CallResult{Outcome: "sent", Output: res}, nil
}

// record counts an accepted message. The message is sent whatever happens here, so a failing recorder
// does not fail the node.
func (n *sendNode) record(ctx context.Context, in catalog.CallInput, acc accepted) {
	if in.Usage == nil || in.Connection == nil {
		return
	}
	qty := int64(acc.segments)
	if qty <= 0 {
		qty = 1
	}
	run := in.RunID
	_ = in.Usage.Record(context.WithoutCancel(ctx), catalog.UsageEntry{
		TenantID: in.TenantID, Kind: n.kind, ConnectionID: in.Connection.ID,
		Unit: UnitSMS, Quantity: qty, RunID: &run, At: n.o.now(),
	})
}

func (n *sendNode) markNeedsReauth(ctx context.Context, in catalog.CallInput) {
	if n.o.statuses == nil || in.Connection == nil || in.Connection.Platform {
		return
	}
	id, err := uuid.Parse(in.Connection.ID)
	if err != nil {
		return
	}
	_ = n.o.statuses.SetStatus(context.WithoutCancel(ctx), in.TenantID, id, connectors.StatusNeedsReauth, "the provider refused the credentials")
}

// scrub makes provider text fit for a CallError message: control characters out, the secrets replaced,
// bounded.
func scrub(s string, secrets ...string) string {
	for _, sec := range secrets {
		if len(sec) >= 3 {
			s = strings.ReplaceAll(s, sec, "***")
		}
	}
	s = strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return ' '
		}
		return r
	}, s)
	s = strings.TrimSpace(s)
	if r := []rune(s); len(r) > 200 {
		s = string(r[:200])
	}
	return s
}

// PlatformCredentials are the platform credentials the environment configures: Eskiz when its email and
// password are set, Twilio when its account SID, auth token and phone number are set. The presence of the
// required fields enables a credential; there are no *_ENABLED flags. Pass the result to
// connectors.NewPlatformCredentials; both connectors declare Platform = opt_in, so a tenant gets the
// platform account only after its administrator enabled it (connectors.PlatformGrants).
func PlatformCredentials(eskiz *eskizconfig.Config, twilio *twilioconfig.Config) map[catalog.ConnectorKind]connectors.PlatformCredential {
	out := map[catalog.ConnectorKind]connectors.PlatformCredential{}
	if eskiz != nil && eskiz.IsConfigured() {
		url := strings.TrimSpace(eskiz.URL)
		if url == "" {
			url = eskizconfig.DefaultURL
		}
		out[KindEskiz] = connectors.PlatformCredential{Credentials: connectors.Credentials{
			"url": url, "email": eskiz.Email, "password": eskiz.Password, "from": eskiz.From,
		}}
	}
	if twilio != nil && twilio.AccountSID != "" && twilio.AuthToken != "" && twilio.PhoneNumber != "" {
		out[KindTwilio] = connectors.PlatformCredential{Credentials: connectors.Credentials{
			"account_sid": twilio.AccountSID, "auth_token": twilio.AuthToken, "from": twilio.PhoneNumber,
		}}
	}
	return out
}
