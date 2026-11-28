// Package openbao is the OpenBao (and Vault) KV v2 adapter of the secret storage port (secrets.Writer,
// secrets.Reader, secrets.MetadataReader).
//
// A Client is one service identity: an Auth that logs the process in (a token file an agent keeps fresh,
// AppRole, Kubernetes or JWT) and the token it holds. The token is renewed before it expires and the
// process logs in again when it cannot be renewed (or OpenBao refuses it), so a long-running process
// outlives its token's TTL and picks up a rotated secret_id or JWT from its file.
//
// The three parts of the port are handed out separately (Client.Writer, Client.Reader,
// Client.MetadataReader), each a type that implements only its part: the composition gives the
// integrations page a Writer built over a role whose policy writes and does not read, and the runtime a
// Reader over a role that reads. A Writer is never also a Reader, and OpenBao refuses a read under a
// write-only policy (secrets.ErrForbidden).
//
// The adapter keeps the port's contract:
//
//   - a Read is one KV v2 version, whole: the fields of a value are never mixed from two versions;
//   - nothing is cached: a Put (a rotation) is what the next Read of any process sees;
//   - a Ref is accepted only when its path lies in its tenant (Config.Owns, by default a "tenants/<id>/"
//     segment pair): a ref of another tenant is secrets.ErrForbidden before any request, and OpenBao's
//     policy, written against the same paths, refuses it again;
//   - an outage (network, a sealed or overloaded server, an expired login that cannot be renewed) is
//     secrets.ErrUnavailable; nothing falls back to the environment or to a value read before;
//   - errors never carry a value or a token: a response body of OpenBao is never quoted.
package openbao

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/iota-uz/iota-sdk/pkg/automation/secrets"
)

const (
	defaultMount   = "secret"
	defaultTimeout = 10 * time.Second
	// minTTL: a token renewed to less than this is replaced by a new login (the role's max_ttl is near).
	minTTL = 10 * time.Second
	// maxResponse bounds a response body read from OpenBao.
	maxResponse = 4 << 20
)

// Config configures a Client.
type Config struct {
	// Address is OpenBao's base URL ("https://openbao.internal:8200").
	Address string
	// Mount is the KV v2 mount (default "secret").
	Mount string
	// Namespace is sent as X-Vault-Namespace when set.
	Namespace string
	// Auth logs the service in; required.
	Auth Auth
	// HTTPClient is the client requests go through (default: one with Timeout).
	HTTPClient *http.Client
	// Timeout bounds every request (default 10s); a context deadline that comes sooner wins.
	Timeout time.Duration
	// Owns reports whether path lies in the tenant (default OwnsTenantSegment). A Ref it refuses is
	// secrets.ErrForbidden before any request.
	Owns func(tenantID uuid.UUID, path string) bool
	// Now is the clock (default time.Now).
	Now func() time.Time
}

// OwnsTenantSegment is the default Config.Owns: the path has the segments "tenants/<tenantID>" (as
// secrets.DefaultLocator forms it, after any environment prefix).
func OwnsTenantSegment(tenantID uuid.UUID, path string) bool {
	segs := strings.Split(path, "/")
	want := tenantID.String()
	for i := 0; i+1 < len(segs); i++ {
		if segs[i] == "tenants" && segs[i+1] == want {
			return true
		}
	}
	return false
}

// Client is one service identity at an OpenBao server. It is safe for concurrent use.
type Client struct {
	base      *url.URL
	mount     string
	namespace string
	auth      Auth
	http      *http.Client
	timeout   time.Duration
	owns      func(uuid.UUID, string) bool
	now       func() time.Time

	// mountOK is when the mount's configuration was last verified (checkMount).
	mountMu sync.Mutex
	mountOK time.Time

	mu        sync.Mutex
	token     string
	renewable bool
	expires   time.Time // zero: the token does not expire
	renewAt   time.Time // zero with expires zero: never
}

