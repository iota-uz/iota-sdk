package connectors

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"golang.org/x/oauth2"

	"github.com/iota-uz/iota-sdk/pkg/automation/catalog"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
)

// Credential fields an OAuth2 connection keeps.
const (
	OAuth2AccessToken  = "access_token"
	OAuth2RefreshToken = "refresh_token"
	OAuth2TokenType    = "token_type"
	OAuth2ExpiresAt    = "expires_at" // RFC 3339; empty: the token does not expire
)

// Client credential fields of the platform credential named by OAuth2Config.ClientCredential.
const (
	OAuth2ClientID     = "client_id"
	OAuth2ClientSecret = "client_secret"
)

// RefreshBefore is how long before expiry Token refreshes.
const RefreshBefore = 15 * time.Minute

const stateAAD = "automation-oauth2-state"

// OAuth2Option configures NewOAuth2.
type OAuth2Option func(*oauth)

// WithHTTPClient is the client token requests use.
func WithHTTPClient(c *http.Client) OAuth2Option { return func(o *oauth) { o.client = c } }

// WithStateTTL is how long a state from Begin is accepted (default 10 minutes).
func WithStateTTL(d time.Duration) OAuth2Option { return func(o *oauth) { o.stateTTL = d } }

type oauth struct {
	s        *store
	dir      Directory
	platform PlatformCredentials
	client   *http.Client
	stateTTL time.Duration
}

// NewOAuth2 is the OAuth2 helper over a store from NewStore: authorization code with PKCE (S256), the
// state sealed with the keyring, client credentials from platform credentials.
func NewOAuth2(st Store, dir Directory, platform PlatformCredentials, opts ...OAuth2Option) (OAuth2, error) {
	s, ok := st.(*store)
	if !ok {
		return nil, fmt.Errorf("%w: NewOAuth2 needs a store from NewStore", ErrInvalid)
	}
	if dir == nil || platform == nil {
		return nil, fmt.Errorf("%w: NewOAuth2 needs a directory and platform credentials", ErrInvalid)
	}
	// The refresh runs under the row lock: a provider that hangs must not hold it for long.
	o := &oauth{s: s, dir: dir, platform: platform, stateTTL: 10 * time.Minute, client: &http.Client{Timeout: 30 * time.Second}}
	for _, opt := range opts {
		opt(o)
	}
	return o, nil
}

type oauthState struct {
	Tenant   uuid.UUID             `json:"t"`
	Kind     catalog.ConnectorKind `json:"k"`
	Verifier string                `json:"v"`
	Redirect string                `json:"r"`
	Expires  int64                 `json:"e"`
}

func (o *oauth) ctx(ctx context.Context) context.Context {
	if o.client != nil {
		return context.WithValue(ctx, oauth2.HTTPClient, o.client)
	}
	return ctx
}

func (o *oauth) config(kind catalog.ConnectorKind, redirect string) (Connector, *oauth2.Config, error) {
	c, ok := o.dir.Get(kind)
	if !ok {
		return nil, nil, fmt.Errorf("%w: connector %s", ErrNotFound, kind)
	}
	d := c.Descriptor()
	if d.Auth != AuthOAuth2 || d.OAuth2 == nil {
		return nil, nil, fmt.Errorf("%w: connector %s does not authorize with OAuth2", ErrInvalid, kind)
	}
	name := catalog.ConnectorKind(d.OAuth2.ClientCredential)
	if name == "" {
		name = kind
	}
	client, _, ok := o.platform.Platform(name)
	if !ok || client[OAuth2ClientID] == "" {
		return nil, nil, fmt.Errorf("%w: OAuth2 client credential %s is not configured", ErrNotConfigured, name)
	}
	return c, &oauth2.Config{
		ClientID: client[OAuth2ClientID], ClientSecret: client[OAuth2ClientSecret],
		Endpoint:    oauth2.Endpoint{AuthURL: d.OAuth2.AuthURL, TokenURL: d.OAuth2.TokenURL},
		RedirectURL: redirect, Scopes: append([]string(nil), d.OAuth2.Scopes...),
	}, nil
}

