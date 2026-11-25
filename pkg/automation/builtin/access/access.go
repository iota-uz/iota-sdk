// Package access is what the built-in Code and HTTP nodes reach outside the sandbox with: the tenant's
// secrets a node declares, the connections it names (their authorization added by the SDK) and the
// guarded network. Every value it hands a node is also masked in what the node returns.
package access

import (
	"context"
	"encoding/json"
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
	CodeSecretMissing     = "secret_missing"
	CodeConnectionMissing = "connection_missing"
	// CodeConnectionDisabled: an administrator disabled the connection. CodeConnectionBroken: it needs
	// re-authorization or its last check failed.
	CodeConnectionDisabled       = "connection_disabled"
	CodeConnectionBroken         = "connection_broken"
	CodeConnectionOriginMismatch = "connection_origin_mismatch"
	CodeNetworkBlocked           = "network_blocked"
	CodeResponseTooLarge         = "response_too_large"
	CodeTimeout                  = "timeout"
	CodeInvalidParams            = "invalid_params"
	// CodePermissionDenied: the run-as subject lacks the permission of a connection operation.
	CodePermissionDenied = "permission_denied"
	// CodeUnavailable: a store the node reads before its call (secrets, connections) failed; nothing was
	// sent, and the failure is retryable.
	CodeUnavailable = catalog.CallUnavailable
)

// unavailable is a failure before the call: nothing was sent, so it is never an unknown outcome.
func unavailable(what string) error {
	return &catalog.CallError{Code: CodeUnavailable, Message: what + " is not available now", Retryable: true}
}

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

// FromResolver reads secrets through the host's secret resolver: the tenant's store. A name
// CONNECTION__FIELD (API_TOKEN is not one) is the secret field FIELD of the tenant's connection named
// connection (lower case: a node declares TELEGRAM__TOKEN for the field token of the connection
// telegram), as connectors.SecretResolver serves it; any other name goes to the resolver as it is (the
// application's secrets behind it). Any refusal of the resolver is a missing secret.
func FromResolver(r host.SecretResolver) Secrets { return resolverSecrets{r: r} }

type resolverSecrets struct{ r host.SecretResolver }

// ConnectionSecret is the connection secret a declared name stands for ("<connection>.<field>"), if it
// is one: CONNECTION__FIELD.
func ConnectionSecret(name string) (string, bool) {
	conn, field, ok := strings.Cut(name, "__")
	if !ok || conn == "" || field == "" {
		return "", false
	}
	return strings.ToLower(conn) + "." + strings.ToLower(field), true
}

func (s resolverSecrets) Secret(ctx context.Context, tenantID uuid.UUID, runAs spi.Subject, name string) (string, error) {
	if s.r == nil {
		return "", ErrSecretMissing
	}
	sub := runAs
	sub.TenantID = tenantID
	names := []string{name}
	if ref, ok := ConnectionSecret(name); ok {
		names = []string{ref, name}
	}
	for _, n := range names {
		v, err := s.r.Secret(ctx, spi.Scope{Subject: sub}, n)
		switch {
		case err != nil && host.IsInfrastructure(err):
			return "", err
		case err == nil && v != "":
			return v, nil
		}
	}
	return "", fmt.Errorf("%w: %s", ErrSecretMissing, name)
}

