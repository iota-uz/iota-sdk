package client_test

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"

	"github.com/iota-uz/iota-sdk/modules/oidc/domain/entities/client"
)

func TestClient_New(t *testing.T) {
	t.Run("WithRequiredFieldsOnly", func(t *testing.T) {
		c := client.New(
			"test-client",
			"Test Client",
			"web",
			[]string{"http://localhost:3000/callback"},
		)

		assert.NotEqual(t, uuid.Nil, c.ID())
		assert.Equal(t, "test-client", c.ClientID())
		assert.Equal(t, "Test Client", c.Name())
		assert.Equal(t, "web", c.ApplicationType())
		assert.Equal(t, []string{"http://localhost:3000/callback"}, c.RedirectURIs())
		assert.Nil(t, c.ClientSecretHash())
		assert.Equal(t, []string{"authorization_code"}, c.GrantTypes())
		assert.Equal(t, []string{"code"}, c.ResponseTypes())
		assert.Equal(t, []string{"openid", "profile", "email"}, c.Scopes())
		assert.Equal(t, "client_secret_basic", c.AuthMethod())
		assert.Equal(t, time.Hour, c.AccessTokenLifetime())
		assert.Equal(t, time.Hour, c.IDTokenLifetime())
		assert.Equal(t, 720*time.Hour, c.RefreshTokenLifetime())
		assert.True(t, c.RequirePKCE())
		assert.True(t, c.IsActive())
		assert.NotNil(t, c.CreatedAt())
		assert.NotNil(t, c.UpdatedAt())
	})

	t.Run("WithAllOptions", func(t *testing.T) {
		customID := uuid.New()
		secretHash := "hashed-secret"
		customAccessLifetime := 2 * time.Hour
		customIDLifetime := 30 * time.Minute
		customRefreshLifetime := 168 * time.Hour

		c := client.New(
			"custom-client",
			"Custom Client",
			"spa",
			[]string{"http://localhost:4000/callback"},
			client.WithID(customID),
			client.WithClientSecretHash(secretHash),
			client.WithGrantTypes([]string{"authorization_code", "refresh_token"}),
			client.WithResponseTypes([]string{"code", "token"}),
			client.WithScopes([]string{"openid", "profile", "email", "offline_access"}),
			client.WithAuthMethod("client_secret_post"),
			client.WithAccessTokenLifetime(customAccessLifetime),
			client.WithIDTokenLifetime(customIDLifetime),
			client.WithRefreshTokenLifetime(customRefreshLifetime),
			client.WithRequirePKCE(false),
			client.WithIsActive(false),
		)

		assert.Equal(t, customID, c.ID())
		assert.NotNil(t, c.ClientSecretHash())
		assert.Equal(t, secretHash, *c.ClientSecretHash())
		assert.Equal(t, []string{"authorization_code", "refresh_token"}, c.GrantTypes())
		assert.Equal(t, []string{"code", "token"}, c.ResponseTypes())
		assert.Contains(t, c.Scopes(), "offline_access")
		assert.Equal(t, "client_secret_post", c.AuthMethod())
		assert.Equal(t, customAccessLifetime, c.AccessTokenLifetime())
		assert.Equal(t, customIDLifetime, c.IDTokenLifetime())
		assert.Equal(t, customRefreshLifetime, c.RefreshTokenLifetime())
		assert.False(t, c.RequirePKCE())
		assert.False(t, c.IsActive())
	})
}

func TestClient_ValidateRedirectURI(t *testing.T) {
	c := client.New(
		"test-client",
		"Test Client",
		"web",
		[]string{
			"http://localhost:3000/callback",
			"http://localhost:3000/callback2",
			"https://example.com/callback",
		},
	)

	t.Run("ValidURI", func(t *testing.T) {
		assert.True(t, c.ValidateRedirectURI("http://localhost:3000/callback"))
		assert.True(t, c.ValidateRedirectURI("http://localhost:3000/callback2"))
		assert.True(t, c.ValidateRedirectURI("https://example.com/callback"))
	})

	t.Run("InvalidURI", func(t *testing.T) {
		assert.False(t, c.ValidateRedirectURI("http://localhost:3000/invalid"))
		assert.False(t, c.ValidateRedirectURI("https://malicious.com/callback"))
		assert.False(t, c.ValidateRedirectURI(""))
	})
}

