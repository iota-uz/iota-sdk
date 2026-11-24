package sms

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"unicode/utf8"

	eskizapi "github.com/iota-uz/eskiz"
	"github.com/sirupsen/logrus"

	"github.com/iota-uz/iota-sdk/pkg/automation/catalog"
	"github.com/iota-uz/iota-sdk/pkg/automation/connectors"
	"github.com/iota-uz/iota-sdk/pkg/config/stdconfig/eskizconfig"
	"github.com/iota-uz/iota-sdk/pkg/eskiz"
	"github.com/iota-uz/iota-sdk/pkg/eskiz/models"
)

// maxEskizClients bounds the clients kept (one per connection); past it the cache starts over.
const maxEskizClients = 1024

const eskizSendPath = "/api/message/sms/send"

// NewEskiz is the connector "eskiz": the node eskiz.send_sms over pkg/eskiz.
func NewEskiz(opts ...Option) connectors.Connector {
	o := newOptions(opts)
	return newConnector(KindEskiz, NodeEskizSend, o, &eskizBackend{o: o, clients: map[string]*eskizClient{}})
}

type eskizBackend struct {
	o       options
	mu      sync.Mutex
	clients map[string]*eskizClient
}

// eskizClient is the service of one connection: its token is kept, so a send does not log in again.
type eskizClient struct {
	fingerprint [sha256.Size]byte
	svc         eskiz.Service
}

func (b *eskizBackend) nodeTitle() string { return "Send SMS (Eskiz)" }

func (b *eskizBackend) descriptor() connectors.Descriptor {
	return connectors.Descriptor{
		Kind:        KindEskiz,
		Title:       map[string]string{"en": "Eskiz SMS", "ru": "Eskiz SMS", "uz": "Eskiz SMS"},
		Description: map[string]string{"en": "Send SMS through an Eskiz account", "ru": "Отправка SMS через аккаунт Eskiz", "uz": "Eskiz akkaunti orqali SMS yuborish"},
		Category:    connectors.CategoryMessaging,
		Icon:        "sms",
		Auth:        connectors.AuthBasic,
		Platform:    connectors.PlatformOptIn,
		Credentials: json.RawMessage(`{
  "type": "object",
  "required": ["email", "password"],
  "properties": {
    "url":      {"type": "string", "maxLength": 200, "default": "` + eskizconfig.DefaultURL + `", "description": "API address"},
    "email":    {"type": "string", "minLength": 1, "maxLength": 200},
    "password": {"type": "string", "minLength": 1, "maxLength": 200, "x-secret": true},
    "from":     {"type": "string", "maxLength": 32, "description": "sender id; empty: the account's default"}
  },
  "additionalProperties": false
}`),
	}
}

// service returns the client of the connection of the call, built on first use and again when its
// credentials changed.
func (b *eskizBackend) service(connID string, c connectors.Credentials) eskiz.Service {
	url := strings.TrimSpace(c["url"])
	if url == "" {
		url = eskizconfig.DefaultURL
	}
	fp := sha256.Sum256([]byte(url + "\x00" + c["email"] + "\x00" + c["password"]))
	if connID == "" {
		return b.build(url, c)
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	if cl, ok := b.clients[connID]; ok && cl.fingerprint == fp {
		return cl.svc
	}
	if len(b.clients) >= maxEskizClients {
		b.clients = map[string]*eskizClient{}
	}
	cl := &eskizClient{fingerprint: fp, svc: b.build(url, c)}
	b.clients[connID] = cl
	return cl.svc
}

func (b *eskizBackend) build(url string, c connectors.Credentials) eskiz.Service {
	// The text limit is counted in characters here; the service counts bytes, so it gets room for any text.
	cfg := eskiz.NewConfig(url, c["email"], c["password"], eskiz.WithMaxMessageSize(b.o.maxSize*utf8.UTFMax+utf8.UTFMax))
	logger := logrus.New()
	logger.SetOutput(io.Discard) // the login request carries the password
	rt := &tracker{base: b.o.transport, isSend: func(r *http.Request) bool { return strings.HasSuffix(r.URL.Path, eskizSendPath) }}
	return eskiz.NewService(cfg, logger, nil, eskiz.WithRoundTripper(rt), eskiz.WithoutRedirects(), eskiz.WithoutAuthReplay())
}

// urlAllowed reports whether a tenant's Eskiz URL is the default API or a trusted origin. Anything else
// would make the server call an address the tenant names.
func (b *eskizBackend) urlAllowed(raw string) bool {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return true
	}
	u, err := url.Parse(raw)
	if err != nil || u.User != nil || u.RawQuery != "" || u.Fragment != "" || (u.Path != "" && u.Path != "/") {
		return false
	}
	origin := strings.ToLower(u.Scheme + "://" + u.Host)
	if origin == strings.ToLower(eskizconfig.DefaultURL) {
		return true
	}
	for _, t := range b.o.trusted {
		if origin == strings.ToLower(strings.TrimRight(t, "/")) {
			return true
		}
	}
	return false
}

