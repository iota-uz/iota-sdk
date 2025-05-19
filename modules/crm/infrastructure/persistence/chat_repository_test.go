package persistence_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	corepersistence "github.com/iota-uz/iota-sdk/modules/core/infrastructure/persistence"
	"github.com/iota-uz/iota-sdk/modules/crm/domain/aggregates/chat"
	"github.com/iota-uz/iota-sdk/modules/crm/domain/aggregates/client"
	"github.com/iota-uz/iota-sdk/modules/crm/infrastructure/persistence"
)

// Create a client and return its ID to use in chat tests
func createClientForTest(t *testing.T, f *testFixtures) client.Client {
	t.Helper()
	repo := persistence.NewClientRepository(
		corepersistence.NewPassportRepository(),
	)

	testClient := createTestClient(t, f.tenant.ID, false)
	created, err := repo.Save(f.ctx, testClient)
	require.NoError(t, err, "Failed to create client for chat test")

	return created
}

func TestChatRepository_Create(t *testing.T) {
	t.Parallel()
	f := setupTest(t)
	repo := persistence.NewChatRepository()

	t.Run("Create chat without messages", func(t *testing.T) {
		clientID := createClientForTest(t, f).ID()
		testChat := chat.New(clientID, chat.WithTenantID(f.tenant.ID))

		created, err := repo.Save(f.ctx, testChat)
		require.NoError(t, err, "Failed to create chat")

		assert.NotZero(t, created.ID(), "Created chat should have a non-zero ID")
		assert.Equal(t, clientID, created.ClientID(), "ClientID should match")

		messages := created.Messages()
		assert.Empty(t, messages, "Expected no messages")
	})

	t.Run("Create chat with messages", func(t *testing.T) {
		client_ := createClientForTest(t, f)
		testChat := chat.New(client_.ID(), chat.WithTenantID(f.tenant.ID))
		member := chat.NewMember(
			chat.NewClientSender(
				client_.ID(),
				client_.Contacts()[0].ID(),
				"1234567890",
				"1234567890",
			),
			chat.TelegramTransport,
			chat.WithMemberTenantID(f.tenant.ID),
		)

		// Add a message to the chat
		message := chat.NewMessage("Hello, world!", member)
		testChat = testChat.AddMessage(message)

		created, err := repo.Save(f.ctx, testChat)
		require.NoError(t, err, "Failed to create chat with message")

		assert.NotZero(t, created.ID(), "Created chat should have a non-zero ID")

		messages := created.Messages()
		require.Len(t, messages, 1, "Expected 1 message")
		assert.Equal(t, "Hello, world!", messages[0].Message(), "Message content should match")
	})
}

func TestChatRepository_GetByID(t *testing.T) {
	t.Parallel()
	f := setupTest(t)
	repo := persistence.NewChatRepository()

	clientID := createClientForTest(t, f).ID()
	testChat := chat.New(clientID, chat.WithTenantID(f.tenant.ID))
	created, err := repo.Save(f.ctx, testChat)
	require.NoError(t, err, "Failed to create test chat")

	t.Run("Get existing chat by ID", func(t *testing.T) {
		retrieved, err := repo.GetByID(f.ctx, created.ID())
		require.NoError(t, err, "Failed to get chat by ID")

		assert.Equal(t, created.ID(), retrieved.ID(), "Chat ID should match")
		assert.Equal(t, created.ClientID(), retrieved.ClientID(), "ClientID should match")
	})

	t.Run("Get non-existent chat by ID", func(t *testing.T) {
		_, err := repo.GetByID(f.ctx, 9999)

		require.Error(t, err, "Expected error when getting non-existent chat")
		assert.ErrorIs(t, err, persistence.ErrChatNotFound, "Error should be ErrNotFound")
	})
}

