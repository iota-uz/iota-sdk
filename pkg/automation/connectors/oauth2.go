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

// KeepSettings is a grant's credentials with the settings of the connection it replaces that a grant does not
// own (the folder a mailbox is read from, say): a re-authorization replaces the token set and what the grant
// brings, and leaves the rest as the person set it. next is not changed.
func KeepSettings(next, previous Credentials) Credentials {
	return carryOver(cloneCredentials(next), previous)
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
	if id, _ := tok.Extra(OAuth2IDToken).(string); id != "" && finishes && validIDToken(id, cfg.ClientID, o.issuersFor(conn, st.Preset), now()) {
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
	// The pair is saved whole or not at all, and only when both work: an account that signs in over one
	// protocol and has the other closed (a mailbox without IMAP) is refused here, where the person sees it,
	// not at the first run of a graph bound to the connection that does not work.
	for _, x := range g.Companions {
		cc, ok := o.dir.Get(x.Kind)
		if !ok {
			return OAuth2Grant{}, serrors.Wrap(op, ErrOAuth2Refused)
		}
		if _, err := cc.Test(ctx, x.Credentials); err != nil {
			return OAuth2Grant{}, serrors.Wrap(op, ErrOAuth2Refused)
		}
	}
	for _, x := range append([]*OAuth2Grant{&g}, companionPtrs(&g)...) {
		delete(x.Credentials, OAuth2IDToken)
		x.Identity = SafeIdentity(x.Identity, secret, x.Credentials)
	}
	return g, nil
}

// issuersFor are the issuers of the identity tokens of the provider of a sign-in.
func (o *oauth) issuersFor(c Connector, preset string) []string {
	d := c.Descriptor()
	if preset != "" {
		return d.OAuth2Presets[preset].IDTokenIssuers
	}
	if d.OAuth2 != nil {
		return d.OAuth2.IDTokenIssuers
	}
	return nil
}

// issuerMatches reports whether iss is one of the accepted issuers: equal, or equal with a "*" of the accepted
// one standing for one path segment.
func issuerMatches(iss string, accepted []string) bool {
	for _, a := range accepted {
		if a == iss {
			return true
		}
		before, after, star := strings.Cut(a, "*")
		if !star || !strings.HasPrefix(iss, before) || !strings.HasSuffix(iss, after) || len(iss) < len(before)+len(after)+1 {
			continue
		}
		if seg := iss[len(before) : len(iss)-len(after)]; seg != "" && !strings.Contains(seg, "/") {
			return true
		}
	}
	return false
}

// validIDToken reports whether an OpenID Connect identity token, received from the token endpoint over TLS
// (its signature is not checked: OpenID Connect Core 3.1.3.7), may be read for who signed in: its issuer is one
// of the provider's (a token without an issuer is not), its audience is this client's alone (no other
// audience, and an authorized party, when there is one, is this client), and its exp is in the future. A
// token that is not is not read.
func validIDToken(raw, clientID string, issuers []string, at time.Time) bool {
	parts := strings.Split(raw, ".")
	if len(parts) != 3 {
		return false
	}
	payload, err := base64.RawURLEncoding.DecodeString(strings.TrimRight(parts[1], "="))
	if err != nil {
		return false
	}
	var claims struct {
		Iss string          `json:"iss"`
		Azp string          `json:"azp"`
		Aud json.RawMessage `json:"aud"`
		Exp float64         `json:"exp"`
	}
	if json.Unmarshal(payload, &claims) != nil || claims.Exp <= float64(at.Unix()) {
		return false
	}
	if claims.Iss == "" || !issuerMatches(claims.Iss, issuers) {
		return false
	}
	if claims.Azp != "" && claims.Azp != clientID {
		return false
	}
	var one string
	if json.Unmarshal(claims.Aud, &one) == nil {
		return one == clientID
	}
	var many []string
	if json.Unmarshal(claims.Aud, &many) != nil || len(many) == 0 {
		return false
	}
	for _, a := range many {
		if a != clientID {
			return false // an audience that is not this client's: the token was not issued for it alone
		}
	}
	return true
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
	// The names are made here (after the kind, the first free): another sign-in that takes the same name
	// between the plan and the write is met by planning again with the grant already exchanged, a few times.
	// One transaction: the pair of connections of a sign-in is saved whole or not at all.
	var lastErr error
	for attempt := 0; attempt < 5; attempt++ {
		items := make([]GroupItem, 0, 1+len(g.Companions))
		primary, err := PlanGrant(ctx, o.s, o.secretOfKind(g.Kind), tenantID, g, 0)
		if err != nil {
			return Connection{}, serrors.Wrap(op, err)
		}
		items = append(items, primary)
		for _, x := range g.Companions {
			it, err := PlanCompanion(ctx, o.s, o.secretOfKind(x.Kind), tenantID, x, 0, primary)
			if err != nil {
				return Connection{}, serrors.Wrap(op, err)
			}
			items = append(items, it)
		}
		saved, err := o.s.SaveGroup(ctx, items)
		if err == nil {
			return saved[0], nil
		}
		if !errors.Is(err, ErrNameTaken) {
			return Connection{}, serrors.Wrap(op, err)
		}
		lastErr = err
	}
	return Connection{}, serrors.Wrap(op, lastErr)
}

// PlanCompanion is PlanGrant for a grant that comes with another (the connection to receive of the
// sign-in that made the connection to send): an existing connection is updated only when it belongs to the
// group of the planned primary — it is the primary's connection, or the one the primary takes its tokens
// from, or one that takes its tokens from either — never by its mailbox alone. A new sign-in, or one for a
// connection that has no group, makes a pair of its own, so it cannot take over the connection of another.
func PlanCompanion(ctx context.Context, s ConnectionStore, secret func(string) bool, tenantID uuid.UUID, g OAuth2Grant, createdBy int64, primary GroupItem) (GroupItem, error) {
	group := map[uuid.UUID]bool{}
	if primary.Connection.Version > 0 {
		group[primary.Connection.ID] = true
		if _, old, err := s.Get(ctx, tenantID, primary.Connection.ID); err == nil {
			if owner, err := uuid.Parse(old[OAuth2TokenFrom]); err == nil {
				group[owner] = true
			}
		}
	}
	existing, err := s.List(ctx, tenantID, g.Kind)
	if err != nil {
		return GroupItem{}, err
	}
	for _, c := range existing {
		if c.Identity.External == "" || c.Identity.External != g.Identity.External {
			continue
		}
		_, old, err := s.Get(ctx, tenantID, c.ID)
		if err != nil {
			return GroupItem{}, err
		}
		owner, _ := uuid.Parse(old[OAuth2TokenFrom])
		if p := g.Credentials[OAuth2Preset]; (p != "" && old[OAuth2Preset] != p) || (!group[c.ID] && !group[owner]) {
			continue
		}
		cur := scrubAcrossChange(c, secret, old, g.Credentials)
		cur.Identity = SafeIdentity(g.Identity, secret, g.Credentials, old)
		if cur.Status != StatusDisabled {
			cur.Status, cur.StatusReason = StatusConnected, ""
		}
		return GroupItem{Connection: cur, Credentials: carryOver(cloneCredentials(g.Credentials), old)}, nil
	}
	return planNew(existing, tenantID, g, createdBy)
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
			if cur.Identity.External != identity.External {
				continue // changed to another account between the list and the read: not this sign-in's
			}
			// A sign-in with a preset updates only a connection made by a sign-in with that preset: a
			// connection of the same mailbox made by a login and a password is another connection, and
			// graphs bound to it keep their server, folder and credentials.
			if p := creds[OAuth2Preset]; p != "" && old[OAuth2Preset] != p {
				continue
			}
			cur = scrubAcrossChange(cur, secret, old, creds)
			cur.Identity = SafeIdentity(identity, secret, creds, old)
			if cur.Status != StatusDisabled { // a connection an administrator switched off stays off
				cur.Status, cur.StatusReason = StatusConnected, ""
			}
			// The settings a grant does not own (the folder of a mailbox, say) stay as the person set them.
			return GroupItem{Connection: cur, Credentials: carryOver(cloneCredentials(creds), old)}, nil
		}
	}
	return planNew(existing, tenantID, g, createdBy)
}

