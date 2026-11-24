package email

import (
	"context"
	"encoding/base64"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/iota-uz/iota-sdk/pkg/automation/catalog"
	"github.com/iota-uz/iota-sdk/pkg/automation/connectors"
	"github.com/iota-uz/iota-sdk/pkg/automation/connectors/email/emailtest"
	itf "github.com/iota-uz/iota-sdk/pkg/itf"
)

// The migrator reads ./migrations relative to the working directory.
func TestMain(m *testing.M) {
	if err := os.Chdir("../../../.."); err != nil {
		panic(err)
	}
	os.Exit(m.Run())
}

func idTokenFor(email, aud string) string {
	return idTokenWith(email, "https://accounts.google.com", strconv.Quote(aud))
}

// idTokenWith is an identity token with the issuer ("" leaves it out) and the audience (a JSON value) given.
func idTokenWith(email, iss, audJSON string) string {
	issClaim := ""
	if iss != "" {
		issClaim = `"iss":` + strconv.Quote(iss) + `,`
	}
	// The client the token was issued to (the tests' "cid") and a future expiry: a token without them is not
	// trusted for the mailbox it names.
	claims := []byte(`{` + issClaim + `"email":` + strconv.Quote(email) + `,"aud":` + audJSON + `,"exp":` + strconv.FormatInt(time.Now().Add(time.Hour).Unix(), 10) + `}`)
	enc := base64.RawURLEncoding.EncodeToString
	return enc([]byte(`{"alg":"none"}`)) + "." + enc(claims) + "."
}

type tokenServer struct {
	*httptest.Server
	refuse    atomic.Bool
	idToken   atomic.Value // when set, the ID token the code gives (instead of the right one)
	noRefresh atomic.Bool  // the authorization code gives no refresh token
	otherAud  atomic.Bool  // and an ID token issued to another client
	n         atomic.Int32
	refreshes atomic.Int32
	current   atomic.Value // the one refresh token the provider honours: it rotates them
}

func newTokenServer(t *testing.T, email string) *tokenServer {
	t.Helper()
	ts := &tokenServer{}
	ts.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		w.Header().Set("Content-Type", "application/json")
		switch r.Form.Get("grant_type") {
		case "authorization_code":
			ts.n.Add(1)
			ts.current.Store("refresh-1")
			refresh, aud := `"refresh_token":"refresh-1",`, "cid"
			if ts.noRefresh.Load() {
				refresh = ""
			}
			if ts.otherAud.Load() {
				aud = "another-client"
			}
			id := idTokenFor(email, aud)
			if v, _ := ts.idToken.Load().(string); v != "" {
				id = v
			}
			_, _ = w.Write([]byte(`{"access_token":"access-1",` + refresh + `"token_type":"Bearer","expires_in":60,"id_token":"` + id + `"}`))
		case "refresh_token":
			// A provider that rotates refresh tokens: the old one dies with the refresh.
			if cur, _ := ts.current.Load().(string); ts.refuse.Load() || r.Form.Get("refresh_token") != cur {
				w.WriteHeader(http.StatusBadRequest)
				_, _ = w.Write([]byte(`{"error":"invalid_grant"}`))
				return
			}
			n := ts.refreshes.Add(1) + 1
			next := fmt.Sprintf("refresh-%d", n)
			ts.current.Store(next)
			_, _ = fmt.Fprintf(w, `{"access_token":"access-%d","refresh_token":%q,"token_type":"Bearer","expires_in":3600}`, n, next)
		default:
			w.WriteHeader(http.StatusBadRequest)
		}
	}))
	t.Cleanup(ts.Close)
	return ts
}

type lateOAuth struct{ o connectors.OAuth2 }

func (l *lateOAuth) Begin(ctx context.Context, t uuid.UUID, k catalog.ConnectorKind, r string) (string, string, error) {
	return l.o.Begin(ctx, t, k, r)
}
func (l *lateOAuth) Complete(ctx context.Context, t uuid.UUID, s, c string) (connectors.Connection, error) {
	return l.o.Complete(ctx, t, s, c)
}
func (l *lateOAuth) Exchange(ctx context.Context, t uuid.UUID, s, c string) (connectors.OAuth2Grant, error) {
	return l.o.Exchange(ctx, t, s, c)
}
func (l *lateOAuth) Token(ctx context.Context, t, id uuid.UUID) (string, error) {
	return l.o.Token(ctx, t, id)
}

