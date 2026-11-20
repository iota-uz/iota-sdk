package controllers_test

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"

	"github.com/iota-uz/iota-sdk/modules"
	"github.com/iota-uz/iota-sdk/modules/oidc/domain/entities/authrequest"
	"github.com/iota-uz/iota-sdk/modules/oidc/domain/entities/client"
	oidcstorage "github.com/iota-uz/iota-sdk/modules/oidc/infrastructure/oidc"
	"github.com/iota-uz/iota-sdk/modules/oidc/infrastructure/persistence"
	"github.com/iota-uz/iota-sdk/modules/oidc/presentation/controllers"
	"github.com/iota-uz/iota-sdk/modules/oidc/services"
	"github.com/iota-uz/iota-sdk/pkg/config/stdconfig/httpconfig"
	"github.com/iota-uz/iota-sdk/pkg/config/stdconfig/oidcconfig"
	"github.com/iota-uz/iota-sdk/pkg/constants"
	"github.com/iota-uz/iota-sdk/pkg/itf"
)

func TestMain(m *testing.M) {
	if err := os.Chdir("../../../.."); err != nil {
		panic(err)
	}
	os.Exit(m.Run())
}

func TestOIDCCallbackCompletesStoredAuthorizationRequest(t *testing.T) {
	t.Parallel()

	env := itf.Setup(t, itf.WithComponents(modules.Components()...))
	clientRepo := persistence.NewClientRepository()
	authRequestRepo := persistence.NewAuthRequestRepository()
	callbackURL := "https://client.example.test/callback"
	testClient := client.New("callback-client", "Callback client", "web", []string{callbackURL})
	_, err := clientRepo.Create(env.Ctx, testClient)
	require.NoError(t, err)
	tenantID := uuid.New()
	_, err = env.Tx.Exec(env.Ctx, `INSERT INTO tenants (id, name) VALUES ($1, $2)`, tenantID, "OIDC callback tenant")
	require.NoError(t, err)
	var userID int
	err = env.Tx.QueryRow(
		env.Ctx,
		`INSERT INTO users (tenant_id, type, first_name, last_name, email, ui_language)
		 VALUES ($1, 'user', 'OIDC', 'Callback', $2, 'en')
		 RETURNING id`,
		tenantID,
		"oidc-callback-"+uuid.NewString()+"@example.test",
	).Scan(&userID)
	require.NoError(t, err)

	request := authrequest.New(
		testClient.ClientID(),
		callbackURL,
		[]string{"openid"},
		"code",
		authrequest.WithState("state-123"),
	).CompleteAuthentication(userID, tenantID)
	require.NoError(t, authRequestRepo.Create(env.Ctx, request))

	cryptoKey := base64.StdEncoding.EncodeToString(make([]byte, 32))
	storage := oidcstorage.NewStorage(
		clientRepo,
		authRequestRepo,
		persistence.NewTokenRepository(),
		nil,
		env.Pool,
		cryptoKey,
		"https://issuer.example.test/oidc",
		0,
		0,
	)
	controller := controllers.NewOIDCController(
		storage,
		&oidcconfig.Config{IssuerURL: "https://issuer.example.test/oidc", CryptoKey: cryptoKey},
		services.NewOIDCService(clientRepo, authRequestRepo),
		nil,
		nil,
		&httpconfig.Config{},
	)
	router := mux.NewRouter()
	controller.Register(router)

	recorder := httptest.NewRecorder()
	requestContext := context.WithValue(env.Ctx, constants.LoggerKey, logrus.New().WithField("test", true))
	httpRequest := httptest.NewRequest(
		http.MethodGet,
		"/oidc/authorize/callback?id="+url.QueryEscape(request.ID().String()),
		nil,
	).WithContext(requestContext)
	router.ServeHTTP(recorder, httpRequest)

	require.Equal(t, http.StatusFound, recorder.Code)
	location, err := url.Parse(recorder.Header().Get("Location"))
	require.NoError(t, err)
	require.Equal(t, callbackURL, location.Scheme+"://"+location.Host+location.Path)
	require.NotEmpty(t, location.Query().Get("code"))
	require.Equal(t, "state-123", location.Query().Get("state"))
}

// Falsely green if discovery advertises refresh tokens the token endpoint
// then refuses as unsupported_grant_type.
func TestOIDCDiscoveryAdvertisesRefreshTokenGrant(t *testing.T) {
	t.Parallel()

	env := itf.Setup(t, itf.WithComponents(modules.Components()...))
	cryptoKey := base64.StdEncoding.EncodeToString(make([]byte, 32))
	clientRepo := persistence.NewClientRepository()
	authRequestRepo := persistence.NewAuthRequestRepository()
	storage := oidcstorage.NewStorage(clientRepo, authRequestRepo, persistence.NewTokenRepository(), nil,
		env.Pool, cryptoKey, "https://issuer.example.test/oidc", 0, 0)
	controller := controllers.NewOIDCController(
		storage,
		&oidcconfig.Config{IssuerURL: "https://issuer.example.test/oidc", CryptoKey: cryptoKey},
		services.NewOIDCService(clientRepo, authRequestRepo),
		nil,
		nil,
		&httpconfig.Config{},
	)
	router := mux.NewRouter()
	controller.Register(router)

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/oidc/.well-known/openid-configuration", nil).WithContext(env.Ctx))
	require.Equal(t, http.StatusOK, recorder.Code)
	var discovery struct {
		GrantTypes []string `json:"grant_types_supported"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &discovery))
	require.Contains(t, discovery.GrantTypes, "refresh_token")

	recorder = httptest.NewRecorder()
	form := url.Values{"grant_type": {"refresh_token"}, "refresh_token": {"unknown"}, "client_id": {"nobody"}}
	tokenRequest := httptest.NewRequest(http.MethodPost, "/oidc/oauth/token", strings.NewReader(form.Encode())).WithContext(env.Ctx)
	tokenRequest.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	router.ServeHTTP(recorder, tokenRequest)
	require.NotContains(t, recorder.Body.String(), "unsupported_grant_type")
}
