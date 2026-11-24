// Package openai is the OpenAI connector of automation graphs and its node ai.llm: a text or JSON
// generation by an OpenAI model, or by any OpenAI-compatible endpoint a connection names, with the answer
// checked against a JSON Schema.
//
// The node works on a tenant's own key or on the platform key (Platform = opt_in: a tenant gets it only
// after its administrator enabled it). The key of BiChat (bichat.openai) is not reused: AI spending of
// automations is a separate item. Every provider answer, a repeat's too, is recorded in the tenant's usage
// as token_in and token_out; the cost the provider reports stays in the node's output.
//
// The node is idempotent: a call has no external effect, so an unknown outcome is repeated (and paid
// again). A 429 and a 5xx are repeated by the engine within the node's timeout, after the provider's
// Retry-After when it gave one.
package openai

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
	"github.com/iota-uz/iota-sdk/pkg/config/stdconfig/openaiconfig"
)

// Kind is the connector kind; NodeLLM is its node type.
const (
	Kind    = catalog.KindOpenAI
	NodeLLM = "ai.llm"
)

// Credentials are the fields of a connection (the form of Descriptor.Credentials); at run time they are a
// catalog.Credentials of these names.
type Credentials struct {
	APIKey  string `json:"api_key" secret:"true"`
	BaseURL string `json:"base_url,omitempty"` // an OpenAI-compatible endpoint; https only
}

// Usage is what a call spent; CostUSD is nil when the provider did not report it.
type Usage = aichat.Usage

// StatusSetter marks a connection whose key the provider refused as needs_reauth
// (connectors.ConnectionStore implements it).
type StatusSetter interface {
	SetStatus(ctx context.Context, tenantID, id uuid.UUID, s connectors.ConnectionStatus, reason string) error
}

// statusWriteTimeout bounds the write of a refused key's status.
const statusWriteTimeout = 5 * time.Second

// Option configures the connector.
type Option func(*options)

type options struct {
	defaultModel string
	transport    http.RoundTripper
	images       ImageSource
	vision       func(model string) bool
	statuses     StatusSetter
	log          *slog.Logger
	now          func() time.Time
}

func newOptions(opts []Option) options {
	o := options{defaultModel: openaiconfig.DefaultModel, vision: aichat.VisionModel, log: slog.Default(), now: time.Now}
	for _, opt := range opts {
		if opt != nil {
			opt(&o)
		}
	}
	return o
}

// WithDefaultModel is the exact model id an ai.llm node saved without a model gets (the platform's
// automation.openai.defaultmodel; default gpt-4o-mini). It is written into the version when the graph is
// saved, so changing it later does not change a saved graph.
func WithDefaultModel(model string) Option {
	return func(o *options) {
		if strings.TrimSpace(model) != "" {
			o.defaultModel = strings.TrimSpace(model)
		}
	}
}

// WithTransport makes the connector send every provider request through rt (default: the default
// transport, with a guard against private addresses for a tenant's base_url). Tests point it at a fake.
func WithTransport(rt http.RoundTripper) Option { return func(o *options) { o.transport = rt } }

// WithImages is where the files of the images param come from. Without it the node refuses images.
func WithImages(src ImageSource) Option { return func(o *options) { o.images = src } }

// WithVision replaces the check of whether a model takes images (default: by the model's family).
func WithVision(has func(model string) bool) Option {
	return func(o *options) {
		if has != nil {
			o.vision = has
		}
	}
}

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

// PlatformCredentials is the platform credential the environment configures: the key of
// automation.openai when it is set. Pass it to connectors.NewPlatformCredentials; the connector declares
// Platform = opt_in, so a tenant gets the key only after its administrator enabled it
// (connectors.PlatformGrants).
func PlatformCredentials(cfg *openaiconfig.Config) map[catalog.ConnectorKind]connectors.PlatformCredential {
	out := map[catalog.ConnectorKind]connectors.PlatformCredential{}
	if cfg == nil || !cfg.IsConfigured() {
		return out
	}
	creds := connectors.Credentials{"api_key": cfg.APIKey}
	if u := strings.TrimSpace(cfg.BaseURL); u != "" {
		creds["base_url"] = u
	}
	out[Kind] = connectors.PlatformCredential{Credentials: creds}
	return out
}

