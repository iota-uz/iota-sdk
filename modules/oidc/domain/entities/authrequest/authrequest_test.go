package authrequest_test

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"

	"github.com/iota-uz/iota-sdk/modules/oidc/domain/entities/authrequest"
)

func TestAuthRequest_New(t *testing.T) {
	t.Run("WithRequiredFieldsOnly", func(t *testing.T) {
		ar := authrequest.New(
			"test-client",
			"http://localhost:3000/callback",
			[]string{"openid", "profile"},
			"code",
		)

		assert.NotEqual(t, uuid.Nil, ar.ID())
		assert.Equal(t, "test-client", ar.ClientID())
		assert.Equal(t, "http://localhost:3000/callback", ar.RedirectURI())
		assert.Equal(t, []string{"openid", "profile"}, ar.Scopes())
		assert.Equal(t, "code", ar.ResponseType())
		assert.Nil(t, ar.State())
		assert.Nil(t, ar.Nonce())
		assert.Nil(t, ar.CodeChallenge())
		assert.Nil(t, ar.CodeChallengeMethod())
		assert.Nil(t, ar.UserID())
		assert.Nil(t, ar.TenantID())
		assert.Nil(t, ar.AuthTime())
		assert.NotNil(t, ar.CreatedAt())
		assert.NotNil(t, ar.ExpiresAt())
		assert.False(t, ar.IsAuthenticated())
	})

	t.Run("WithAllOptions", func(t *testing.T) {
		customID := uuid.New()
		customExpiry := time.Now().Add(10 * time.Minute)
		userID := 123
		tenantID := uuid.New()
		authTime := time.Now()

		ar := authrequest.New(
			"custom-client",
			"http://localhost:4000/callback",
			[]string{"openid", "profile", "email"},
			"code",
			authrequest.WithID(customID),
			authrequest.WithState("state-123"),
			authrequest.WithNonce("nonce-456"),
			authrequest.WithCodeChallenge("challenge-hash", "S256"),
			authrequest.WithUserID(userID),
			authrequest.WithTenantID(tenantID),
			authrequest.WithAuthTime(authTime),
			authrequest.WithExpiresAt(customExpiry),
		)

		assert.Equal(t, customID, ar.ID())
		assert.NotNil(t, ar.State())
		assert.Equal(t, "state-123", *ar.State())
		assert.NotNil(t, ar.Nonce())
		assert.Equal(t, "nonce-456", *ar.Nonce())
		assert.NotNil(t, ar.CodeChallenge())
		assert.Equal(t, "challenge-hash", *ar.CodeChallenge())
		assert.NotNil(t, ar.CodeChallengeMethod())
		assert.Equal(t, "S256", *ar.CodeChallengeMethod())
		assert.NotNil(t, ar.UserID())
		assert.Equal(t, userID, *ar.UserID())
		assert.NotNil(t, ar.TenantID())
		assert.Equal(t, tenantID, *ar.TenantID())
		assert.NotNil(t, ar.AuthTime())
		assert.WithinDuration(t, authTime, *ar.AuthTime(), time.Second)
		assert.WithinDuration(t, customExpiry, ar.ExpiresAt(), time.Second)
	})

	t.Run("DefaultExpirationTime", func(t *testing.T) {
		now := time.Now()
		ar := authrequest.New(
			"test-client",
			"http://localhost:3000/callback",
			[]string{"openid"},
			"code",
		)

		expectedExpiry := now.Add(5 * time.Minute)
		assert.WithinDuration(t, expectedExpiry, ar.ExpiresAt(), 2*time.Second)
	})
}

func TestAuthRequest_SetState(t *testing.T) {
	original := authrequest.New(
		"test-client",
		"http://localhost:3000/callback",
		[]string{"openid"},
		"code",
	)

	assert.Nil(t, original.State())

	// Set state (immutable pattern)
	updated := original.SetState("new-state-789")

	// Verify new instance has the state
	assert.NotNil(t, updated.State())
	assert.Equal(t, "new-state-789", *updated.State())

	// Verify original is unchanged
	assert.Nil(t, original.State())
}

func TestAuthRequest_SetNonce(t *testing.T) {
	original := authrequest.New(
		"test-client",
		"http://localhost:3000/callback",
		[]string{"openid"},
		"code",
	)

	assert.Nil(t, original.Nonce())

	// Set nonce (immutable pattern)
	updated := original.SetNonce("new-nonce-xyz")

	// Verify new instance has the nonce
	assert.NotNil(t, updated.Nonce())
	assert.Equal(t, "new-nonce-xyz", *updated.Nonce())

	// Verify original is unchanged
	assert.Nil(t, original.Nonce())
}

