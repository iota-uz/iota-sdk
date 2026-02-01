package persistence_test

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/iota-uz/iota-sdk/modules/oidc/domain/entities/authrequest"
	"github.com/iota-uz/iota-sdk/modules/oidc/domain/entities/client"
	"github.com/iota-uz/iota-sdk/modules/oidc/infrastructure/persistence"
)

func TestAuthRequestRepository_Create(t *testing.T) {
	t.Parallel()
	f := setupTest(t)

	clientRepo := persistence.NewClientRepository()
	authRequestRepo := persistence.NewAuthRequestRepository()

	// Create a client first
	testClient := client.New(
		"authreq-test-client",
		"Test Client",
		"web",
		[]string{"http://localhost:3000/callback"},
	)
	_, err := clientRepo.Create(f.Ctx, testClient)
	require.NoError(t, err)

	t.Run("Success", func(t *testing.T) {
		authReq := authrequest.New(
			"authreq-test-client",
			"http://localhost:3000/callback",
			[]string{"openid", "profile", "email"},
			"code",
		)

		err := authRequestRepo.Create(f.Ctx, authReq)
		require.NoError(t, err)

		// Retrieve and verify
		retrieved, err := authRequestRepo.GetByID(f.Ctx, authReq.ID())
		require.NoError(t, err)
		assert.Equal(t, authReq.ID(), retrieved.ID())
		assert.Equal(t, "authreq-test-client", retrieved.ClientID())
		assert.Equal(t, "http://localhost:3000/callback", retrieved.RedirectURI())
		assert.Equal(t, []string{"openid", "profile", "email"}, retrieved.Scopes())
		assert.Equal(t, "code", retrieved.ResponseType())
	})

	t.Run("WithOptionalFields", func(t *testing.T) {
		authReq := authrequest.New(
			"authreq-test-client",
			"http://localhost:3000/callback",
			[]string{"openid"},
			"code",
			authrequest.WithState("test-state-123"),
			authrequest.WithNonce("test-nonce-456"),
			authrequest.WithCodeChallenge("challenge-hash", "S256"),
		)

		err := authRequestRepo.Create(f.Ctx, authReq)
		require.NoError(t, err)

		retrieved, err := authRequestRepo.GetByID(f.Ctx, authReq.ID())
		require.NoError(t, err)
		assert.NotNil(t, retrieved.State())
		assert.Equal(t, "test-state-123", *retrieved.State())
		assert.NotNil(t, retrieved.Nonce())
		assert.Equal(t, "test-nonce-456", *retrieved.Nonce())
		assert.NotNil(t, retrieved.CodeChallenge())
		assert.Equal(t, "challenge-hash", *retrieved.CodeChallenge())
		assert.NotNil(t, retrieved.CodeChallengeMethod())
		assert.Equal(t, "S256", *retrieved.CodeChallengeMethod())
	})

	t.Run("WithCustomExpiration", func(t *testing.T) {
		customExpiry := time.Now().Add(10 * time.Minute)
		authReq := authrequest.New(
			"authreq-test-client",
			"http://localhost:3000/callback",
			[]string{"openid"},
			"code",
			authrequest.WithExpiresAt(customExpiry),
		)

		err := authRequestRepo.Create(f.Ctx, authReq)
		require.NoError(t, err)

		retrieved, err := authRequestRepo.GetByID(f.Ctx, authReq.ID())
		require.NoError(t, err)
		assert.WithinDuration(t, customExpiry, retrieved.ExpiresAt(), time.Second)
	})
}

func TestAuthRequestRepository_GetByID(t *testing.T) {
	t.Parallel()
	f := setupTest(t)

	clientRepo := persistence.NewClientRepository()
	authRequestRepo := persistence.NewAuthRequestRepository()

	// Create a client first
	testClient := client.New(
		"getbyid-test-client",
		"Test Client",
		"web",
		[]string{"http://localhost:3000/callback"},
	)
	_, err := clientRepo.Create(f.Ctx, testClient)
	require.NoError(t, err)

	t.Run("Success", func(t *testing.T) {
		authReq := authrequest.New(
			"getbyid-test-client",
			"http://localhost:3000/callback",
			[]string{"openid"},
			"code",
		)

		err := authRequestRepo.Create(f.Ctx, authReq)
		require.NoError(t, err)

		retrieved, err := authRequestRepo.GetByID(f.Ctx, authReq.ID())
		require.NoError(t, err)
		assert.Equal(t, authReq.ID(), retrieved.ID())
	})

	t.Run("NotFound", func(t *testing.T) {
		_, err := authRequestRepo.GetByID(f.Ctx, uuid.New())
		assert.Error(t, err)
	})
}

