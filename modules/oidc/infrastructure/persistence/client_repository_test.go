package persistence_test

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/iota-uz/iota-sdk/modules/oidc/domain/entities/client"
	"github.com/iota-uz/iota-sdk/modules/oidc/infrastructure/persistence"
	"github.com/iota-uz/iota-sdk/pkg/repo"
)

func TestClientRepository_Create(t *testing.T) {
	t.Parallel()
	f := setupTest(t)

	clientRepo := persistence.NewClientRepository()

	t.Run("Success", func(t *testing.T) {
		testClient := client.New(
			"test-client-id",
			"Test Client",
			"web",
			[]string{"http://localhost:3000/callback"},
		)

		createdClient, err := clientRepo.Create(f.Ctx, testClient)
		require.NoError(t, err)
		assert.NotEqual(t, uuid.Nil, createdClient.ID())
		assert.Equal(t, "test-client-id", createdClient.ClientID())
		assert.Equal(t, "Test Client", createdClient.Name())
		assert.Equal(t, "web", createdClient.ApplicationType())
		assert.Equal(t, []string{"http://localhost:3000/callback"}, createdClient.RedirectURIs())
		assert.True(t, createdClient.IsActive())
		assert.NotNil(t, createdClient.CreatedAt())
		assert.NotNil(t, createdClient.UpdatedAt())
	})

	t.Run("WithCustomOptions", func(t *testing.T) {
		secretHash := "hashed-secret"
		testClient := client.New(
			"custom-client-id",
			"Custom Client",
			"spa",
			[]string{"http://localhost:4000/callback"},
			client.WithClientSecretHash(secretHash),
			client.WithGrantTypes([]string{"authorization_code", "refresh_token"}),
			client.WithScopes([]string{"openid", "profile", "email", "offline_access"}),
			client.WithRequirePKCE(false),
		)

		createdClient, err := clientRepo.Create(f.Ctx, testClient)
		require.NoError(t, err)
		assert.Equal(t, "custom-client-id", createdClient.ClientID())
		assert.NotNil(t, createdClient.ClientSecretHash())
		assert.Equal(t, secretHash, *createdClient.ClientSecretHash())
		assert.Contains(t, createdClient.GrantTypes(), "refresh_token")
		assert.Contains(t, createdClient.Scopes(), "offline_access")
		assert.False(t, createdClient.RequirePKCE())
	})

	t.Run("DuplicateClientID", func(t *testing.T) {
		// Create first client
		firstClient := client.New(
			"duplicate-id",
			"First Client",
			"web",
			[]string{"http://localhost:5000/callback"},
		)
		_, err := clientRepo.Create(f.Ctx, firstClient)
		require.NoError(t, err)

		// Attempt to create second client with same client_id
		secondClient := client.New(
			"duplicate-id",
			"Second Client",
			"web",
			[]string{"http://localhost:6000/callback"},
		)
		_, err = clientRepo.Create(f.Ctx, secondClient)
		assert.Error(t, err)
	})
}

func TestClientRepository_GetByID(t *testing.T) {
	t.Parallel()
	f := setupTest(t)

	clientRepo := persistence.NewClientRepository()

	t.Run("Success", func(t *testing.T) {
		testClient := client.New(
			"getbyid-client",
			"GetByID Client",
			"web",
			[]string{"http://localhost:3000/callback"},
		)

		createdClient, err := clientRepo.Create(f.Ctx, testClient)
		require.NoError(t, err)

		retrievedClient, err := clientRepo.GetByID(f.Ctx, createdClient.ID())
		require.NoError(t, err)
		assert.Equal(t, createdClient.ID(), retrievedClient.ID())
		assert.Equal(t, createdClient.ClientID(), retrievedClient.ClientID())
		assert.Equal(t, createdClient.Name(), retrievedClient.Name())
	})

	t.Run("NotFound", func(t *testing.T) {
		_, err := clientRepo.GetByID(f.Ctx, uuid.New())
		assert.Error(t, err)
	})
}

func TestClientRepository_GetByClientID(t *testing.T) {
	t.Parallel()
	f := setupTest(t)

	clientRepo := persistence.NewClientRepository()

	t.Run("Success", func(t *testing.T) {
		testClient := client.New(
			"getbyclientid-client",
			"GetByClientID Client",
			"web",
			[]string{"http://localhost:3000/callback"},
		)

		_, err := clientRepo.Create(f.Ctx, testClient)
		require.NoError(t, err)

		retrievedClient, err := clientRepo.GetByClientID(f.Ctx, "getbyclientid-client")
		require.NoError(t, err)
		assert.Equal(t, "getbyclientid-client", retrievedClient.ClientID())
		assert.Equal(t, "GetByClientID Client", retrievedClient.Name())
	})

	t.Run("NotFound", func(t *testing.T) {
		_, err := clientRepo.GetByClientID(f.Ctx, "non-existent-client")
		assert.Error(t, err)
	})
}