// One Google sign-in is a connection to send and a connection to receive, with the same tokens; a refresh
// keeps what a token set does not own; a refresh the provider refuses sends both to needs_reauth.
func TestOneSignInIsTwoConnectionsThatShareTheLogin(t *testing.T) {
	h := itf.NewHarness(t, itf.HarnessConfig{
		Name:      "automation_email",
		Database:  itf.DatabaseConfig{Provisioning: itf.ProvisioningPerTestDatabase},
		Isolation: itf.IsolationConfig{Mode: itf.IsolationCommitted},
	})
	scope := h.Scope(t)
	ctx := context.Background()
	ts := newTokenServer(t, mailbox)
	keys, err := connectors.NewKeyring("k1", map[string][]byte{"k1": []byte("0123456789abcdef0123456789abcdef")})
	require.NoError(t, err)

	late := &lateOAuth{}
	var store connectors.Store
	mailSrv := emailtest.New(t, emailtest.ModeTLS, emailtest.Account{User: mailbox, Password: password})
	mailSrv.AcceptToken(mailbox, "access-1")
	mailSrv.AcceptToken(mailbox, "access-2")
	mailSrv.AcceptToken(mailbox, "access-3")
	providers := map[string]Provider{PresetGmail: {
		AuthURL: ts.URL + "/auth", TokenURL: ts.URL + "/token",
		SMTPHost: "127.0.0.1", SMTPPort: atoi(port(mailSrv.SMTPAddr)), IMAPHost: "127.0.0.1", IMAPPort: atoi(port(mailSrv.IMAPAddr)), Security: "tls",
	}}
	cfg := Config{OAuth: late, Providers: providers, TLSConfig: mailSrv.TLS}
	dir := connectors.NewDirectory(nil)
	for _, c := range New(cfg) {
		require.NoError(t, dir.Register(c))
	}
	platform := connectors.NewPlatformCredentials(dir, map[catalog.ConnectorKind]connectors.PlatformCredential{
		GoogleClient: {Credentials: connectors.Credentials{"client_id": "cid", "client_secret": "csecret"}},
	})
	store = connectors.NewStore(scope.Pool, keys, nil, platform, nil)
	o, err := connectors.NewOAuth2(store, dir, platform, connectors.WithHTTPClient(ts.Client()))
	require.NoError(t, err)
	late.o = o
	// The connectors read the store the sign-in saved into, to link the connections of one login.
	cs := New(Config{OAuth: o, Connections: store, Providers: providers, TLSConfig: mailSrv.TLS})
	hubOf := cs[0].(*smtpConnector).hub

	preset, ok := o.(connectors.PresetOAuth2)
	require.True(t, ok)
	authURL, state, err := preset.BeginPreset(ctx, scope.Tenant.ID, KindSMTP, PresetGmail, "https://app.test/callback")
	require.NoError(t, err)
	u, err := url.Parse(authURL)
	require.NoError(t, err)
	assert.Equal(t, "consent", u.Query().Get("prompt"), "Google gives a refresh token on every sign-in")
	assert.Equal(t, "offline", u.Query().Get("access_type"))
	assert.Contains(t, strings.Fields(u.Query().Get("scope")), "https://mail.google.com/")
	assert.Equal(t, "cid", u.Query().Get("client_id"))
	assert.NotContains(t, authURL, "csecret")
	_, _, err = preset.BeginPreset(ctx, scope.Tenant.ID, KindSMTP, "unknown", "https://app.test/callback")
	require.Error(t, err)

	smtpConn, err := o.Complete(ctx, scope.Tenant.ID, state, "code")
	require.NoError(t, err)
	assert.Equal(t, KindSMTP, smtpConn.Kind)
	assert.Equal(t, connectors.Identity{Display: mailbox, External: mailbox, Kind: connectors.IdentityEmail}, smtpConn.Identity)
	imaps, err := store.List(ctx, scope.Tenant.ID, KindIMAP)
	require.NoError(t, err)
	require.Len(t, imaps, 1, "the sign-in also created the connection to receive")
	imapConn := imaps[0]
	_, sc, err := store.Get(ctx, scope.Tenant.ID, smtpConn.ID)
	require.NoError(t, err)
	_, ic, err := store.Get(ctx, scope.Tenant.ID, imapConn.ID)
	require.NoError(t, err)
	assert.Equal(t, PresetGmail, sc["preset"])
	assert.Equal(t, PresetGmail, ic["preset"])
	assert.Equal(t, mailbox, sc["username"])
	assert.Equal(t, mailbox, sc["from_address"])
	assert.NotEmpty(t, sc["login_id"])
	assert.Equal(t, sc["login_id"], ic["login_id"])
	assert.Equal(t, "refresh-1", ic["refresh_token"], "the tokens are the same")
	assert.Equal(t, sc["access_token"], ic["access_token"])

	// Signing in again as the same mailbox updates both; it adds none.
	_, state, err = preset.BeginPreset(ctx, scope.Tenant.ID, KindIMAP, PresetGmail, "https://app.test/callback")
	require.NoError(t, err)
	again, err := o.Complete(ctx, scope.Tenant.ID, state, "code")
	require.NoError(t, err)
	assert.Equal(t, imapConn.ID, again.ID)
	smtps, err := store.List(ctx, scope.Tenant.ID, KindSMTP)
	require.NoError(t, err)
	assert.Len(t, smtps, 1)

	// The last sign-in began from the receiving connection: it owns the token set, the sending one follows.
	// The sending connection is used first and its set is stale: the refresh is the owner's, once; the
	// provider rotates refresh tokens, and neither connection ever offers the dead one.
	tok, err := o.Token(ctx, scope.Tenant.ID, smtpConn.ID)
	require.NoError(t, err)
	assert.Equal(t, "access-2", tok)
	assert.Equal(t, int32(1), ts.refreshes.Load())
	tok, err = o.Token(ctx, scope.Tenant.ID, imapConn.ID)
	require.NoError(t, err)
	assert.Equal(t, "access-2", tok, "the owner has the refreshed token too: one refresh served both")
	assert.Equal(t, int32(1), ts.refreshes.Load())
	_, sc, err = store.Get(ctx, scope.Tenant.ID, smtpConn.ID)
	require.NoError(t, err)
	_, ic, err = store.Get(ctx, scope.Tenant.ID, imapConn.ID)
	require.NoError(t, err)
	assert.Equal(t, "refresh-2", ic["refresh_token"], "the rotated refresh token is the owner's")
	assert.Equal(t, "refresh-2", sc["refresh_token"], "and the other's copy follows it")
	assert.Equal(t, imapConn.ID.String(), sc[connectors.OAuth2TokenFrom])
	assert.Empty(t, ic[connectors.OAuth2TokenFrom])
	for _, c := range []connectors.Credentials{sc, ic} {
		assert.Equal(t, PresetGmail, c["preset"])
		assert.Equal(t, mailbox, c["username"])
		assert.NotEmpty(t, c["login_id"])
		assert.Empty(t, c["id_token"], "the identity token of the sign-in is not kept")
	}
	got0, _, err := store.Get(ctx, scope.Tenant.ID, smtpConn.ID)
	require.NoError(t, err)
	assert.Equal(t, mailbox, got0.Identity.Display, "a refresh keeps the mailbox shown: it is not a secret")

	// The access is revoked in the account: the refresh is refused, and both connections need a new sign-in.
	ts.refuse.Store(true)
	for _, id := range []uuid.UUID{smtpConn.ID, imapConn.ID} {
		c, cr, err := store.Get(ctx, scope.Tenant.ID, id)
		require.NoError(t, err)
		cr["expires_at"] = "2000-01-01T00:00:00Z" // the access token has run out
		_, err = store.Save(ctx, c, cr)
		require.NoError(t, err)
	}
	_, ic, err = store.Get(ctx, scope.Tenant.ID, imapConn.ID)
	require.NoError(t, err)
	_, err = hubOf.accessToken(ctx, scope.Tenant.ID, imapConn.ID.String(), ic)
	require.ErrorIs(t, err, connectors.ErrNeedsReauth)
	for _, id := range []uuid.UUID{smtpConn.ID, imapConn.ID} {
		c, _, err := store.Get(ctx, scope.Tenant.ID, id)
		require.NoError(t, err)
		assert.Equal(t, connectors.StatusNeedsReauth, c.Status, "both connections of the sign-in")
	}
}