func TestChatRepository_GetByClientID(t *testing.T) {
	t.Parallel()
	f := setupTest(t)
	repo := persistence.NewChatRepository()

	clientID := createClientForTest(t, f).ID()
	testChat := chat.New(clientID, chat.WithTenantID(f.tenant.ID))
	created, err := repo.Save(f.ctx, testChat)
	require.NoError(t, err, "Failed to create test chat")

	t.Run("Get existing chat by client ID", func(t *testing.T) {
		retrieved, err := repo.GetByClientID(f.ctx, clientID)
		require.NoError(t, err, "Failed to get chat by client ID")

		assert.Equal(t, created.ID(), retrieved.ID(), "Chat ID should match")
		assert.Equal(t, clientID, retrieved.ClientID(), "ClientID should match")
	})

	t.Run("Get non-existent chat by client ID", func(t *testing.T) {
		_, err := repo.GetByClientID(f.ctx, 9999)

		require.Error(t, err, "Expected error when getting non-existent chat")
		assert.ErrorIs(t, err, persistence.ErrChatNotFound, "Error should be ErrChatNotFound")
	})
}

// Note: We're not testing AddMessage since it's not part of the public Repository interface.
// Instead, we test the functionality of adding a message through Update method.
func TestChatRepository_AddMessageThroughUpdate(t *testing.T) {
	t.Parallel()
	f := setupTest(t)
	repo := persistence.NewChatRepository()

	// Create a chat first
	client_ := createClientForTest(t, f)
	testChat := chat.New(client_.ID(), chat.WithTenantID(f.tenant.ID))
	created, err := repo.Save(f.ctx, testChat)
	require.NoError(t, err, "Failed to create test chat")

	// Add a message to the chat domain entity
	message := chat.NewMessage("Test message", chat.NewMember(
		chat.NewClientSender(
			client_.ID(),
			client_.Contacts()[0].ID(),
			client_.FirstName(),
			client_.LastName(),
		),
		chat.TelegramTransport,
		chat.WithMemberTenantID(f.tenant.ID),
	))
	updatedChat := created.AddMessage(message)

	// Update the chat with the new message
	result, err := repo.Save(f.ctx, updatedChat)
	require.NoError(t, err, "Failed to update chat with new message")

	// Verify the message was added
	messages := result.Messages()
	require.Len(t, messages, 1, "Expected 1 message in chat")
	assert.Equal(t, "Test message", messages[0].Message(), "Message content should match")
}

func TestChatRepository_Update(t *testing.T) {
	t.Parallel()
	f := setupTest(t)
	repo := persistence.NewChatRepository()

	// Create a chat first
	testClient := createClientForTest(t, f).AddContact(client.NewContact(
		client.ContactTypeEmail,
		"test@gmail.com",
	))
	clientRepo := persistence.NewClientRepository(
		corepersistence.NewPassportRepository(),
	)
	_, err := clientRepo.Save(f.ctx, testClient)
	require.NoError(t, err, "Failed to save test client")

	testChat := chat.New(testClient.ID(), chat.WithTenantID(f.tenant.ID))
	created, err := repo.Save(f.ctx, testChat)
	require.NoError(t, err, "Failed to create test chat")

	// Add a message
	message := chat.NewMessage("Original message", chat.NewMember(
		chat.NewClientSender(
			testClient.ID(),
			testClient.Contacts()[0].ID(),
			testClient.FirstName(),
			testClient.LastName(),
		),
		chat.TelegramTransport,
	))
	updatedChat := created.AddMessage(message)

	// Update the chat
	updated, err := repo.Save(f.ctx, updatedChat)
	require.NoError(t, err, "Failed to update chat")

	// Verify the chat was updated
	messages := updated.Messages()
	require.Len(t, messages, 1, "Expected 1 message in chat")
	assert.Equal(t, "Original message", messages[0].Message(), "Message content should match")

	// Add another message and update again
	secondMessage := chat.NewMessage("Second message", chat.NewMember(
		chat.NewClientSender(
			testClient.ID(),
			testClient.Contacts()[1].ID(),
			testClient.FirstName(),
			testClient.LastName(),
		),
		chat.TelegramTransport,
		chat.WithMemberTenantID(f.tenant.ID),
	))
	chatWithTwoMessages := updated.AddMessage(secondMessage)

	secondUpdate, err := repo.Save(f.ctx, chatWithTwoMessages)
	require.NoError(t, err, "Failed to update chat with second message")

	// Verify both messages are there
	updatedMessages := secondUpdate.Messages()
	assert.Len(t, updatedMessages, 2, "Expected 2 messages in chat")
}

