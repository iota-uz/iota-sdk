package token_test

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"

	"github.com/iota-uz/iota-sdk/modules/oidc/domain/entities/token"
)

func TestRefreshToken_New(t *testing.T) {
	t.Run("WithRequiredFieldsOnly", func(t *testing.T) {
		tokenHash := "hash-123"
		clientID := "test-client"
		userID := 1
		tenantID := uuid.New()
		scopes := []string{"openid", "profile"}
		authTime := time.Now()
		lifetime := 720 * time.Hour

		rt := token.New(
			tokenHash,
			clientID,
			userID,
			tenantID,
			scopes,
			authTime,
			lifetime,
		)

		assert.NotEqual(t, uuid.Nil, rt.ID())
		assert.Equal(t, tokenHash, rt.TokenHash())
		assert.Equal(t, clientID, rt.ClientID())
		assert.Equal(t, userID, rt.UserID())
		assert.Equal(t, tenantID, rt.TenantID())
		assert.Equal(t, scopes, rt.Scopes())
		assert.Empty(t, rt.Audience())
		assert.WithinDuration(t, authTime, rt.AuthTime(), time.Second)
		assert.Equal(t, []string{"pwd"}, rt.AMR()) // Default AMR
		assert.NotNil(t, rt.ExpiresAt())
		assert.NotNil(t, rt.CreatedAt())
	})

	t.Run("WithAllOptions", func(t *testing.T) {
		customID := uuid.New()
		audience := []string{"https://api.example.com", "https://api2.example.com"}
		amr := []string{"pwd", "mfa", "otp"}
		customExpiry := time.Now().Add(168 * time.Hour)

		rt := token.New(
			"hash-456",
			"custom-client",
			2,
			uuid.New(),
			[]string{"openid", "profile", "email", "offline_access"},
			time.Now(),
			168*time.Hour,
			token.WithID(customID),
			token.WithAudience(audience),
			token.WithAMR(amr),
			token.WithExpiresAt(customExpiry),
		)

		assert.Equal(t, customID, rt.ID())
		assert.Equal(t, audience, rt.Audience())
		assert.Equal(t, amr, rt.AMR())
		assert.WithinDuration(t, customExpiry, rt.ExpiresAt(), time.Second)
	})

	t.Run("ExpirationTimeCalculation", func(t *testing.T) {
		authTime := time.Now()
		lifetime := 48 * time.Hour

		rt := token.New(
			"hash-789",
			"test-client",
			3,
			uuid.New(),
			[]string{"openid"},
			authTime,
			lifetime,
		)

		expectedExpiry := authTime.Add(lifetime)
		assert.WithinDuration(t, expectedExpiry, rt.ExpiresAt(), 2*time.Second)
	})
}

func TestRefreshToken_IsExpired(t *testing.T) {
	t.Run("NotExpired", func(t *testing.T) {
		rt := token.New(
			"not-expired-hash",
			"test-client",
			4,
			uuid.New(),
			[]string{"openid"},
			time.Now(),
			24*time.Hour,
		)

		assert.False(t, rt.IsExpired())
	})

	t.Run("Expired", func(t *testing.T) {
		// Create token that should have expired
		pastTime := time.Now().Add(-25 * time.Hour)
		rt := token.New(
			"expired-hash",
			"test-client",
			5,
			uuid.New(),
			[]string{"openid"},
			pastTime,
			24*time.Hour,
			token.WithExpiresAt(pastTime.Add(24*time.Hour)), // Explicitly set expiry in the past
		)

		assert.True(t, rt.IsExpired())
	})

	t.Run("JustExpired", func(t *testing.T) {
		rt := token.New(
			"just-expired-hash",
			"test-client",
			6,
			uuid.New(),
			[]string{"openid"},
			time.Now().Add(-1*time.Second),
			0, // Expires immediately
		)

		time.Sleep(2 * time.Second)
		assert.True(t, rt.IsExpired())
	})

	t.Run("AlmostExpired", func(t *testing.T) {
		rt := token.New(
			"almost-expired-hash",
			"test-client",
			7,
			uuid.New(),
			[]string{"openid"},
			time.Now(),
			5*time.Second,
		)

		assert.False(t, rt.IsExpired())
		// Should still be valid for a few seconds
		time.Sleep(2 * time.Second)
		assert.False(t, rt.IsExpired())
	})
}