func (b *eskizBackend) check(c connectors.Credentials, m message, platform bool) *catalog.CallError {
	if !platform && !b.urlAllowed(c["url"]) {
		return &catalog.CallError{Code: catalog.CallInvalidRequest, Message: "the connection's Eskiz URL is not an allowed address"}
	}
	if c["email"] == "" || c["password"] == "" {
		return &catalog.CallError{Code: catalog.CallUnauthorized, Message: "the connection has no email or password"}
	}
	if n := utf8.RuneCountInString(m.text); n > b.o.maxSize {
		return &catalog.CallError{Code: catalog.CallInvalidRequest, Message: fmt.Sprintf("the text is %d characters; Eskiz takes at most %d", n, b.o.maxSize)}
	}
	return nil
}

func (b *eskizBackend) send(ctx context.Context, in catalog.CallInput, m message) (accepted, *failure) {
	trace := &callTrace{}
	svc := b.service(connectionID(in), in.Credentials)
	req, err := models.NewSendSMS(strings.TrimPrefix(m.to, "+"), m.text, models.SendSmsWithFrom(m.from))
	if err != nil {
		return accepted{}, &failure{err: &catalog.CallError{Code: catalog.CallInvalidRequest, Message: "the message is invalid"}}
	}
	res, err := svc.SendSMS(withTrace(ctx, trace), req)
	if err != nil {
		hp := trace.snapshot()
		if trace.sendRefused() {
			// The send endpoint refused the SMS with a 401, so it was not accepted and the answers after
			// it (the token renewal) decide nothing about repeating it: never a retry.
			switch {
			case hp.status == http.StatusOK:
				return accepted{}, &failure{err: &catalog.CallError{Code: catalog.CallUnavailable, Message: "the session token had expired and was renewed; nothing was sent"}}
			case hp.status >= 400 && hp.status < 500 && hp.status != http.StatusTooManyRequests && hp.status != http.StatusRequestTimeout:
				return accepted{}, &failure{err: &catalog.CallError{Code: catalog.CallUnauthorized, Message: "the provider refused the credentials"}}
			}
			return accepted{}, &failure{err: &catalog.CallError{Code: catalog.CallUnavailable, Message: "the session token had expired and could not be renewed; nothing was sent"}}
		}
		f := classify(hp, scrub(eskizDetail(err), in.Credentials["password"], in.Credentials["email"]))
		return accepted{}, &f
	}
	if res.ID() == "" {
		// A success without an acknowledgement is not proof that the message was accepted.
		f := classify(hop{send: true, written: true, status: http.StatusOK}, "")
		return accepted{}, &f
	}
	return accepted{id: res.ID()}, nil
}

func (b *eskizBackend) test(ctx context.Context, c connectors.Credentials) (connectors.Identity, error) {
	if c["email"] == "" || c["password"] == "" {
		return connectors.Identity{}, errors.New("eskiz: email and password are required")
	}
	if !b.urlAllowed(c["url"]) {
		return connectors.Identity{}, errors.New("eskiz: the URL is not an allowed address")
	}
	trace := &callTrace{}
	if _, err := b.service("", c).GetBalance(withTrace(ctx, trace)); err != nil {
		f := classify(trace.snapshot(), "")
		if f.err == nil {
			return connectors.Identity{}, errors.New("eskiz: the provider is rate limiting requests")
		}
		return connectors.Identity{}, errors.New("eskiz: " + f.err.Message)
	}
	return connectors.Identity{Display: c["email"], External: c["email"], Kind: connectors.IdentityEmail}, nil
}

func connectionID(in catalog.CallInput) string {
	if in.Connection == nil {
		return ""
	}
	return in.Connection.ID
}

// eskizDetail is the provider's own message of a refusal, if it gave one.
func eskizDetail(err error) string {
	var ge *eskizapi.GenericOpenAPIError
	if !errors.As(err, &ge) {
		return ""
	}
	var body struct {
		Message string `json:"message"`
	}
	if json.Unmarshal(ge.Body(), &body) != nil {
		return ""
	}
	return body.Message
}