// The pair of connections of a sign-in is saved whole or not at all.
func TestSignInPairIsSavedAtomically(t *testing.T) {
	h := itf.NewHarness(t, itf.HarnessConfig{
		Name:      "automation_email_group",
		Database:  itf.DatabaseConfig{Provisioning: itf.ProvisioningPerTestDatabase},
		Isolation: itf.IsolationConfig{Mode: itf.IsolationCommitted},
	})
	scope := h.Scope(t)
	ctx := context.Background()
	keys, err := connectors.NewKeyring("k1", map[string][]byte{"k1": []byte("0123456789abcdef0123456789abcdef")})
	require.NoError(t, err)
	store := connectors.NewStore(scope.Pool, keys, nil, nil, nil)
	good := connectors.Connection{TenantID: scope.Tenant.ID, Kind: KindSMTP, Name: "smtp", Status: connectors.StatusConnected}
	bad := connectors.Connection{TenantID: scope.Tenant.ID, Kind: KindIMAP, Name: "Not A Valid Name", Status: connectors.StatusConnected}
	_, err = store.(connectors.GroupSaver).SaveGroup(ctx, []connectors.GroupItem{
		{Connection: good, Credentials: connectors.Credentials{"access_token": "a"}},
		{Connection: bad, Credentials: connectors.Credentials{"access_token": "a"}},
	})
	require.Error(t, err)
	left, err := store.List(ctx, scope.Tenant.ID, "")
	require.NoError(t, err)
	assert.Empty(t, left, "the second write failed: the first did not stay")
}