func (o *oauth) Begin(ctx context.Context, tenantID uuid.UUID, kind catalog.ConnectorKind, redirectURL string) (string, string, error) {
	const op = serrors.Op("automation.connectors.OAuth2.Begin")
	if err := o.s.configured(); err != nil {
		return "", "", serrors.Wrap(op, err)
	}
	_, cfg, err := o.config(kind, redirectURL)
	if err != nil {
		return "", "", serrors.Wrap(op, err)
	}
	verifier := oauth2.GenerateVerifier()
	state, err := o.sealState(oauthState{Tenant: tenantID, Kind: kind, Verifier: verifier, Redirect: redirectURL, Expires: now().Add(o.stateTTL).Unix()})
	if err != nil {
		return "", "", serrors.Wrap(op, err)
	}
	return cfg.AuthCodeURL(state, oauth2.AccessTypeOffline, oauth2.S256ChallengeOption(verifier)), state, nil
}

func (o *oauth) sealState(st oauthState) (string, error) {
	plain, err := json.Marshal(st)
	if err != nil {
		return "", err
	}
	keyID, key := o.s.keys.Active()
	nonce, sealed, err := seal(key, plain, []byte(stateAAD))
	if err != nil {
		return "", err
	}
	return keyID + "." + base64.RawURLEncoding.EncodeToString(append(nonce, sealed...)), nil
}

var errBadState = fmt.Errorf("%w: OAuth2 state is invalid or expired", ErrInvalid)

func (o *oauth) openState(state string) (oauthState, error) {
	i := strings.LastIndexByte(state, '.')
	if i <= 0 {
		return oauthState{}, errBadState
	}
	keyID, body := state[:i], state[i+1:]
	key, ok := o.s.keys.Key(keyID)
	if !ok {
		return oauthState{}, errBadState
	}
	raw, err := base64.RawURLEncoding.DecodeString(body)
	if err != nil || len(raw) < 12 {
		return oauthState{}, errBadState
	}
	plain, err := open(key, raw[:12], raw[12:], []byte(stateAAD))
	if err != nil {
		return oauthState{}, errBadState
	}
	var st oauthState
	if err := json.Unmarshal(plain, &st); err != nil || now().Unix() > st.Expires {
		return oauthState{}, errBadState
	}
	return st, nil
}

func tokenCredentials(t *oauth2.Token, previous Credentials) Credentials {
	c := Credentials{OAuth2AccessToken: t.AccessToken, OAuth2TokenType: t.TokenType}
	c[OAuth2RefreshToken] = t.RefreshToken
	if t.RefreshToken == "" && previous != nil {
		c[OAuth2RefreshToken] = previous[OAuth2RefreshToken] // a provider may not rotate it
	}
	if !t.Expiry.IsZero() {
		c[OAuth2ExpiresAt] = t.Expiry.UTC().Format(time.RFC3339)
	}
	return c
}

// tokenError hides the provider's response: its error code, description and status text are the
// provider's and may echo what was sent, so only the numeric HTTP status is kept.
func tokenError(err error) error {
	var re *oauth2.RetrieveError
	if errors.As(err, &re) {
		if re.Response != nil {
			return fmt.Errorf("automation connections: the provider refused the token request (HTTP %d)", re.Response.StatusCode)
		}
		return fmt.Errorf("automation connections: the provider refused the token request")
	}
	return fmt.Errorf("automation connections: token request failed")
}

// usable reports whether the access token has not expired yet (a token without expiry is usable).
func usable(c Credentials) bool {
	if c[OAuth2AccessToken] == "" {
		return false
	}
	exp := c[OAuth2ExpiresAt]
	if exp == "" {
		return true
	}
	at, err := time.Parse(time.RFC3339, exp)
	return err == nil && at.After(now())
}