func TestRefreshToken_SetAudience(t *testing.T) {
	original := token.New(
		"hash-audience-test",
		"test-client",
		8,
		uuid.New(),
		[]string{"openid"},
		time.Now(),
		24*time.Hour,
	)

	assert.Empty(t, original.Audience())

	// Set audience (immutable pattern)
	newAudience := []string{"https://api.example.com", "https://api2.example.com"}
	updated := original.SetAudience(newAudience)

	// Verify new instance has the audience
	assert.Equal(t, newAudience, updated.Audience())

	// Verify original is unchanged
	assert.Empty(t, original.Audience())
}

func TestRefreshToken_SetAMR(t *testing.T) {
	original := token.New(
		"hash-amr-test",
		"test-client",
		9,
		uuid.New(),
		[]string{"openid"},
		time.Now(),
		24*time.Hour,
	)

	assert.Equal(t, []string{"pwd"}, original.AMR()) // Default AMR

	// Set AMR (immutable pattern)
	newAMR := []string{"pwd", "mfa", "fingerprint"}
	updated := original.SetAMR(newAMR)

	// Verify new instance has the new AMR
	assert.Equal(t, newAMR, updated.AMR())

	// Verify original is unchanged
	assert.Equal(t, []string{"pwd"}, original.AMR())
}

func TestRefreshToken_ImmutabilityPattern(t *testing.T) {
	original := token.New(
		"hash-immutable-test",
		"test-client",
		10,
		uuid.New(),
		[]string{"openid"},
		time.Now(),
		24*time.Hour,
	)

	// Chain multiple immutable operations
	updated := original.
		SetAudience([]string{"https://api.example.com"}).
		SetAMR([]string{"pwd", "mfa"})

	// Verify original remains completely unchanged
	assert.Empty(t, original.Audience())
	assert.Equal(t, []string{"pwd"}, original.AMR())

	// Verify updated has all changes
	assert.Equal(t, []string{"https://api.example.com"}, updated.Audience())
	assert.Equal(t, []string{"pwd", "mfa"}, updated.AMR())
}

func TestRefreshToken_Scopes(t *testing.T) {
	t.Run("SingleScope", func(t *testing.T) {
		rt := token.New(
			"hash-single-scope",
			"test-client",
			11,
			uuid.New(),
			[]string{"openid"},
			time.Now(),
			24*time.Hour,
		)

		assert.Len(t, rt.Scopes(), 1)
		assert.Contains(t, rt.Scopes(), "openid")
	})

	t.Run("MultipleScopes", func(t *testing.T) {
		scopes := []string{"openid", "profile", "email", "offline_access"}
		rt := token.New(
			"hash-multi-scope",
			"test-client",
			12,
			uuid.New(),
			scopes,
			time.Now(),
			24*time.Hour,
		)

		assert.Len(t, rt.Scopes(), 4)
		assert.Equal(t, scopes, rt.Scopes())
	})

	t.Run("EmptyScopes", func(t *testing.T) {
		rt := token.New(
			"hash-empty-scope",
			"test-client",
			13,
			uuid.New(),
			[]string{},
			time.Now(),
			24*time.Hour,
		)

		assert.Empty(t, rt.Scopes())
	})
}

