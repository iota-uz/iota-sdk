package services_test

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/iota-uz/iota-sdk/modules/automations/infrastructure/persistence"
	"github.com/iota-uz/iota-sdk/modules/automations/services"
	"github.com/iota-uz/iota-sdk/pkg/automation/catalog"
	"github.com/iota-uz/iota-sdk/pkg/automation/connectors"
	"github.com/iota-uz/iota-sdk/pkg/automation/connectors/email"
	"github.com/iota-uz/iota-sdk/pkg/automation/connectors/email/emailtest"
	"github.com/iota-uz/iota-sdk/pkg/itf"
)

const signInMailbox = "robot@example.com"

type lateOAuth struct{ connectors.OAuth2 }

// The tenant entry point of a sign-in with Google: the preset is offered when the application has the
// provider's OAuth2 client, a start with it gives the provider's URL, and one callback saves the pair of
// connections — a smtp to send and an imap to receive — together, the second taking its tokens from the
// first.
func TestEmailSignInWithGoogleSavesBothConnectionsOnOneCallback(t *testing.T) {
	h := itf.NewHarness(t, itf.HarnessConfig{
		Name:      "automations_email_signin",
		Database:  itf.DatabaseConfig{Provisioning: itf.ProvisioningPerTestDatabase},
		Isolation: itf.IsolationConfig{Mode: itf.IsolationCommitted},
	})
	scope := h.Scope(t)
	ctx := context.Background()
	var user int64
	require.NoError(t, scope.Pool.QueryRow(ctx, `INSERT INTO users (tenant_id, type, first_name, last_name, email, ui_language)
		VALUES ($1, 'user', 'Ann', 'Admin', $2, 'en') RETURNING id`, scope.Tenant.ID, "admin-"+uuid.NewString()+"@example.com").Scan(&user))
	actor := services.Actor{TenantID: scope.Tenant.ID, UserID: user}

	mail := emailtest.New(t, emailtest.ModeTLS, emailtest.Account{User: signInMailbox, Password: "pw"})
	mail.AcceptToken(signInMailbox, "access-1")
	var issuer atomic.Value // the issuer of the ID token the provider gives
	issuer.Store("https://accounts.google.com")
	var address atomic.Value // the mailbox the ID token is for
	address.Store(signInMailbox)
	var err error
	idToken := func() string {
		claims, err := json.Marshal(map[string]any{"iss": issuer.Load(), "email": address.Load(), "aud": "cid", "exp": time.Now().Add(time.Hour).Unix()})
		require.NoError(t, err)
		return base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"none"}`)) + "." + base64.RawURLEncoding.EncodeToString(claims) + "."
	}
	token := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"access_token":"access-1","refresh_token":"refresh-1","token_type":"Bearer","expires_in":3600,"id_token":"` + idToken() + `"}`))
	}))
	t.Cleanup(token.Close)
	_, smtpPort := emailtest.HostPort(mail.SMTPAddr)
	_, imapPort := emailtest.HostPort(mail.IMAPAddr)
	atoi := func(s string) int { n, _ := strconv.Atoi(s); return n }
	late := &lateOAuth{}
	cfg := email.Config{
		OAuth: late, TLSConfig: mail.TLS,
		Providers: map[string]email.Provider{
			email.PresetGmail: {
				AuthURL: token.URL + "/auth", TokenURL: token.URL + "/token",
				SMTPHost: "127.0.0.1", SMTPPort: atoi(smtpPort), IMAPHost: "127.0.0.1", IMAPPort: atoi(imapPort), Security: "tls",
			},
			email.PresetM365: {
				AuthURL: token.URL + "/auth", TokenURL: token.URL + "/token",
				SMTPHost: "127.0.0.1", SMTPPort: atoi(smtpPort), IMAPHost: "127.0.0.1", IMAPPort: atoi(imapPort), Security: "tls",
			},
		},
	}

	build := func(withClient bool, extra ...catalog.ConnectorKind) *services.IntegrationsService {
		clients := map[catalog.ConnectorKind]connectors.PlatformCredential{}
		if withClient {
			clients[email.GoogleClient] = connectors.PlatformCredential{Credentials: connectors.Credentials{"client_id": "cid", "client_secret": "csecret"}}
		}
		for _, k := range extra {
			clients[k] = connectors.PlatformCredential{Credentials: connectors.Credentials{"client_id": "cid", "client_secret": "csecret"}}
		}
		keys, err := connectors.NewKeyring("k1", map[string][]byte{"k1": bytes.Repeat([]byte{7}, 32)})
		require.NoError(t, err)
		dir := connectors.NewDirectory(nil)
		for _, c := range email.New(cfg) {
			require.NoError(t, dir.Register(c))
		}
		platform := connectors.NewPlatformCredentials(dir, clients)
		store := connectors.NewStore(scope.Pool, keys, nil, platform, nil)
		oauth, err := connectors.NewOAuth2(store, dir, platform, connectors.WithHTTPClient(token.Client()))
		require.NoError(t, err)
		late.OAuth2 = oauth
		return services.NewIntegrationsService(services.IntegrationsDeps{
			Directory: dir, Store: store, Platform: platform, OAuth: oauth,
			States: persistence.NewOAuthStates(scope.Pool), Checks: persistence.NewConnectionChecks(scope.Pool), Requests: persistence.NewRequests(scope.Pool),
		})
	}
	presets := func(svc *services.IntegrationsService) map[catalog.ConnectorKind][]string {
		list, err := svc.List(ctx, actor, services.ListOptions{})
		require.NoError(t, err)
		out := map[catalog.ConnectorKind][]string{}
		for _, e := range list {
			out[e.Connector.Kind()] = e.OAuthPresets
		}
		return out
	}

	// Without the provider's OAuth2 client the preset is not offered and cannot be started.
	svc := build(false)
	assert.Empty(t, presets(svc)[email.KindSMTP])
	_, err = svc.OAuthStartPreset(ctx, actor, email.KindSMTP, email.PresetGmail, "mail", "", uuid.Nil, "https://app.test/cb")
	require.ErrorIs(t, err, services.ErrOAuthUnavailable)

	svc = build(true)
	assert.Equal(t, []string{email.PresetGmail}, presets(svc)[email.KindSMTP], "m365 has no client here")
	assert.Equal(t, []string{email.PresetGmail}, presets(svc)[email.KindIMAP])

	var invalid *services.InvalidInputError
	_, err = svc.OAuthStartPreset(ctx, actor, email.KindSMTP, "yahoo", "mail", "", uuid.Nil, "https://app.test/cb")
	require.ErrorAs(t, err, &invalid, "an unknown preset")
	_, err = svc.OAuthStart(ctx, actor, email.KindSMTP, "mail", "", uuid.Nil, "https://app.test/cb")
	require.ErrorAs(t, err, &invalid, "the form kind is not a sign-in without a preset")

	authURL, err := svc.OAuthStartPreset(ctx, actor, email.KindSMTP, email.PresetGmail, "mail", "", uuid.Nil, "https://app.test/cb")
	require.NoError(t, err)
	u, err := url.Parse(authURL)
	require.NoError(t, err)
	assert.Equal(t, "consent", u.Query().Get("prompt"))
	assert.Equal(t, "cid", u.Query().Get("client_id"))
	state := u.Query().Get("state")
	require.NotEmpty(t, state)

	saved, pending, err := svc.OAuthComplete(ctx, actor, state, "code")
	require.NoError(t, err)
	assert.Equal(t, email.KindSMTP, pending.Kind)
	assert.Equal(t, email.KindSMTP, saved.Kind)
	assert.Equal(t, signInMailbox, saved.Identity.Display, "the mailbox is shown: it is not a secret")

	keys, _ := connectors.NewKeyring("k1", map[string][]byte{"k1": bytes.Repeat([]byte{7}, 32)})
	store := connectors.NewStore(scope.Pool, keys, nil, nil, nil)
	smtps, err := store.List(ctx, actor.TenantID, email.KindSMTP)
	require.NoError(t, err)
	imaps, err := store.List(ctx, actor.TenantID, email.KindIMAP)
	require.NoError(t, err)
	require.Len(t, smtps, 1)
	require.Len(t, imaps, 1, "one callback made both connections")
	assert.Equal(t, connectors.StatusConnected, imaps[0].Status)
	_, sc, err := store.Get(ctx, actor.TenantID, smtps[0].ID)
	require.NoError(t, err)
	_, ic, err := store.Get(ctx, actor.TenantID, imaps[0].ID)
	require.NoError(t, err)
	// What the callback persisted, before any refresh: the token set the provider returned (the refresh token
	// among it) on the primary and the same set on the second, and the ID token on neither.
	assert.Equal(t, "access-1", sc[connectors.OAuth2AccessToken])
	assert.Equal(t, "refresh-1", sc[connectors.OAuth2RefreshToken])
	assert.Equal(t, "access-1", ic[connectors.OAuth2AccessToken])
	assert.Equal(t, "refresh-1", ic[connectors.OAuth2RefreshToken])
	assert.NotContains(t, sc, connectors.OAuth2IDToken)
	assert.NotContains(t, ic, connectors.OAuth2IDToken)
	assert.Equal(t, sc["login_id"], ic["login_id"])
	assert.NotEmpty(t, sc["login_id"])
	assert.Empty(t, sc[connectors.OAuth2TokenFrom], "the first holds the token set")
	assert.Equal(t, smtps[0].ID.String(), ic[connectors.OAuth2TokenFrom], "the second takes its tokens from it")
	assert.Equal(t, signInMailbox, ic["username"])

	// The state is spent: a second callback with it is refused and saves nothing.
	_, _, err = svc.OAuthComplete(ctx, actor, state, "code")
	require.ErrorIs(t, err, services.ErrStateInvalid)
	imaps, _ = store.List(ctx, actor.TenantID, email.KindIMAP)
	assert.Len(t, imaps, 1)

	// Signing in again from the sending connection keeps the folder the receiving one reads.
	imapConn, ic2, err := store.Get(ctx, actor.TenantID, imaps[0].ID)
	require.NoError(t, err)
	ic2["mailbox"] = "Support"
	_, err = store.Save(ctx, imapConn, ic2)
	require.NoError(t, err)
	reauth := func(svc *services.IntegrationsService, preset string) error {
		authURL, err := svc.OAuthStartPreset(ctx, actor, email.KindSMTP, preset, "mail", "", smtps[0].ID, "https://app.test/cb")
		require.NoError(t, err)
		u, err := url.Parse(authURL)
		require.NoError(t, err)
		_, _, err = svc.OAuthComplete(ctx, actor, u.Query().Get("state"), "code")
		return err
	}
	require.NoError(t, reauth(svc, email.PresetGmail))
	_, got, err := store.Get(ctx, actor.TenantID, imaps[0].ID)
	require.NoError(t, err)
	assert.Equal(t, "Support", got["mailbox"], "the folder stays when the pair is signed in again")
	assert.Equal(t, "refresh-1", got["refresh_token"])

	// A sign-in with another provider for the same address does not replace the pair's grant: the receiving
	// connection would stay on Gmail's servers with a Microsoft token.
	both := build(true, email.MicrosoftClient)
	issuer.Store("https://login.microsoftonline.com/tid/v2.0")
	require.ErrorIs(t, reauth(both, email.PresetM365), services.ErrOAuthRefused)
	_, still, err := store.Get(ctx, actor.TenantID, smtps[0].ID)
	require.NoError(t, err)
	assert.Equal(t, email.PresetGmail, still[connectors.OAuth2Preset], "the pair is still Gmail's")

	// The page's «Sign in with Google» on a card names no connection: the same account signs its pair in again,
	// another account makes a pair of its own under free names — never a collision with the first.
	issuer.Store("https://accounts.google.com")
	fromCard := func() connectors.Connection {
		authURL, err := svc.OAuthStartPreset(ctx, actor, email.KindIMAP, email.PresetGmail, "", "", uuid.Nil, "https://app.test/cb")
		require.NoError(t, err)
		u, err := url.Parse(authURL)
		require.NoError(t, err)
		saved, _, err := svc.OAuthComplete(ctx, actor, u.Query().Get("state"), "code")
		require.NoError(t, err)
		return saved
	}
	again := fromCard()
	assert.Equal(t, imaps[0].ID, again.ID, "the same account's receiving connection is signed in again")
	imaps, _ = store.List(ctx, actor.TenantID, email.KindIMAP)
	assert.Len(t, imaps, 1)

	const second = "second@example.com"
	mail.AddAccount(emailtest.Account{User: second, Password: "pw"})
	mail.AcceptToken(second, "access-1")
	address.Store(second)
	other := fromCard()
	assert.NotEqual(t, imaps[0].ID, other.ID)
	assert.Equal(t, second, other.Identity.Display)
	smtps, _ = store.List(ctx, actor.TenantID, email.KindSMTP)
	imaps, _ = store.List(ctx, actor.TenantID, email.KindIMAP)
	require.Len(t, smtps, 2, "a pair of its own")
	require.Len(t, imaps, 2)
	assert.NotEqual(t, imaps[0].Name, imaps[1].Name)
	assert.NotEqual(t, smtps[0].Name, smtps[1].Name)
}
