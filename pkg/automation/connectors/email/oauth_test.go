package email

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"sync/atomic"
	"testing"

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

func idToken(email string) string {
	claims, _ := json.Marshal(map[string]string{"email": email})
	enc := base64.RawURLEncoding.EncodeToString
	return enc([]byte(`{"alg":"none"}`)) + "." + enc(claims) + "."
}

type tokenServer struct {
	*httptest.Server
	refuse atomic.Bool
	n      atomic.Int32
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
			_, _ = w.Write([]byte(`{"access_token":"access-1","refresh_token":"refresh-1","token_type":"Bearer","expires_in":60,"id_token":"` + idToken(email) + `"}`))
		case "refresh_token":
			if ts.refuse.Load() {
				w.WriteHeader(http.StatusBadRequest)
				_, _ = w.Write([]byte(`{"error":"invalid_grant"}`))
				return
			}
			_, _ = w.Write([]byte(`{"access_token":"access-2","token_type":"Bearer","expires_in":3600}`))
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

	// A refresh keeps the preset, the mailbox and the login.
	tok, err := o.Token(ctx, scope.Tenant.ID, smtpConn.ID)
	require.NoError(t, err)
	assert.Equal(t, "access-2", tok)
	_, sc, err = store.Get(ctx, scope.Tenant.ID, smtpConn.ID)
	require.NoError(t, err)
	assert.Equal(t, PresetGmail, sc["preset"])
	assert.Equal(t, mailbox, sc["username"])
	assert.NotEmpty(t, sc["login_id"])
	assert.Empty(t, sc["id_token"], "the identity token of the sign-in is not kept")
	got0, _, err := store.Get(ctx, scope.Tenant.ID, smtpConn.ID)
	require.NoError(t, err)
	assert.Equal(t, mailbox, got0.Identity.Display, "a refresh keeps the mailbox shown: it is not a secret")
	assert.Equal(t, "refresh-1", sc["refresh_token"])

	// The access is revoked in the account: the refresh is refused, and both connections need a new sign-in.
	_, ic, err = store.Get(ctx, scope.Tenant.ID, imapConn.ID)
	require.NoError(t, err)
	ts.refuse.Store(true)
	hub := hubOf
	hub.cfg.OAuth = o
	_, err = o.Token(ctx, scope.Tenant.ID, smtpConn.ID) // fresh: no refresh yet
	require.NoError(t, err)
	// Age the token so the next call refreshes.
	ic["expires_at"] = "2000-01-01T00:00:00Z"
	got, _, err := store.Get(ctx, scope.Tenant.ID, imapConn.ID)
	require.NoError(t, err)
	_, err = store.Save(ctx, got, ic)
	require.NoError(t, err)
	_, err = hub.accessToken(ctx, scope.Tenant.ID, imapConn.ID.String(), ic)
	require.ErrorIs(t, err, connectors.ErrNeedsReauth)
	for _, id := range []uuid.UUID{smtpConn.ID, imapConn.ID} {
		c, _, err := store.Get(ctx, scope.Tenant.ID, id)
		require.NoError(t, err)
		assert.Equal(t, connectors.StatusNeedsReauth, c.Status, "both connections of the sign-in")
	}
}
