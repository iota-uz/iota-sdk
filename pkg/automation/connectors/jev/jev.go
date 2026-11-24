// Package jev is the Jev connector of automation graphs and its node ai.jev: a classifier that chooses
// one of the given choices for an input and takes the graph along the outcome of the choice, with the
// confidence of the choice deciding whether to trust it.
//
// A connection's provider decides: "openrouter" (a model of OpenRouter) or "openai". The node itself is
// the same, so each node has one connector it requires and its type is registered once.
//
// The node has an outcome per choice, low_confidence (the confidence is below the node's min_confidence;
// the chosen value is still in output.choice) and failed (the provider failed, refused, or answered
// outside the choices). It is idempotent: a call has no external effect, so an unknown outcome is
// repeated (and paid again). Every provider answer is recorded in the tenant's usage as token_in and
// token_out.
package jev

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/iota-uz/iota-sdk/pkg/automation/catalog"
	"github.com/iota-uz/iota-sdk/pkg/automation/connectors"
	"github.com/iota-uz/iota-sdk/pkg/automation/connectors/internal/aichat"
	"github.com/iota-uz/iota-sdk/pkg/config/stdconfig/jevconfig"
)

// Kind is the connector kind; NodeJev is its node type.
const (
	Kind    = catalog.KindJEV
	NodeJev = "ai.jev"
)

// Providers of a connection.
const (
	ProviderOpenRouter = jevconfig.ProviderOpenRouter
	ProviderOpenAI     = jevconfig.ProviderOpenAI
)

// Endpoints of the providers.
const (
	OpenRouterBaseURL = "https://openrouter.ai/api/v1"
	OpenAIBaseURL     = "https://api.openai.com/v1"
)

// Default models of a node saved without one, by the provider of the platform credential.
const (
	DefaultModelOpenRouter = "openai/gpt-4o-mini"
	DefaultModelOpenAI     = "gpt-4o-mini"
)

// Credentials are the fields of a connection (the form of Descriptor.Credentials); at run time they are a
// catalog.Credentials of these names.
type Credentials struct {
	Provider string `json:"provider"`              // "openrouter" | "openai"
	APIKey   string `json:"api_key" secret:"true"` // the key of the chosen provider
}

// Usage is what a call spent; CostUSD is nil when the provider did not report it.
type Usage = aichat.Usage

// StatusSetter marks a connection whose key the provider refused as needs_reauth
// (connectors.ConnectionStore implements it).
type StatusSetter interface {
	SetStatus(ctx context.Context, tenantID, id uuid.UUID, s connectors.ConnectionStatus, reason string) error
}

// Option configures the connector.
type Option func(*options)

type options struct {
	defaultModel string
	transport    http.RoundTripper
	statuses     StatusSetter
	log          *slog.Logger
	now          func() time.Time
}

func newOptions(opts []Option) options {
	o := options{defaultModel: DefaultModelOpenRouter, log: slog.Default(), now: time.Now}
	for _, opt := range opts {
		if opt != nil {
			opt(&o)
		}
	}
	return o
}

// WithDefaultModel is the exact model id an ai.jev node saved without a model gets (the platform's
// automation.jev.defaultmodel; default openai/gpt-4o-mini, the OpenRouter id). It is written into the
// version when the graph is saved, so changing it later does not change a saved graph. It must be an id of
// the provider of the connections that use it.
func WithDefaultModel(model string) Option {
	return func(o *options) {
		if strings.TrimSpace(model) != "" {
			o.defaultModel = strings.TrimSpace(model)
		}
	}
}

// WithTransport makes the connector send every provider request through rt (default: the default
// transport). Tests point it at a fake.
func WithTransport(rt http.RoundTripper) Option { return func(o *options) { o.transport = rt } }

// WithConnectionStatus lets the node mark a connection needs_reauth when the provider refuses its key.
func WithConnectionStatus(s StatusSetter) Option { return func(o *options) { o.statuses = s } }

// WithLogger sets where a failure to record usage is logged (default slog.Default()).
func WithLogger(l *slog.Logger) Option {
	return func(o *options) {
		if l != nil {
			o.log = l
		}
	}
}

// DefaultModel is the default model of a platform configuration: its defaultmodel, else the provider's
// default.
func DefaultModel(cfg *jevconfig.Config) string {
	if cfg == nil {
		return DefaultModelOpenRouter
	}
	if m := strings.TrimSpace(cfg.DefaultModel); m != "" {
		return m
	}
	if cfg.Provider == ProviderOpenAI {
		return DefaultModelOpenAI
	}
	return DefaultModelOpenRouter
}

// PlatformCredentials is the platform credential the environment configures: the key of automation.jev
// when it is set. Pass it to connectors.NewPlatformCredentials; the connector declares Platform = opt_in,
// so a tenant gets the key only after its administrator enabled it (connectors.PlatformGrants).
func PlatformCredentials(cfg *jevconfig.Config) map[catalog.ConnectorKind]connectors.PlatformCredential {
	out := map[catalog.ConnectorKind]connectors.PlatformCredential{}
	if cfg == nil || !cfg.IsConfigured() {
		return out
	}
	provider := cfg.Provider
	if provider == "" {
		provider = jevconfig.DefaultProvider
	}
	out[Kind] = connectors.PlatformCredential{Credentials: connectors.Credentials{"provider": provider, "api_key": cfg.APIKey}}
	return out
}

