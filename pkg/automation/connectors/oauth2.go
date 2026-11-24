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
	// OAuth2IDToken is the OpenID Connect identity token of a sign-in with the openid scope. It is in the
	// credentials the connector's Test and OAuth2Finisher see, to read who the token is for (the mailbox
	// address), and is never saved: it carries the account's claims, which are not the connection's secrets.
	OAuth2IDToken = "id_token"
	// OAuth2TokenFrom is the credential of a connection that shares the token set of another (the other
	// connection of one email sign-in): the id of the connection that owns the tokens. Only the owner
	// refreshes; the others copy the refreshed set from it, so a provider that rotates refresh tokens never
	// sees two of them.
	OAuth2TokenFrom = "token_from"
	// OAuth2Preset is the credential that names the provider of a connector with OAuth2Presets.
	OAuth2Preset = "preset"
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
	Preset   string                `json:"p,omitempty"`
}

func (o *oauth) ctx(ctx context.Context) context.Context {
	if o.client != nil {
		return context.WithValue(ctx, oauth2.HTTPClient, o.client)
	}
	return ctx
}

func (o *oauth) config(kind catalog.ConnectorKind, redirect string) (Connector, *oauth2.Config, error) {
	return o.configFor(kind, "", redirect)
}

// configFor is the OAuth2 config of a connector: its own, or the provider of a preset.
func (o *oauth) configFor(kind catalog.ConnectorKind, preset, redirect string) (Connector, *oauth2.Config, error) {
	c, ok := o.dir.Get(kind)
	if !ok {
		return nil, nil, fmt.Errorf("%w: connector %s", ErrNotFound, kind)
	}
	d := c.Descriptor()
	provider := d.OAuth2
	if preset != "" {
		p, ok := d.OAuth2Presets[preset]
		if !ok {
			return nil, nil, fmt.Errorf("%w: connector %s has no OAuth2 preset %q", ErrInvalid, kind, preset)
		}
		provider = &p
	} else if d.Auth != AuthOAuth2 {
		provider = nil
	}
	if provider == nil {
		return nil, nil, fmt.Errorf("%w: connector %s does not authorize with OAuth2", ErrInvalid, kind)
	}
	name := catalog.ConnectorKind(provider.ClientCredential)
	if name == "" {
		name = kind
	}
	client, _, ok := o.platform.Platform(name)
	if !ok || client[OAuth2ClientID] == "" {
		return nil, nil, fmt.Errorf("%w: OAuth2 client credential %s is not configured", ErrNotConfigured, name)
	}
	return c, &oauth2.Config{
		ClientID: client[OAuth2ClientID], ClientSecret: client[OAuth2ClientSecret],
		Endpoint:    oauth2.Endpoint{AuthURL: provider.AuthURL, TokenURL: provider.TokenURL},
		RedirectURL: redirect, Scopes: append([]string(nil), provider.Scopes...),
	}, nil
}

func (o *oauth) Begin(ctx context.Context, tenantID uuid.UUID, kind catalog.ConnectorKind, redirectURL string) (string, string, error) {
	return o.begin(tenantID, kind, "", redirectURL, serrors.Op("automation.connectors.OAuth2.Begin"))
}

var _ PresetOAuth2 = (*oauth)(nil)

// BeginPreset is Begin for a provider of the connector's OAuth2Presets.
func (o *oauth) BeginPreset(_ context.Context, tenantID uuid.UUID, kind catalog.ConnectorKind, preset, redirectURL string) (string, string, error) {
	if preset == "" {
		return "", "", serrors.Wrap("automation.connectors.OAuth2.BeginPreset", fmt.Errorf("%w: a preset is named", ErrInvalid))
	}
	return o.begin(tenantID, kind, preset, redirectURL, serrors.Op("automation.connectors.OAuth2.BeginPreset"))
}

