package services_test

import (
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/iota-uz/iota-sdk/modules"
	"github.com/iota-uz/iota-sdk/modules/oidc/domain/entities/authrequest"
	"github.com/iota-uz/iota-sdk/modules/oidc/domain/entities/client"
	"github.com/iota-uz/iota-sdk/modules/oidc/infrastructure/persistence"
	"github.com/iota-uz/iota-sdk/modules/oidc/services"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/itf"
)

func TestMain(m *testing.M) {
	if err := os.Chdir("../../../"); err != nil {
		panic(err)
	}
	os.Exit(m.Run())
}

func createOIDCTestTenantAndUser(t *testing.T, env *itf.TestEnvironment, userID int) uuid.UUID {
	t.Helper()

	tx, err := composables.UseTx(env.Ctx)
	require.NoError(t, err)

	tenantID := uuid.New()
	tenantName := fmt.Sprintf("oidc-service-tenant-%s", tenantID.String())

	_, err = tx.Exec(env.Ctx, `INSERT INTO tenants (id, name) VALUES ($1, $2)`, tenantID, tenantName)
	require.NoError(t, err)

	email := fmt.Sprintf("oidc-service-user-%d-%s@example.com", userID, tenantID.String()[:8])
	_, err = tx.Exec(
		env.Ctx,
		`INSERT INTO users (id, tenant_id, type, first_name, last_name, email, ui_language)
		 VALUES ($1, $2, 'user', 'OIDC', 'Service', $3, 'en')
		 ON CONFLICT (id) DO NOTHING`,
		userID,
		tenantID,
		email,
	)
	require.NoError(t, err)

	return tenantID
}

func TestOIDCService_CompleteAuthRequest(t *testing.T) {
	t.Parallel()

	// Setup test environment
	env := itf.Setup(t, itf.WithModules(modules.BuiltInModules...))

	// Create repositories
	clientRepo := persistence.NewClientRepository()
	authRequestRepo := persistence.NewAuthRequestRepository()

	// Create service
	oidcService := services.NewOIDCService(clientRepo, authRequestRepo)

	// Create a test client first
	testClient := client.New(
		"test-client-id",
		"Test Client",
		"web",
		[]string{"http://localhost:3000/callback"},
	)
	_, err := clientRepo.Create(env.Ctx, testClient)
	require.NoError(t, err)

	// Create a test auth request
	testAuthReq := authrequest.New(
		testClient.ClientID(),
		"http://localhost:3000/callback",
		[]string{"openid", "profile", "email"},
		"code",
		authrequest.WithState("test-state"),
		authrequest.WithNonce("test-nonce"),
	)
	err = authRequestRepo.Create(env.Ctx, testAuthReq)
	require.NoError(t, err)

	// Complete the auth request
	userID := 1
	tenantID := createOIDCTestTenantAndUser(t, env, userID)
	err = oidcService.CompleteAuthRequest(env.Ctx, testAuthReq.ID().String(), userID, tenantID)
	require.NoError(t, err)

	// Verify auth request was updated
	updatedAuthReq, err := authRequestRepo.GetByID(env.Ctx, testAuthReq.ID())
	require.NoError(t, err)
	require.True(t, updatedAuthReq.IsAuthenticated())
	require.NotNil(t, updatedAuthReq.UserID())
	require.Equal(t, userID, *updatedAuthReq.UserID())
	require.NotNil(t, updatedAuthReq.TenantID())
	require.Equal(t, tenantID, *updatedAuthReq.TenantID())
}

func TestOIDCService_CompleteAuthRequest_Expired(t *testing.T) {
	t.Parallel()

	// Setup test environment
	env := itf.Setup(t, itf.WithModules(modules.BuiltInModules...))

	// Create repositories
	clientRepo := persistence.NewClientRepository()
	authRequestRepo := persistence.NewAuthRequestRepository()

	// Create service
	oidcService := services.NewOIDCService(clientRepo, authRequestRepo)

	// Create a test client first
	testClient := client.New(
		"test-client-id-2",
		"Test Client 2",
		"web",
		[]string{"http://localhost:3000/callback"},
	)
	_, err := clientRepo.Create(env.Ctx, testClient)
	require.NoError(t, err)

	// Create an expired auth request
	testAuthReq := authrequest.New(
		testClient.ClientID(),
		"http://localhost:3000/callback",
		[]string{"openid", "profile", "email"},
		"code",
		authrequest.WithExpiresAt(time.Now().Add(-1*time.Hour)), // Expired 1 hour ago
	)
	err = authRequestRepo.Create(env.Ctx, testAuthReq)
	require.NoError(t, err)

	// Attempt to complete the expired auth request
	userID := 1
	tenantID := createOIDCTestTenantAndUser(t, env, userID)
	err = oidcService.CompleteAuthRequest(env.Ctx, testAuthReq.ID().String(), userID, tenantID)
	require.Error(t, err)
	require.Contains(t, err.Error(), "expired")
}

func TestOIDCService_GetAuthRequest(t *testing.T) {
	t.Parallel()

	// Setup test environment
	env := itf.Setup(t, itf.WithModules(modules.BuiltInModules...))

	// Create repositories
	clientRepo := persistence.NewClientRepository()
	authRequestRepo := persistence.NewAuthRequestRepository()

	// Create service
	oidcService := services.NewOIDCService(clientRepo, authRequestRepo)

	// Create a test client first
	testClient := client.New(
		"test-client-id-3",
		"Test Client 3",
		"web",
		[]string{"http://localhost:3000/callback"},
	)
	_, err := clientRepo.Create(env.Ctx, testClient)
	require.NoError(t, err)

	// Create a test auth request
	testAuthReq := authrequest.New(
		testClient.ClientID(),
		"http://localhost:3000/callback",
		[]string{"openid", "profile", "email"},
		"code",
	)
	err = authRequestRepo.Create(env.Ctx, testAuthReq)
	require.NoError(t, err)

	// Get the auth request
	retrievedAuthReq, err := oidcService.GetAuthRequest(env.Ctx, testAuthReq.ID().String())
	require.NoError(t, err)
	require.NotNil(t, retrievedAuthReq)
	require.Equal(t, testAuthReq.ID(), retrievedAuthReq.ID())
	require.Equal(t, testAuthReq.ClientID(), retrievedAuthReq.ClientID())
	require.Equal(t, testAuthReq.RedirectURI(), retrievedAuthReq.RedirectURI())
}

func TestOIDCService_GetAuthRequest_Invalid(t *testing.T) {
	t.Parallel()

	// Setup test environment
	env := itf.Setup(t, itf.WithModules(modules.BuiltInModules...))

	// Create repositories
	clientRepo := persistence.NewClientRepository()
	authRequestRepo := persistence.NewAuthRequestRepository()

	// Create service
	oidcService := services.NewOIDCService(clientRepo, authRequestRepo)

	// Attempt to get a non-existent auth request
	_, err := oidcService.GetAuthRequest(env.Ctx, uuid.New().String())
	require.Error(t, err)
}