type signIn struct {
	t          *testing.T
	ctx        context.Context
	ts         *tokenServer
	store      connectors.Store
	o          connectors.OAuth2
	tenant     uuid.UUID
	smtp, imap connectors.Connection
	mail       *emailtest.Server
}

// signInEnv is a tenant that has signed in with Google once: a sending and a receiving connection.
func signInEnv(t *testing.T) *signIn {
	t.Helper()
	h := itf.NewHarness(t, itf.HarnessConfig{
		Name:      "automation_email_lifecycle",
		Database:  itf.DatabaseConfig{Provisioning: itf.ProvisioningPerTestDatabase},
		Isolation: itf.IsolationConfig{Mode: itf.IsolationCommitted},
	})
	scope := h.Scope(t)
	e := &signIn{t: t, ctx: context.Background(), ts: newTokenServer(t, mailbox), tenant: scope.Tenant.ID}
	keys, err := connectors.NewKeyring("k1", map[string][]byte{"k1": []byte("0123456789abcdef0123456789abcdef")})
	require.NoError(t, err)
	mailSrv := emailtest.New(t, emailtest.ModeTLS, emailtest.Account{User: mailbox, Password: password})
	e.mail = mailSrv
	for _, tok := range []string{"access-1", "access-2", "access-3"} {
		mailSrv.AcceptToken(mailbox, tok)
	}
	providers := map[string]Provider{PresetGmail: {
		AuthURL: e.ts.URL + "/auth", TokenURL: e.ts.URL + "/token",
		SMTPHost: "127.0.0.1", SMTPPort: atoi(port(mailSrv.SMTPAddr)), IMAPHost: "127.0.0.1", IMAPPort: atoi(port(mailSrv.IMAPAddr)), Security: "tls",
	}}
	dir := connectors.NewDirectory(nil)
	for _, c := range New(Config{Providers: providers, TLSConfig: mailSrv.TLS}) {
		require.NoError(t, dir.Register(c))
	}
	platform := connectors.NewPlatformCredentials(dir, map[catalog.ConnectorKind]connectors.PlatformCredential{
		GoogleClient: {Credentials: connectors.Credentials{"client_id": "cid", "client_secret": "csecret"}},
	})
	e.store = connectors.NewStore(scope.Pool, keys, nil, platform, nil)
	e.o, err = connectors.NewOAuth2(e.store, dir, platform, connectors.WithHTTPClient(e.ts.Client()))
	require.NoError(t, err)
	_, state, err := e.o.(connectors.PresetOAuth2).BeginPreset(e.ctx, e.tenant, KindSMTP, PresetGmail, "https://app.test/cb")
	require.NoError(t, err)
	e.smtp, err = e.o.Complete(e.ctx, e.tenant, state, "code")
	require.NoError(t, err)
	imaps, err := e.store.List(e.ctx, e.tenant, KindIMAP)
	require.NoError(t, err)
	require.Len(t, imaps, 1)
	e.imap = imaps[0]
	return e
}

