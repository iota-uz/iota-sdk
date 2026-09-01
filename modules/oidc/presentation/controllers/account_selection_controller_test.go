package controllers_test

import (
	"context"
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"

	"github.com/iota-uz/iota-sdk/modules"
	coresession "github.com/iota-uz/iota-sdk/modules/core/domain/entities/session"
	coreservices "github.com/iota-uz/iota-sdk/modules/core/services"
	"github.com/iota-uz/iota-sdk/modules/oidc/domain/entities/authrequest"
	"github.com/iota-uz/iota-sdk/modules/oidc/domain/entities/client"
	oidcstorage "github.com/iota-uz/iota-sdk/modules/oidc/infrastructure/oidc"
	"github.com/iota-uz/iota-sdk/modules/oidc/infrastructure/persistence"
	"github.com/iota-uz/iota-sdk/modules/oidc/presentation/controllers"
	oidcservices "github.com/iota-uz/iota-sdk/modules/oidc/services"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/config/stdconfig/httpconfig"
	"github.com/iota-uz/iota-sdk/pkg/config/stdconfig/oidcconfig"
	"github.com/iota-uz/iota-sdk/pkg/constants"
	"github.com/iota-uz/iota-sdk/pkg/itf"
)

func TestOIDCAccountSelection_Scenarios(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		run  func(*testing.T)
	}{
		{name: "derives identity from browser session", run: testOIDCAccountSelectionDerivesIdentityFromBrowserSession},
		{name: "rejects expired authorization request", run: testOIDCAccountSelectionRejectsExpiredAuthorizationRequest},
	}
	for _, tt := range tests {
		t.Run(tt.name, tt.run)
	}
}

func newAccountSelectionRouter(
	env *itf.TestEnvironment,
	clientRepo client.Repository,
	authRequestRepo authrequest.Repository,
) *mux.Router {
	cryptoKey := base64.StdEncoding.EncodeToString(make([]byte, 32))
	storage := oidcstorage.NewStorage(
		clientRepo, authRequestRepo, persistence.NewTokenRepository(), nil, env.Pool,
		cryptoKey, "https://issuer.example.test/oidc", 0, 0,
	)
	controller := controllers.NewOIDCController(
		storage,
		&oidcconfig.Config{IssuerURL: "https://issuer.example.test/oidc", CryptoKey: cryptoKey},
		oidcservices.NewOIDCService(clientRepo, authRequestRepo),
		itf.GetService[coreservices.SessionService](env),
		itf.GetService[coreservices.BrowserSessionService](env),
		&httpconfig.Config{},
	)
	router := mux.NewRouter()
	controller.Register(router)
	return router
}

func testOIDCAccountSelectionDerivesIdentityFromBrowserSession(t *testing.T) {
	env := itf.Setup(t, itf.WithComponents(modules.Components()...))
	clientRepo := persistence.NewClientRepository()
	authRequestRepo := persistence.NewAuthRequestRepository()
	callbackURL := "https://client.example.test/callback"
	_, err := clientRepo.Create(env.Ctx, client.New("picker-client", "Picker client", "web", []string{callbackURL}))
	require.NoError(t, err)

	tenantID := uuid.New()
	_, err = env.Tx.Exec(env.Ctx, `INSERT INTO tenants (id, name) VALUES ($1, $2)`, tenantID, "Picker tenant")
	require.NoError(t, err)
	var userID uint
	err = env.Tx.QueryRow(env.Ctx, `
		INSERT INTO users (tenant_id, type, first_name, last_name, email, ui_language)
		VALUES ($1, 'user', 'Picker', 'User', $2, 'en') RETURNING id`,
		tenantID,
		"picker-"+uuid.NewString()+"@example.test",
	).Scan(&userID)
	require.NoError(t, err)

	coreSessionService := itf.GetService[coreservices.SessionService](env)
	browserSessions := itf.GetService[coreservices.BrowserSessionService](env)
	token := "oidc-picker-" + uuid.NewString()
	require.NoError(t, coreSessionService.Create(composables.WithTenantID(env.Ctx, tenantID), &coresession.CreateDTO{
		Token: token, UserID: userID, TenantID: tenantID, IP: "127.0.0.1", UserAgent: "controller-test",
	}))
	sess, err := coreSessionService.GetBrowserSessionByToken(env.Ctx, token)
	require.NoError(t, err)
	browserCookie, err := browserSessions.Add(env.Ctx, "", sess)
	require.NoError(t, err)

	request := authrequest.New(
		"picker-client",
		callbackURL,
		[]string{"openid"},
		"code",
		authrequest.WithState("preserved-state"),
		authrequest.WithNonce("preserved-nonce"),
		authrequest.WithCodeChallenge("challenge", "S256"),
	)
	require.NoError(t, authRequestRepo.Create(env.Ctx, request))

	router := newAccountSelectionRouter(env, clientRepo, authRequestRepo)

	form := url.Values{
		"SessionReference": {coreservices.BrowserSessionReference(token)},
		"user_id":          {"999999"},
		"tenant_id":        {uuid.NewString()},
	}
	recorder := httptest.NewRecorder()
	requestContext := context.WithValue(env.Ctx, constants.LoggerKey, logrus.New().WithField("test", true))
	httpRequest := httptest.NewRequest(
		http.MethodPost,
		"/oidc/authorize/select?auth_request="+request.ID().String(),
		strings.NewReader(form.Encode()),
	).WithContext(requestContext)
	httpRequest.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	httpRequest.AddCookie(browserCookie)
	router.ServeHTTP(recorder, httpRequest)

	require.Equal(t, http.StatusSeeOther, recorder.Code)
	require.Equal(t, "/oidc/authorize/callback?id="+request.ID().String(), recorder.Header().Get("Location"))
	completed, err := authRequestRepo.GetByID(env.Ctx, request.ID())
	require.NoError(t, err)
	require.Equal(t, int(userID), *completed.UserID())
	require.Equal(t, tenantID, *completed.TenantID())
	require.Equal(t, "preserved-state", *completed.State())
	require.Equal(t, "preserved-nonce", *completed.Nonce())
	require.Equal(t, "challenge", *completed.CodeChallenge())
}

func testOIDCAccountSelectionRejectsExpiredAuthorizationRequest(t *testing.T) {
	env := itf.Setup(t, itf.WithComponents(modules.Components()...))
	clientRepo := persistence.NewClientRepository()
	authRequestRepo := persistence.NewAuthRequestRepository()
	_, err := clientRepo.Create(env.Ctx, client.New(
		"expired-picker-client", "Expired picker client", "web", []string{"https://client.example.test/callback"},
	))
	require.NoError(t, err)
	request := authrequest.New(
		"expired-picker-client",
		"https://client.example.test/callback",
		[]string{"openid"},
		"code",
		authrequest.WithExpiresAt(time.Now().Add(-time.Minute)),
	)
	require.NoError(t, authRequestRepo.Create(env.Ctx, request))

	router := newAccountSelectionRouter(env, clientRepo, authRequestRepo)

	recorder := httptest.NewRecorder()
	httpRequest := httptest.NewRequest(
		http.MethodPost,
		"/oidc/authorize/select?auth_request="+request.ID().String(),
		strings.NewReader(url.Values{"SessionReference": {"untrusted"}}.Encode()),
	).WithContext(context.WithValue(env.Ctx, constants.LoggerKey, logrus.New().WithField("test", true)))
	httpRequest.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	router.ServeHTTP(recorder, httpRequest)

	require.Equal(t, http.StatusBadRequest, recorder.Code)
	require.Contains(t, recorder.Body.String(), "start again")
}
