// Package access is what the built-in Code and HTTP nodes reach outside the sandbox with: the tenant's
// secrets a node declares, the connections it names (their authorization added by the SDK) and the
// guarded network. Every value it hands a node is also masked in what the node returns.
package access

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/google/uuid"

	"github.com/iota-uz/iota-sdk/pkg/automation/catalog"
	"github.com/iota-uz/iota-sdk/pkg/automation/code"
	"github.com/iota-uz/iota-sdk/pkg/automation/connectors"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/host"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/spi"
)

// Codes of the failures of built-in nodes (output.error.code).
const (
	CodeSecretMissing            = "secret_missing"
	CodeConnectionMissing        = "connection_missing"
	CodeConnectionOriginMismatch = "connection_origin_mismatch"
	CodeNetworkBlocked           = "network_blocked"
	CodeResponseTooLarge         = "response_too_large"
	CodeTimeout                  = "timeout"
	CodeInvalidParams            = "invalid_params"
)

// ErrSecretMissing reports that the tenant has no secret of the name.
var ErrSecretMissing = errors.New("automation builtin: secret is missing")

// ErrConnectionMissing reports that the tenant has no connected connection of the name whose connector offers code
// access (connectors.CodeAccessor), or several.
var ErrConnectionMissing = errors.New("automation builtin: connection is missing")

// Secrets are the tenant's secrets a node reads by name, as the run-as.
type Secrets interface {
	// Secret is the value of a secret; ErrSecretMissing when the tenant has none of the name.
	Secret(ctx context.Context, tenantID uuid.UUID, runAs spi.Subject, name string) (string, error)
}

// FromResolver reads secrets through the host's secret resolver (the tenant's store: the connections'
// secret fields "<connection>.<field>" and what the application adds behind them). Any refusal of the
// resolver is a missing secret.
func FromResolver(r host.SecretResolver) Secrets { return resolverSecrets{r: r} }

type resolverSecrets struct{ r host.SecretResolver }

func (s resolverSecrets) Secret(ctx context.Context, tenantID uuid.UUID, runAs spi.Subject, name string) (string, error) {
	if s.r == nil {
		return "", ErrSecretMissing
	}
	sub := runAs
	sub.TenantID = tenantID
	v, err := s.r.Secret(ctx, spi.Scope{Subject: sub}, name)
	if err != nil {
		if host.IsInfrastructure(err) {
			return "", err
		}
		return "", fmt.Errorf("%w: %s", ErrSecretMissing, name)
	}
	if v == "" {
		return "", fmt.Errorf("%w: %s", ErrSecretMissing, name)
	}
	return v, nil
}

// ReadSecrets reads the declared secrets of a call; a missing one is a CallError secret_missing.
func ReadSecrets(ctx context.Context, s Secrets, in catalog.CallInput, names []string) (map[string]string, error) {
	out := make(map[string]string, len(names))
	for _, name := range names {
		if s == nil {
			return nil, &catalog.CallError{Code: CodeSecretMissing, Message: "secret " + name + " is not set"}
		}
		v, err := s.Secret(ctx, in.TenantID, in.RunAs, name)
		switch {
		case errors.Is(err, ErrSecretMissing):
			return nil, &catalog.CallError{Code: CodeSecretMissing, Message: "secret " + name + " is not set"}
		case err != nil:
			return nil, err
		}
		out[name] = v
	}
	return out, nil
}

// Connection is a connection a node opened: its API and how to authorize a request of it. Values are
// its credentials, for the masker; never give them to code.
type Connection struct {
	Name   string
	Kind   catalog.ConnectorKind
	Access connectors.CodeAccess
	Values []string
	auth   func(ctx context.Context, header http.Header) error
}

// Authorize sets the connection's authorization on a request's headers.
func (c Connection) Authorize(ctx context.Context, header http.Header) error {
	if c.auth == nil {
		return nil
	}
	return c.auth(ctx, header)
}

// NewConnection builds a Connection (tests, other stores).
func NewConnection(name string, kind catalog.ConnectorKind, a connectors.CodeAccess, values []string, auth func(ctx context.Context, header http.Header) error) Connection {
	return Connection{Name: name, Kind: kind, Access: a, Values: values, auth: auth}
}