// ReadSecrets reads the declared secrets of a call; a missing one is a CallError secret_missing.
func ReadSecrets(ctx context.Context, s Secrets, in catalog.CallInput, names []string) (map[string]string, error) {
	out := make(map[string]string, len(names))
	for _, name := range names {
		if s == nil {
			return nil, &catalog.CallError{Code: CodeSecretMissing, Message: "unknown secret " + name}
		}
		v, err := s.Secret(ctx, in.TenantID, in.RunAs, name)
		switch {
		case errors.Is(err, ErrSecretMissing):
			return nil, &catalog.CallError{Code: CodeSecretMissing, Message: "unknown secret " + name}
		case errors.Is(err, ErrSecretForbidden):
			return nil, &catalog.CallError{Code: CodeSecretForbidden, Message: "secret " + name + " is not available to the user the automation runs as"}
		case err != nil:
			return nil, unavailable("the secret store")
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
	// operator serves Access.Operations; info and creds are the connection's own, given to it with the call.
	operator connectors.CodeOperator
	info     *catalog.ConnectionInfo
	creds    connectors.Credentials
	// check re-reads the connection's status before each call (nil: nothing to re-read).
	check func(ctx context.Context) error
}

// WithCheck is the connection with check run before each of its calls: it returns a typed refusal
// (connectors.ErrCodeConnectionDisabled, ErrCodeConnectionBroken, ErrConnectionMissing) when the connection
// changed since it was opened, so a run never keeps calling a connection an administrator disabled.
func (c Connection) WithCheck(check func(ctx context.Context) error) Connection {
	c.check = check
	return c
}

// Check is the connection's current state: nil while it may be called.
func (c Connection) Check(ctx context.Context) error {
	if c.check == nil {
		return nil
	}
	return c.check(ctx)
}

// OperatorBinding is what an operator-backed connection carries besides its HTTP access.
type OperatorBinding struct {
	Operator connectors.CodeOperator
	Info     catalog.ConnectionInfo
	// Credentials are the connection's decrypted credentials; they reach only the operator.
	Credentials connectors.Credentials
}

// WithOperator is the connection with its operations served by b.Operator.
func (c Connection) WithOperator(b OperatorBinding) Connection {
	info := b.Info
	c.operator, c.info, c.creds = b.Operator, &info, b.Credentials
	return c
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
	case cur.Name != name:
		return Connection{}, fmt.Errorf("%w: %s is not connected", ErrConnectionMissing, name)
	case cur.Status == connectors.StatusDisabled:
		return Connection{}, fmt.Errorf("%w: %s", connectors.ErrCodeConnectionDisabled, name)
	case cur.Status == connectors.StatusNeedsReauth || cur.Status == connectors.StatusError:
		return Connection{}, fmt.Errorf("%w: %s", connectors.ErrCodeConnectionBroken, name)
	case cur.Status != connectors.StatusConnected:
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
			return Connection{}, fmt.Errorf("%w: %s needs to be authorized again", connectors.ErrCodeConnectionBroken, name)
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
	// Only what is secret is masked: the x-secret fields and the access token, not an account id or a
	// region that would turn every equal value of the output into ***.
	secret := connectors.SecretFields(conn.Descriptor())
	secret[connectors.OAuth2AccessToken] = true
	values := make([]string, 0, len(secret))
	for k, v := range creds {
		if secret[k] {
			values = append(values, v)
		}
	}
	access := ca.CodeAccess()
	if r, ok := ca.(connectors.CodeBaseURLResolver); ok {
		base, err := r.CodeBaseURL(creds)
		if err == nil {
			err = connectors.CheckCodeBaseURL(base)
		}
		if err != nil {
			// The URL may carry a token: the error says what is wrong with the connection, never the URL.
			return Connection{}, fmt.Errorf("%w: %s has no usable base URL", connectors.ErrCodeConnectionBroken, name)
		}
		access.BaseURL = base
	}
	id, kind := cur.ID, cur.Kind
	out := Connection{
		Name: name, Kind: cur.Kind, Access: access, Values: values,
		auth: func(ctx context.Context, header http.Header) error { return ca.Authorize(ctx, creds, header) },
		// The status is read again before each call, without the credentials: a connection disabled or
		// marked broken during the run is refused from then on.
		check: func(ctx context.Context) error { return s.status(ctx, tenantID, id, kind, name) },
	}
	if op, ok := ca.(connectors.CodeOperator); ok && len(access.Operations) > 0 {
		out = out.WithOperator(OperatorBinding{
			Operator:    op,
			Info:        catalog.ConnectionInfo{ID: cur.ID.String(), Kind: cur.Kind, Name: cur.Name},
			Credentials: creds,
		})
	}
	return out, nil
}

// status is the typed state of an opened connection now: nil while it is connected under its name.
func (s storeConnections) status(ctx context.Context, tenantID, id uuid.UUID, kind catalog.ConnectorKind, name string) error {
	list, err := s.store.List(ctx, tenantID, kind)
	if err != nil {
		return err
	}
	for _, c := range list {
		if c.ID != id {
			continue
		}
		switch {
		case c.Name != name:
			return fmt.Errorf("%w: %s was renamed", ErrConnectionMissing, name)
		case c.Status == connectors.StatusConnected:
			return nil
		case c.Status == connectors.StatusDisabled:
			return fmt.Errorf("%w: %s", connectors.ErrCodeConnectionDisabled, name)
		case c.Status == connectors.StatusNeedsReauth || c.Status == connectors.StatusError:
			return fmt.Errorf("%w: %s", connectors.ErrCodeConnectionBroken, name)
		}
		return fmt.Errorf("%w: %s is not connected", ErrConnectionMissing, name)
	}
	return fmt.Errorf("%w: %s was deleted", ErrConnectionMissing, name)
}

// OpenAll opens the connections a node declares; a missing one is a CallError connection_missing.
func OpenAll(ctx context.Context, cs Connections, tenantID uuid.UUID, names []string) (map[string]Connection, error) {
	return OpenFor(ctx, cs, catalog.CallInput{TenantID: tenantID}, names, "")
}

// OpenFor is OpenAll for a node's call: with use set, the run-as subject must hold that permission to use
// the tenant's connections at all (CallError permission_denied before any connection is opened).
func OpenFor(ctx context.Context, cs Connections, in catalog.CallInput, names []string, use string) (map[string]Connection, error) {
	tenantID := in.TenantID
	if len(names) > 0 && use != "" && !in.RunAs.Has(use) {
		return nil, &catalog.CallError{Code: CodePermissionDenied, Message: "the user the automation runs as needs " + use + " to use connections"}
	}
	out := make(map[string]Connection, len(names))
	for _, name := range names {
		if cs == nil {
			return nil, &catalog.CallError{Code: CodeConnectionMissing, Message: "connection " + name + " is not connected"}
		}
		c, err := cs.Open(ctx, tenantID, name)
		switch {
		case errors.Is(err, connectors.ErrCodeConnectionDisabled):
			return nil, &catalog.CallError{Code: CodeConnectionDisabled, Message: "connection " + name + " is disabled"}
		case errors.Is(err, connectors.ErrCodeConnectionBroken):
			return nil, &catalog.CallError{Code: CodeConnectionBroken, Message: "connection " + name + " needs to be reconnected"}
		case errors.Is(err, ErrConnectionMissing):
			return nil, &catalog.CallError{Code: CodeConnectionMissing, Message: "connection " + name + " is not connected or offers no code access"}
		case err != nil:
			return nil, unavailable("the connection store")
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
	// The target is classified by what it parses to, never by its text: "://" in a query is a relative path's.
	parsed, err := url.Parse(target)
	if err != nil {
		return nil, &code.ConnectionRefusedError{Code: code.ReasonConnectionOriginMismatch, Message: "the URL is invalid"}
	}
	u := parsed
	switch {
	case parsed.Scheme != "" || parsed.Host != "":
		// An absolute URL: it must be on the base URL's origin (checked below).
	case parsed.Opaque != "":
		return nil, &code.ConnectionRefusedError{Code: code.ReasonConnectionOriginMismatch, Message: "the path is invalid"}
	default:
		// A path relative to the base URL: the escaped paths are joined and the decoded path derived from the
		// result, so the two always agree and an escaped separator (%2F, also a leading one) stays one segment.
		joined := *base
		raw := strings.TrimSuffix(base.EscapedPath(), "/") + "/" + strings.TrimPrefix(parsed.EscapedPath(), "/")
		decoded, err := url.PathUnescape(raw)
		if err != nil {
			return nil, &code.ConnectionRefusedError{Code: code.ReasonConnectionOriginMismatch, Message: "the path is invalid"}
		}
		joined.Path, joined.RawPath = decoded, raw
		if joined.EscapedPath() != raw {
			joined.RawPath = ""
		}
		joined.RawQuery = parsed.RawQuery
		joined.Fragment, joined.RawFragment = "", ""
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

// Gateway performs the requests and the operations of env.connections through a run's opened connections
// and the guarded fetcher.
type Gateway struct {
	Fetcher     code.Fetcher
	Connections map[string]Connection
	// Input is the Code node's call: operations are checked against Input.RunAs and receive it (with the
	// connection's own info and credentials) as their call.
	Input catalog.CallInput
	// Use is the permission the run-as subject needs for every call through a connection, HTTP and
	// operations alike ("": none beyond an operation's own).
	Use string
}

// admit checks a call before it reaches the provider: the run-as subject may use connections, and the
// connection is still connected (its status is read again: an administrator may have disabled it during
// the run).
func (g *Gateway) admit(ctx context.Context, c Connection) error {
	if g.Use != "" && !g.Input.RunAs.Has(g.Use) {
		return &code.ConnectionRefusedError{Code: CodePermissionDenied, Message: "the user the automation runs as needs " + g.Use + " to use connections"}
	}
	err := c.Check(ctx)
	switch {
	case err == nil:
		return nil
	case errors.Is(err, connectors.ErrCodeConnectionDisabled):
		return &code.ConnectionRefusedError{Code: CodeConnectionDisabled, Message: "connection " + c.Name + " is disabled"}
	case errors.Is(err, connectors.ErrCodeConnectionBroken):
		return &code.ConnectionRefusedError{Code: CodeConnectionBroken, Message: "connection " + c.Name + " needs to be reconnected"}
	case errors.Is(err, ErrConnectionMissing):
		return &code.ConnectionRefusedError{Code: code.ReasonConnectionMissing, Message: "connection " + c.Name + " is no longer connected"}
	case ctx.Err() != nil:
		return ctx.Err()
	}
	return &code.ConnectionRefusedError{Code: CodeUnavailable, Message: "the connection store is not available now", Retryable: true}
}

var (
	_ code.ConnectionCaller   = (*Gateway)(nil)
	_ code.ConnectionOperator = (*Gateway)(nil)
)

// hopByHop are headers a request through a connection never takes from the code, besides the ones the
// sandbox refuses.
var hopByHop = []string{"Connection", "Keep-Alive", "Proxy-Authenticate", "Proxy-Authorization", "Te", "Trailer",
	"Transfer-Encoding", "Upgrade", "Host", "Content-Length"}

// CallConnection implements code.ConnectionCaller.
func (g *Gateway) CallConnection(ctx context.Context, req code.ConnectionRequest) (*code.FetchResponse, error) {
	c, ok := g.Connections[req.Connection]
	if !ok {
		return nil, &code.ConnectionRefusedError{Code: code.ReasonConnectionMissing, Message: "connection " + req.Connection + " is not open"}
	}
	var httpMethod string
	header := http.Header{}
	var sensitive []string
	var authHeader http.Header
	if req.Fetch {
		if !c.Access.Fetch {
			return nil, fmt.Errorf("connection %s has no fetch", req.Connection)
		}
		httpMethod = strings.ToUpper(req.HTTPMethod)
		if httpMethod == "" {
			httpMethod = http.MethodGet
		}
		for k, vs := range req.Header {
			header[http.CanonicalHeaderKey(k)] = append([]string(nil), vs...)
		}
		for _, h := range hopByHop {
			header.Del(h)
		}
	} else {
		m, ok := c.Access.Method(req.Method)
		if !ok {
			return nil, fmt.Errorf("connection %s has no method %s", req.Connection, req.Method)
		}
		httpMethod = strings.ToUpper(m.HTTPMethod)
		header.Set("Accept", "application/json")
		if len(req.Body) > 0 {
			header.Set("Content-Type", "application/json")
		}
	}
	u, err := Target(c, req.Path)
	if err != nil {
		return nil, err
	}
	if err := g.admit(ctx, c); err != nil {
		return nil, err
	}
	var added []string
	if req.Fetch {
		// Authorize runs on its own headers and wins: whatever the code set under a name it sets is replaced.
		authHeader = http.Header{}
		if err := c.Authorize(ctx, authHeader); err != nil {
			return nil, fmt.Errorf("connection %s could not authorize the request", req.Connection)
		}
		for k, vs := range authHeader {
			header.Del(k)
			header[k] = append([]string(nil), vs...)
			sensitive = append(sensitive, k)
		}
		added = AddedValues(http.Header{}, authHeader)
	} else {
		before := header.Clone()
		if err := c.Authorize(ctx, header); err != nil {
			return nil, fmt.Errorf("connection %s could not authorize the request", req.Connection)
		}
		added = AddedValues(before, header)
	}
	masker := Masker(append(append([]string(nil), c.Values...), added...)...)
	if g.Fetcher == nil {
		return nil, &code.NetworkBlockedError{Reason: "disabled"}
	}
	resp, err := g.Fetcher.Fetch(ctx, code.FetchRequest{
		URL: u.String(), Method: httpMethod, Header: header, Body: req.Body, SameOrigin: true, Sensitive: sensitive, Redirect: req.Redirect,
	})
	if err != nil {
		var left *code.OriginError
		var blocked *code.NetworkBlockedError
		switch {
		case errors.As(err, &left):
			return nil, &code.ConnectionRefusedError{Code: code.ReasonConnectionOriginMismatch, Message: "connection " + c.Name + " was redirected off its origin"}
		case errors.As(err, &blocked):
			return nil, err
		}
		// An API may echo what it was sent: the error reaches the code only masked.
		return nil, errors.New(masker.Mask(err.Error()))
	}
	return RedactResponse(resp, masker), nil
}

// CallOperation implements code.ConnectionOperator: the operation must be declared by the connection, the
// run-as subject must hold its permission, and neither its result nor its error carries the connection's
// credentials to the code.
func (g *Gateway) CallOperation(ctx context.Context, req code.OperationRequest) (json.RawMessage, error) {
	c, ok := g.Connections[req.Connection]
	if !ok {
		return nil, &code.ConnectionRefusedError{Code: code.ReasonConnectionMissing, Message: "connection " + req.Connection + " is not open"}
	}
	op, ok := c.Access.Operation(req.Operation)
	if !ok || c.operator == nil {
		return nil, fmt.Errorf("connection %s has no operation %s", req.Connection, req.Operation)
	}
	if err := g.admit(ctx, c); err != nil {
		return nil, err
	}
	if !g.Input.RunAs.Has(op.Permission) {
		return nil, &code.ConnectionRefusedError{Code: CodePermissionDenied,
			Message: "the user the automation runs as may not call " + req.Connection + "." + req.Operation}
	}
	in := g.Input
	in.Connection = c.info
	in.Credentials = c.creds
	masker := Masker(c.Values...)
	result, err := c.operator.CallCodeOperation(ctx, connectors.CodeCall{Operation: req.Operation, Args: json.RawMessage(req.Args), Input: in})
	if err != nil {
		var opErr *connectors.CodeOperationError
		if errors.As(err, &opErr) {
			return nil, &code.ConnectionRefusedError{Code: code.FailureReason(opErr.Code), Message: masker.Mask(opErr.Message), Retryable: opErr.Retryable}
		}
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return nil, errors.New(masker.Mask(err.Error()))
	}
	raw, err := json.Marshal(result)
	if err != nil {
		return nil, fmt.Errorf("connection %s: operation %s returned a value that is not JSON", req.Connection, req.Operation)
	}
	return masker.MaskJSON(raw), nil
}

// AddedValues are the header values Authorize set or changed (and their parts after a scheme), so a
// token it computed (Basic credentials) is masked too.
func AddedValues(before, after http.Header) []string {
	var out []string
	for k, vs := range after {
		prev := strings.Join(before.Values(k), "\x00")
		for _, v := range vs {
			if strings.Contains(prev, v) {
				continue
			}
			out = append(out, v)
			if _, rest, ok := strings.Cut(v, " "); ok && rest != "" {
				out = append(out, rest)
			}
		}
	}
	return out
}

// RedactResponse is a response with the masker's values replaced in its header names and values, its
// body and its URL: what an API reflects of the credentials never reaches the code. A JSON body is
// redacted as JSON (its strings, keys and numbers decoded first, so an escaped spelling of a credential is
// found too); any other body as text.
func RedactResponse(r *code.FetchResponse, m code.Masker) *code.FetchResponse {
	if r == nil {
		return nil
	}
	out := *r
	out.Header = make(http.Header, len(r.Header))
	for k, vs := range r.Header {
		name := m.Mask(k)
		for _, v := range vs {
			out.Header.Add(name, m.Mask(v))
		}
	}
	if json.Valid(r.Body) {
		out.Body = m.MaskJSON(r.Body)
	} else {
		out.Body = []byte(m.Mask(string(r.Body)))
	}
	out.URL = m.Mask(r.URL)
	out.StatusText = m.Mask(r.StatusText)
	return &out
}

// Masker masks the values (secrets, credentials) in everything a node returns.
func Masker(values ...string) code.Masker {
	m := make(map[string]string, len(values))
	for i, v := range values {
		m[fmt.Sprintf("v%d", i)] = v
	}
	return code.NewMasker(m)
}

// HasOperator reports that the connection's operations are served (WithOperator).
func (c Connection) HasOperator() bool { return c.operator != nil }
