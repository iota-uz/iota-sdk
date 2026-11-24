// Package mcp is the connector of remote MCP servers and its node mcp_tool, which calls any tool of the
// server. One connection is one server, reached over Streamable HTTP at an https URL (the legacy HTTP+SSE
// transport and stdio are not supported), authenticated with nothing, a static bearer token or OAuth 2.1
// as the MCP authorization specification describes it: discovery of the authorization server from the
// server's protected resource metadata, PKCE S256, Dynamic Client Registration when no client_id is
// given and resource = the server URL.
//
// Every call opens its own MCP session: initialize, then the request (tools/list or tools/call), then the
// session is closed. The client offers no capabilities, so the server's requests (sampling, elicitation,
// roots) are refused. Every address the client dials (the server, every redirect, every OAuth endpoint)
// passes the SSRF guard: private, loopback, link-local and metadata addresses are refused at dial time.
//
// The schema of a tool is pinned in the node's params when a graph is saved (ToolPin): a run calls the
// tool with the arguments rendered by the pinned schema, and a server that changed the schema since is a
// warning of the editor's validation, never a silent change of the graph version.
package mcp

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/iota-uz/iota-sdk/pkg/automation/catalog"
	"github.com/iota-uz/iota-sdk/pkg/automation/connectors"
)

// Kind is the connector kind of MCP servers.
const Kind catalog.ConnectorKind = catalog.KindMCP

// Credential keys of a connection (catalog.Credentials, flat). The form shows server_url, auth, token,
// oauth_client_id, oauth_client_secret and oauth_scopes; the rest are written by the connector after an
// OAuth sign-in and never shown.
const (
	KeyServerURL    = "server_url"          // https, passes the SSRF guard
	KeyAuth         = "auth"                // "none" | "bearer" | "oauth2"
	KeyToken        = "token"               // x-secret; auth=bearer
	KeyClientID     = "oauth_client_id"     // given by the administrator or registered with DCR
	KeyClientSecret = "oauth_client_secret" // x-secret, optional
	KeyScopes       = "oauth_scopes"        // separated by spaces
	KeyAccessToken  = "oauth_access_token"  // x-secret
	KeyRefreshToken = "oauth_refresh_token" // x-secret
	KeyExpiresAt    = "oauth_expires_at"    // RFC 3339
	KeyIssuer       = "oauth_issuer"        // the authorization server from discovery
	// KeyTokenEndpoint is the token endpoint of the issuer, kept for refreshes.
	KeyTokenEndpoint = "oauth_token_endpoint"
	// KeyResource is the server URL the tokens were issued for: a token of another URL is not sent.
	KeyResource = "oauth_resource"
	// KeyRegistered marks a client_id this connector registered with DCR (not the administrator's).
	KeyRegistered = "oauth_client_registered"
)

// Auth kinds of KeyAuth.
const (
	AuthNone   = "none"
	AuthBearer = "bearer"
	AuthOAuth2 = "oauth2"
)

// Limits of the connector.
const (
	// MaxResponseBytes caps a response of the server (a JSON body or an SSE stream).
	MaxResponseBytes = 1 << 20
	// ToolsCacheTTL is how long a tools/list of a connection is reused.
	ToolsCacheTTL = 5 * time.Minute
	// StateTTL is how long an OAuth sign-in in progress is accepted.
	StateTTL = 10 * time.Minute
)

// Tool is a tool of an MCP server.
type Tool struct {
	Name         string          `json:"name"`
	Title        string          `json:"title,omitempty"`
	Description  string          `json:"description,omitempty"`
	InputSchema  json.RawMessage `json:"input_schema"`
	OutputSchema json.RawMessage `json:"output_schema,omitempty"`
	// ReadOnly is the server's readOnlyHint: a dry run executes the tool instead of capturing it. It is a
	// hint, not a guarantee: the node is never repeated automatically whatever it says.
	ReadOnly bool `json:"read_only,omitempty"`
}