func TestChatRepository_GetPaginated(t *testing.T) {
	t.Parallel()
	f := setupTest(t)
	chatRepo := persistence.NewChatRepository()

	for i := 0; i < 5; i++ {
		testClient := createClientForTest(t, f)
		testChat := chat.New(testClient.ID(), chat.WithTenantID(f.tenant.ID))

		// Add a message with the client's ID in it
		message := chat.NewMessage("Message for client "+string('0'+byte(i)), chat.NewMember(
			chat.NewClientSender(
				testClient.ID(),
				testClient.Contacts()[0].ID(),
				testClient.FirstName(),
				testClient.LastName(),
			),
			chat.TelegramTransport,
		))
		testChat = testChat.AddMessage(message)

		_, err := chatRepo.Save(f.ctx, testChat)
		require.NoError(t, err, "Failed to create test chat %d", i)
	}

	t.Run("Get chats with limit and offset", func(t *testing.T) {
		params := &chat.FindParams{
			Limit:  2,
			Offset: 1,
			SortBy: chat.SortBy{
				Fields: []chat.SortByField{
					{
						Field:     chat.CreatedAtField,
						Ascending: true,
					},
				},
			},
		}

		chats, err := chatRepo.GetPaginated(f.ctx, params)
		require.NoError(t, err, "Failed to get paginated chats")
		assert.Len(t, chats, 2, "Expected 2 chats")
	})

	t.Run("Get chats sorted by last message date", func(t *testing.T) {
		params := &chat.FindParams{
			Limit:  5,
			Offset: 0,
			SortBy: chat.SortBy{
				Fields: []chat.SortByField{
					{
						Field:     chat.LastMessageAtField,
						Ascending: false,
					},
				},
			},
		}

		chats, err := chatRepo.GetPaginated(f.ctx, params)
		require.NoError(t, err, "Failed to get paginated chats")
		require.NotEmpty(t, chats, "Expected at least one chat")

		// We should have chats sorted by last message date DESC
		if len(chats) >= 2 {
			if chats[0].LastMessageAt() == nil || chats[1].LastMessageAt() == nil {
				t.Skip("Last message timestamps not available for comparison")
			}

			assert.False(t, chats[0].LastMessageAt().Before(*chats[1].LastMessageAt()),
				"Expected chats to be sorted by last message date DESC")
		}
	})
}

func TestChatRepository_Count(t *testing.T) {
	t.Parallel()
	f := setupTest(t)
	repo := persistence.NewChatRepository()

	// Get initial count
	initialCount, err := repo.Count(f.ctx)
	require.NoError(t, err, "Failed to get initial chat count")

	// Create a few chats
	numChats := 3
	for i := 0; i < numChats; i++ {
		clientID := createClientForTest(t, f).ID()
		testChat := chat.New(clientID, chat.WithTenantID(f.tenant.ID))
		_, err := repo.Save(f.ctx, testChat)
		require.NoError(t, err, "Failed to create test chat %d", i)
	}

	// Get new count
	newCount, err := repo.Count(f.ctx)
	require.NoError(t, err, "Failed to get updated chat count")

	expectedCount := initialCount + int64(numChats)
	assert.Equal(t, expectedCount, newCount, "Chat count should match expected value")
}

func TestChatRepository_GetAll(t *testing.T) {
	t.Parallel()
	f := setupTest(t)
	repo := persistence.NewChatRepository()

	// Create a known number of chats
	initialCount, err := repo.Count(f.ctx)
	require.NoError(t, err, "Failed to get initial chat count")

	numNewChats := 2
	for i := 0; i < numNewChats; i++ {
		clientID := createClientForTest(t, f).ID()
		testChat := chat.New(clientID, chat.WithTenantID(f.tenant.ID))
		_, err := repo.Save(f.ctx, testChat)
		require.NoError(t, err, "Failed to create test chat %d", i)
	}

	// Get all chats
	allChats, err := repo.GetPaginated(f.ctx, &chat.FindParams{})
	require.NoError(t, err, "Failed to get all chats")

	expectedCount := int(initialCount) + numNewChats
	assert.Len(t, allChats, expectedCount, "Number of chats should match expected count")
}

