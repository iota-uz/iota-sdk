package persistence_test

import (
	"errors"
	"testing"

	corepersistence "github.com/iota-uz/iota-sdk/modules/core/infrastructure/persistence"
	"github.com/iota-uz/iota-sdk/modules/crm/domain/aggregates/chat"
	"github.com/iota-uz/iota-sdk/modules/crm/infrastructure/persistence"
)

// Create a client and return its ID to use in chat tests
func createClientForTest(t *testing.T, f *testFixtures) uint {
	t.Helper()
	repo := persistence.NewClientRepository(
		corepersistence.NewPassportRepository(),
	)

	testClient := createTestClient(t, false)
	created, err := repo.Create(f.ctx, testClient)
	if err != nil {
		t.Fatalf("Failed to create client for chat test: %v", err)
	}

	return created.ID()
}

func createTestChat(t *testing.T, clientID uint) chat.Chat {
	t.Helper()
	return chat.New(clientID)
}

func createTestMessage(t *testing.T, transport chat.Transport, content string) chat.Message {
	t.Helper()
	var sender chat.Sender

	// Create a client sender
	sender = chat.NewClientSender(transport, 1, "John", "Doe")

	return chat.NewMessage(
		content,
		sender,
	)
}

func TestChatRepository_Create(t *testing.T) {
	t.Parallel()
	f := setupTest(t)
	repo := persistence.NewChatRepository()

	t.Run("Create chat without messages", func(t *testing.T) {
		clientID := createClientForTest(t, f)
		testChat := createTestChat(t, clientID)

		created, err := repo.Create(f.ctx, testChat)
		if err != nil {
			t.Fatalf("Failed to create chat: %v", err)
		}

		if created.ID() == 0 {
			t.Error("Created chat should have a non-zero ID")
		}

		if created.ClientID() != clientID {
			t.Errorf("Expected ClientID to be %d, got %d", clientID, created.ClientID())
		}

		messages := created.Messages()
		if len(messages) != 0 {
			t.Errorf("Expected no messages, got %d", len(messages))
		}
	})

	t.Run("Create chat with messages", func(t *testing.T) {
		clientID := createClientForTest(t, f)
		testChat := createTestChat(t, clientID)

		// Add a message to the chat
		message := createTestMessage(t, chat.TelegramTransport, "Hello, world!")
		testChat = testChat.AddMessage(message)

		created, err := repo.Create(f.ctx, testChat)
		if err != nil {
			t.Fatalf("Failed to create chat with message: %v", err)
		}

		if created.ID() == 0 {
			t.Error("Created chat should have a non-zero ID")
		}

		messages := created.Messages()
		if len(messages) != 1 {
			t.Errorf("Expected 1 message, got %d", len(messages))
		}

		if messages[0].Message() != "Hello, world!" {
			t.Errorf("Expected message content 'Hello, world!', got '%s'", messages[0].Message())
		}
	})
}

func TestChatRepository_GetByID(t *testing.T) {
	t.Parallel()
	f := setupTest(t)
	repo := persistence.NewChatRepository()

	clientID := createClientForTest(t, f)
	testChat := createTestChat(t, clientID)
	created, err := repo.Create(f.ctx, testChat)
	if err != nil {
		t.Fatalf("Failed to create test chat: %v", err)
	}

	t.Run("Get existing chat by ID", func(t *testing.T) {
		retrieved, err := repo.GetByID(f.ctx, created.ID())
		if err != nil {
			t.Fatalf("Failed to get chat by ID: %v", err)
		}

		if retrieved.ID() != created.ID() {
			t.Errorf("Expected ID %d, got %d", created.ID(), retrieved.ID())
		}

		if retrieved.ClientID() != created.ClientID() {
			t.Errorf("Expected ClientID %d, got %d", created.ClientID(), retrieved.ClientID())
		}
	})

	t.Run("Get non-existent chat by ID", func(t *testing.T) {
		_, err := repo.GetByID(f.ctx, 9999)
		if err == nil {
			t.Fatal("Expected error when getting non-existent chat, got nil")
		}

		if !errors.Is(err, persistence.ErrChatNotFound) {
			t.Errorf("Expected ErrChatNotFound, got %v", err)
		}
	})
}