func (o *oauth) Exchange(ctx context.Context, tenantID uuid.UUID, state, code string) (OAuth2Grant, error) {
	const op = serrors.Op("automation.connectors.OAuth2.Exchange")
	if err := o.s.configured(); err != nil {
		return OAuth2Grant{}, serrors.Wrap(op, err)
	}
	st, err := o.openState(state)
	if err != nil {
		return OAuth2Grant{}, serrors.Wrap(op, err)
	}
	if st.Tenant != tenantID {
		return OAuth2Grant{}, serrors.Wrap(op, errBadState)
	}
	conn, cfg, err := o.config(st.Kind, st.Redirect)
	if err != nil {
		return OAuth2Grant{}, serrors.Wrap(op, err)
	}
	tok, err := cfg.Exchange(o.ctx(ctx), code, oauth2.VerifierOption(st.Verifier))
	if err != nil {
		return OAuth2Grant{}, serrors.Wrap(op, tokenError(err))
	}
	creds := tokenCredentials(tok, nil)
	identity, err := conn.Test(ctx, creds)
	if err != nil {
		return OAuth2Grant{}, serrors.Wrap(op, ErrOAuth2Refused)
	}
	return OAuth2Grant{Kind: st.Kind, Identity: SafeIdentity(identity, allSecret, creds), Credentials: creds}, nil
}

func (o *oauth) Complete(ctx context.Context, tenantID uuid.UUID, state, code string) (Connection, error) {
	const op = serrors.Op("automation.connectors.OAuth2.Complete")
	g, err := o.Exchange(ctx, tenantID, state, code)
	if err != nil {
		return Connection{}, serrors.Wrap(op, err)
	}
	st := oauthState{Kind: g.Kind}
	creds, identity := g.Credentials, g.Identity
	// The same identity re-authorized: refresh that connection.
	if identity.External != "" {
		existing, err := o.s.List(ctx, tenantID, st.Kind)
		if err != nil {
			return Connection{}, serrors.Wrap(op, err)
		}
		for _, c := range existing {
			if c.Identity.External == identity.External {
				_, old, err := o.s.load(ctx, o.s.db, tenantID, c.ID, false)
				if err != nil {
					return Connection{}, serrors.Wrap(op, err)
				}
				c = scrubAcrossChange(c, old, creds)
				c.Identity, c.Status, c.StatusReason = SafeIdentity(identity, allSecret, creds, old), StatusConnected, ""
				saved, err := o.s.Save(ctx, c, creds)
				return saved, serrors.Wrap(op, err)
			}
		}
	}
	base := nameFrom(string(st.Kind))
	display := identity.Display
	if display == "" {
		display = base
	}
	for i := 1; i <= 50; i++ {
		name := base
		if i > 1 {
			suffix := fmt.Sprintf("_%d", i)
			name = base[:min(len(base), 40-len(suffix))] + suffix
		}
		saved, err := o.s.Save(ctx, Connection{TenantID: tenantID, Kind: st.Kind, Name: name, DisplayName: display, Identity: identity, Status: StatusConnected}, creds)
		if errors.Is(err, ErrNameTaken) {
			continue
		}
		return saved, serrors.Wrap(op, err)
	}
	return Connection{}, serrors.Wrap(op, ErrNameTaken)
}

// nameFrom is a connection name from a kind.
func nameFrom(s string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(s) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
		default:
			b.WriteRune('_')
		}
	}
	out := b.String()
	if out == "" || out[0] < 'a' || out[0] > 'z' {
		out = "c_" + out
	}
	if len(out) > 40 {
		out = out[:40]
	}
	return out
}

func fresh(c Credentials) bool {
	exp := c[OAuth2ExpiresAt]
	if exp == "" {
		return c[OAuth2AccessToken] != ""
	}
	at, err := time.Parse(time.RFC3339, exp)
	return err == nil && at.Sub(now()) > RefreshBefore
}