// planNew is a new connection for a grant, named after its kind (the first name of the kind that is free).
func planNew(existing []Connection, tenantID uuid.UUID, g OAuth2Grant, createdBy int64) (GroupItem, error) {
	base := nameFrom(string(g.Kind))
	display := g.Identity.Display
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
			return GroupItem{Connection: Connection{TenantID: tenantID, Kind: g.Kind, Name: name, DisplayName: display, Identity: g.Identity, Status: StatusConnected, CreatedBy: createdBy}, Credentials: g.Credentials}, nil
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
	return o.tokenAs(ctx, tenantID, connectionID, check, false)
}

// tokenAs is token; asOwner: the connection owns the token set another connection shares, so it serves
// that one whether it is connected or disabled: the owner's own status is its own business (a disabled
// owner is not used by graphs itself, and still keeps the tokens of the connection that is).
//
// A connection can change what it is while a call looks at it (a sign-in makes it a dependent of another
// connection, or a deletion makes it an owner): every step that decides by its link checks it again under
// the row lock and, when it changed, the call starts over with the current state.
func (o *oauth) tokenAs(ctx context.Context, tenantID, connectionID uuid.UUID, check, asOwner bool) (string, error) {
	for attempt := 0; attempt < 4; attempt++ {
		tok, again, err := o.tokenOnce(ctx, tenantID, connectionID, check, asOwner)
		if !again {
			return tok, err
		}
	}
	return "", serrors.Wrap("automation.connectors.OAuth2.Token", fmt.Errorf("%w: the connection kept changing", ErrInvalid))
}