func TestRefreshToken_Lifetime(t *testing.T) {
	t.Run("ShortLifetime", func(t *testing.T) {
		authTime := time.Now()
		lifetime := 1 * time.Hour

		rt := token.New(
			"hash-short-lifetime",
			"test-client",
			14,
			uuid.New(),
			[]string{"openid"},
			authTime,
			lifetime,
		)

		expectedExpiry := authTime.Add(lifetime)
		assert.WithinDuration(t, expectedExpiry, rt.ExpiresAt(), 2*time.Second)
	})

	t.Run("LongLifetime", func(t *testing.T) {
		authTime := time.Now()
		lifetime := 720 * time.Hour // 30 days

		rt := token.New(
			"hash-long-lifetime",
			"test-client",
			15,
			uuid.New(),
			[]string{"openid", "offline_access"},
			authTime,
			lifetime,
		)

		expectedExpiry := authTime.Add(lifetime)
		assert.WithinDuration(t, expectedExpiry, rt.ExpiresAt(), 2*time.Second)
	})

	t.Run("CustomLifetime", func(t *testing.T) {
		authTime := time.Now()
		lifetime := 168 * time.Hour // 7 days

		rt := token.New(
			"hash-custom-lifetime",
			"test-client",
			16,
			uuid.New(),
			[]string{"openid"},
			authTime,
			lifetime,
		)

		expectedExpiry := authTime.Add(lifetime)
		assert.WithinDuration(t, expectedExpiry, rt.ExpiresAt(), 2*time.Second)
	})
}

func TestRefreshToken_AuthTime(t *testing.T) {
	pastAuthTime := time.Now().Add(-1 * time.Hour)

	rt := token.New(
		"hash-auth-time",
		"test-client",
		17,
		uuid.New(),
		[]string{"openid"},
		pastAuthTime,
		24*time.Hour,
	)

	assert.WithinDuration(t, pastAuthTime, rt.AuthTime(), time.Second)
	assert.True(t, rt.AuthTime().Before(time.Now()))
}

func TestRefreshToken_MultipleAudiences(t *testing.T) {
	t.Run("SingleAudience", func(t *testing.T) {
		audience := []string{"https://api.example.com"}

		rt := token.New(
			"hash-single-audience",
			"test-client",
			18,
			uuid.New(),
			[]string{"openid"},
			time.Now(),
			24*time.Hour,
			token.WithAudience(audience),
		)

		assert.Len(t, rt.Audience(), 1)
		assert.Equal(t, audience, rt.Audience())
	})

	t.Run("MultipleAudiences", func(t *testing.T) {
		audiences := []string{
			"https://api.example.com",
			"https://api2.example.com",
			"https://api3.example.com",
		}

		rt := token.New(
			"hash-multi-audience",
			"test-client",
			19,
			uuid.New(),
			[]string{"openid"},
			time.Now(),
			24*time.Hour,
			token.WithAudience(audiences),
		)

		assert.Len(t, rt.Audience(), 3)
		assert.Equal(t, audiences, rt.Audience())
	})

	t.Run("EmptyAudience", func(t *testing.T) {
		rt := token.New(
			"hash-empty-audience",
			"test-client",
			20,
			uuid.New(),
			[]string{"openid"},
			time.Now(),
			24*time.Hour,
		)

		assert.Empty(t, rt.Audience())
	})
}

func TestRefreshToken_AMRMethods(t *testing.T) {
	t.Run("PasswordOnly", func(t *testing.T) {
		rt := token.New(
			"hash-pwd-only",
			"test-client",
			21,
			uuid.New(),
			[]string{"openid"},
			time.Now(),
			24*time.Hour,
		)

		assert.Equal(t, []string{"pwd"}, rt.AMR())
	})

	t.Run("PasswordAndMFA", func(t *testing.T) {
		amr := []string{"pwd", "mfa"}

		rt := token.New(
			"hash-pwd-mfa",
			"test-client",
			22,
			uuid.New(),
			[]string{"openid"},
			time.Now(),
			24*time.Hour,
			token.WithAMR(amr),
		)

		assert.Equal(t, amr, rt.AMR())
	})

	t.Run("MultipleAuthMethods", func(t *testing.T) {
		amr := []string{"pwd", "mfa", "otp", "sms", "fingerprint"}

		rt := token.New(
			"hash-multi-amr",
			"test-client",
			23,
			uuid.New(),
			[]string{"openid"},
			time.Now(),
			24*time.Hour,
			token.WithAMR(amr),
		)

		assert.Equal(t, amr, rt.AMR())
	})
}