// New is the connector "openai": the node ai.llm.
func New(opts ...Option) connectors.Connector {
	o := newOptions(opts)
	c := &connector{o: o}
	c.node = newLLMNode(o, &aichat.Client{Transport: o.transport}, c.markNeedsReauth)
	return c
}

type connector struct {
	o    options
	node *llmNode
}

func (c *connector) Kind() catalog.ConnectorKind       { return Kind }
func (c *connector) Nodes() []catalog.NodeType         { return []catalog.NodeType{c.node} }
func (c *connector) Triggers() []catalog.TriggerSource { return nil }

func (c *connector) Descriptor() connectors.Descriptor {
	return connectors.Descriptor{
		Kind:        Kind,
		Title:       map[string]string{"en": "OpenAI", "ru": "OpenAI", "uz": "OpenAI"},
		Description: map[string]string{"en": "Generate text or JSON with an OpenAI model", "ru": "Генерация текста и JSON моделью OpenAI", "uz": "OpenAI modeli bilan matn va JSON yaratish"},
		Category:    connectors.CategoryAI,
		Icon:        "ai",
		Auth:        connectors.AuthAPIKey,
		Platform:    connectors.PlatformOptIn,
		Credentials: json.RawMessage(`{
  "type": "object",
  "required": ["api_key"],
  "properties": {
    "api_key":  {"type": "string", "minLength": 1, "maxLength": 512, "x-secret": true},
    "base_url": {"type": "string", "pattern": "^https://[^\\s]+$", "maxLength": 300, "description": "an OpenAI-compatible endpoint, https only; left out: api.openai.com"}
  },
  "additionalProperties": false
}`),
	}
}

// endpoint is the API a connection's credentials name.
func endpointOf(c connectors.Credentials, platform bool) (aichat.Endpoint, error) {
	key := strings.TrimSpace(c["api_key"])
	if key == "" {
		return aichat.Endpoint{}, errNoKey
	}
	base := strings.TrimSpace(c["base_url"])
	if base == "" {
		base = openaiconfig.DefaultBaseURL
	}
	if err := aichat.CheckBaseURL(base); err != nil {
		return aichat.Endpoint{}, err
	}
	return aichat.Endpoint{BaseURL: base, APIKey: key, Guarded: !platform}, nil
}

var errNoKey = errors.New("the connection has no API key")

// Test lists the models (GET /models), which checks the key, and reports the organization or project the
// provider names in its headers.
func (c *connector) Test(ctx context.Context, creds connectors.Credentials) (connectors.Identity, error) {
	ep, err := endpointOf(creds, false)
	if err != nil {
		return connectors.Identity{}, errors.New("openai: " + err.Error())
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimRight(ep.BaseURL, "/")+"/models", nil)
	if err != nil {
		return connectors.Identity{}, errors.New("openai: the endpoint URL is not valid")
	}
	req.Header.Set("Authorization", "Bearer "+ep.APIKey)
	cl := &aichat.Client{Transport: c.o.transport}
	resp, err := cl.Do(ep, req)
	if err != nil {
		return connectors.Identity{}, errors.New("openai: the provider was not reached")
	}
	defer func() { _ = resp.Body.Close() }()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if resp.StatusCode != http.StatusOK {
		ce := aichat.StatusError(resp, raw, ep.APIKey)
		return connectors.Identity{}, errors.New("openai: " + ce.Message)
	}
	var list struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if json.Unmarshal(raw, &list) != nil || list.Data == nil {
		return connectors.Identity{}, errors.New("openai: the provider's list of models could not be read")
	}
	id := connectors.Identity{
		Display:  firstNonEmpty(resp.Header.Get("openai-project"), resp.Header.Get("openai-organization")),
		External: firstNonEmpty(resp.Header.Get("openai-organization"), resp.Header.Get("openai-project")),
	}
	return id, nil
}

func firstNonEmpty(list ...string) string {
	for _, s := range list {
		if s = strings.TrimSpace(s); s != "" {
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
	// The write outlives the node's deadline but not a stalled database: it has a bound of its own.
	wctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), statusWriteTimeout)
	defer cancel()
	_ = c.o.statuses.SetStatus(wctx, in.TenantID, id, connectors.StatusNeedsReauth, "the provider refused the API key")
}