func TestAuthRequestRepository_Update(t *testing.T) {
	t.Parallel()
	f := setupTest(t)

	clientRepo := persistence.NewClientRepository()
	authRequestRepo := persistence.NewAuthRequestRepository()

	// Create a client first
	testClient := client.New(
		"update-test-client",
		"Test Client",
		"web",
		[]string{"http://localhost:3000/callback"},
	)
	_, err := clientRepo.Create(f.Ctx, testClient)
	require.NoError(t, err)

	t.Run("Success", func(t *testing.T) {
		authReq := authrequest.New(
			"update-test-client",
			"http://localhost:3000/callback",
			[]string{"openid"},
			"code",
		)

		err := authRequestRepo.Create(f.Ctx, authReq)
		require.NoError(t, err)

		// Complete authentication
		userID := 123
		tenantID := uuid.New()
		updatedReq := authReq.CompleteAuthentication(userID, tenantID)

		err = authRequestRepo.Update(f.Ctx, updatedReq)
		require.NoError(t, err)

		// Retrieve and verify
		retrieved, err := authRequestRepo.GetByID(f.Ctx, authReq.ID())
		require.NoError(t, err)
		assert.True(t, retrieved.IsAuthenticated())
		assert.NotNil(t, retrieved.UserID())
		assert.Equal(t, userID, *retrieved.UserID())
		assert.NotNil(t, retrieved.TenantID())
		assert.Equal(t, tenantID, *retrieved.TenantID())
		assert.NotNil(t, retrieved.AuthTime())
	})

	t.Run("NotFound", func(t *testing.T) {
		nonExistentReq := authrequest.New(
			"update-test-client",
			"http://localhost:3000/callback",
			[]string{"openid"},
			"code",
			authrequest.WithID(uuid.New()),
		)

		err := authRequestRepo.Update(f.Ctx, nonExistentReq)
		assert.Error(t, err)
	})
}

func TestAuthRequestRepository_Delete(t *testing.T) {
	t.Parallel()
	f := setupTest(t)

	clientRepo := persistence.NewClientRepository()
	authRequestRepo := persistence.NewAuthRequestRepository()

	// Create a client first
	testClient := client.New(
		"delete-test-client",
		"Test Client",
		"web",
		[]string{"http://localhost:3000/callback"},
	)
	_, err := clientRepo.Create(f.Ctx, testClient)
	require.NoError(t, err)

	t.Run("Success", func(t *testing.T) {
		authReq := authrequest.New(
			"delete-test-client",
			"http://localhost:3000/callback",
			[]string{"openid"},
			"code",
		)

		err := authRequestRepo.Create(f.Ctx, authReq)
		require.NoError(t, err)

		err = authRequestRepo.Delete(f.Ctx, authReq.ID())
		require.NoError(t, err)

		_, err = authRequestRepo.GetByID(f.Ctx, authReq.ID())
		assert.Error(t, err)
	})

	t.Run("NotFound", func(t *testing.T) {
		err := authRequestRepo.Delete(f.Ctx, uuid.New())
		assert.Error(t, err)
	})
}

func TestAuthRequestRepository_DeleteExpired(t *testing.T) {
	t.Parallel()
	f := setupTest(t)

	clientRepo := persistence.NewClientRepository()
	authRequestRepo := persistence.NewAuthRequestRepository()

	// Create a client first
	testClient := client.New(
		"expired-test-client",
		"Test Client",
		"web",
		[]string{"http://localhost:3000/callback"},
	)
	_, err := clientRepo.Create(f.Ctx, testClient)
	require.NoError(t, err)

	// Create an expired auth request
	expiredReq := authrequest.New(
		"expired-test-client",
		"http://localhost:3000/callback",
		[]string{"openid"},
		"code",
		authrequest.WithExpiresAt(time.Now().Add(-1*time.Hour)),
	)

	err = authRequestRepo.Create(f.Ctx, expiredReq)
	require.NoError(t, err)

	// Create a valid auth request
	validReq := authrequest.New(
		"expired-test-client",
		"http://localhost:3000/callback",
		[]string{"openid"},
		"code",
		authrequest.WithExpiresAt(time.Now().Add(1*time.Hour)),
	)

	err = authRequestRepo.Create(f.Ctx, validReq)
	require.NoError(t, err)

	// Delete expired
	err = authRequestRepo.DeleteExpired(f.Ctx)
	require.NoError(t, err)

	// Verify expired is gone
	_, err = authRequestRepo.GetByID(f.Ctx, expiredReq.ID())
	assert.Error(t, err)

	// Verify valid still exists
	retrieved, err := authRequestRepo.GetByID(f.Ctx, validReq.ID())
	require.NoError(t, err)
	assert.Equal(t, validReq.ID(), retrieved.ID())
}