// Connections open the tenant's connections by name.
type Connections interface {
	// Open is the connected connection of the name whose connector offers code access; ErrConnectionMissing
	// otherwise.
	Open(ctx context.Context, tenantID uuid.UUID, name string) (Connection, error)
}

// StoreConnections opens connections from the connection store; an OAuth2 connection's access token is
// refreshed through oauth (nil: the stored token).
func StoreConnections(store connectors.ConnectionStore, dir connectors.Directory, oauth connectors.OAuth2) Connections {
	return storeConnections{store: store, dir: dir, oauth: oauth}
}

type storeConnections struct {
	store connectors.ConnectionStore
	dir   connectors.Directory
	oauth connectors.OAuth2
}

func (s storeConnections) Open(ctx context.Context, tenantID uuid.UUID, name string) (Connection, error) {
	if s.store == nil || s.dir == nil {
		return Connection{}, ErrConnectionMissing
	}
	list, err := s.store.List(ctx, tenantID, "")
	if errors.Is(err, connectors.ErrNotConfigured) {
		return Connection{}, ErrConnectionMissing
	}
	if err != nil {
		return Connection{}, err
	}
	var match []connectors.Connection
	for _, c := range list {
		if c.Name != name {
			continue
		}
		if conn, ok := s.dir.Get(c.Kind); ok {
			if _, codes := conn.(connectors.CodeAccessor); codes {
				match = append(match, c)
			}
		}
	}
	if len(match) != 1 {
		return Connection{}, fmt.Errorf("%w: %d connections named %s offer code access", ErrConnectionMissing, len(match), name)
	}
	cur, creds, err := s.store.Get(ctx, tenantID, match[0].ID)
	switch {
	case errors.Is(err, connectors.ErrNotFound), errors.Is(err, connectors.ErrUndecryptable), errors.Is(err, connectors.ErrNotConfigured):
		return Connection{}, fmt.Errorf("%w: %s", ErrConnectionMissing, name)
	case err != nil:
		return Connection{}, err
	case cur.Status != connectors.StatusConnected || cur.Name != name:
		return Connection{}, fmt.Errorf("%w: %s is not connected", ErrConnectionMissing, name)
	}
	conn, ok := s.dir.Get(cur.Kind)
	if !ok {
		return Connection{}, fmt.Errorf("%w: %s", ErrConnectionMissing, name)
	}
	ca, ok := conn.(connectors.CodeAccessor)
	if !ok {
		return Connection{}, fmt.Errorf("%w: %s", ErrConnectionMissing, name)
	}
	if conn.Descriptor().Auth == connectors.AuthOAuth2 && s.oauth != nil {
		token, err := s.oauth.Token(ctx, tenantID, cur.ID)
		switch {
		case errors.Is(err, connectors.ErrNeedsReauth), errors.Is(err, connectors.ErrNotFound):
			return Connection{}, fmt.Errorf("%w: %s needs to be authorized again", ErrConnectionMissing, name)
		case err != nil:
			return Connection{}, err
		}
		next := make(connectors.Credentials, len(creds)+1)
		for k, v := range creds {
			next[k] = v
		}
		next[connectors.OAuth2AccessToken] = token
		creds = next
	}
	values := make([]string, 0, len(creds))
	for _, v := range creds {
		values = append(values, v)
	}
	return Connection{
		Name: name, Kind: cur.Kind, Access: ca.CodeAccess(), Values: values,
		auth: func(ctx context.Context, header http.Header) error { return ca.Authorize(ctx, creds, header) },
	}, nil
}

// OpenAll opens the connections a node declares; a missing one is a CallError connection_missing.
func OpenAll(ctx context.Context, cs Connections, tenantID uuid.UUID, names []string) (map[string]Connection, error) {
	out := make(map[string]Connection, len(names))
	for _, name := range names {
		if cs == nil {
			return nil, &catalog.CallError{Code: CodeConnectionMissing, Message: "connection " + name + " is not connected"}
		}
		c, err := cs.Open(ctx, tenantID, name)
		switch {
		case errors.Is(err, ErrConnectionMissing):
			return nil, &catalog.CallError{Code: CodeConnectionMissing, Message: "connection " + name + " is not connected or offers no code access"}
		case err != nil:
			return nil, err
		}
		out[name] = c
	}
	return out, nil
}