func TestChatRepository_GetByClientID(t *testing.T) {
	t.Parallel()
	f := setupTest(t)
	repo := persistence.NewChatRepository()

	clientID := createClientForTest(t, f)
	testChat := createTestChat(t, clientID)
	created, err := repo.Create(f.ctx, testChat)
	if err != nil {
		t.Fatalf("Failed to create test chat: %v", err)
	}

	t.Run("Get existing chat by client ID", func(t *testing.T) {
		retrieved, err := repo.GetByClientID(f.ctx, clientID)
		if err != nil {
			t.Fatalf("Failed to get chat by client ID: %v", err)
		}

		if retrieved.ID() != created.ID() {
			t.Errorf("Expected ID %d, got %d", created.ID(), retrieved.ID())
		}

		if retrieved.ClientID() != clientID {
			t.Errorf("Expected ClientID %d, got %d", clientID, retrieved.ClientID())
		}
	})

	t.Run("Get non-existent chat by client ID", func(t *testing.T) {
		_, err := repo.GetByClientID(f.ctx, 9999)
		if err == nil {
			t.Fatal("Expected error when getting non-existent chat, got nil")
		}

		if !errors.Is(err, persistence.ErrChatNotFound) {
			t.Errorf("Expected ErrChatNotFound, got %v", err)
		}
	})
}

// Note: We're not testing AddMessage since it's not part of the public Repository interface.
// Instead, we test the functionality of adding a message through Update method.
func TestChatRepository_AddMessageThroughUpdate(t *testing.T) {
	t.Parallel()
	f := setupTest(t)
	repo := persistence.NewChatRepository()

	// Create a chat first
	clientID := createClientForTest(t, f)
	testChat := createTestChat(t, clientID)
	created, err := repo.Create(f.ctx, testChat)
	if err != nil {
		t.Fatalf("Failed to create test chat: %v", err)
	}

	// Add a message to the chat domain entity
	message := createTestMessage(t, chat.TelegramTransport, "Test message")
	updatedChat := created.AddMessage(message)

	// Update the chat with the new message
	result, err := repo.Update(f.ctx, updatedChat)
	if err != nil {
		t.Fatalf("Failed to update chat with new message: %v", err)
	}

	// Verify the message was added
	messages := result.Messages()
	if len(messages) != 1 {
		t.Errorf("Expected 1 message in chat, got %d", len(messages))
	}

	if len(messages) > 0 && messages[0].Message() != "Test message" {
		t.Errorf("Expected message content 'Test message', got '%s'", messages[0].Message())
	}
}

func TestChatRepository_Update(t *testing.T) {
	t.Parallel()
	f := setupTest(t)
	repo := persistence.NewChatRepository()

	// Create a chat first
	clientID := createClientForTest(t, f)
	testChat := createTestChat(t, clientID)
	created, err := repo.Create(f.ctx, testChat)
	if err != nil {
		t.Fatalf("Failed to create test chat: %v", err)
	}

	// Add a message
	message := createTestMessage(t, chat.SMSTransport, "Original message")
	updatedChat := created.AddMessage(message)

	// Update the chat
	updated, err := repo.Update(f.ctx, updatedChat)
	if err != nil {
		t.Fatalf("Failed to update chat: %v", err)
	}

	// Verify the chat was updated
	messages := updated.Messages()
	if len(messages) != 1 {
		t.Errorf("Expected 1 message in chat, got %d", len(messages))
	}

	if len(messages) > 0 && messages[0].Message() != "Original message" {
		t.Errorf("Expected message content 'Original message', got '%s'", messages[0].Message())
	}

	// Add another message and update again
	secondMessage := createTestMessage(t, chat.EmailTransport, "Second message")
	chatWithTwoMessages := updated.AddMessage(secondMessage)

	secondUpdate, err := repo.Update(f.ctx, chatWithTwoMessages)
	if err != nil {
		t.Fatalf("Failed to update chat with second message: %v", err)
	}

	// Verify both messages are there
	updatedMessages := secondUpdate.Messages()
	if len(updatedMessages) != 2 {
		t.Errorf("Expected 2 messages in chat, got %d", len(updatedMessages))
	}
}