func TestAuthRequest_SetPKCE(t *testing.T) {
	original := authrequest.New(
		"test-client",
		"http://localhost:3000/callback",
		[]string{"openid"},
		"code",
	)

	assert.Nil(t, original.CodeChallenge())
	assert.Nil(t, original.CodeChallengeMethod())

	// Set PKCE parameters (immutable pattern)
	updated := original.SetPKCE("E9Melhoa2OwvFrEMTJguCHaoeK1t8URWbuGJSstw-cM", "S256")

	// Verify new instance has PKCE parameters
	assert.NotNil(t, updated.CodeChallenge())
	assert.Equal(t, "E9Melhoa2OwvFrEMTJguCHaoeK1t8URWbuGJSstw-cM", *updated.CodeChallenge())
	assert.NotNil(t, updated.CodeChallengeMethod())
	assert.Equal(t, "S256", *updated.CodeChallengeMethod())

	// Verify original is unchanged
	assert.Nil(t, original.CodeChallenge())
	assert.Nil(t, original.CodeChallengeMethod())
}

func TestAuthRequest_CompleteAuthentication(t *testing.T) {
	original := authrequest.New(
		"test-client",
		"http://localhost:3000/callback",
		[]string{"openid", "profile"},
		"code",
		authrequest.WithState("state-123"),
		authrequest.WithNonce("nonce-456"),
	)

	assert.False(t, original.IsAuthenticated())
	assert.Nil(t, original.UserID())
	assert.Nil(t, original.TenantID())
	assert.Nil(t, original.AuthTime())

	// Complete authentication (immutable pattern)
	userID := 789
	tenantID := uuid.New()
	beforeCompletion := time.Now()

	completed := original.CompleteAuthentication(userID, tenantID)

	// Verify new instance is authenticated
	assert.True(t, completed.IsAuthenticated())
	assert.NotNil(t, completed.UserID())
	assert.Equal(t, userID, *completed.UserID())
	assert.NotNil(t, completed.TenantID())
	assert.Equal(t, tenantID, *completed.TenantID())
	assert.NotNil(t, completed.AuthTime())
	assert.True(t, completed.AuthTime().After(beforeCompletion) || completed.AuthTime().Equal(beforeCompletion))

	// Verify original is unchanged
	assert.False(t, original.IsAuthenticated())
	assert.Nil(t, original.UserID())
	assert.Nil(t, original.TenantID())
	assert.Nil(t, original.AuthTime())
}

func TestAuthRequest_IsExpired(t *testing.T) {
	t.Run("NotExpired", func(t *testing.T) {
		ar := authrequest.New(
			"test-client",
			"http://localhost:3000/callback",
			[]string{"openid"},
			"code",
			authrequest.WithExpiresAt(time.Now().Add(10*time.Minute)),
		)

		assert.False(t, ar.IsExpired())
	})

	t.Run("Expired", func(t *testing.T) {
		ar := authrequest.New(
			"test-client",
			"http://localhost:3000/callback",
			[]string{"openid"},
			"code",
			authrequest.WithExpiresAt(time.Now().Add(-10*time.Minute)),
		)

		assert.True(t, ar.IsExpired())
	})

	t.Run("JustExpired", func(t *testing.T) {
		ar := authrequest.New(
			"test-client",
			"http://localhost:3000/callback",
			[]string{"openid"},
			"code",
			authrequest.WithExpiresAt(time.Now().Add(-1*time.Second)),
		)

		assert.True(t, ar.IsExpired())
	})
}