func TestClient_ValidateGrantType(t *testing.T) {
	c := client.New(
		"test-client",
		"Test Client",
		"web",
		[]string{"http://localhost:3000/callback"},
		client.WithGrantTypes([]string{"authorization_code", "refresh_token"}),
	)

	t.Run("ValidGrantType", func(t *testing.T) {
		assert.True(t, c.ValidateGrantType("authorization_code"))
		assert.True(t, c.ValidateGrantType("refresh_token"))
	})

	t.Run("InvalidGrantType", func(t *testing.T) {
		assert.False(t, c.ValidateGrantType("client_credentials"))
		assert.False(t, c.ValidateGrantType("password"))
		assert.False(t, c.ValidateGrantType(""))
	})
}

func TestClient_ValidateScope(t *testing.T) {
	c := client.New(
		"test-client",
		"Test Client",
		"web",
		[]string{"http://localhost:3000/callback"},
		client.WithScopes([]string{"openid", "profile", "email", "offline_access"}),
	)

	t.Run("ValidScope", func(t *testing.T) {
		assert.True(t, c.ValidateScope("openid"))
		assert.True(t, c.ValidateScope("profile"))
		assert.True(t, c.ValidateScope("email"))
		assert.True(t, c.ValidateScope("offline_access"))
	})

	t.Run("InvalidScope", func(t *testing.T) {
		assert.False(t, c.ValidateScope("admin"))
		assert.False(t, c.ValidateScope("superuser"))
		assert.False(t, c.ValidateScope(""))
	})
}

func TestClient_SetClientSecretHash(t *testing.T) {
	original := client.New(
		"test-client",
		"Test Client",
		"web",
		[]string{"http://localhost:3000/callback"},
	)

	assert.Nil(t, original.ClientSecretHash())

	// Set secret hash (immutable pattern)
	newHash := "hashed-secret-123"
	updated := original.SetClientSecretHash(newHash)

	// Verify new instance has the hash
	assert.NotNil(t, updated.ClientSecretHash())
	assert.Equal(t, newHash, *updated.ClientSecretHash())

	// Verify original is unchanged
	assert.Nil(t, original.ClientSecretHash())

	// Verify it's a different instance
	assert.NotEqual(t, original, updated)
}

func TestClient_SetRedirectURIs(t *testing.T) {
	original := client.New(
		"test-client",
		"Test Client",
		"web",
		[]string{"http://localhost:3000/callback"},
	)

	newURIs := []string{
		"http://localhost:3000/callback",
		"http://localhost:4000/callback",
		"https://example.com/callback",
	}

	updated := original.SetRedirectURIs(newURIs)

	assert.Equal(t, newURIs, updated.RedirectURIs())
	assert.Len(t, updated.RedirectURIs(), 3)

	// Verify original is unchanged
	assert.Len(t, original.RedirectURIs(), 1)
}

func TestClient_SetScopes(t *testing.T) {
	original := client.New(
		"test-client",
		"Test Client",
		"web",
		[]string{"http://localhost:3000/callback"},
	)

	assert.Equal(t, []string{"openid", "profile", "email"}, original.Scopes())

	newScopes := []string{"openid", "profile", "email", "offline_access", "custom"}
	updated := original.SetScopes(newScopes)

	assert.Equal(t, newScopes, updated.Scopes())

	// Verify original is unchanged
	assert.Equal(t, []string{"openid", "profile", "email"}, original.Scopes())
}