func (o *oauth) Token(ctx context.Context, tenantID, connectionID uuid.UUID) (string, error) {
	const op = serrors.Op("automation.connectors.OAuth2.Token")
	c, creds, err := o.s.load(ctx, o.s.db, tenantID, connectionID, false)
	if err != nil {
		return "", serrors.Wrap(op, err)
	}
	if c.Status == StatusNeedsReauth {
		return "", serrors.Wrap(op, ErrNeedsReauth)
	}
	if c.Status != StatusConnected {
		return "", serrors.Wrap(op, ErrNotConnected)
	}
	if fresh(creds) {
		return creds[OAuth2AccessToken], nil
	}
	var (
		token  string
		reauth bool
	)
	seenVersion := c.Version
	err = pgx.BeginFunc(ctx, o.s.db, func(tx pgx.Tx) error {
		c, creds, err := o.s.load(ctx, tx, tenantID, connectionID, true)
		if err != nil {
			return err
		}
		// The row may have changed while this call waited for the lock: check it again.
		switch c.Status {
		case StatusConnected:
		case StatusNeedsReauth:
			reauth = true
			return nil
		case StatusError, StatusDisabled:
			return ErrNotConnected
		default:
			return ErrNotConnected
		}
		if fresh(creds) {
			token = creds[OAuth2AccessToken]
			return nil
		}
		// Another call refreshed while this one waited: reuse its token while it is valid, even when the
		// provider issues tokens shorter-lived than RefreshBefore.
		if c.Version != seenVersion && usable(creds) {
			token = creds[OAuth2AccessToken]
			return nil
		}
		_, cfg, err := o.config(c.Kind, "")
		if err != nil {
			return err
		}
		if creds[OAuth2RefreshToken] == "" {
			return o.markReauth(ctx, tx, c, "no refresh token", &reauth)
		}
		expired := &oauth2.Token{AccessToken: creds[OAuth2AccessToken], RefreshToken: creds[OAuth2RefreshToken], Expiry: now().Add(-time.Minute)}
		tok, err := cfg.TokenSource(o.ctx(ctx), expired).Token()
		if err != nil {
			var re *oauth2.RetrieveError
			// Only a refused grant needs the user; invalid_client, 429 and the like are the platform's or
			// transient and must not send every tenant to re-authorize.
			if errors.As(err, &re) && re.ErrorCode == "invalid_grant" {
				return o.markReauth(ctx, tx, c, "the provider refused the refresh token", &reauth)
			}
			return tokenError(err)
		}
		next := tokenCredentials(tok, creds)
		// The tokens change and the metadata stays: once the old token is gone it could no longer be
		// recognised in the display name or the identity, so they are checked against both now (a row
		// written before this check included).
		c = scrubAcrossChange(c, creds, next)
		identity, _ := json.Marshal(c.Identity)
		if _, err := o.s.update(ctx, tx, c, identity, next); err != nil {
			return err
		}
		token = next[OAuth2AccessToken]
		return nil
	})
	if err != nil {
		return "", serrors.Wrap(op, err)
	}
	if reauth {
		return "", serrors.Wrap(op, ErrNeedsReauth)
	}
	return token, nil
}

func (o *oauth) markReauth(ctx context.Context, tx pgx.Tx, c Connection, reason string, reauth *bool) error {
	_, err := tx.Exec(ctx, `UPDATE automation.connections SET status = $3, status_reason = $4, version = version + 1, updated_at = now()
		WHERE tenant_id = $1 AND id = $2`, c.TenantID, c.ID, string(StatusNeedsReauth), reason)
	if err == nil {
		*reauth = true
	}
	return err
}

// allSecret treats every field as secret: a token set has no public fields.
func allSecret(string) bool { return true }

// scrubAcrossChange is c with the metadata it keeps across a credential change cleared where it carries
// a value of the old or the new credentials: the display name (the name stays: it is chosen by an
// administrator or derived from the kind, never from a provider, and references resolve by it) and the
// identity (SafeIdentity).
func scrubAcrossChange(c Connection, old, next Credentials) Connection {
	if CarriesSecret(c.DisplayName, allSecret, old, next) {
		c.DisplayName = c.Name
	}
	c.Identity = SafeIdentity(c.Identity, allSecret, old, next)
	return c
}