// New is the connector "jev": the node ai.jev.
func New(opts ...Option) connectors.Connector {
	o := newOptions(opts)
	c := &connector{o: o}
	c.node = newNode(o, &aichat.Client{Transport: o.transport}, c.markNeedsReauth)
	return c
}

type connector struct {
	o    options
	node *jevNode
}

func (c *connector) Kind() catalog.ConnectorKind       { return Kind }
func (c *connector) Nodes() []catalog.NodeType         { return []catalog.NodeType{c.node} }
func (c *connector) Triggers() []catalog.TriggerSource { return nil }

func (c *connector) Descriptor() connectors.Descriptor {
	return connectors.Descriptor{
		Kind:        Kind,
		Title:       map[string]string{"en": "Jev classifier", "ru": "Классификатор Jev", "uz": "Jev klassifikatori"},
		Description: map[string]string{"en": "Choose one of the given options with a model, with a confidence", "ru": "Выбор одного из заданных вариантов моделью с оценкой уверенности", "uz": "Berilgan variantlardan birini model orqali tanlash"},
		Category:    connectors.CategoryAI,
		Icon:        "ai",
		Auth:        connectors.AuthAPIKey,
		Platform:    connectors.PlatformOptIn,
		Credentials: json.RawMessage(`{
  "type": "object",
  "required": ["provider", "api_key"],
  "properties": {
    "provider": {"type": "string", "enum": ["openrouter", "openai"], "description": "whose key it is: OpenRouter or OpenAI"},
    "api_key":  {"type": "string", "minLength": 1, "maxLength": 512, "x-secret": true}
  },
  "additionalProperties": false
}`),
	}
}

var errNoKey = errors.New("the connection has no API key")

// endpointOf is the API a connection's credentials name.
func endpointOf(c connectors.Credentials) (aichat.Endpoint, error) {
	key := strings.TrimSpace(c["api_key"])
	if key == "" {
		return aichat.Endpoint{}, errNoKey
	}
	switch c["provider"] {
	case ProviderOpenRouter:
		return aichat.Endpoint{BaseURL: OpenRouterBaseURL, APIKey: key, OpenRouter: true}, nil
	case ProviderOpenAI:
		return aichat.Endpoint{BaseURL: OpenAIBaseURL, APIKey: key}, nil
	}
	return aichat.Endpoint{}, errors.New("the connection's provider is openrouter or openai")
}

// Test lists the models (GET /models). OpenRouter lists them without a key, so its key is checked with
// GET /auth/key, which also names the key (its label is the identity); OpenAI names the organization or
// project in the headers of its answer.
func (c *connector) Test(ctx context.Context, creds connectors.Credentials) (connectors.Identity, error) {
	ep, err := endpointOf(creds)
	if err != nil {
		return connectors.Identity{}, errors.New("jev: " + err.Error())
	}
	cl := &aichat.Client{Transport: c.o.transport}
	get := func(path string) (*http.Response, []byte, error) {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimRight(ep.BaseURL, "/")+path, nil)
		if err != nil {
			return nil, nil, err
		}
		req.Header.Set("Authorization", "Bearer "+ep.APIKey)
		resp, err := cl.Do(ep, req)
		if err != nil {
			return nil, nil, err
		}
		defer func() { _ = resp.Body.Close() }()
		raw, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
		return resp, raw, nil
	}
	fail := func(resp *http.Response, raw []byte) error {
		return errors.New("jev: " + aichat.StatusError(resp, raw, ep.APIKey).Message)
	}
	var id connectors.Identity
	if ep.OpenRouter {
		resp, raw, err := get("/auth/key")
		if err != nil {
			return connectors.Identity{}, errors.New("jev: the provider was not reached")
		}
		if resp.StatusCode != http.StatusOK {
			return connectors.Identity{}, fail(resp, raw)
		}
		var key struct {
			Data struct {
				Label string `json:"label"`
			} `json:"data"`
		}
		if json.Unmarshal(raw, &key) != nil {
			return connectors.Identity{}, errors.New("jev: the provider's answer about the key could not be read")
		}
		id = connectors.Identity{Display: strings.TrimSpace(key.Data.Label), External: strings.TrimSpace(key.Data.Label)}
	}
	resp, raw, err := get("/models")
	if err != nil {
		return connectors.Identity{}, errors.New("jev: the provider was not reached")
	}
	if resp.StatusCode != http.StatusOK {
		return connectors.Identity{}, fail(resp, raw)
	}
	var list struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if json.Unmarshal(raw, &list) != nil || list.Data == nil {
		return connectors.Identity{}, errors.New("jev: the provider's list of models could not be read")
	}
	if !ep.OpenRouter {
		org, project := strings.TrimSpace(resp.Header.Get("openai-organization")), strings.TrimSpace(resp.Header.Get("openai-project"))
		id = connectors.Identity{Display: firstNonEmpty(project, org), External: firstNonEmpty(org, project)}
	}
	return id, nil
}

func firstNonEmpty(list ...string) string {
	for _, s := range list {
		if s != "" {
			return s
		}
	}
	return ""
}

func (c *connector) markNeedsReauth(ctx context.Context, in catalog.CallInput) {
	if c.o.statuses == nil || in.Connection == nil || in.Connection.Platform {
		return
	}
	id, err := uuid.Parse(in.Connection.ID)
	if err != nil {
		return
	}
	_ = c.o.statuses.SetStatus(context.WithoutCancel(ctx), in.TenantID, id, connectors.StatusNeedsReauth, "the provider refused the API key")
}
