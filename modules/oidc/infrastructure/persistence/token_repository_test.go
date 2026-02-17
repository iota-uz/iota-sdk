package persistence_test

import (
	"crypto/sha256"
	"encoding/hex"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/iota-uz/iota-sdk/modules/oidc/domain/entities/token"
	"github.com/iota-uz/iota-sdk/modules/oidc/infrastructure/persistence"
)

func hashToken(tokenString string) string {
	hash := sha256.Sum256([]byte(tokenString))
	return hex.EncodeToString(hash[:])
}

func TestTokenRepository_Create(t *testing.T) {
	t.Parallel()
	f := setupTest(t)

	tokenRepo := persistence.NewTokenRepository()

	tests := []struct {
		name           string
		tokenString    string
		clientID       string
		userID         int
		scopes         []string
		lifetime       time.Duration
		options        []token.Option
		expectedAud    []string
		expectedAMR    []string
		expectError    bool
		setupDuplicate bool // If true, create a token with same hash first
	}{
		{
			name:        "Success",
			tokenString: "test-refresh-token-123",
			clientID:    "test-client-id",
			userID:      1,
			scopes:      []string{"openid", "profile", "email"},
			lifetime:    720 * time.Hour, // 30 days
			options:     nil,
			expectedAud: nil,
			expectedAMR: []string{"pwd"}, // Default AMR
			expectError: false,
		},
		{
			name:        "WithCustomOptions",
			tokenString: "custom-refresh-token-456",
			clientID:    "custom-client-id",
			userID:      2,
			scopes:      []string{"openid", "offline_access"},
			lifetime:    168 * time.Hour, // 7 days
			options: []token.Option{
				token.WithAudience([]string{"https://api.example.com"}),
				token.WithAMR([]string{"pwd", "mfa"}),
			},
			expectedAud: []string{"https://api.example.com"},
			expectedAMR: []string{"pwd", "mfa"},
			expectError: false,
		},
		{
			name:           "DuplicateTokenHash",
			tokenString:    "duplicate-token-789",
			clientID:       "client-2",
			userID:         4,
			scopes:         []string{"openid"},
			lifetime:       24 * time.Hour,
			options:        nil,
			expectError:    true,
			setupDuplicate: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tokenHash := hashToken(tc.tokenString)

			// Setup: create duplicate token first if needed
			if tc.setupDuplicate {
				firstToken := token.New(
					tokenHash,
					"client-1",
					3,
					uuid.New(),
					[]string{"openid"},
					time.Now(),
					24*time.Hour,
				)
				err := tokenRepo.Create(f.Ctx, firstToken)
				require.NoError(t, err, "failed to create setup token for duplicate test")
			}

			// Create the test token
			refreshToken := token.New(
				tokenHash,
				tc.clientID,
				tc.userID,
				uuid.New(),
				tc.scopes,
				time.Now(),
				tc.lifetime,
				tc.options...,
			)

			err := tokenRepo.Create(f.Ctx, refreshToken)

			if tc.expectError {
				assert.Error(t, err, "expected error for %s", tc.name)
				return
			}

			require.NoError(t, err, "unexpected error creating token")

			// Retrieve and verify
			retrieved, err := tokenRepo.GetByTokenHash(f.Ctx, tokenHash)
			require.NoError(t, err, "failed to retrieve token")

			assert.Equal(t, tokenHash, retrieved.TokenHash())
			assert.Equal(t, tc.clientID, retrieved.ClientID())
			assert.Equal(t, tc.userID, retrieved.UserID())
			assert.Equal(t, tc.scopes, retrieved.Scopes())

			if tc.expectedAud != nil {
				assert.Equal(t, tc.expectedAud, retrieved.Audience())
			}
			if tc.expectedAMR != nil {
				assert.Equal(t, tc.expectedAMR, retrieved.AMR())
			}
		})
	}
}