func (o *oauth) begin(tenantID uuid.UUID, kind catalog.ConnectorKind, preset, redirectURL string, op serrors.Op) (string, string, error) {
	if err := o.s.configured(); err != nil {
		return "", "", serrors.Wrap(op, err)
	}
	c, cfg, err := o.configFor(kind, preset, redirectURL)
	if err != nil {
		return "", "", serrors.Wrap(op, err)
	}
	verifier := oauth2.GenerateVerifier()
	state, err := o.sealState(oauthState{Tenant: tenantID, Kind: kind, Verifier: verifier, Redirect: redirectURL, Expires: now().Add(o.stateTTL).Unix(), Preset: preset})
	if err != nil {
		return "", "", serrors.Wrap(op, err)
	}
	opts := []oauth2.AuthCodeOption{oauth2.AccessTypeOffline, oauth2.S256ChallengeOption(verifier)}
	d := c.Descriptor()
	extra := d.OAuth2Presets[preset].AuthParams
	if preset == "" && d.OAuth2 != nil {
		extra = d.OAuth2.AuthParams
	}
	for k, v := range extra {
		opts = append(opts, oauth2.SetAuthURLParam(k, v))
	}
	return cfg.AuthCodeURL(state, opts...), state, nil
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

// carryOver is next with the credentials of previous that are not a token set's own (a preset, the
// mailbox a token is for): a refresh replaces the tokens and keeps the rest of the connection's
// credentials.
func carryOver(next, previous Credentials) Credentials {
	for k, v := range previous {
		if _, own := next[k]; own || isTokenField(k) {
			continue
		}
		next[k] = v
	}
	return next
}

func isTokenField(k string) bool {
	switch k {
	case OAuth2AccessToken, OAuth2RefreshToken, OAuth2TokenType, OAuth2ExpiresAt, OAuth2IDToken:
		return true
	}
	return false
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
	conn, cfg, err := o.configFor(st.Kind, st.Preset, st.Redirect)
	if err != nil {
		return OAuth2Grant{}, serrors.Wrap(op, err)
	}
	tok, err := cfg.Exchange(o.ctx(ctx), code, oauth2.VerifierOption(st.Verifier))
	if err != nil {
		return OAuth2Grant{}, serrors.Wrap(op, tokenError(err))
	}
	creds := tokenCredentials(tok, nil)
	if st.Preset != "" {
		creds[OAuth2Preset] = st.Preset
	}
	finisher, finishes := conn.(OAuth2Finisher)
	if id, _ := tok.Extra(OAuth2IDToken).(string); id != "" && finishes {
		creds[OAuth2IDToken] = id // for Test and the finisher only
	}
	identity, err := conn.Test(ctx, creds)
	if err != nil {
		return OAuth2Grant{}, serrors.Wrap(op, ErrOAuth2Refused)
	}
	secret := o.secretFor(conn)
	g := OAuth2Grant{Kind: st.Kind, Identity: identity, Credentials: creds}
	if finishes {
		if g, err = finisher.FinishGrant(g); err != nil {
			return OAuth2Grant{}, serrors.Wrap(op, ErrOAuth2Refused)
		}
	}
	for _, x := range append([]*OAuth2Grant{&g}, companionPtrs(&g)...) {
		delete(x.Credentials, OAuth2IDToken)
		x.Identity = SafeIdentity(x.Identity, secret, x.Credentials)
	}
	return g, nil
}

func companionPtrs(g *OAuth2Grant) []*OAuth2Grant {
	out := make([]*OAuth2Grant, len(g.Companions))
	for i := range g.Companions {
		out[i] = &g.Companions[i]
	}
	return out
}

// secretFor tells which credential fields of a connector are secret: every field of an OAuth2 connector
// (a token set has no public fields), else the x-secret fields of its schema (a connector with a form
// besides its sign-in keeps its login and its mailbox in the clear); every field when it declares none.
func (o *oauth) secretFor(c Connector) func(string) bool {
	d := c.Descriptor()
	if d.Auth == AuthOAuth2 {
		return allSecret
	}
	fields := SecretFields(d)
	if len(fields) == 0 {
		return allSecret
	}
	return func(f string) bool { return fields[f] }
}

// secretOfKind is secretFor for a kind of the directory; every field when the kind is unknown.
func (o *oauth) secretOfKind(kind catalog.ConnectorKind) func(string) bool {
	if c, ok := o.dir.Get(kind); ok {
		return o.secretFor(c)
	}
	return allSecret
}

func (o *oauth) Complete(ctx context.Context, tenantID uuid.UUID, state, code string) (Connection, error) {
	const op = serrors.Op("automation.connectors.OAuth2.Complete")
	g, err := o.Exchange(ctx, tenantID, state, code)
	if err != nil {
		return Connection{}, serrors.Wrap(op, err)
	}
	items := make([]GroupItem, 0, 1+len(g.Companions))
	for _, x := range append([]OAuth2Grant{g}, g.Companions...) {
		it, err := PlanGrant(ctx, o.s, o.secretOfKind(x.Kind), tenantID, x, 0)
		if err != nil {
			return Connection{}, serrors.Wrap(op, err)
		}
		items = append(items, it)
	}
	// One transaction: the pair of connections of a sign-in is saved whole or not at all.
	saved, err := o.s.SaveGroup(ctx, items)
	if err != nil {
		return Connection{}, serrors.Wrap(op, err)
	}
	return saved[0], nil
}

// PlanGrant is what saves a grant: an update of the connection of the same identity (re-authorizing it,
// connected again), else a new connection named after the kind. createdBy is the user of a new one.
func PlanGrant(ctx context.Context, s ConnectionStore, secret func(string) bool, tenantID uuid.UUID, g OAuth2Grant, createdBy int64) (GroupItem, error) {
	creds, identity := g.Credentials, g.Identity
	existing, err := s.List(ctx, tenantID, g.Kind)
	if err != nil {
		return GroupItem{}, err
	}
	if identity.External != "" {
		for _, c := range existing {
			if c.Identity.External != identity.External {
				continue
			}
			cur, old, err := s.Get(ctx, tenantID, c.ID)
			if err != nil {
				return GroupItem{}, err
			}
			cur = scrubAcrossChange(cur, secret, old, creds)
			cur.Identity, cur.Status, cur.StatusReason = SafeIdentity(identity, secret, creds, old), StatusConnected, ""
			return GroupItem{Connection: cur, Credentials: creds}, nil
		}
	}
	base := nameFrom(string(g.Kind))
	display := identity.Display
	if display == "" {
		display = base
	}
	taken := map[string]bool{}
	for _, c := range existing {
		taken[c.Name] = true
	}
	for i := 1; i <= 50; i++ {
		name := base
		if i > 1 {
			suffix := fmt.Sprintf("_%d", i)
			name = base[:min(len(base), 40-len(suffix))] + suffix
		}
		if !taken[name] {
			return GroupItem{Connection: Connection{TenantID: tenantID, Kind: g.Kind, Name: name, DisplayName: display, Identity: identity, Status: StatusConnected, CreatedBy: createdBy}, Credentials: creds}, nil
		}
	}
	return GroupItem{}, ErrNameTaken
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
	return o.token(ctx, tenantID, connectionID, false)
}

var _ OAuth2Checker = (*oauth)(nil)

func (o *oauth) CheckToken(ctx context.Context, tenantID, connectionID uuid.UUID) (string, error) {
	return o.token(ctx, tenantID, connectionID, true)
}

// serves is the status token works on: connected, or disabled for a check (CheckToken).
func serves(status ConnectionStatus, check bool) bool {
	if check {
		return status == StatusDisabled
	}
	return status == StatusConnected
}

// token is Token, or CheckToken with check: then the connection must be disabled, and a refused refresh
// is reported without recording needs_reauth.
func (o *oauth) token(ctx context.Context, tenantID, connectionID uuid.UUID, check bool) (string, error) {
	const op = serrors.Op("automation.connectors.OAuth2.Token")
	c, creds, err := o.s.load(ctx, o.s.db, tenantID, connectionID, false)
	if err != nil {
		return "", serrors.Wrap(op, err)
	}
	if c.Status == StatusNeedsReauth {
		return "", serrors.Wrap(op, ErrNeedsReauth)
	}
	if !serves(c.Status, check) {
		return "", serrors.Wrap(op, ErrNotConnected)
	}
	if fresh(creds) {
		return creds[OAuth2AccessToken], nil
	}
	if owner, err := uuid.Parse(creds[OAuth2TokenFrom]); err == nil {
		return o.tokenOfOwner(ctx, tenantID, connectionID, owner, check)
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
		switch {
		case serves(c.Status, check):
		case c.Status == StatusNeedsReauth:
			reauth = true
			return nil
		default:
			return ErrNotConnected
		}
		refused := func(reason string) error {
			if check {
				// A check records nothing: its caller saves the outcome against the version it read.
				reauth = true
				return nil
			}
			return o.markReauth(ctx, tx, c, reason, &reauth)
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
		_, cfg, err := o.configFor(c.Kind, creds[OAuth2Preset], "")
		if err != nil {
			return err
		}
		if creds[OAuth2RefreshToken] == "" {
			return refused("no refresh token")
		}
		expired := &oauth2.Token{AccessToken: creds[OAuth2AccessToken], RefreshToken: creds[OAuth2RefreshToken], Expiry: now().Add(-time.Minute)}
		tok, err := cfg.TokenSource(o.ctx(ctx), expired).Token()
		if err != nil {
			var re *oauth2.RetrieveError
			// Only a refused grant needs the user; invalid_client, 429 and the like are the platform's or
			// transient and must not send every tenant to re-authorize.
			if errors.As(err, &re) && re.ErrorCode == "invalid_grant" {
				return refused("the provider refused the refresh token")
			}
			return tokenError(err)
		}
		next := carryOver(tokenCredentials(tok, creds), creds)
		// The tokens change and the metadata stays: once the old token is gone it could no longer be
		// recognised in the display name or the identity, so they are checked against both now (a row
		// written before this check included).
		c = scrubAcrossChange(c, o.secretOfKind(c.Kind), creds, next)
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

// tokenOfOwner is the token of a connection that shares the token set of another: the owner refreshes it
// (once, under its own lock), and this connection takes the refreshed set as its own. The owner's
// re-authorization is this one's too.
func (o *oauth) tokenOfOwner(ctx context.Context, tenantID, id, owner uuid.UUID, check bool) (string, error) {
	const op = serrors.Op("automation.connectors.OAuth2.Token")
	tok, err := o.token(ctx, tenantID, owner, false)
	if errors.Is(err, ErrNotConnected) && check {
		tok, err = o.token(ctx, tenantID, owner, true)
	}
	if errors.Is(err, ErrNeedsReauth) || errors.Is(err, ErrNotFound) {
		if !check {
			_ = pgx.BeginFunc(ctx, o.s.db, func(tx pgx.Tx) error {
				var reauth bool
				return o.markReauth(ctx, tx, Connection{TenantID: tenantID, ID: id}, "the provider refused the refresh token", &reauth)
			})
		}
		return "", serrors.Wrap(op, ErrNeedsReauth)
	}
	if err != nil {
		return "", err
	}
	err = pgx.BeginFunc(ctx, o.s.db, func(tx pgx.Tx) error {
		c, creds, err := o.s.load(ctx, tx, tenantID, id, true)
		if err != nil {
			return err
		}
		_, theirs, err := o.s.load(ctx, tx, tenantID, owner, false)
		if err != nil {
			return err
		}
		if creds[OAuth2AccessToken] == theirs[OAuth2AccessToken] && creds[OAuth2RefreshToken] == theirs[OAuth2RefreshToken] {
			return nil
		}
		next := Credentials{}
		for _, k := range []string{OAuth2AccessToken, OAuth2RefreshToken, OAuth2TokenType, OAuth2ExpiresAt} {
			if v := theirs[k]; v != "" {
				next[k] = v
			}
		}
		next = carryOver(next, creds)
		c = scrubAcrossChange(c, o.secretOfKind(c.Kind), creds, next)
		identity, _ := json.Marshal(c.Identity)
		_, err = o.s.update(ctx, tx, c, identity, next)
		return err
	})
	if err != nil {
		return "", serrors.Wrap(op, err)
	}
	return tok, nil
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
func scrubAcrossChange(c Connection, secret func(string) bool, old, next Credentials) Connection {
	if CarriesSecret(c.DisplayName, secret, old, next) {
		c.DisplayName = c.Name
	}
	c.Identity = SafeIdentity(c.Identity, secret, old, next)
	return c
}