func TestChatRepository_GetPaginated(t *testing.T) {
	t.Parallel()
	f := setupTest(t)
	repo := persistence.NewChatRepository()

	// Create multiple chats for pagination testing
	for i := 0; i < 5; i++ {
		clientID := createClientForTest(t, f)
		testChat := createTestChat(t, clientID)

		// Add a message with the client's ID in it
		message := createTestMessage(t, chat.TelegramTransport, "Message for client "+string('0'+byte(i)))
		testChat = testChat.AddMessage(message)

		_, err := repo.Create(f.ctx, testChat)
		if err != nil {
			t.Fatalf("Failed to create test chat %d: %v", i, err)
		}
	}

	t.Run("Get chats with limit and offset", func(t *testing.T) {
		params := &chat.FindParams{
			Limit:  2,
			Offset: 1,
			SortBy: chat.SortBy{
				Fields:    []chat.Field{chat.CreatedAt},
				Ascending: true,
			},
		}

		chats, err := repo.GetPaginated(f.ctx, params)
		if err != nil {
			t.Fatalf("Failed to get paginated chats: %v", err)
		}

		if len(chats) != 2 {
			t.Errorf("Expected 2 chats, got %d", len(chats))
		}
	})

	t.Run("Get chats sorted by last message date", func(t *testing.T) {
		params := &chat.FindParams{
			Limit:  5,
			Offset: 0,
			SortBy: chat.SortBy{
				Fields:    []chat.Field{chat.LastMessageAt},
				Ascending: false,
			},
		}

		chats, err := repo.GetPaginated(f.ctx, params)
		if err != nil {
			t.Fatalf("Failed to get paginated chats: %v", err)
		}

		if len(chats) == 0 {
			t.Fatalf("Expected at least one chat, got 0")
		}

		// We should have chats sorted by last message date DESC
		if len(chats) >= 2 {
			if chats[0].LastMessageAt() == nil || chats[1].LastMessageAt() == nil {
				t.Skip("Last message timestamps not available for comparison")
			}

			if chats[0].LastMessageAt().Before(*chats[1].LastMessageAt()) {
				t.Errorf("Expected chats to be sorted by last message date DESC")
			}
		}
	})
}

func TestChatRepository_Count(t *testing.T) {
	t.Parallel()
	f := setupTest(t)
	repo := persistence.NewChatRepository()

	// Get initial count
	initialCount, err := repo.Count(f.ctx)
	if err != nil {
		t.Fatalf("Failed to get initial chat count: %v", err)
	}

	// Create a few chats
	numChats := 3
	for i := 0; i < numChats; i++ {
		clientID := createClientForTest(t, f)
		testChat := createTestChat(t, clientID)
		_, err := repo.Create(f.ctx, testChat)
		if err != nil {
			t.Fatalf("Failed to create test chat %d: %v", i, err)
		}
	}

	// Get new count
	newCount, err := repo.Count(f.ctx)
	if err != nil {
		t.Fatalf("Failed to get updated chat count: %v", err)
	}

	expectedCount := initialCount + int64(numChats)
	if newCount != expectedCount {
		t.Errorf("Expected count to be %d, got %d", expectedCount, newCount)
	}
}

func TestChatRepository_GetAll(t *testing.T) {
	t.Parallel()
	f := setupTest(t)
	repo := persistence.NewChatRepository()

	// Create a known number of chats
	initialCount, err := repo.Count(f.ctx)
	if err != nil {
		t.Fatalf("Failed to get initial chat count: %v", err)
	}

	numNewChats := 2
	for i := 0; i < numNewChats; i++ {
		clientID := createClientForTest(t, f)
		testChat := createTestChat(t, clientID)
		_, err := repo.Create(f.ctx, testChat)
		if err != nil {
			t.Fatalf("Failed to create test chat %d: %v", i, err)
		}
	}

	// Get all chats
	allChats, err := repo.GetAll(f.ctx)
	if err != nil {
		t.Fatalf("Failed to get all chats: %v", err)
	}

	expectedCount := int(initialCount) + numNewChats
	if len(allChats) != expectedCount {
		t.Errorf("Expected %d chats, got %d", expectedCount, len(allChats))
	}
}

func TestChatRepository_Delete(t *testing.T) {
	t.Parallel()
	f := setupTest(t)
	repo := persistence.NewChatRepository()

	// Create a chat
	clientID := createClientForTest(t, f)
	testChat := createTestChat(t, clientID)

	// Add a message to test cascade deletion
	message := createTestMessage(t, chat.TelegramTransport, "Message to be deleted")
	testChat = testChat.AddMessage(message)

	created, err := repo.Create(f.ctx, testChat)
	if err != nil {
		t.Fatalf("Failed to create test chat: %v", err)
	}

	// Delete the chat
	err = repo.Delete(f.ctx, created.ID())
	if err != nil {
		t.Fatalf("Failed to delete chat: %v", err)
	}

	// Verify chat was deleted
	_, err = repo.GetByID(f.ctx, created.ID())
	if err == nil {
		t.Error("Expected error when getting deleted chat, got nil")
	}

	if !errors.Is(err, persistence.ErrChatNotFound) {
		t.Errorf("Expected ErrChatNotFound, got %v", err)
	}
}