func TestTokenRepository_GetByTokenHash(t *testing.T) {
	t.Parallel()
	f := setupTest(t)

	tokenRepo := persistence.NewTokenRepository()

	t.Run("Success", func(t *testing.T) {
		tokenString := "get-token-test"
		tokenHash := hashToken(tokenString)

		refreshToken := token.New(
			tokenHash,
			"test-client",
			5,
			uuid.New(),
			[]string{"openid"},
			time.Now(),
			24*time.Hour,
		)

		err := tokenRepo.Create(f.Ctx, refreshToken)
		require.NoError(t, err)

		retrieved, err := tokenRepo.GetByTokenHash(f.Ctx, tokenHash)
		require.NoError(t, err)
		assert.Equal(t, tokenHash, retrieved.TokenHash())
		assert.Equal(t, "test-client", retrieved.ClientID())
		assert.Equal(t, 5, retrieved.UserID())
	})

	t.Run("NotFound", func(t *testing.T) {
		_, err := tokenRepo.GetByTokenHash(f.Ctx, "non-existent-hash")
		assert.Error(t, err)
	})
}

func TestTokenRepository_Delete(t *testing.T) {
	t.Parallel()
	f := setupTest(t)

	tokenRepo := persistence.NewTokenRepository()

	t.Run("Success", func(t *testing.T) {
		tokenString := "delete-token-test"
		tokenHash := hashToken(tokenString)

		refreshToken := token.New(
			tokenHash,
			"test-client",
			6,
			uuid.New(),
			[]string{"openid"},
			time.Now(),
			24*time.Hour,
		)

		err := tokenRepo.Create(f.Ctx, refreshToken)
		require.NoError(t, err)

		err = tokenRepo.Delete(f.Ctx, refreshToken.ID())
		require.NoError(t, err)

		_, err = tokenRepo.GetByTokenHash(f.Ctx, tokenHash)
		assert.Error(t, err)
	})

	t.Run("NotFound", func(t *testing.T) {
		err := tokenRepo.Delete(f.Ctx, uuid.New())
		assert.Error(t, err)
	})
}

func TestTokenRepository_DeleteByTokenHash(t *testing.T) {
	t.Parallel()
	f := setupTest(t)

	tokenRepo := persistence.NewTokenRepository()

	t.Run("Success", func(t *testing.T) {
		tokenString := "delete-by-hash-test"
		tokenHash := hashToken(tokenString)

		refreshToken := token.New(
			tokenHash,
			"test-client",
			7,
			uuid.New(),
			[]string{"openid"},
			time.Now(),
			24*time.Hour,
		)

		err := tokenRepo.Create(f.Ctx, refreshToken)
		require.NoError(t, err)

		err = tokenRepo.DeleteByTokenHash(f.Ctx, tokenHash)
		require.NoError(t, err)

		_, err = tokenRepo.GetByTokenHash(f.Ctx, tokenHash)
		assert.Error(t, err)
	})

	t.Run("NotFound", func(t *testing.T) {
		err := tokenRepo.DeleteByTokenHash(f.Ctx, "non-existent-hash")
		assert.Error(t, err)
	})
}

