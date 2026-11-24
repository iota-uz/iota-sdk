package sms

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	twilio "github.com/twilio/twilio-go"
	twilioclient "github.com/twilio/twilio-go/client"
	twilioapi "github.com/twilio/twilio-go/rest/api/v2010"

	"github.com/iota-uz/iota-sdk/pkg/automation/catalog"
	"github.com/iota-uz/iota-sdk/pkg/automation/connectors"
)

// twilioTimeout bounds a request when the call has no earlier deadline.
const twilioTimeout = 30 * time.Second

var (
	messagingServiceSID = regexp.MustCompile(`^MG[0-9a-fA-F]{32}$`)
	twilioKey           = regexp.MustCompile(`^[A-Za-z0-9]+$`) // twilio-go refuses anything else
	twilioSender        = regexp.MustCompile(`^(\+[1-9][0-9]{6,14}|MG[0-9a-fA-F]{32})$`)
)

// NewTwilio is the connector "twilio": the node twilio.send_sms over twilio-go.
func NewTwilio(opts ...Option) connectors.Connector {
	o := newOptions(opts)
	return newConnector(KindTwilio, NodeTwilioSend, o, &twilioBackend{o: o})
}

type twilioBackend struct{ o options }

func (b *twilioBackend) nodeTitle() string { return "Send SMS (Twilio)" }

func (b *twilioBackend) descriptor() connectors.Descriptor {
	return connectors.Descriptor{
		Kind:        KindTwilio,
		Title:       map[string]string{"en": "Twilio SMS", "ru": "Twilio SMS", "uz": "Twilio SMS"},
		Description: map[string]string{"en": "Send SMS through a Twilio account", "ru": "Отправка SMS через аккаунт Twilio", "uz": "Twilio akkaunti orqali SMS yuborish"},
		Category:    connectors.CategoryMessaging,
		Icon:        "sms",
		Auth:        connectors.AuthBasic,
		Platform:    connectors.PlatformOptIn,
		Credentials: json.RawMessage(`{
  "type": "object",
  "required": ["account_sid", "auth_token", "from"],
  "properties": {
    "account_sid": {"type": "string", "pattern": "^[A-Za-z0-9]+$", "maxLength": 64},
    "auth_token":  {"type": "string", "pattern": "^[A-Za-z0-9]+$", "maxLength": 128, "x-secret": true},
    "from":        {"type": "string", "pattern": "^(\\+[1-9][0-9]{6,14}|MG[0-9a-fA-F]{32})$", "description": "an E.164 number or a Messaging Service SID (MG…)"}
  },
  "additionalProperties": false
}`),
	}
}

func (b *twilioBackend) check(c connectors.Credentials, m message, _ bool) *catalog.CallError {
	if !twilioKey.MatchString(c["account_sid"]) || !twilioKey.MatchString(c["auth_token"]) {
		return &catalog.CallError{Code: catalog.CallUnauthorized, Message: "the connection has no valid account SID or auth token"}
	}
	if !twilioSender.MatchString(c["from"]) {
		return &catalog.CallError{Code: catalog.CallInvalidRequest, Message: "the connection's sender is neither an E.164 number nor a Messaging Service SID"}
	}
	return nil
}

// restClient is a client for one call: twilio-go takes no context, so the context travels in the
// transport, and so does the trace of what was sent.
func (b *twilioBackend) restClient(ctx context.Context, c connectors.Credentials, trace *callTrace) *twilio.RestClient {
	timeout := twilioTimeout
	if dl, ok := ctx.Deadline(); ok {
		timeout = max(time.Until(dl), time.Millisecond)
	}
	httpc := &http.Client{
		Timeout:       timeout,
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
		Transport: &tracker{
			base: b.o.transport, fixed: trace, ctx: ctx,
			isSend: func(r *http.Request) bool {
				return r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/Messages.json")
			},
		},
	}
	cl := &twilioclient.Client{Credentials: twilioclient.NewCredentials(c["account_sid"], c["auth_token"]), HTTPClient: httpc}
	cl.SetAccountSid(c["account_sid"])
	return twilio.NewRestClientWithParams(twilio.ClientParams{Client: cl})
}

func (b *twilioBackend) send(ctx context.Context, in catalog.CallInput, m message) (accepted, *failure) {
	trace := &callTrace{}
	c := in.Credentials
	params := &twilioapi.CreateMessageParams{}
	params.SetTo(m.to)
	params.SetBody(m.text)
	from := m.from
	if messagingServiceSID.MatchString(from) {
		params.SetMessagingServiceSid(from)
	} else {
		params.SetFrom(from)
	}
	res, err := b.restClient(ctx, c, trace).Api.CreateMessage(params)
	if err != nil {
		f := classify(trace.snapshot(), scrub(twilioDetail(err), c["auth_token"]))
		return accepted{}, &f
	}
	if res == nil || res.Sid == nil || *res.Sid == "" {
		f := classify(hop{send: true, written: true, status: http.StatusOK}, "")
		return accepted{}, &f
	}
	acc := accepted{id: *res.Sid}
	if res.NumSegments != nil {
		if n, err := strconv.Atoi(*res.NumSegments); err == nil && n > 0 {
			acc.segments = n
		}
	}
	return acc, nil
}

func (b *twilioBackend) test(ctx context.Context, c connectors.Credentials) (connectors.Identity, error) {
	if !twilioKey.MatchString(c["account_sid"]) || !twilioKey.MatchString(c["auth_token"]) {
		return connectors.Identity{}, errors.New("twilio: a valid account SID and auth token are required")
	}
	trace := &callTrace{}
	if _, err := b.restClient(ctx, c, trace).Api.FetchAccount(c["account_sid"]); err != nil {
		f := classify(trace.snapshot(), "")
		if f.err == nil {
			return connectors.Identity{}, errors.New("twilio: the provider is rate limiting requests")
		}
		return connectors.Identity{}, errors.New("twilio: " + f.err.Message)
	}
	// External is the account and the sender, so a Messaging Service SID (not a public identity to show)
	// is kept too.
	id := connectors.Identity{External: c["account_sid"] + "/" + c["from"], Display: c["from"]}
	if strings.HasPrefix(c["from"], "+") {
		id.Kind = connectors.IdentityPhone
	} else {
		id.Display = ""
	}
	return id, nil
}

// twilioDetail is the provider's own message of a refusal, if it gave one.
func twilioDetail(err error) string {
	var re *twilioclient.TwilioRestError
	if errors.As(err, &re) {
		return re.Message
	}
	return ""
}