// Origin is scheme://host:port of a URL, lower case, with the default port.
func Origin(u *url.URL) string {
	port := u.Port()
	if port == "" {
		port = "443"
		if u.Scheme == "http" {
			port = "80"
		}
	}
	return strings.ToLower(u.Scheme) + "://" + strings.ToLower(u.Hostname()) + ":" + port
}

// Target is the URL a request through a connection goes to: a path relative to the API's base URL, or
// an absolute URL that must be on the base URL's origin. A URL elsewhere is connection_origin_mismatch.
func Target(c Connection, target string) (*url.URL, error) {
	base, err := url.Parse(c.Access.BaseURL)
	if err != nil {
		return nil, &code.ConnectionRefusedError{Code: code.ReasonConnectionOriginMismatch, Message: "the connector's base URL is invalid"}
	}
	var u *url.URL
	if strings.Contains(target, "://") || strings.HasPrefix(target, "//") {
		if u, err = url.Parse(target); err != nil {
			return nil, &code.ConnectionRefusedError{Code: code.ReasonConnectionOriginMismatch, Message: "the URL is invalid"}
		}
	} else {
		rel, err := url.Parse(target)
		if err != nil || rel.Scheme != "" || rel.Host != "" {
			return nil, &code.ConnectionRefusedError{Code: code.ReasonConnectionOriginMismatch, Message: "the path is invalid"}
		}
		joined := *base
		joined.Path = strings.TrimSuffix(base.Path, "/") + "/" + strings.TrimPrefix(rel.Path, "/")
		joined.RawPath = ""
		joined.RawQuery = rel.RawQuery
		u = &joined
	}
	if u.User != nil || Origin(u) != Origin(base) {
		return nil, &code.ConnectionRefusedError{Code: code.ReasonConnectionOriginMismatch,
			Message: fmt.Sprintf("connection %s reaches only %s", c.Name, Origin(base))}
	}
	return u, nil
}

// BlockedMessage is the message of a request the network policy refused: the host and the reason, never
// the address it resolved to.
func BlockedMessage(b *code.NetworkBlockedError) string {
	switch b.Reason {
	case "disabled":
		return "network blocked: this node has no network access"
	case "redirect_limit":
		return "network blocked: too many redirects at " + b.Host
	case "scheme":
		return "network blocked: only https URLs are allowed: " + b.Host
	case "port":
		return "network blocked: the port is not allowed: " + b.Host
	}
	return "network blocked: " + b.Host + " is not reachable from automations"
}

// Gateway performs the requests of env.connections through a run's opened connections and the guarded
// fetcher.
type Gateway struct {
	Fetcher     code.Fetcher
	Connections map[string]Connection
}

var _ code.ConnectionCaller = (*Gateway)(nil)

// CallConnection implements code.ConnectionCaller.
func (g *Gateway) CallConnection(ctx context.Context, req code.ConnectionRequest) (*code.FetchResponse, error) {
	c, ok := g.Connections[req.Connection]
	if !ok {
		return nil, &code.ConnectionRefusedError{Code: code.ReasonConnectionMissing, Message: "connection " + req.Connection + " is not open"}
	}
	m, ok := c.Access.Method(req.Method)
	if !ok {
		return nil, fmt.Errorf("connection %s has no method %s", req.Connection, req.Method)
	}
	u, err := Target(c, req.Path)
	if err != nil {
		return nil, err
	}
	header := http.Header{}
	header.Set("Accept", "application/json")
	if len(req.Body) > 0 {
		header.Set("Content-Type", "application/json")
	}
	if err := c.Authorize(ctx, header); err != nil {
		return nil, fmt.Errorf("connection %s could not authorize the request", req.Connection)
	}
	if g.Fetcher == nil {
		return nil, &code.NetworkBlockedError{Reason: "disabled"}
	}
	return g.Fetcher.Fetch(ctx, code.FetchRequest{URL: u.String(), Method: strings.ToUpper(m.HTTPMethod), Header: header, Body: req.Body})
}

// Masker masks the values (secrets, credentials) in everything a node returns.
func Masker(values ...string) code.Masker {
	m := make(map[string]string, len(values))
	for i, v := range values {
		m[fmt.Sprintf("v%d", i)] = v
	}
	return code.NewMasker(m)
}