func TestClientRepository_Update(t *testing.T) {
	t.Parallel()
	f := setupTest(t)

	clientRepo := persistence.NewClientRepository()

	t.Run("Success", func(t *testing.T) {
		testClient := client.New(
			"update-client",
			"Update Client",
			"web",
			[]string{"http://localhost:3000/callback"},
		)

		createdClient, err := clientRepo.Create(f.Ctx, testClient)
		require.NoError(t, err)

		// Update client
		updatedClient := createdClient.SetRedirectURIs([]string{
			"http://localhost:3000/callback",
			"http://localhost:4000/callback",
		})
		updatedClient = updatedClient.AddScope("custom_scope")

		err = clientRepo.Update(f.Ctx, updatedClient)
		require.NoError(t, err)

		// Retrieve and verify update
		retrievedClient, err := clientRepo.GetByID(f.Ctx, createdClient.ID())
		require.NoError(t, err)
		assert.Len(t, retrievedClient.RedirectURIs(), 2)
		assert.Contains(t, retrievedClient.Scopes(), "custom_scope")
	})

	t.Run("NotFound", func(t *testing.T) {
		nonExistentClient := client.New(
			"non-existent",
			"Non Existent",
			"web",
			[]string{"http://localhost:3000/callback"},
			client.WithID(uuid.New()),
		)

		err := clientRepo.Update(f.Ctx, nonExistentClient)
		assert.Error(t, err)
	})
}

func TestClientRepository_Delete(t *testing.T) {
	t.Parallel()
	f := setupTest(t)

	clientRepo := persistence.NewClientRepository()

	t.Run("Success", func(t *testing.T) {
		testClient := client.New(
			"delete-client",
			"Delete Client",
			"web",
			[]string{"http://localhost:3000/callback"},
		)

		createdClient, err := clientRepo.Create(f.Ctx, testClient)
		require.NoError(t, err)

		err = clientRepo.Delete(f.Ctx, createdClient.ID())
		require.NoError(t, err)

		_, err = clientRepo.GetByID(f.Ctx, createdClient.ID())
		assert.Error(t, err)
	})

	t.Run("NotFound", func(t *testing.T) {
		err := clientRepo.Delete(f.Ctx, uuid.New())
		assert.Error(t, err)
	})
}

func TestClientRepository_ClientIDExists(t *testing.T) {
	t.Parallel()
	f := setupTest(t)

	clientRepo := persistence.NewClientRepository()

	t.Run("Exists", func(t *testing.T) {
		testClient := client.New(
			"exists-client",
			"Exists Client",
			"web",
			[]string{"http://localhost:3000/callback"},
		)

		_, err := clientRepo.Create(f.Ctx, testClient)
		require.NoError(t, err)

		exists, err := clientRepo.ClientIDExists(f.Ctx, "exists-client")
		require.NoError(t, err)
		assert.True(t, exists)
	})

	t.Run("NotExists", func(t *testing.T) {
		exists, err := clientRepo.ClientIDExists(f.Ctx, "non-existent-client")
		require.NoError(t, err)
		assert.False(t, exists)
	})
}

func TestClientRepository_GetPaginated(t *testing.T) {
	t.Parallel()
	f := setupTest(t)

	clientRepo := persistence.NewClientRepository()

	// Create test clients
	for i := 1; i <= 5; i++ {
		testClient := client.New(
			uuid.New().String(),
			"Test Client",
			"web",
			[]string{"http://localhost:3000/callback"},
		)
		_, err := clientRepo.Create(f.Ctx, testClient)
		require.NoError(t, err)
	}

	t.Run("WithPagination", func(t *testing.T) {
		params := &client.FindParams{
			SortBy: client.SortBy{
				Fields: []repo.SortByField[client.Field]{
					{Field: client.CreatedAtField, Ascending: false},
				},
			},
			Limit:  3,
			Offset: 0,
		}

		clients, err := clientRepo.GetPaginated(f.Ctx, params)
		require.NoError(t, err)
		assert.LessOrEqual(t, len(clients), 3)
	})

	t.Run("WithSearch", func(t *testing.T) {
		// Create a client with specific name for searching
		searchClient := client.New(
			"search-test-client",
			"Searchable Client Name",
			"web",
			[]string{"http://localhost:3000/callback"},
		)
		_, err := clientRepo.Create(f.Ctx, searchClient)
		require.NoError(t, err)

		params := &client.FindParams{
			Search: "Searchable",
			SortBy: client.SortBy{
				Fields: []repo.SortByField[client.Field]{
					{Field: client.NameField, Ascending: true},
				},
			},
			Limit:  10,
			Offset: 0,
		}

		clients, err := clientRepo.GetPaginated(f.Ctx, params)
		require.NoError(t, err)
		assert.Greater(t, len(clients), 0)

		// Verify at least one result matches
		found := false
		for _, c := range clients {
			if c.ClientID() == "search-test-client" {
				found = true
				break
			}
		}
		assert.True(t, found)
	})
}