// ToolLister lists the tools of a connection, for the editor and the validator; a list is reused for
// at most ToolsCacheTTL.
type ToolLister interface {
	Tools(ctx context.Context, tenantID, connectionID uuid.UUID) ([]Tool, error)
}

// Store is what the connector needs of the connection store: the credentials of a connection, their
// update under the row lock (an OAuth refresh) and the sealing of OAuth states. connectors.NewStore
// implements it.
type Store interface {
	Get(ctx context.Context, tenantID, id uuid.UUID) (connectors.Connection, connectors.Credentials, error)
	connectors.CredentialLocker
	connectors.Sealer
}

// Connector is the MCP connector: connectors.Connector, connectors.ConnectionAuthorizer for OAuth and
// ToolLister. Its store is bound by the application (BindStore) before connections are used.
type Connector struct {
	net       *network
	clientID  string // the client name sent in initialize and DCR
	now       func() time.Time
	mu        sync.RWMutex
	store     Store
	cache     *toolCache
	node      *ToolNode
	userAgent string
}

var (
	_ connectors.Connector            = (*Connector)(nil)
	_ connectors.ConnectionAuthorizer = (*Connector)(nil)
	_ connectors.StoreBinder          = (*Connector)(nil)
	_ ToolLister                      = (*Connector)(nil)
)

// Option configures New.
type Option func(*Connector)

// WithAllowedPorts sets the ports a server URL, a redirect and an OAuth endpoint may name (default 443).
func WithAllowedPorts(ports ...uint16) Option {
	return func(c *Connector) {
		if len(ports) > 0 {
			c.net.ports = append([]uint16(nil), ports...)
		}
	}
}

// WithTLSConfig is the TLS configuration of the client (a private CA; tests trust their own server).
func WithTLSConfig(cfg *tls.Config) Option { return func(c *Connector) { c.net.tls = cfg } }

// WithAddressPolicy replaces the SSRF guard: allow returns nil for an address ("ip:port") the client may
// dial. For tests and for a deployment that deliberately calls an MCP server on a private network; the
// default refuses private, loopback, link-local and metadata addresses.
func WithAddressPolicy(allow func(network, address string) error) Option {
	return func(c *Connector) { c.net.guard = allow }
}

// WithStore binds the connection store at construction (BindStore does it later).
func WithStore(s Store) Option { return func(c *Connector) { c.store = s } }

// New is the MCP connector.
func New(opts ...Option) *Connector {
	c := &Connector{net: newNetwork(), clientID: "IOTA SDK automations", now: time.Now, cache: newToolCache(), userAgent: "iota-sdk-automations-mcp/1"}
	for _, o := range opts {
		o(c)
	}
	c.net.build()
	c.node = &ToolNode{c: c}
	return c
}

// BindStore gives the connector the connection store (connectors.StoreBinder): the application calls it
// once the store is built. A store without UpdateCredentials and sealing is ignored.
func (c *Connector) BindStore(s connectors.Store) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.store == nil {
		c.store = s
	}
}

func (c *Connector) boundStore() (Store, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.store == nil {
		return nil, fmt.Errorf("%w: the MCP connector has no connection store", connectors.ErrNotConfigured)
	}
	return c.store, nil
}

func (c *Connector) Kind() catalog.ConnectorKind { return Kind }

// Node is the mcp_tool node of the connector.
func (c *Connector) Node() *ToolNode { return c.node }

func (c *Connector) Nodes() []catalog.NodeType { return []catalog.NodeType{c.node} }

func (c *Connector) Triggers() []catalog.TriggerSource { return nil }