func TestChatRepository_Delete(t *testing.T) {
	t.Parallel()
	f := setupTest(t)
	repo := persistence.NewChatRepository()

	// Create a chat
	testClient := createClientForTest(t, f)
	testChat := chat.New(testClient.ID(), chat.WithTenantID(f.tenant.ID))

	// Add a message to test cascade deletion
	message := chat.NewMessage("Message to be deleted", chat.NewMember(
		chat.NewClientSender(
			testClient.ID(),
			testClient.Contacts()[0].ID(),
			testClient.FirstName(),
			testClient.LastName(),
		),
		chat.TelegramTransport,
		chat.WithMemberTenantID(f.tenant.ID),
	))
	testChat = testChat.AddMessage(message)

	created, err := repo.Save(f.ctx, testChat)
	require.NoError(t, err, "Failed to create test chat")

	// Delete the chat
	err = repo.Delete(f.ctx, created.ID())
	require.NoError(t, err, "Failed to delete chat")

	// Verify chat was deleted
	_, err = repo.GetByID(f.ctx, created.ID())
	require.Error(t, err, "Expected error when getting deleted chat")
	assert.ErrorIs(t, err, persistence.ErrChatNotFound, "Error should be ErrChatNotFound")
}

func TestChatRepository_Search(t *testing.T) {
	t.Parallel()
	f := setupTest(t)
	chatRepo := persistence.NewChatRepository()

	for i := 0; i < 5; i++ {
		testClient := createClientForTest(t, f)
		testChat := chat.New(testClient.ID(), chat.WithTenantID(f.tenant.ID))
		_, err := chatRepo.Save(f.ctx, testChat)
		require.NoError(t, err, "Failed to create test chat for client %d", i)
	}

	t.Run("Search by first name", func(t *testing.T) {
		params := &chat.FindParams{
			Limit:  10,
			Offset: 0,
			Search: "John",
			SortBy: chat.SortBy{
				Fields: []chat.SortByField{
					{
						Field:     chat.CreatedAtField,
						Ascending: true,
					},
				},
			},
		}

		_, err := chatRepo.GetPaginated(f.ctx, params)
		require.NoError(t, err, "Failed to search chats by first name")
	})

	t.Run("Search by last name", func(t *testing.T) {
		params := &chat.FindParams{
			Limit:  10,
			Offset: 0,
			Search: "Doe",
			SortBy: chat.SortBy{
				Fields: []chat.SortByField{
					{
						Field:     chat.CreatedAtField,
						Ascending: true,
					},
				},
			},
		}

		_, err := chatRepo.GetPaginated(f.ctx, params)
		require.NoError(t, err, "Failed to search chats by last name")
	})

	t.Run("Search by phone number", func(t *testing.T) {
		params := &chat.FindParams{
			Limit:  10,
			Offset: 0,
			Search: "12345",
			SortBy: chat.SortBy{
				Fields: []chat.SortByField{
					{
						Field:     chat.CreatedAtField,
						Ascending: true,
					},
				},
			},
		}

		_, err := chatRepo.GetPaginated(f.ctx, params)
		require.NoError(t, err, "Failed to search chats by phone number")
	})

	t.Run("Search with no matches", func(t *testing.T) {
		params := &chat.FindParams{
			Limit:  10,
			Offset: 0,
			Search: "NonExistentName",
			SortBy: chat.SortBy{
				Fields: []chat.SortByField{{
					Field:     chat.CreatedAtField,
					Ascending: true,
				}},
			},
		}

		chats, err := chatRepo.GetPaginated(f.ctx, params)
		require.NoError(t, err, "Failed to execute search with no expected matches")
		assert.Empty(t, chats, "Expected no chats for non-existent name")
	})
}