// age makes the access token of a connection run out.
func (e *signIn) age(id uuid.UUID) {
	e.t.Helper()
	c, cr, err := e.store.Get(e.ctx, e.tenant, id)
	require.NoError(e.t, err)
	cr["expires_at"] = "2000-01-01T00:00:00Z"
	_, err = e.store.Save(e.ctx, c, cr)
	require.NoError(e.t, err)
}

// Disabling the connection that owns the token set does not break the one that is still on: the shared
// grant is not tied to either connection's status.
func TestDisabledOwnerStillServesTheOtherConnection(t *testing.T) {
	e := signInEnv(t)
	require.NoError(t, e.store.SetStatus(e.ctx, e.tenant, e.smtp.ID, connectors.StatusDisabled, "off"))
	e.age(e.imap.ID)
	e.age(e.smtp.ID)
	tok, err := e.o.Token(e.ctx, e.tenant, e.imap.ID)
	require.NoError(t, err, "the enabled connection refreshes through its disabled owner")
	assert.Equal(t, "access-2", tok)
	owner, _, err := e.store.Get(e.ctx, e.tenant, e.smtp.ID)
	require.NoError(t, err)
	assert.Equal(t, connectors.StatusDisabled, owner.Status, "and the owner stays disabled")
	_, err = e.o.Token(e.ctx, e.tenant, e.smtp.ID)
	require.ErrorIs(t, err, connectors.ErrNotConnected, "a disabled connection itself gives no token")
}

// An owner that needs a new sign-in has no token to hand on: the connection that takes its tokens from it
// does not copy or return what the owner still holds, and is marked, as the sign-in is.
func TestOwnerThatNeedsReauthGivesNoTokenToItsDependent(t *testing.T) {
	e := signInEnv(t)
	require.NoError(t, e.store.SetStatus(e.ctx, e.tenant, e.smtp.ID, connectors.StatusNeedsReauth, "the provider refused the refresh token"))
	e.age(e.imap.ID)
	_, err := e.o.Token(e.ctx, e.tenant, e.imap.ID)
	require.ErrorIs(t, err, connectors.ErrNeedsReauth)
	got, _, err := e.store.Get(e.ctx, e.tenant, e.imap.ID)
	require.NoError(t, err)
	assert.Equal(t, connectors.StatusNeedsReauth, got.Status)
}

// Deleting the owner hands the token set to the connection that took it from it: it goes on working.
func TestDeletingTheOwnerKeepsTheOtherConnectionWorking(t *testing.T) {
	e := signInEnv(t)
	require.NoError(t, e.store.Delete(e.ctx, e.tenant, e.smtp.ID))
	_, cr, err := e.store.Get(e.ctx, e.tenant, e.imap.ID)
	require.NoError(t, err)
	assert.Empty(t, cr[connectors.OAuth2TokenFrom], "it owns its tokens now")
	assert.Equal(t, "refresh-1", cr["refresh_token"])
	assert.Equal(t, PresetGmail, cr["preset"])
	e.age(e.imap.ID)
	tok, err := e.o.Token(e.ctx, e.tenant, e.imap.ID)
	require.NoError(t, err)
	assert.Equal(t, "access-2", tok)
}