func TestClient_AddScope(t *testing.T) {
	original := client.New(
		"test-client",
		"Test Client",
		"web",
		[]string{"http://localhost:3000/callback"},
	)

	t.Run("AddNewScope", func(t *testing.T) {
		updated := original.AddScope("offline_access")
		assert.Contains(t, updated.Scopes(), "offline_access")
		assert.Len(t, updated.Scopes(), 4) // openid, profile, email + offline_access

		// Verify original is unchanged
		assert.NotContains(t, original.Scopes(), "offline_access")
		assert.Len(t, original.Scopes(), 3)
	})

	t.Run("AddExistingScope", func(t *testing.T) {
		updated := original.AddScope("openid")
		assert.Equal(t, original.Scopes(), updated.Scopes())
		assert.Len(t, updated.Scopes(), 3) // No duplicate
	})
}

func TestClient_RemoveScope(t *testing.T) {
	original := client.New(
		"test-client",
		"Test Client",
		"web",
		[]string{"http://localhost:3000/callback"},
	)

	t.Run("RemoveExistingScope", func(t *testing.T) {
		updated := original.RemoveScope("profile")
		assert.NotContains(t, updated.Scopes(), "profile")
		assert.Len(t, updated.Scopes(), 2) // openid, email

		// Verify original is unchanged
		assert.Contains(t, original.Scopes(), "profile")
		assert.Len(t, original.Scopes(), 3)
	})

	t.Run("RemoveNonExistentScope", func(t *testing.T) {
		updated := original.RemoveScope("non-existent")
		assert.Equal(t, original.Scopes(), updated.Scopes())
		assert.Len(t, updated.Scopes(), 3)
	})
}

func TestClient_Activate(t *testing.T) {
	// Create inactive client
	original := client.New(
		"test-client",
		"Test Client",
		"web",
		[]string{"http://localhost:3000/callback"},
		client.WithIsActive(false),
	)

	assert.False(t, original.IsActive())

	// Activate
	activated := original.Activate()
	assert.True(t, activated.IsActive())

	// Verify original is unchanged
	assert.False(t, original.IsActive())
}

func TestClient_Deactivate(t *testing.T) {
	// Create active client
	original := client.New(
		"test-client",
		"Test Client",
		"web",
		[]string{"http://localhost:3000/callback"},
	)

	assert.True(t, original.IsActive())

	// Deactivate
	deactivated := original.Deactivate()
	assert.False(t, deactivated.IsActive())

	// Verify original is unchanged
	assert.True(t, original.IsActive())
}

func TestClient_ImmutabilityPattern(t *testing.T) {
	original := client.New(
		"test-client",
		"Test Client",
		"web",
		[]string{"http://localhost:3000/callback"},
	)

	// Chain multiple immutable operations
	updated := original.
		SetClientSecretHash("new-hash").
		AddScope("offline_access").
		AddScope("custom").
		RemoveScope("email").
		Activate()

	// Verify original remains completely unchanged
	assert.Nil(t, original.ClientSecretHash())
	assert.NotContains(t, original.Scopes(), "offline_access")
	assert.NotContains(t, original.Scopes(), "custom")
	assert.Contains(t, original.Scopes(), "email")
	assert.Len(t, original.Scopes(), 3)

	// Verify updated has all changes
	assert.NotNil(t, updated.ClientSecretHash())
	assert.Equal(t, "new-hash", *updated.ClientSecretHash())
	assert.Contains(t, updated.Scopes(), "offline_access")
	assert.Contains(t, updated.Scopes(), "custom")
	assert.NotContains(t, updated.Scopes(), "email")
	assert.True(t, updated.IsActive())
}

func TestClient_UpdatedAtChanges(t *testing.T) {
	original := client.New(
		"test-client",
		"Test Client",
		"web",
		[]string{"http://localhost:3000/callback"},
	)

	originalUpdatedAt := original.UpdatedAt()

	// Wait a brief moment to ensure time difference
	time.Sleep(10 * time.Millisecond)

	// Any modification should update the UpdatedAt timestamp
	updated := original.SetClientSecretHash("new-hash")

	assert.True(t, updated.UpdatedAt().After(originalUpdatedAt))
}