func TestTokenRepository_DeleteByUserAndClient(t *testing.T) {
	t.Parallel()
	f := setupTest(t)

	tokenRepo := persistence.NewTokenRepository()

	userID := 8
	clientID := "user-client-test"
	tenantID := uuid.New()

	// Create multiple tokens for same user+client
	token1 := token.New(
		hashToken("token-1"),
		clientID,
		userID,
		tenantID,
		[]string{"openid"},
		time.Now(),
		24*time.Hour,
	)
	err := tokenRepo.Create(f.Ctx, token1)
	require.NoError(t, err)

	token2 := token.New(
		hashToken("token-2"),
		clientID,
		userID,
		tenantID,
		[]string{"openid"},
		time.Now(),
		24*time.Hour,
	)
	err = tokenRepo.Create(f.Ctx, token2)
	require.NoError(t, err)

	// Create token for different client
	token3 := token.New(
		hashToken("token-3"),
		"different-client",
		userID,
		tenantID,
		[]string{"openid"},
		time.Now(),
		24*time.Hour,
	)
	err = tokenRepo.Create(f.Ctx, token3)
	require.NoError(t, err)

	// Delete all tokens for user+client
	err = tokenRepo.DeleteByUserAndClient(f.Ctx, userID, clientID)
	require.NoError(t, err)

	// Verify tokens for user+client are deleted
	_, err = tokenRepo.GetByTokenHash(f.Ctx, hashToken("token-1"))
	require.Error(t, err)
	_, err = tokenRepo.GetByTokenHash(f.Ctx, hashToken("token-2"))
	require.Error(t, err)

	// Verify token for different client still exists
	retrieved, err := tokenRepo.GetByTokenHash(f.Ctx, hashToken("token-3"))
	require.NoError(t, err)
	assert.Equal(t, "different-client", retrieved.ClientID())
}

func TestTokenRepository_DeleteExpired(t *testing.T) {
	t.Parallel()
	f := setupTest(t)

	tokenRepo := persistence.NewTokenRepository()

	// Create expired token
	expiredToken := token.New(
		hashToken("expired-token"),
		"test-client",
		9,
		uuid.New(),
		[]string{"openid"},
		time.Now().Add(-2*time.Hour),
		1*time.Hour,
		token.WithExpiresAt(time.Now().Add(-1*time.Hour)), // Explicitly set past expiry
	)
	err := tokenRepo.Create(f.Ctx, expiredToken)
	require.NoError(t, err)

	// Create valid token
	validToken := token.New(
		hashToken("valid-token"),
		"test-client",
		10,
		uuid.New(),
		[]string{"openid"},
		time.Now(),
		24*time.Hour,
	)
	err = tokenRepo.Create(f.Ctx, validToken)
	require.NoError(t, err)

	// Delete expired tokens
	err = tokenRepo.DeleteExpired(f.Ctx)
	require.NoError(t, err)

	// Verify expired token is gone
	_, err = tokenRepo.GetByTokenHash(f.Ctx, hashToken("expired-token"))
	require.Error(t, err)

	// Verify valid token still exists
	retrieved, err := tokenRepo.GetByTokenHash(f.Ctx, hashToken("valid-token"))
	require.NoError(t, err)
	assert.Equal(t, 10, retrieved.UserID())
}

func TestTokenRepository_IsExpired(t *testing.T) {
	t.Parallel()
	f := setupTest(t)

	tokenRepo := persistence.NewTokenRepository()

	t.Run("NotExpired", func(t *testing.T) {
		tokenHash := hashToken("not-expired-token")
		refreshToken := token.New(
			tokenHash,
			"test-client",
			11,
			uuid.New(),
			[]string{"openid"},
			time.Now(),
			24*time.Hour,
		)

		err := tokenRepo.Create(f.Ctx, refreshToken)
		require.NoError(t, err)

		retrieved, err := tokenRepo.GetByTokenHash(f.Ctx, tokenHash)
		require.NoError(t, err)
		assert.False(t, retrieved.IsExpired())
	})

	t.Run("Expired", func(t *testing.T) {
		tokenHash := hashToken("expired-token-check")
		refreshToken := token.New(
			tokenHash,
			"test-client",
			12,
			uuid.New(),
			[]string{"openid"},
			time.Now().Add(-2*time.Hour),
			1*time.Hour,
			token.WithExpiresAt(time.Now().Add(-1*time.Hour)), // Explicitly set past expiry
		)

		err := tokenRepo.Create(f.Ctx, refreshToken)
		require.NoError(t, err)

		retrieved, err := tokenRepo.GetByTokenHash(f.Ctx, tokenHash)
		require.NoError(t, err)
		assert.True(t, retrieved.IsExpired())
	})
}