// A sign-in does not rewrite a connection of the same mailbox that was made with a login and a password:
// graphs bound to that connection keep its server, folder and credentials, and a connection switched off
// stays off.
func TestSignInDoesNotOverwriteAPasswordConnection(t *testing.T) {
	e := signInEnv(t)
	for _, id := range []uuid.UUID{e.smtp.ID, e.imap.ID} {
		require.NoError(t, e.store.Delete(e.ctx, e.tenant, id))
	}
	password, err := e.store.Save(e.ctx, connectors.Connection{TenantID: e.tenant, Kind: KindIMAP, Name: "imap", DisplayName: "Old box", Status: connectors.StatusDisabled,
		Identity: connectors.Identity{Display: mailbox, External: mailbox, Kind: connectors.IdentityEmail}},
		connectors.Credentials{"preset": "custom", "host": "imap.corp.example", "port": "993", "username": mailbox, "password": "pw", "mailbox": "Support"})
	require.NoError(t, err)

	_, state, err := e.o.(connectors.PresetOAuth2).BeginPreset(e.ctx, e.tenant, KindSMTP, PresetGmail, "https://app.test/cb")
	require.NoError(t, err)
	_, err = e.o.Complete(e.ctx, e.tenant, state, "code")
	require.NoError(t, err)

	got, cr, err := e.store.Get(e.ctx, e.tenant, password.ID)
	require.NoError(t, err)
	assert.Equal(t, password.Version, got.Version, "untouched")
	assert.Equal(t, connectors.StatusDisabled, got.Status)
	assert.Equal(t, "imap.corp.example", cr["host"])
	assert.Equal(t, "Support", cr["mailbox"])
	imaps, err := e.store.List(e.ctx, e.tenant, KindIMAP)
	require.NoError(t, err)
	require.Len(t, imaps, 2, "the sign-in made its own connection to receive")

	// Signing in again as the same mailbox updates the sign-in's connections, and keeps a switched-off one off.
	var signed connectors.Connection
	for _, c := range imaps {
		if c.ID != password.ID {
			signed = c
		}
	}
	require.NoError(t, e.store.SetStatus(e.ctx, e.tenant, signed.ID, connectors.StatusDisabled, "off"))
	_, state, err = e.o.(connectors.PresetOAuth2).BeginPreset(e.ctx, e.tenant, KindSMTP, PresetGmail, "https://app.test/cb")
	require.NoError(t, err)
	_, err = e.o.Complete(e.ctx, e.tenant, state, "code")
	require.NoError(t, err)
	again, _, err := e.store.Get(e.ctx, e.tenant, signed.ID)
	require.NoError(t, err)
	assert.Equal(t, connectors.StatusDisabled, again.Status)
	imaps, _ = e.store.List(e.ctx, e.tenant, KindIMAP)
	assert.Len(t, imaps, 2)
}

// A sign-in that the provider did not give a refresh token, or whose ID token was issued to another client,
// is refused, and saves nothing: the mailbox it names is not taken on trust.
func TestSignInIsRefusedWithoutARefreshTokenOrForAnotherClient(t *testing.T) {
	for name, set := range map[string]func(*tokenServer){
		"no refresh token":       func(ts *tokenServer) { ts.noRefresh.Store(true) },
		"an ID token of another": func(ts *tokenServer) { ts.otherAud.Store(true) },
	} {
		t.Run(name, func(t *testing.T) {
			e := signInEnv(t)
			for _, id := range []uuid.UUID{e.smtp.ID, e.imap.ID} {
				require.NoError(t, e.store.Delete(e.ctx, e.tenant, id))
			}
			set(e.ts)
			_, state, err := e.o.(connectors.PresetOAuth2).BeginPreset(e.ctx, e.tenant, KindSMTP, PresetGmail, "https://app.test/cb")
			require.NoError(t, err)
			_, err = e.o.Complete(e.ctx, e.tenant, state, "code")
			require.Error(t, err)
			for _, kind := range []catalog.ConnectorKind{KindSMTP, KindIMAP} {
				list, err := e.store.List(e.ctx, e.tenant, kind)
				require.NoError(t, err)
				assert.Empty(t, list, "nothing was saved")
			}
		})
	}
}