func TestAuthRequest_IsAuthenticated(t *testing.T) {
	t.Run("NotAuthenticated", func(t *testing.T) {
		ar := authrequest.New(
			"test-client",
			"http://localhost:3000/callback",
			[]string{"openid"},
			"code",
		)

		assert.False(t, ar.IsAuthenticated())
	})

	t.Run("AuthenticatedWithUserIDOnly", func(t *testing.T) {
		// Having only UserID is not sufficient
		ar := authrequest.New(
			"test-client",
			"http://localhost:3000/callback",
			[]string{"openid"},
			"code",
			authrequest.WithUserID(123),
		)

		assert.False(t, ar.IsAuthenticated()) // Requires both UserID and TenantID
	})

	t.Run("AuthenticatedWithTenantIDOnly", func(t *testing.T) {
		// Having only TenantID is not sufficient
		ar := authrequest.New(
			"test-client",
			"http://localhost:3000/callback",
			[]string{"openid"},
			"code",
			authrequest.WithTenantID(uuid.New()),
		)

		assert.False(t, ar.IsAuthenticated()) // Requires both UserID and TenantID
	})

	t.Run("AuthenticatedWithBoth", func(t *testing.T) {
		ar := authrequest.New(
			"test-client",
			"http://localhost:3000/callback",
			[]string{"openid"},
			"code",
			authrequest.WithUserID(123),
			authrequest.WithTenantID(uuid.New()),
		)

		assert.True(t, ar.IsAuthenticated())
	})

	t.Run("AuthenticatedViaCompleteAuthentication", func(t *testing.T) {
		ar := authrequest.New(
			"test-client",
			"http://localhost:3000/callback",
			[]string{"openid"},
			"code",
		)

		completed := ar.CompleteAuthentication(456, uuid.New())

		assert.True(t, completed.IsAuthenticated())
	})
}

func TestAuthRequest_ImmutabilityPattern(t *testing.T) {
	original := authrequest.New(
		"test-client",
		"http://localhost:3000/callback",
		[]string{"openid"},
		"code",
	)

	// Chain multiple immutable operations
	updated := original.
		SetState("state-abc").
		SetNonce("nonce-def").
		SetPKCE("challenge-xyz", "S256").
		CompleteAuthentication(999, uuid.New())

	// Verify original remains completely unchanged
	assert.Nil(t, original.State())
	assert.Nil(t, original.Nonce())
	assert.Nil(t, original.CodeChallenge())
	assert.Nil(t, original.CodeChallengeMethod())
	assert.False(t, original.IsAuthenticated())
	assert.Nil(t, original.UserID())
	assert.Nil(t, original.TenantID())
	assert.Nil(t, original.AuthTime())

	// Verify updated has all changes
	assert.NotNil(t, updated.State())
	assert.Equal(t, "state-abc", *updated.State())
	assert.NotNil(t, updated.Nonce())
	assert.Equal(t, "nonce-def", *updated.Nonce())
	assert.NotNil(t, updated.CodeChallenge())
	assert.Equal(t, "challenge-xyz", *updated.CodeChallenge())
	assert.NotNil(t, updated.CodeChallengeMethod())
	assert.Equal(t, "S256", *updated.CodeChallengeMethod())
	assert.True(t, updated.IsAuthenticated())
	assert.NotNil(t, updated.UserID())
	assert.Equal(t, 999, *updated.UserID())
	assert.NotNil(t, updated.TenantID())
	assert.NotNil(t, updated.AuthTime())
}

func TestAuthRequest_MultipleScopes(t *testing.T) {
	t.Run("SingleScope", func(t *testing.T) {
		ar := authrequest.New(
			"test-client",
			"http://localhost:3000/callback",
			[]string{"openid"},
			"code",
		)

		assert.Len(t, ar.Scopes(), 1)
		assert.Contains(t, ar.Scopes(), "openid")
	})

	t.Run("MultipleScopes", func(t *testing.T) {
		scopes := []string{"openid", "profile", "email", "offline_access"}
		ar := authrequest.New(
			"test-client",
			"http://localhost:3000/callback",
			scopes,
			"code",
		)

		assert.Len(t, ar.Scopes(), 4)
		assert.Equal(t, scopes, ar.Scopes())
	})
}

func TestAuthRequest_ResponseTypes(t *testing.T) {
	t.Run("CodeResponseType", func(t *testing.T) {
		ar := authrequest.New(
			"test-client",
			"http://localhost:3000/callback",
			[]string{"openid"},
			"code",
		)

		assert.Equal(t, "code", ar.ResponseType())
	})

	t.Run("TokenResponseType", func(t *testing.T) {
		ar := authrequest.New(
			"test-client",
			"http://localhost:3000/callback",
			[]string{"openid"},
			"token",
		)

		assert.Equal(t, "token", ar.ResponseType())
	})

	t.Run("HybridResponseType", func(t *testing.T) {
		ar := authrequest.New(
			"test-client",
			"http://localhost:3000/callback",
			[]string{"openid"},
			"code token",
		)

		assert.Equal(t, "code token", ar.ResponseType())
	})
}
