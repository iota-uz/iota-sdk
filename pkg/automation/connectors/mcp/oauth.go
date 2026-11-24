package mcp

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"golang.org/x/oauth2"

	"github.com/iota-uz/iota-sdk/pkg/automation/catalog"
	"github.com/iota-uz/iota-sdk/pkg/automation/connectors"
)

// Errors of the OAuth sign-in. Their text names a step, never a server's answer.
var (
	ErrDiscovery    = errors.New("mcp: the authorization server of the MCP server was not found")
	ErrRegistration = errors.New("mcp: the authorization server refused to register the client")
	ErrNoClient     = fmt.Errorf("%w: the authorization server does not register clients: set oauth_client_id", connectors.ErrClientRequired)
	ErrExchange     = errors.New("mcp: the authorization server did not issue a token")
	ErrState        = fmt.Errorf("%w: the sign-in state is invalid or expired", connectors.ErrInvalid)
)

const statePurpose = "mcp-oauth-state"

// authServer is what the sign-in needs of the authorization server's metadata.
type authServer struct {
	Issuer        string
	Authorization string
	Token         string
	Registration  string
	Scopes        []string
}

// discover finds the authorization server of an MCP server: its protected resource metadata (from the
// WWW-Authenticate of an unauthenticated request, else the well-known URIs), then the authorization
// server's metadata (RFC 8414, then OpenID Connect discovery). A server without resource metadata is
// its own authorization server (the 2025-03-26 revision).
func (c *Connector) discover(ctx context.Context, cf config) (authServer, error) {
	httpc := c.net.client(nil, nil)
	server := cf.server.String()
	var prm *resourceMetadata
	if h := c.probe(ctx, httpc, server); h != nil {
		if u := resourceMetadataURL(h); u != "" {
			prm, _ = protectedResource(ctx, httpc, u, server)
		}
	}
	for _, u := range resourceCandidates(cf.server) {
		if prm != nil {
			break
		}
		prm, _ = protectedResource(ctx, httpc, u, server)
	}
	issuer := cf.server.Scheme + "://" + cf.server.Host
	var scopes []string
	if prm != nil {
		if len(prm.AuthorizationServers) == 0 {
			return authServer{}, ErrDiscovery
		}
		issuer, scopes = prm.AuthorizationServers[0], prm.ScopesSupported
	}
	iu, err := url.Parse(issuer)
	if err != nil || c.net.checkURL(iu) != nil {
		return authServer{}, ErrDiscovery
	}
	for _, metaURL := range metadataURLs(iu) {
		meta, err := authorizationServer(ctx, httpc, metaURL, issuer)
		if err != nil || meta == nil {
			continue
		}
		as := authServer{Issuer: issuer, Authorization: meta.AuthorizationEndpoint, Token: meta.TokenEndpoint, Registration: meta.RegistrationEndpoint, Scopes: scopes}
		if len(as.Scopes) == 0 {
			as.Scopes = meta.ScopesSupported
		}
		for _, e := range []string{as.Authorization, as.Token} {
			u, err := url.Parse(e)
			if e == "" || err != nil || c.net.checkURL(u) != nil {
				return authServer{}, ErrDiscovery
			}
		}
		if as.Registration != "" {
			if u, err := url.Parse(as.Registration); err != nil || c.net.checkURL(u) != nil {
				as.Registration = ""
			}
		}
		return as, nil
	}
	return authServer{}, ErrDiscovery
}