const credentialsSchema = `{
	"type": "object",
	"required": ["server_url", "auth"],
	"properties": {
		"server_url": {"type": "string", "title": "Server URL", "x-label": {"en":"Server URL","ru":"URL сервера","uz":"Server URL manzili","oz":"Сервер URL манзили"}, "description": "The Streamable HTTP endpoint of the MCP server (https)", "pattern": "^https://", "maxLength": 2000, "examples": ["https://mcp.example.com/mcp"]},
		"auth": {"type": "string", "title": "Authentication", "x-label": {"en":"Authentication","ru":"Аутентификация","uz":"Autentifikatsiya","oz":"Аутентификация"}, "enum": ["none", "bearer", "oauth2"], "default": "none"},
		"token": {"type": "string", "title": "Bearer token", "x-label": {"en":"Bearer token","ru":"Bearer-токен","uz":"Bearer token","oz":"Bearer токен"}, "description": "For authentication bearer", "x-secret": true, "maxLength": 8000},
		"oauth_client_id": {"type": "string", "title": "OAuth client ID", "x-label": {"en":"OAuth client ID","ru":"OAuth client ID","uz":"OAuth client ID","oz":"OAuth client ID"}, "description": "For OAuth; left empty, the client registers itself with the server (DCR)", "maxLength": 500},
		"oauth_client_secret": {"type": "string", "title": "OAuth client secret", "x-label": {"en":"OAuth client secret","ru":"OAuth client secret","uz":"OAuth client secret","oz":"OAuth client secret"}, "description": "For OAuth, when the client has one", "x-secret": true, "maxLength": 2000},
		"oauth_scopes": {"type": "string", "title": "OAuth scopes", "x-label": {"en":"OAuth scopes","ru":"OAuth scopes (через пробел)","uz":"OAuth scopes (bo‘sh joy bilan)","oz":"OAuth scopes (бўш жой билан)"}, "description": "Separated by spaces", "maxLength": 2000},
		"oauth_access_token": {"type": "string", "x-secret": true, "x-internal": true},
		"oauth_refresh_token": {"type": "string", "x-secret": true, "x-internal": true},
		"oauth_expires_at": {"type": "string", "x-internal": true},
		"oauth_issuer": {"type": "string", "x-internal": true},
		"oauth_token_endpoint": {"type": "string", "x-internal": true},
		"oauth_resource": {"type": "string", "x-internal": true},
		"oauth_client_registered": {"type": "string", "x-internal": true}
	},
	"additionalProperties": false
}`

// Descriptor describes the connector. Auth is custom: the OAuth endpoints and client are each
// connection's own, so the connector signs in itself (connectors.ConnectionAuthorizer).
func (c *Connector) Descriptor() connectors.Descriptor {
	return connectors.Descriptor{
		Kind:        Kind,
		Title:       map[string]string{"en": "MCP server", "ru": "MCP-сервер", "uz": "MCP server", "oz": "MCP сервер"},
		Description: map[string]string{"en": "Tools of a remote MCP server (Streamable HTTP)", "ru": "Инструменты удалённого MCP-сервера (Streamable HTTP)", "uz": "Masofaviy MCP server vositalari (Streamable HTTP)", "oz": "Масофавий MCP сервер воситалари (Streamable HTTP)"},
		Category:    connectors.CategoryAI,
		Icon:        "plugs-connected",
		Auth:        connectors.AuthCustom,
		Credentials: json.RawMessage(credentialsSchema),
		Platform:    connectors.PlatformNone,
	}
}

// config is a connection's credentials, read.
type config struct {
	server *url.URL
	auth   string
	creds  connectors.Credentials
}

// errConfig is a connection whose credentials do not describe a server it may call.
var errConfig = errors.New("mcp: the connection's settings are invalid")