func TestClientRepository_Count(t *testing.T) {
	t.Parallel()
	f := setupTest(t)

	clientRepo := persistence.NewClientRepository()

	// Create test clients
	initialCount, err := clientRepo.Count(f.Ctx, &client.FindParams{})
	require.NoError(t, err)

	for i := 1; i <= 3; i++ {
		testClient := client.New(
			uuid.New().String(),
			"Count Test Client",
			"web",
			[]string{"http://localhost:3000/callback"},
		)
		_, err := clientRepo.Create(f.Ctx, testClient)
		require.NoError(t, err)
	}

	newCount, err := clientRepo.Count(f.Ctx, &client.FindParams{})
	require.NoError(t, err)
	assert.Equal(t, initialCount+3, newCount)
}

func TestClientRepository_GetAll(t *testing.T) {
	t.Parallel()
	f := setupTest(t)

	clientRepo := persistence.NewClientRepository()

	// Create some active clients
	for i := 1; i <= 3; i++ {
		testClient := client.New(
			uuid.New().String(),
			"Active Client",
			"web",
			[]string{"http://localhost:3000/callback"},
		)
		_, err := clientRepo.Create(f.Ctx, testClient)
		require.NoError(t, err)
	}

	// Create an inactive client
	inactiveClient := client.New(
		uuid.New().String(),
		"Inactive Client",
		"web",
		[]string{"http://localhost:3000/callback"},
	)
	createdInactive, err := clientRepo.Create(f.Ctx, inactiveClient)
	require.NoError(t, err)

	// Deactivate the client
	deactivatedClient := createdInactive.Deactivate()
	err = clientRepo.Update(f.Ctx, deactivatedClient)
	require.NoError(t, err)

	// GetAll should only return active clients
	allClients, err := clientRepo.GetAll(f.Ctx)
	require.NoError(t, err)

	// Verify all returned clients are active
	for _, c := range allClients {
		assert.True(t, c.IsActive())
	}
}

func TestClientRepository_ActivateDeactivate(t *testing.T) {
	t.Parallel()
	f := setupTest(t)

	clientRepo := persistence.NewClientRepository()

	t.Run("Activate", func(t *testing.T) {
		testClient := client.New(
			"activate-client",
			"Activate Client",
			"web",
			[]string{"http://localhost:3000/callback"},
			client.WithIsActive(false),
		)

		createdClient, err := clientRepo.Create(f.Ctx, testClient)
		require.NoError(t, err)
		assert.False(t, createdClient.IsActive())

		// Activate
		activatedClient := createdClient.Activate()
		err = clientRepo.Update(f.Ctx, activatedClient)
		require.NoError(t, err)

		retrievedClient, err := clientRepo.GetByID(f.Ctx, createdClient.ID())
		require.NoError(t, err)
		assert.True(t, retrievedClient.IsActive())
	})

	t.Run("Deactivate", func(t *testing.T) {
		testClient := client.New(
			"deactivate-client",
			"Deactivate Client",
			"web",
			[]string{"http://localhost:3000/callback"},
		)

		createdClient, err := clientRepo.Create(f.Ctx, testClient)
		require.NoError(t, err)
		assert.True(t, createdClient.IsActive())

		// Deactivate
		deactivatedClient := createdClient.Deactivate()
		err = clientRepo.Update(f.Ctx, deactivatedClient)
		require.NoError(t, err)

		retrievedClient, err := clientRepo.GetByID(f.Ctx, createdClient.ID())
		require.NoError(t, err)
		assert.False(t, retrievedClient.IsActive())
	})
}

func TestClientRepository_TokenLifetimes(t *testing.T) {
	t.Parallel()
	f := setupTest(t)

	clientRepo := persistence.NewClientRepository()

	customAccessLifetime := 2 * time.Hour
	customIDLifetime := 30 * time.Minute
	customRefreshLifetime := 168 * time.Hour // 7 days

	testClient := client.New(
		"lifetime-client",
		"Lifetime Client",
		"web",
		[]string{"http://localhost:3000/callback"},
		client.WithAccessTokenLifetime(customAccessLifetime),
		client.WithIDTokenLifetime(customIDLifetime),
		client.WithRefreshTokenLifetime(customRefreshLifetime),
	)

	createdClient, err := clientRepo.Create(f.Ctx, testClient)
	require.NoError(t, err)

	assert.Equal(t, customAccessLifetime, createdClient.AccessTokenLifetime())
	assert.Equal(t, customIDLifetime, createdClient.IDTokenLifetime())
	assert.Equal(t, customRefreshLifetime, createdClient.RefreshTokenLifetime())
}