// An ID token is read for who signed in only when it has the provider's issuer and is for this client alone.
func TestSignInIsRefusedForAnIDTokenOfAnotherIssuerOrAudience(t *testing.T) {
	for name, token := range map[string]string{
		"no issuer":                  idTokenWith(mailbox, "", `"cid"`),
		"another issuer":             idTokenWith(mailbox, "https://evil.example", `"cid"`),
		"an extra audience":          idTokenWith(mailbox, "https://accounts.google.com", `["cid","untrusted-client"]`),
		"an audience that is not it": idTokenWith(mailbox, "https://accounts.google.com", `["other"]`),
	} {
		t.Run(name, func(t *testing.T) {
			e := signInEnv(t)
			for _, id := range []uuid.UUID{e.smtp.ID, e.imap.ID} {
				require.NoError(t, e.store.Delete(e.ctx, e.tenant, id))
			}
			e.ts.idToken.Store(token)
			_, state, err := e.o.(connectors.PresetOAuth2).BeginPreset(e.ctx, e.tenant, KindSMTP, PresetGmail, "https://app.test/cb")
			require.NoError(t, err)
			_, err = e.o.Complete(e.ctx, e.tenant, state, "code")
			require.Error(t, err)
			list, err := e.store.List(e.ctx, e.tenant, KindSMTP)
			require.NoError(t, err)
			assert.Empty(t, list, "nothing was saved")
		})
	}
}

// A sign-in is saved only when both connections of the pair work: an account whose IMAP is closed is refused
// where the person sees it, and neither connection is saved.
func TestSignInIsRefusedWhenTheOtherProtocolDoesNotWork(t *testing.T) {
	e := signInEnv(t)
	for _, id := range []uuid.UUID{e.smtp.ID, e.imap.ID} {
		require.NoError(t, e.store.Delete(e.ctx, e.tenant, id))
	}
	e.mail.RefuseIMAPLogins(true)
	_, state, err := e.o.(connectors.PresetOAuth2).BeginPreset(e.ctx, e.tenant, KindSMTP, PresetGmail, "https://app.test/cb")
	require.NoError(t, err)
	_, err = e.o.Complete(e.ctx, e.tenant, state, "code")
	require.ErrorIs(t, err, connectors.ErrOAuth2Refused)
	for _, kind := range []catalog.ConnectorKind{KindSMTP, KindIMAP} {
		list, err := e.store.List(e.ctx, e.tenant, kind)
		require.NoError(t, err)
		assert.Empty(t, list, "neither connection of the pair was saved")
	}
}

// Signing in again as the same account replaces the grant and keeps the settings a grant does not own: the
// folder a receiving connection reads stays.
func TestSignInAgainKeepsTheFolderOfTheReceivingConnection(t *testing.T) {
	e := signInEnv(t)
	c, cr, err := e.store.Get(e.ctx, e.tenant, e.imap.ID)
	require.NoError(t, err)
	cr["mailbox"] = "Support"
	_, err = e.store.Save(e.ctx, c, cr)
	require.NoError(t, err)

	for _, kind := range []catalog.ConnectorKind{KindIMAP, KindSMTP} { // from either side
		_, state, err := e.o.(connectors.PresetOAuth2).BeginPreset(e.ctx, e.tenant, kind, PresetGmail, "https://app.test/cb")
		require.NoError(t, err)
		_, err = e.o.Complete(e.ctx, e.tenant, state, "code")
		require.NoError(t, err)
		_, got, err := e.store.Get(e.ctx, e.tenant, e.imap.ID)
		require.NoError(t, err)
		assert.Equal(t, "Support", got["mailbox"], "signing in from %s", kind)
		assert.Equal(t, "refresh-1", got["refresh_token"], "and the grant is the new one")
	}
	imaps, err := e.store.List(e.ctx, e.tenant, KindIMAP)
	require.NoError(t, err)
	assert.Len(t, imaps, 1, "the same connection, not another")
}

// A dependent whose cached token is still fresh learns that its owner needs a new sign-in: the owner is
// consulted before the copy is handed on.
func TestDependentWithAFreshTokenLearnsThatItsOwnerNeedsReauth(t *testing.T) {
	e := signInEnv(t)
	require.NoError(t, e.store.SetStatus(e.ctx, e.tenant, e.smtp.ID, connectors.StatusNeedsReauth, "refused"))
	_, err := e.o.Token(e.ctx, e.tenant, e.imap.ID) // not aged: its copy has 59 minutes left
	require.ErrorIs(t, err, connectors.ErrNeedsReauth)
}