// New builds a Client; it does not contact OpenBao (the first call logs in).
func New(cfg Config) (*Client, error) {
	addr := strings.TrimSpace(cfg.Address)
	if addr == "" {
		return nil, errors.New("openbao: address is required")
	}
	base, err := url.Parse(addr)
	if err != nil || (base.Scheme != "http" && base.Scheme != "https") || base.Host == "" {
		return nil, errors.New("openbao: address is not an http(s) URL")
	}
	base.Path = strings.TrimRight(base.Path, "/")
	if cfg.Auth == nil {
		return nil, errors.New("openbao: auth is required")
	}
	mount := strings.Trim(strings.TrimSpace(cfg.Mount), "/")
	if mount == "" {
		mount = defaultMount
	}
	if !cleanPath(mount, false) {
		return nil, errors.New("openbao: mount is not a clean path")
	}
	c := &Client{
		base: base, mount: mount, namespace: strings.TrimSpace(cfg.Namespace), auth: cfg.Auth,
		http: cfg.HTTPClient, timeout: cfg.Timeout, owns: cfg.Owns, now: cfg.Now,
	}
	if c.timeout <= 0 {
		c.timeout = defaultTimeout
	}
	if c.http == nil {
		c.http = &http.Client{Timeout: c.timeout}
	}
	if c.owns == nil {
		c.owns = OwnsTenantSegment
	}
	if c.now == nil {
		c.now = time.Now
	}
	return c, nil
}

// Writer is the write part of the port over this client's identity.
func (c *Client) Writer() secrets.Writer { return writer{c} }

// Reader is the read part of the port over this client's identity.
func (c *Client) Reader() secrets.Reader { return reader{c} }

// MetadataReader is the metadata part of the port over this client's identity.
func (c *Client) MetadataReader() secrets.MetadataReader { return metadataReader{c} }

// Store is all three parts over one identity (a policy that reads and writes).
func (c *Client) Store() secrets.Store { return store{writer{c}, reader{c}, metadataReader{c}} }

// cleanPath reports whether p is relative segments without "", "." or ".." (a trailing "/" allowed when dir).
func cleanPath(p string, dir bool) bool {
	if dir {
		p = strings.TrimSuffix(p, "/")
	}
	if p == "" || strings.HasPrefix(p, "/") {
		return false
	}
	for _, s := range strings.Split(p, "/") {
		if s == "" || s == "." || s == ".." || strings.ContainsAny(s, "?#%\\") {
			return false
		}
	}
	return true
}

// errForeign: a ref whose path lies outside its tenant.
var errForeign = errors.New("the path is not the tenant's")

// check is the tenant gate of every call: a ref without a tenant or with a path that is not clean is
// secrets.ErrForbidden; a path outside the tenant is errForeign (the caller maps it), never sent.
func (c *Client) check(ref secrets.Ref, dir bool) error {
	if ref.TenantID == uuid.Nil || ref.Path == "" || !cleanPath(ref.Path, dir) {
		return secrets.ErrForbidden
	}
	if !c.owns(ref.TenantID, ref.Path) {
		return errForeign
	}
	return nil
}

// checkRead is check for a read: another tenant's path is secrets.ErrNotFound for this tenant (the port's
// contract: never revealing that it exists elsewhere).
func (c *Client) checkRead(ref secrets.Ref) error {
	if err := c.check(ref, false); errors.Is(err, errForeign) {
		return secrets.ErrNotFound
	} else if err != nil {
		return err
	}
	return nil
}

// checkWrite is check for a write, a delete or a listing: another tenant's path is secrets.ErrForbidden.
func (c *Client) checkWrite(ref secrets.Ref, dir bool) error {
	if err := c.check(ref, dir); errors.Is(err, errForeign) {
		return fmt.Errorf("%w: %w", secrets.ErrForbidden, errForeign)
	} else if err != nil {
		return err
	}
	return nil
}

// unavailable wraps a cause as secrets.ErrUnavailable; the cause never holds a value or a token.
func unavailable(op string, cause error) error {
	return fmt.Errorf("%w: openbao %s: %w", secrets.ErrUnavailable, op, cause)
}