func readConfig(creds connectors.Credentials) (config, error) {
	raw := strings.TrimSpace(creds[KeyServerURL])
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "https" || u.Host == "" || u.User != nil || u.Fragment != "" {
		return config{}, fmt.Errorf("%w: server_url is an https URL", errConfig)
	}
	auth := creds[KeyAuth]
	if auth == "" {
		auth = AuthNone
	}
	switch auth {
	case AuthNone:
	case AuthBearer:
		if creds[KeyToken] == "" {
			return config{}, fmt.Errorf("%w: auth bearer needs a token", errConfig)
		}
	case AuthOAuth2:
	default:
		return config{}, fmt.Errorf("%w: auth is none, bearer or oauth2", errConfig)
	}
	return config{server: u, auth: auth, creds: creds}, nil
}

// authorized reports whether OAuth credentials hold a token issued for the configured server.
func (cf config) authorized() bool {
	return cf.creds[KeyAccessToken] != "" && cf.creds[KeyResource] == cf.server.String()
}

// Test runs initialize and tools/list with the credentials and returns the server's identity: its
// name and version and the number of its tools. The error names a category, never a credential.
func (c *Connector) Test(ctx context.Context, creds connectors.Credentials) (connectors.Identity, error) {
	cf, err := readConfig(creds)
	if err != nil {
		return connectors.Identity{}, err
	}
	if cf.auth == AuthOAuth2 && !cf.authorized() {
		return connectors.Identity{}, errors.New("mcp: unauthorized: the connection needs an OAuth sign-in")
	}
	info, tools, err := c.list(ctx, cf, cf.creds[accessKey(cf.auth)])
	if err != nil {
		return connectors.Identity{}, testError(err)
	}
	sc := newScrubber(cf.creds)
	info.Name, info.Version = sc.text(info.Name), sc.text(info.Version)
	return identityOf(info, len(tools)), nil
}

// testError is a Test failure by category: the integrations page classifies it by its words
// (unauthorized, rate limited, unavailable), and nothing of the server's text is kept.
func testError(err error) error {
	var rl *rateLimitedError
	if errors.As(err, &rl) {
		return errors.New("mcp: rate limited by the server (HTTP 429)")
	}
	var ce *catalog.CallError
	if errors.As(err, &ce) {
		switch ce.Code {
		case catalog.CallUnauthorized:
			return errors.New("mcp: unauthorized: the server refused the credentials (HTTP 401)")
		case catalog.CallRateLimited:
			return errors.New("mcp: rate limited by the server (HTTP 429)")
		case catalog.CallUnavailable:
			return errors.New("mcp: the server is unavailable")
		}
		return errors.New("mcp: the server gave an invalid answer")
	}
	return errors.New("mcp: the server is unavailable")
}

// identityOf is the non-secret facts of a connection: the server's name and version and how many of
// its tools it lists, as a typed identity (connectors.IdentityServer) the page shows. Characters outside
// the identity's shape are replaced; the store still refuses one that carries a secret.
func identityOf(info serverInfo, tools int) connectors.Identity {
	name := safeLabel(info.Name, 100)
	if name == "" {
		name = "MCP"
	}
	if v := safeLabel(info.Version, 40); v != "" {
		name += " " + v
	}
	unit := "tools"
	if tools == 1 {
		unit = "tool"
	}
	return connectors.Identity{Display: fmt.Sprintf("%s (%d %s)", name, tools, unit), External: safeLabel(info.Name, 100), Kind: connectors.IdentityServer}
}

// safeLabel keeps letters, digits and . _ + - / of s, other runs as one "-", at most n bytes.
func safeLabel(s string, n int) string {
	var b strings.Builder
	dash := false
	for _, r := range s {
		ok := (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || strings.ContainsRune("._+-/", r)
		switch {
		case ok:
			b.WriteRune(r)
			dash = false
		case !dash && b.Len() > 0:
			b.WriteByte('-')
			dash = true
		}
		if b.Len() >= n {
			break
		}
	}
	return strings.Trim(b.String(), "-")
}

func accessKey(auth string) string {
	switch auth {
	case AuthBearer:
		return KeyToken
	case AuthOAuth2:
		return KeyAccessToken
	}
	return ""
}