func TestAuthRequestRepository_CompleteAuthentication(t *testing.T) {
	t.Parallel()
	f := setupTest(t)

	clientRepo := persistence.NewClientRepository()
	authRequestRepo := persistence.NewAuthRequestRepository()

	// Create a client first
	testClient := client.New(
		"complete-auth-client",
		"Test Client",
		"web",
		[]string{"http://localhost:3000/callback"},
	)
	_, err := clientRepo.Create(f.Ctx, testClient)
	require.NoError(t, err)

	t.Run("SuccessfulCompletion", func(t *testing.T) {
		authReq := authrequest.New(
			"complete-auth-client",
			"http://localhost:3000/callback",
			[]string{"openid", "profile"},
			"code",
			authrequest.WithState("state-123"),
			authrequest.WithNonce("nonce-456"),
		)

		err := authRequestRepo.Create(f.Ctx, authReq)
		require.NoError(t, err)

		// Initially not authenticated
		assert.False(t, authReq.IsAuthenticated())
		assert.Nil(t, authReq.UserID())
		assert.Nil(t, authReq.TenantID())

		// Complete authentication
		userID := 456
		tenantID := uuid.New()
		completedReq := authReq.CompleteAuthentication(userID, tenantID)

		err = authRequestRepo.Update(f.Ctx, completedReq)
		require.NoError(t, err)

		// Retrieve and verify
		retrieved, err := authRequestRepo.GetByID(f.Ctx, authReq.ID())
		require.NoError(t, err)
		assert.True(t, retrieved.IsAuthenticated())
		assert.Equal(t, userID, *retrieved.UserID())
		assert.Equal(t, tenantID, *retrieved.TenantID())
		assert.NotNil(t, retrieved.AuthTime())
	})
}

func TestAuthRequestRepository_IsExpired(t *testing.T) {
	t.Parallel()
	f := setupTest(t)

	clientRepo := persistence.NewClientRepository()
	authRequestRepo := persistence.NewAuthRequestRepository()

	// Create a client first
	testClient := client.New(
		"expiry-test-client",
		"Test Client",
		"web",
		[]string{"http://localhost:3000/callback"},
	)
	_, err := clientRepo.Create(f.Ctx, testClient)
	require.NoError(t, err)

	t.Run("NotExpired", func(t *testing.T) {
		authReq := authrequest.New(
			"expiry-test-client",
			"http://localhost:3000/callback",
			[]string{"openid"},
			"code",
			authrequest.WithExpiresAt(time.Now().Add(10*time.Minute)),
		)

		err := authRequestRepo.Create(f.Ctx, authReq)
		require.NoError(t, err)

		retrieved, err := authRequestRepo.GetByID(f.Ctx, authReq.ID())
		require.NoError(t, err)
		assert.False(t, retrieved.IsExpired())
	})

	t.Run("Expired", func(t *testing.T) {
		authReq := authrequest.New(
			"expiry-test-client",
			"http://localhost:3000/callback",
			[]string{"openid"},
			"code",
			authrequest.WithExpiresAt(time.Now().Add(-10*time.Minute)),
		)

		err := authRequestRepo.Create(f.Ctx, authReq)
		require.NoError(t, err)

		retrieved, err := authRequestRepo.GetByID(f.Ctx, authReq.ID())
		require.NoError(t, err)
		assert.True(t, retrieved.IsExpired())
	})
}

func TestAuthRequestRepository_PKCEFlow(t *testing.T) {
	t.Parallel()
	f := setupTest(t)

	clientRepo := persistence.NewClientRepository()
	authRequestRepo := persistence.NewAuthRequestRepository()

	// Create a client that requires PKCE
	testClient := client.New(
		"pkce-test-client",
		"PKCE Test Client",
		"web",
		[]string{"http://localhost:3000/callback"},
		client.WithRequirePKCE(true),
	)
	_, err := clientRepo.Create(f.Ctx, testClient)
	require.NoError(t, err)

	t.Run("WithPKCE", func(t *testing.T) {
		codeChallenge := "E9Melhoa2OwvFrEMTJguCHaoeK1t8URWbuGJSstw-cM"
		challengeMethod := "S256"

		authReq := authrequest.New(
			"pkce-test-client",
			"http://localhost:3000/callback",
			[]string{"openid"},
			"code",
			authrequest.WithCodeChallenge(codeChallenge, challengeMethod),
		)

		err := authRequestRepo.Create(f.Ctx, authReq)
		require.NoError(t, err)

		retrieved, err := authRequestRepo.GetByID(f.Ctx, authReq.ID())
		require.NoError(t, err)
		assert.NotNil(t, retrieved.CodeChallenge())
		assert.Equal(t, codeChallenge, *retrieved.CodeChallenge())
		assert.NotNil(t, retrieved.CodeChallengeMethod())
		assert.Equal(t, challengeMethod, *retrieved.CodeChallengeMethod())
	})
}