// ErrPermissionDenied is the cause (under secrets.ErrUnavailable) of a request OpenBao refused by policy:
// a write-only role reading a value, a tenant-scoped role reaching another tenant's path.
var ErrPermissionDenied = errors.New("permission denied by the OpenBao policy")

// statusError is a non-2xx answer of OpenBao: only its status, never its body.
type statusError struct{ code int }

func (e statusError) Error() string { return fmt.Sprintf("status %d", e.code) }

// response is a decoded OpenBao answer.
type response struct {
	status int
	body   []byte
}

// do sends one request with a token, logging in again once when OpenBao refuses the token (403).
// It returns the response of any status; transport failures are ErrUnavailable.
func (c *Client) do(ctx context.Context, op, method, path string, query url.Values, body any) (response, error) {
	tok, err := c.currentToken(ctx)
	if err != nil {
		return response{}, err
	}
	resp, err := c.send(ctx, op, method, path, query, body, tok)
	if err != nil {
		return response{}, err
	}
	if resp.status == http.StatusForbidden && c.auth.CanRelogin() {
		// A revoked or expired token answers 403 like a policy refusal: log in once more and retry; a
		// second 403 is the policy's.
		c.invalidate(tok)
		if tok, err = c.currentToken(ctx); err != nil {
			return response{}, err
		}
		if resp, err = c.send(ctx, op, method, path, query, body, tok); err != nil {
			return response{}, err
		}
	}
	return resp, nil
}

func (c *Client) send(ctx context.Context, op, method, path string, query url.Values, body any, token string) (response, error) {
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()
	u := *c.base
	u.Path = c.base.Path + "/v1/" + path
	u.RawQuery = query.Encode()
	var rd io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			return response{}, fmt.Errorf("%w: openbao %s: the request does not encode", secrets.ErrInvalid, op)
		}
		rd = bytes.NewReader(raw)
	}
	req, err := http.NewRequestWithContext(ctx, method, u.String(), rd)
	if err != nil {
		return response{}, unavailable(op, errors.New("the request is not valid"))
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		req.Header.Set("X-Vault-Token", token)
	}
	if c.namespace != "" {
		req.Header.Set("X-Vault-Namespace", c.namespace)
	}
	req.Header.Set("X-Vault-Request", "true")
	res, err := c.http.Do(req)
	if err != nil {
		// url.Error names the URL (paths of ids, never values); report only what failed.
		var ue *url.Error
		if errors.As(err, &ue) {
			err = ue.Err
		}
		return response{}, unavailable(op, err)
	}
	defer func() { _ = res.Body.Close() }()
	raw, err := io.ReadAll(io.LimitReader(res.Body, maxResponse))
	if err != nil {
		return response{}, unavailable(op, errors.New("the response could not be read"))
	}
	return response{status: res.StatusCode, body: raw}, nil
}

// classify maps a status other than the call's success to the port's errors.
func classify(op string, status int) error {
	switch status {
	case http.StatusNotFound:
		return secrets.ErrNotFound
	case http.StatusForbidden, http.StatusUnauthorized:
		// The process's own token is refused: its policy (or the deployment) is wrong, which callers see as
		// an outage of the store. secrets.ErrForbidden stays the port's answer to a ref outside its tenant.
		return unavailable(op, ErrPermissionDenied)
	case http.StatusBadRequest:
		return fmt.Errorf("%w: openbao %s: the request was refused", secrets.ErrInvalid, op)
	default:
		// 429, 5xx (503: sealed or in standby), anything unexpected: retry later.
		return unavailable(op, statusError{status})
	}
}

// refusedLogin: the service could not log in (a wrong or expired credential, a role gone, an outage):
// nothing it asked for can be served now, which callers see as an outage, never as a refusal of the ref.
func refusedLogin(op string, status int) error {
	return unavailable(op, statusError{status})
}