// probe sends an unauthenticated initialize; a 401 answer's headers point at the resource metadata.
func (c *Connector) probe(ctx context.Context, httpc *http.Client, server string) http.Header {
	body := `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18","capabilities":{},"clientInfo":{"name":"iota-sdk","version":"1"}}}`
	pctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(pctx, http.MethodPost, server, strings.NewReader(body))
	if err != nil {
		return nil
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	res, err := httpc.Do(req)
	if err != nil {
		return nil
	}
	defer func() { _ = res.Body.Close() }()
	_, _ = io.Copy(io.Discard, io.LimitReader(res.Body, 64<<10))
	if res.StatusCode != http.StatusUnauthorized {
		return nil
	}
	return res.Header
}

// metadataURLs are the well-known URIs of an issuer's metadata, in the order the MCP specification
// tries them.
func metadataURLs(issuer *url.URL) []string {
	origin := issuer.Scheme + "://" + issuer.Host
	path := strings.TrimSuffix(issuer.Path, "/")
	if path == "" {
		return []string{origin + "/.well-known/oauth-authorization-server", origin + "/.well-known/openid-configuration"}
	}
	return []string{
		origin + "/.well-known/oauth-authorization-server" + path,
		origin + "/.well-known/openid-configuration" + path,
		origin + path + "/.well-known/openid-configuration",
	}
}

// signInState is what a sign-in in progress keeps in its sealed state.
type signInState struct {
	Tenant     uuid.UUID `json:"t"`
	Connection uuid.UUID `json:"c"`
	Verifier   string    `json:"v"`
	Redirect   string    `json:"r"`
	Issuer     string    `json:"i"`
	Expires    int64     `json:"e"`
	Nonce      string    `json:"n"`
}

// SignsIn reports whether the credentials are of auth oauth2.
func (c *Connector) SignsIn(creds connectors.Credentials) bool {
	cf, err := readConfig(creds)
	return err == nil && cf.auth == AuthOAuth2
}

// NeedsAuthorization reports whether credentials of auth oauth2 have no token for their server yet.
func (c *Connector) NeedsAuthorization(creds connectors.Credentials) bool {
	cf, err := readConfig(creds)
	return err == nil && cf.auth == AuthOAuth2 && !cf.authorized()
}

// BeginAuthorization discovers the authorization server, registers a client with DCR when the connection
// has none (under the row lock: a second sign-in reuses it) and returns the authorization URL and state.
func (c *Connector) BeginAuthorization(ctx context.Context, in connectors.AuthorizationStart) (string, string, error) {
	st, err := c.boundStore()
	if err != nil {
		return "", "", err
	}
	_, creds, err := st.Get(ctx, in.TenantID, in.ConnectionID)
	if err != nil {
		return "", "", err
	}
	cf, err := readConfig(creds)
	if err != nil {
		return "", "", fmt.Errorf("%w: %v", connectors.ErrInvalid, err)
	}
	if cf.auth != AuthOAuth2 {
		return "", "", fmt.Errorf("%w: the connection does not sign in with OAuth", connectors.ErrInvalid)
	}
	as, err := c.discover(ctx, cf)
	if err != nil {
		return "", "", err
	}
	_, creds, err = st.UpdateCredentials(ctx, in.TenantID, in.ConnectionID, func(_ connectors.Connection, cur connectors.Credentials) (*connectors.CredentialChange, error) {
		next := cur
		changed := false
		set := func(k, v string) {
			if next[k] != v {
				next[k], changed = v, true
			}
		}
		// A client this connector registered is the issuer's: another issuer needs its own.
		stale := next[KeyRegistered] == "true" && next[KeyIssuer] != as.Issuer
		if next[KeyClientID] == "" || stale {
			if as.Registration == "" {
				return nil, ErrNoClient
			}
			reg, err := c.register(ctx, as, in.RedirectURL, next[KeyScopes])
			if err != nil {
				return nil, err
			}
			set(KeyClientID, reg.ClientID)
			set(KeyClientSecret, reg.ClientSecret)
			set(KeyRegistered, "true")
		}
		if next[KeyIssuer] != as.Issuer {
			// Tokens of another issuer are no longer this connection's.
			for _, k := range []string{KeyAccessToken, KeyRefreshToken, KeyExpiresAt, KeyResource} {
				if next[k] != "" {
					delete(next, k)
					changed = true
				}
			}
		}
		set(KeyIssuer, as.Issuer)
		set(KeyTokenEndpoint, as.Token)
		if !changed {
			return nil, nil //nolint:nilnil // no change: the connection keeps its credentials
		}
		return &connectors.CredentialChange{Credentials: next}, nil
	})
	if err != nil {
		return "", "", err
	}
	verifier := oauth2.GenerateVerifier()
	nonce := make([]byte, 16)
	_, _ = rand.Read(nonce)
	plain, err := json.Marshal(signInState{Tenant: in.TenantID, Connection: in.ConnectionID, Verifier: verifier, Redirect: in.RedirectURL,
		Issuer: as.Issuer, Expires: c.now().Add(StateTTL).Unix(), Nonce: base64.RawURLEncoding.EncodeToString(nonce)})
	if err != nil {
		return "", "", err
	}
	state, err := st.Seal(statePurpose, plain)
	if err != nil {
		return "", "", err
	}
	q := url.Values{
		"response_type":         {"code"},
		"client_id":             {creds[KeyClientID]},
		"redirect_uri":          {in.RedirectURL},
		"state":                 {state},
		"code_challenge":        {oauth2.S256ChallengeFromVerifier(verifier)},
		"code_challenge_method": {"S256"},
		"resource":              {cf.server.String()},
	}
	scopes := strings.TrimSpace(creds[KeyScopes])
	if scopes == "" {
		scopes = strings.Join(as.Scopes, " ")
	}
	if scopes != "" {
		q.Set("scope", scopes)
	}
	auth, err := url.Parse(as.Authorization)
	if err != nil {
		return "", "", ErrDiscovery
	}
	for k, v := range auth.Query() {
		if _, ours := q[k]; !ours {
			q[k] = v
		}
	}
	auth.RawQuery = q.Encode()
	return auth.String(), state, nil
}

// register is Dynamic Client Registration (RFC 7591) of a public client with the callback.
func (c *Connector) register(ctx context.Context, as authServer, redirect, scopes string) (*clientRegistration, error) {
	meta := map[string]any{
		"redirect_uris": []string{redirect}, "token_endpoint_auth_method": "none",
		"grant_types": []string{"authorization_code", "refresh_token"}, "response_types": []string{"code"},
		"client_name": c.clientID,
	}
	if scopes != "" {
		meta["scope"] = scopes
	}
	res, err := registerClient(ctx, c.net.secretClient(), as.Registration, meta)
	if err != nil {
		return nil, ErrRegistration // the server's text stays out
	}
	return res, nil
}

func (c *Connector) openState(st Store, state string) (signInState, error) {
	plain, err := st.Unseal(statePurpose, state)
	if err != nil {
		return signInState{}, ErrState
	}
	var s signInState
	if json.Unmarshal(plain, &s) != nil || c.now().Unix() > s.Expires {
		return signInState{}, ErrState
	}
	return s, nil
}

// CompleteAuthorization exchanges the code with the PKCE verifier and stores the tokens.
func (c *Connector) CompleteAuthorization(ctx context.Context, in connectors.AuthorizationReturn) (connectors.Credentials, error) {
	st, err := c.boundStore()
	if err != nil {
		return nil, err
	}
	s, err := c.openState(st, in.State)
	if err != nil {
		return nil, err
	}
	if s.Tenant != in.TenantID || s.Connection != in.ConnectionID || in.Code == "" {
		return nil, ErrState
	}
	_, creds, err := st.Get(ctx, in.TenantID, in.ConnectionID)
	if err != nil {
		return nil, err
	}
	cf, err := readConfig(creds)
	if err != nil || cf.auth != AuthOAuth2 || creds[KeyIssuer] != s.Issuer || creds[KeyTokenEndpoint] == "" {
		return nil, ErrState // the settings changed since the sign-in began
	}
	tok, err := c.tokenRequest(ctx, creds, url.Values{
		"grant_type": {"authorization_code"}, "code": {in.Code}, "redirect_uri": {s.Redirect},
		"code_verifier": {s.Verifier}, "resource": {cf.server.String()},
	})
	if err != nil {
		return nil, ErrExchange
	}
	server := cf.server.String()
	_, out, err := st.UpdateCredentials(ctx, in.TenantID, in.ConnectionID, func(_ connectors.Connection, cur connectors.Credentials) (*connectors.CredentialChange, error) {
		if cur[KeyServerURL] != creds[KeyServerURL] || cur[KeyIssuer] != s.Issuer {
			return nil, ErrState
		}
		return &connectors.CredentialChange{Credentials: withTokens(cur, tok, server)}, nil
	})
	return out, err
}

// tokenResponse is a token endpoint's answer (RFC 6749 §5).
type tokenResponse struct {
	AccessToken  string      `json:"access_token"`
	TokenType    string      `json:"token_type"`
	RefreshToken string      `json:"refresh_token"`
	ExpiresIn    json.Number `json:"expires_in"`
	Error        string      `json:"error"`
}

// errInvalidGrant is a refresh token the authorization server no longer accepts.
var errInvalidGrant = errors.New("mcp: the authorization server refused the grant")

// tokenRequest posts a grant to the connection's token endpoint, authenticating as its client.
func (c *Connector) tokenRequest(ctx context.Context, creds connectors.Credentials, form url.Values) (tokenResponse, error) {
	endpoint := creds[KeyTokenEndpoint]
	if endpoint == "" {
		return tokenResponse{}, ErrDiscovery
	}
	rctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	secret := creds[KeyClientSecret]
	if secret == "" {
		form.Set("client_id", creds[KeyClientID])
	}
	req, err := http.NewRequestWithContext(rctx, http.MethodPost, endpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return tokenResponse{}, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	if secret != "" {
		req.SetBasicAuth(url.QueryEscape(creds[KeyClientID]), url.QueryEscape(secret))
	}
	res, err := c.net.secretClient().Do(req)
	if err != nil {
		return tokenResponse{}, fmt.Errorf("mcp: the token request failed")
	}
	defer func() { _ = res.Body.Close() }()
	body, err := io.ReadAll(io.LimitReader(res.Body, 1<<20))
	if err != nil {
		return tokenResponse{}, fmt.Errorf("mcp: the token response was not read")
	}
	var tr tokenResponse
	dec := json.NewDecoder(bytes.NewReader(body))
	dec.UseNumber()
	_ = dec.Decode(&tr)
	if res.StatusCode != http.StatusOK || tr.AccessToken == "" {
		if tr.Error == "invalid_grant" {
			return tokenResponse{}, errInvalidGrant
		}
		return tokenResponse{}, fmt.Errorf("mcp: the token endpoint answered HTTP %d", res.StatusCode)
	}
	return tr, nil
}

// withTokens is credentials with the tokens of a token response for a server.
func withTokens(cur connectors.Credentials, tok tokenResponse, server string) connectors.Credentials {
	next := make(connectors.Credentials, len(cur)+4)
	for k, v := range cur {
		next[k] = v
	}
	next[KeyAccessToken] = tok.AccessToken
	if tok.RefreshToken != "" {
		next[KeyRefreshToken] = tok.RefreshToken // a server may not rotate it
	}
	delete(next, KeyExpiresAt)
	if n, err := strconv.ParseInt(tok.ExpiresIn.String(), 10, 64); err == nil && n > 0 {
		next[KeyExpiresAt] = time.Now().Add(time.Duration(n) * time.Second).UTC().Format(time.RFC3339)
	}
	next[KeyResource] = server
	return next
}

// expiresWithin reports whether the access token expires within d (a token without expiry never does).
func expiresWithin(creds connectors.Credentials, d time.Duration, now time.Time) bool {
	exp := creds[KeyExpiresAt]
	if exp == "" {
		return false
	}
	at, err := time.Parse(time.RFC3339, exp)
	return err != nil || at.Sub(now) <= d
}

// FreshCredentials is the connection's credentials with an access token valid for RefreshBefore,
// refreshed under the row lock when it is not.
func (c *Connector) FreshCredentials(ctx context.Context, tenantID, connectionID uuid.UUID, check bool) (connectors.Credentials, error) {
	st, err := c.boundStore()
	if err != nil {
		return nil, err
	}
	_, creds, err := st.Get(ctx, tenantID, connectionID)
	if err != nil {
		return nil, err
	}
	cf, err := readConfig(creds)
	if err != nil || cf.auth != AuthOAuth2 {
		return creds, nil //nolint:nilerr // credentials without OAuth have nothing to refresh
	}
	if !cf.authorized() {
		return nil, connectors.ErrNeedsReauth
	}
	if !expiresWithin(creds, connectors.RefreshBefore, c.now()) {
		return creds, nil
	}
	return c.refresh(ctx, tenantID, connectionID, creds, false, check)
}

// refresh refreshes the access token under the row lock. A token another call refreshed since seen was
// read is taken as it is; force refreshes a token the server refused (401) even when it is not expiring.
// A refused refresh sets needs_reauth (unless check) and is ErrNeedsReauth.
// errSettingsChanged: the connection's server or OAuth client changed between the read of the call's
// credentials and the row lock. A token of the row then belongs to other settings and is not used with
// the call's server.
var errSettingsChanged = errors.New("mcp: the connection's settings changed")

// sameSettings reports whether two credential sets name the same server and OAuth client.
func sameSettings(a, b connectors.Credentials) bool {
	for _, k := range []string{KeyServerURL, KeyAuth, KeyClientID, KeyIssuer, KeyTokenEndpoint} {
		if a[k] != b[k] {
			return false
		}
	}
	return true
}

func (c *Connector) refresh(ctx context.Context, tenantID, connectionID uuid.UUID, read connectors.Credentials, force, check bool) (connectors.Credentials, error) {
	seen := read[KeyAccessToken]
	st, err := c.boundStore()
	if err != nil {
		return nil, err
	}
	reauth := false
	_, out, err := st.UpdateCredentials(ctx, tenantID, connectionID, func(conn connectors.Connection, cur connectors.Credentials) (*connectors.CredentialChange, error) {
		switch {
		case conn.Status == connectors.StatusConnected:
		case check && conn.Status == connectors.StatusDisabled:
		case conn.Status == connectors.StatusNeedsReauth:
			reauth = true
			return nil, nil //nolint:nilnil // no change: the caller reports ErrNeedsReauth
		default:
			return nil, connectors.ErrNotConnected
		}
		// The token is used with the server the call read: settings changed meanwhile give no token.
		if !sameSettings(cur, read) || cur[KeyResource] != read[KeyResource] {
			return nil, errSettingsChanged
		}
		if cur[KeyAccessToken] != seen && cur[KeyAccessToken] != "" && !expiresWithin(cur, 0, c.now()) {
			return nil, nil //nolint:nilnil // another call refreshed while this one waited for the lock
		}
		if !force && !expiresWithin(cur, connectors.RefreshBefore, c.now()) {
			return nil, nil //nolint:nilnil // still fresh: no change
		}
		refused := func(reason string) (*connectors.CredentialChange, error) {
			reauth = true
			if check {
				return nil, nil //nolint:nilnil // a check records nothing
			}
			return &connectors.CredentialChange{Status: connectors.StatusNeedsReauth, StatusReason: reason}, nil
		}
		if cur[KeyRefreshToken] == "" {
			return refused("no refresh token")
		}
		cf, err := readConfig(cur)
		if err != nil {
			return nil, err
		}
		tok, err := c.tokenRequest(ctx, cur, url.Values{"grant_type": {"refresh_token"}, "refresh_token": {cur[KeyRefreshToken]}, "resource": {cf.server.String()}})
		if errors.Is(err, errInvalidGrant) {
			return refused("the authorization server refused the refresh token")
		}
		if err != nil {
			return nil, err
		}
		return &connectors.CredentialChange{Credentials: withTokens(cur, tok, cf.server.String())}, nil
	})
	if err != nil {
		return nil, err
	}
	if reauth {
		c.cache.drop(tenantID, connectionID)
		return nil, connectors.ErrNeedsReauth
	}
	return out, nil
}

// tokenFor is the bearer a request of the connection carries: none, the static token, or a fresh OAuth
// access token (force: refreshed because the server refused the current one). refreshed reports a
// refresh made now.
func (c *Connector) tokenFor(ctx context.Context, tenantID, connectionID uuid.UUID, cf config, force bool) (string, bool, error) {
	switch cf.auth {
	case AuthNone:
		return "", false, nil
	case AuthBearer:
		return cf.creds[KeyToken], false, nil
	}
	if !cf.authorized() {
		return "", false, &catalog.CallError{Code: catalog.CallUnauthorized, Message: "the connection needs an OAuth sign-in"}
	}
	if !force && !expiresWithin(cf.creds, connectors.RefreshBefore, c.now()) {
		return cf.creds[KeyAccessToken], false, nil
	}
	if connectionID == uuid.Nil {
		return cf.creds[KeyAccessToken], false, nil
	}
	creds, err := c.refresh(ctx, tenantID, connectionID, cf.creds, force, false)
	switch {
	case errors.Is(err, errSettingsChanged):
		// Nothing was sent: the call may be made again with the connection as it is now.
		return "", true, &catalog.CallError{Code: catalog.CallUnavailable, Message: "the connection's settings changed during the call; it is made again with the new ones", Retryable: true}
	case errors.Is(err, connectors.ErrNeedsReauth):
		return "", true, &catalog.CallError{Code: catalog.CallUnauthorized, Message: "the authorization server refused the refresh: the connection needs a new sign-in"}
	case err != nil:
		return "", true, &catalog.CallError{Code: catalog.CallUnavailable, Message: "the access token could not be refreshed", Retryable: true}
	}
	return creds[KeyAccessToken], true, nil
}

// markReauth sets needs_reauth on a connected connection the server refused after a refresh: only while
// the row still holds the settings and the access token that were refused, so a late refusal never
// undoes a sign-in or a refresh committed meanwhile.
func (c *Connector) markReauth(ctx context.Context, tenantID, connectionID uuid.UUID, used connectors.Credentials, token, reason string) {
	c.cache.drop(tenantID, connectionID)
	st, err := c.boundStore()
	if err != nil {
		return
	}
	mctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()
	_, _, err = st.UpdateCredentials(mctx, tenantID, connectionID, func(conn connectors.Connection, cur connectors.Credentials) (*connectors.CredentialChange, error) {
		if conn.Status != connectors.StatusConnected || !sameSettings(cur, used) || cur[KeyAccessToken] != token {
			return nil, nil //nolint:nilnil // already not connected: nothing to mark
		}
		return &connectors.CredentialChange{Status: connectors.StatusNeedsReauth, StatusReason: reason}, nil
	})
	_ = err // best effort: the call already reports unauthorized
}