func (o *oauth) tokenOnce(ctx context.Context, tenantID, connectionID uuid.UUID, check, asOwner bool) (string, bool, error) {
	serves := func(status ConnectionStatus, check bool) bool {
		return serves(status, check) || (asOwner && status == StatusDisabled)
	}
	const op = serrors.Op("automation.connectors.OAuth2.Token")
	c, creds, err := o.s.load(ctx, o.s.db, tenantID, connectionID, false)
	if err != nil {
		return "", false, serrors.Wrap(op, err)
	}
	if c.Status == StatusNeedsReauth {
		return "", false, serrors.Wrap(op, ErrNeedsReauth)
	}
	if !serves(c.Status, check) {
		return "", false, serrors.Wrap(op, ErrNotConnected)
	}
	owner, linkErr := uuid.Parse(creds[OAuth2TokenFrom])
	isDependent := linkErr == nil
	// A fresh token is returned as it is for a connection that owns its tokens. A dependent's is the owner's
	// copy: the owner is consulted first, because one that already needs a new sign-in leaves the grant dead
	// whatever the copy still says, and the caller must learn that now (ErrNeedsReauth), not when the copy ends.
	if fresh(creds) && (!isDependent || !o.ownerNeedsReauth(ctx, tenantID, owner)) {
		return creds[OAuth2AccessToken], false, nil
	}
	if isDependent {
		return o.tokenOfOwner(ctx, tenantID, connectionID, owner, creds[OAuth2RefreshToken], check)
	}
	var (
		token     string
		reauth    bool
		dependent bool
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
		if _, err := uuid.Parse(creds[OAuth2TokenFrom]); err == nil {
			// A sign-in made it a dependent of another connection meanwhile: its refresh token is a copy,
			// and the owner is the one that refreshes.
			dependent = true
			return nil
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
		return "", false, serrors.Wrap(op, err)
	}
	if dependent {
		return "", true, nil
	}
	if reauth {
		return "", false, serrors.Wrap(op, ErrNeedsReauth)
	}
	return token, false, nil
}

// ownerNeedsReauth reports whether the owner of a token set is marked as needing a new sign-in, or is gone.
func (o *oauth) ownerNeedsReauth(ctx context.Context, tenantID, owner uuid.UUID) bool {
	var status string
	if err := o.s.db.QueryRow(ctx, `SELECT status FROM automation.connections WHERE tenant_id = $1 AND id = $2`, tenantID, owner).Scan(&status); err != nil {
		return true // gone (a deletion is promoting its dependents) or unreadable: the owner path decides
	}
	return ConnectionStatus(status) == StatusNeedsReauth
}

// tokenSet are the credentials of a token set (what a refresh replaces).
var tokenSet = []string{OAuth2AccessToken, OAuth2RefreshToken, OAuth2TokenType, OAuth2ExpiresAt}

// tokenOfOwner is the token of a connection that shares the token set of another: the owner refreshes it
// (once, under its own lock), and this connection takes the refreshed set as its own. The owner's
// re-authorization is this one's too. again: the link changed meanwhile; the call starts over.
//
// grant is the refresh token this connection held when the call began: it names the grant the call acts for. A
// refusal is recorded only against that grant; a connection that was authorized again meanwhile (the same
// owner, a new grant) is not marked, and the call starts over with what it is now.
func (o *oauth) tokenOfOwner(ctx context.Context, tenantID, id, owner uuid.UUID, grant string, check bool) (string, bool, error) {
	const op = serrors.Op("automation.connectors.OAuth2.Token")
	_, err := o.tokenAs(ctx, tenantID, owner, false, true)
	if errors.Is(err, ErrNeedsReauth) || errors.Is(err, ErrNotFound) {
		again := false
		if !check {
			// Marked under the lock, and only while the grant that was refused is still this connection's: one
			// a deletion made independent (it owns its tokens now), or a sign-in gave a new grant, is not.
			err := pgx.BeginFunc(ctx, o.s.db, func(tx pgx.Tx) error {
				c, creds, err := o.s.load(ctx, tx, tenantID, id, true)
				if err != nil {
					return err
				}
				if creds[OAuth2TokenFrom] != owner.String() || creds[OAuth2RefreshToken] != grant {
					again = true
					return nil
				}
				var reauth bool
				return o.markReauth(ctx, tx, c, "the provider refused the refresh token", &reauth)
			})
			if err != nil {
				return "", false, serrors.Wrap(op, err)
			}
		}
		if again {
			return "", true, nil
		}
		return "", false, serrors.Wrap(op, ErrNeedsReauth)
	}
	if err != nil {
		return "", false, err
	}
	var stored string
	retry, reauth := false, false
	err = pgx.BeginFunc(ctx, o.s.db, func(tx pgx.Tx) error {
		c, creds, err := o.s.load(ctx, tx, tenantID, id, true)
		if err != nil {
			return err
		}
		// Under the lock: still this owner's dependent, and still serving.
		if creds[OAuth2TokenFrom] != owner.String() {
			retry = true
			return nil
		}
		if !serves(c.Status, check) {
			return ErrNotConnected
		}
		oc, theirs, err := o.s.load(ctx, tx, tenantID, owner, false)
		if err != nil {
			return err
		}
		// The owner was marked as needing a new sign-in after the refresh above gave its token: what it
		// holds is not a token to hand on. (A switched-off owner still serves; see markReauth.)
		if oc.Status == StatusNeedsReauth {
			if check {
				reauth = true
				return nil
			}
			return o.markReauth(ctx, tx, c, "the provider refused the refresh token", &reauth)
		}
		if !usable(theirs) {
			// The owner's token ended while this call waited for the row: it is not handed on. The call starts
			// over, and the owner refreshes it.
			retry = true
			return nil
		}
		stored = theirs[OAuth2AccessToken]
		same := true
		for _, k := range tokenSet {
			same = same && creds[k] == theirs[k]
		}
		if same {
			return nil
		}
		next := Credentials{}
		for _, k := range tokenSet {
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
		return "", false, serrors.Wrap(op, err)
	}
	if retry {
		return "", true, nil
	}
	if reauth {
		return "", false, serrors.Wrap(op, ErrNeedsReauth)
	}
	return stored, false, nil
}

func (o *oauth) markReauth(ctx context.Context, tx pgx.Tx, c Connection, reason string, reauth *bool) error {
	// A connection an administrator switched off stays off: the grant needs a new sign-in, which is the
	// companion's to report (it is the one that is in use), and a sign-in does not enable the owner.
	_, err := tx.Exec(ctx, `UPDATE automation.connections SET status = CASE WHEN status = 'disabled' THEN status ELSE $3 END,
		status_reason = $4, version = version + 1, updated_at = now()
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