// currentToken is a valid token, renewed or logged in for as needed.
func (c *Client) currentToken(ctx context.Context) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	now := c.now()
	if c.token != "" && (c.renewAt.IsZero() && c.expires.IsZero() || now.Before(c.renewAt)) {
		return c.token, nil
	}
	if c.token != "" && c.renewable && now.Before(c.expires) {
		if t, err := c.renewSelf(ctx, c.token); err == nil && t.TTL >= minTTL {
			c.set(t, now)
			return c.token, nil
		}
	}
	t, err := c.auth.Login(ctx, loginer{c})
	if err != nil {
		c.token = ""
		if errors.Is(err, secrets.ErrUnavailable) {
			return "", err
		}
		return "", unavailable("login", err)
	}
	if t.Value == "" {
		c.token = ""
		return "", unavailable("login", errors.New("no token was issued"))
	}
	c.set(t, now)
	return c.token, nil
}

func (c *Client) set(t Token, now time.Time) {
	c.token, c.renewable = t.Value, t.Renewable
	if t.TTL <= 0 {
		c.expires, c.renewAt = time.Time{}, time.Time{}
		return
	}
	c.expires = now.Add(t.TTL)
	c.renewAt = now.Add(t.TTL * 2 / 3)
}

// invalidate drops tok when it is still the held one (another call may have logged in already).
func (c *Client) invalidate(tok string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.token == tok {
		c.token = ""
	}
}

type authResponse struct {
	Auth *struct {
		ClientToken   string `json:"client_token"`
		LeaseDuration int64  `json:"lease_duration"`
		Renewable     bool   `json:"renewable"`
	} `json:"auth"`
	Data *struct {
		TTL       int64 `json:"ttl"`
		Renewable bool  `json:"renewable"`
	} `json:"data"`
}

func (c *Client) renewSelf(ctx context.Context, tok string) (Token, error) {
	resp, err := c.send(ctx, "renew", http.MethodPost, "auth/token/renew-self", nil, map[string]any{}, tok)
	if err != nil {
		return Token{}, err
	}
	if resp.status/100 != 2 {
		return Token{}, refusedLogin("renew", resp.status)
	}
	var ar authResponse
	if json.Unmarshal(resp.body, &ar) != nil || ar.Auth == nil {
		return Token{}, unavailable("renew", errors.New("the response does not decode"))
	}
	return Token{Value: tok, TTL: time.Duration(ar.Auth.LeaseDuration) * time.Second, Renewable: ar.Auth.Renewable}, nil
}

// loginer is the Login an Auth gets: the requests of a login, without a token.
type loginer struct{ c *Client }

func (l loginer) Login(ctx context.Context, authPath string, body map[string]any) (Token, error) {
	if !cleanPath(authPath, false) {
		return Token{}, errors.New("the auth path is not a clean path")
	}
	resp, err := l.c.send(ctx, "login", http.MethodPost, authPath, nil, body, "")
	if err != nil {
		return Token{}, err
	}
	if resp.status/100 != 2 {
		return Token{}, refusedLogin("login", resp.status)
	}
	var ar authResponse
	if json.Unmarshal(resp.body, &ar) != nil || ar.Auth == nil {
		return Token{}, unavailable("login", errors.New("the response does not decode"))
	}
	return Token{Value: ar.Auth.ClientToken, TTL: time.Duration(ar.Auth.LeaseDuration) * time.Second, Renewable: ar.Auth.Renewable}, nil
}

func (l loginer) LookupSelf(ctx context.Context, token string) (Token, error) {
	resp, err := l.c.send(ctx, "lookup", http.MethodGet, "auth/token/lookup-self", nil, nil, token)
	if err != nil {
		return Token{}, err
	}
	if resp.status/100 != 2 {
		return Token{}, refusedLogin("lookup", resp.status)
	}
	var ar authResponse
	if json.Unmarshal(resp.body, &ar) != nil || ar.Data == nil {
		return Token{}, unavailable("lookup", errors.New("the response does not decode"))
	}
	return Token{Value: token, TTL: time.Duration(ar.Data.TTL) * time.Second, Renewable: ar.Data.Renewable}, nil
}