func TestTokenRepository_TokenLifetime(t *testing.T) {
	t.Parallel()
	f := setupTest(t)

	tokenRepo := persistence.NewTokenRepository()

	customLifetime := 168 * time.Hour // 7 days
	authTime := time.Now()

	tokenHash := hashToken("lifetime-test-token")
	refreshToken := token.New(
		tokenHash,
		"test-client",
		13,
		uuid.New(),
		[]string{"openid", "offline_access"},
		authTime,
		customLifetime,
	)

	err := tokenRepo.Create(f.Ctx, refreshToken)
	require.NoError(t, err)

	retrieved, err := tokenRepo.GetByTokenHash(f.Ctx, tokenHash)
	require.NoError(t, err)

	expectedExpiry := authTime.Add(customLifetime)
	assert.WithinDuration(t, expectedExpiry, retrieved.ExpiresAt(), 2*time.Second)
	assert.False(t, retrieved.IsExpired())
}

func TestTokenRepository_Scopes(t *testing.T) {
	t.Parallel()
	f := setupTest(t)

	tokenRepo := persistence.NewTokenRepository()

	t.Run("MultipleScopes", func(t *testing.T) {
		tokenHash := hashToken("multi-scope-token")
		scopes := []string{"openid", "profile", "email", "offline_access"}

		refreshToken := token.New(
			tokenHash,
			"test-client",
			14,
			uuid.New(),
			scopes,
			time.Now(),
			24*time.Hour,
		)

		err := tokenRepo.Create(f.Ctx, refreshToken)
		require.NoError(t, err)

		retrieved, err := tokenRepo.GetByTokenHash(f.Ctx, tokenHash)
		require.NoError(t, err)
		assert.Equal(t, scopes, retrieved.Scopes())
	})

	t.Run("EmptyScopes", func(t *testing.T) {
		tokenHash := hashToken("empty-scope-token")

		refreshToken := token.New(
			tokenHash,
			"test-client",
			15,
			uuid.New(),
			[]string{},
			time.Now(),
			24*time.Hour,
		)

		err := tokenRepo.Create(f.Ctx, refreshToken)
		require.NoError(t, err)

		retrieved, err := tokenRepo.GetByTokenHash(f.Ctx, tokenHash)
		require.NoError(t, err)
		assert.Empty(t, retrieved.Scopes())
	})
}

func TestTokenRepository_AMR(t *testing.T) {
	t.Parallel()
	f := setupTest(t)

	tokenRepo := persistence.NewTokenRepository()

	t.Run("DefaultAMR", func(t *testing.T) {
		tokenHash := hashToken("default-amr-token")

		refreshToken := token.New(
			tokenHash,
			"test-client",
			16,
			uuid.New(),
			[]string{"openid"},
			time.Now(),
			24*time.Hour,
		)

		err := tokenRepo.Create(f.Ctx, refreshToken)
		require.NoError(t, err)

		retrieved, err := tokenRepo.GetByTokenHash(f.Ctx, tokenHash)
		require.NoError(t, err)
		assert.Equal(t, []string{"pwd"}, retrieved.AMR())
	})

	t.Run("CustomAMR", func(t *testing.T) {
		tokenHash := hashToken("custom-amr-token")
		customAMR := []string{"pwd", "mfa", "otp"}

		refreshToken := token.New(
			tokenHash,
			"test-client",
			17,
			uuid.New(),
			[]string{"openid"},
			time.Now(),
			24*time.Hour,
			token.WithAMR(customAMR),
		)

		err := tokenRepo.Create(f.Ctx, refreshToken)
		require.NoError(t, err)

		retrieved, err := tokenRepo.GetByTokenHash(f.Ctx, tokenHash)
		require.NoError(t, err)
		assert.Equal(t, customAMR, retrieved.AMR())
	})
}
