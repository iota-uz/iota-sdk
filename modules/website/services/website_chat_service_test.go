package services_test

import (
	"testing"

	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/user"
	"github.com/iota-uz/iota-sdk/modules/core/domain/value_objects/internet"
	"github.com/iota-uz/iota-sdk/modules/core/domain/value_objects/phone"
	corePersistence "github.com/iota-uz/iota-sdk/modules/core/infrastructure/persistence"
	"github.com/iota-uz/iota-sdk/modules/crm/domain/aggregates/chat"
	"github.com/iota-uz/iota-sdk/modules/crm/domain/aggregates/client"
	crmPersistence "github.com/iota-uz/iota-sdk/modules/crm/infrastructure/persistence"
	"github.com/iota-uz/iota-sdk/modules/website/services"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// setupChatTest extends the setupTest with WebsiteChatService
func setupChatTest(t *testing.T) (*testFixtures, *services.WebsiteChatService, client.Repository) {
	t.Helper()

	fixtures := setupTest(t)

	// Create repositories
	userRepo := corePersistence.NewUserRepository(corePersistence.NewUploadRepository())
	passportRepo := corePersistence.NewPassportRepository()
	clientRepo := crmPersistence.NewClientRepository(passportRepo)
	chatRepo := crmPersistence.NewChatRepository()

	// Create the website chat service
	websiteChatService := services.NewWebsiteChatService(userRepo, clientRepo, chatRepo)

	return fixtures, websiteChatService, clientRepo
}

func TestWebsiteChatService_CreateThread_WithEmail(t *testing.T) {
	t.Parallel()
	fixtures, sut, clientRepo := setupChatTest(t)

	// Test email contact
	emailStr := "test@example.com"

	// Create thread
	thread, err := sut.CreateThread(fixtures.ctx, emailStr)
	require.NoError(t, err)
	require.NotNil(t, thread)

	// Verify thread
	assert.NotZero(t, thread.ID())
	assert.NotEmpty(t, thread.Members())

	// Verify client was created
	email, _ := internet.NewEmail(emailStr)
	client, err := clientRepo.GetByContactValue(fixtures.ctx, client.ContactTypeEmail, email.Value())
	require.NoError(t, err)
	assert.Equal(t, email.Value(), client.Email().Value())

	// Verify thread has correct client ID
	assert.Equal(t, client.ID(), thread.ClientID())
}

func TestWebsiteChatService_CreateThread_WithPhone(t *testing.T) {
	t.Parallel()
	fixtures, sut, clientRepo := setupChatTest(t)

	// Test phone contact
	phoneStr := "+12126647665" // Valid US number format

	// Create thread
	thread, err := sut.CreateThread(fixtures.ctx, phoneStr)
	require.NoError(t, err)
	require.NotNil(t, thread)

	// Verify thread
	assert.NotZero(t, thread.ID())
	assert.NotEmpty(t, thread.Members())

	// Verify client was created
	p, _ := phone.NewFromE164(phoneStr)
	client, err := clientRepo.GetByPhone(fixtures.ctx, p.Value())
	require.NoError(t, err)
	assert.Equal(t, p.Value(), client.Phone().Value())

	// Verify thread has correct client ID
	assert.Equal(t, client.ID(), thread.ClientID())
}

func TestWebsiteChatService_CreateThread_ExistingClient(t *testing.T) {
	t.Parallel()
	fixtures, sut, clientRepo := setupChatTest(t)

	// Create an existing client first
	emailStr := "existing@example.com"
	email, _ := internet.NewEmail(emailStr)
	existingClient, err := client.New(emailStr, client.WithEmail(email))
	require.NoError(t, err)

	savedClient, err := clientRepo.Save(fixtures.ctx, existingClient)
	require.NoError(t, err)

	// Create thread with existing client's email
	thread, err := sut.CreateThread(fixtures.ctx, emailStr)
	require.NoError(t, err)
	require.NotNil(t, thread)

	// Verify the thread has the same client ID as our pre-created client
	assert.Equal(t, savedClient.ID(), thread.ClientID())
}

func TestWebsiteChatService_CreateThread_InvalidContact(t *testing.T) {
	t.Parallel()
	fixtures, sut, _ := setupChatTest(t)

	// Test invalid contact
	invalidContact := "not-an-email-or-phone"

	// Should fail
	_, err := sut.CreateThread(fixtures.ctx, invalidContact)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid contact")
}

func TestWebsiteChatService_CreateThread_WithEmailAndPhone(t *testing.T) {
	t.Parallel()
	fixtures, sut, _ := setupChatTest(t)

	// Test multiple contacts
	tests := []struct {
		name      string
		contact   string
		expectErr bool
	}{
		{
			name:      "Valid email",
			contact:   "test1@example.com",
			expectErr: false,
		},
		{
			name:      "Valid phone",
			contact:   "+12126647667",
			expectErr: false,
		},
		{
			name:      "Invalid contact",
			contact:   "invalid-contact",
			expectErr: true,
		},
		{
			name:      "Empty contact",
			contact:   "",
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			thread, err := sut.CreateThread(fixtures.ctx, tt.contact)

			if tt.expectErr {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.NotNil(t, thread)
				assert.NotZero(t, thread.ID())
			}
		})
	}
}

func TestWebsiteChatService_SendMessageToThread(t *testing.T) {
	t.Parallel()
	fixtures, sut, _ := setupChatTest(t)

	// Create a thread first
	emailStr := "sendmessage@example.com"
	thread, err := sut.CreateThread(fixtures.ctx, emailStr)
	require.NoError(t, err)
	require.NotNil(t, thread)

	// Create message DTO
	dto := services.SendMessageToThreadDTO{
		ChatID:  thread.ID(),
		Message: "Hello from client",
	}

	// Send message
	updatedThread, err := sut.SendMessageToThread(fixtures.ctx, dto)
	require.NoError(t, err)
	require.NotNil(t, updatedThread)

	// Verify message was added
	lastMsg, err := updatedThread.LastMessage()
	require.NoError(t, err)
	assert.Equal(t, "Hello from client", lastMsg.Message())

	// Verify sender is a client
	sender := lastMsg.Sender().Sender()
	clientSender, ok := sender.(chat.ClientSender)
	require.True(t, ok, "Message sender should be a ClientSender")
	assert.Equal(t, thread.ClientID(), clientSender.ClientID())
	assert.Equal(t, chat.WebsiteTransport, clientSender.Transport())
}

func TestWebsiteChatService_SendMessageToThread_EmptyMessage(t *testing.T) {
	t.Parallel()
	fixtures, sut, _ := setupChatTest(t)

	// Create a thread first
	emailStr := "emptymessage@example.com"
	thread, err := sut.CreateThread(fixtures.ctx, emailStr)
	require.NoError(t, err)

	// Try to send empty message
	dto := services.SendMessageToThreadDTO{
		ChatID:  thread.ID(),
		Message: "",
	}

	// Should fail
	_, err = sut.SendMessageToThread(fixtures.ctx, dto)
	require.Error(t, err)
	assert.Equal(t, chat.ErrEmptyMessage, err)
}

func TestWebsiteChatService_ReplyToThread(t *testing.T) {
	t.Parallel()
	fixtures, sut, _ := setupChatTest(t)

	userRepo := corePersistence.NewUserRepository(corePersistence.NewUploadRepository())

	// Create a thread first
	emailStr := "reply@example.com"
	thread, err := sut.CreateThread(fixtures.ctx, emailStr)
	require.NoError(t, err)
	require.NotNil(t, thread)

	createdUser, err := userRepo.Create(fixtures.ctx, user.New(
		"Support",
		"Agent",
		internet.MustParseEmail("test@gmail.com"),
		user.UILanguageEN,
	))
	require.NoError(t, err)

	// Create reply DTO
	dto := services.ReplyToThreadDTO{
		ChatID:  thread.ID(),
		UserID:  createdUser.ID(),
		Message: "Reply from support agent",
	}

	// Send reply
	repliedThread, err := sut.ReplyToThread(fixtures.ctx, dto)
	require.NoError(t, err)
	require.NotNil(t, repliedThread)

	// Verify message was added
	lastMsg, err := repliedThread.LastMessage()
	require.NoError(t, err)
	assert.Equal(t, "Reply from support agent", lastMsg.Message())

	// Verify sender is a user
	sender := lastMsg.Sender().Sender()
	userSender, ok := sender.(chat.UserSender)
	require.True(t, ok, "Message sender should be a UserSender")
	assert.Equal(t, createdUser.ID(), userSender.UserID())
	assert.Equal(t, chat.WebsiteTransport, userSender.Transport())
}

func TestWebsiteChatService_ReplyToThread_EmptyMessage(t *testing.T) {
	t.Parallel()
	fixtures, sut, _ := setupChatTest(t)

	emailStr := "emptyreply@example.com"
	thread, err := sut.CreateThread(fixtures.ctx, emailStr)
	require.NoError(t, err)

	// Should fail
	_, err = sut.ReplyToThread(fixtures.ctx, services.ReplyToThreadDTO{
		ChatID:  thread.ID(),
		UserID:  1,
		Message: "",
	})
	require.Error(t, err)
	require.ErrorIs(t, err, chat.ErrEmptyMessage)
}

func TestWebsiteChatService_ReplyToThread_UserNotFound(t *testing.T) {
	t.Parallel()
	fixtures, sut, _ := setupChatTest(t)

	// Create a thread first
	emailStr := "usernotfound@example.com"
	thread, err := sut.CreateThread(fixtures.ctx, emailStr)
	require.NoError(t, err)

	// Try to reply with non-existent user
	dto := services.ReplyToThreadDTO{
		ChatID:  thread.ID(),
		UserID:  999, // Non-existent user
		Message: "This should fail",
	}

	// Should fail because user is not a member
	_, err = sut.ReplyToThread(fixtures.ctx, dto)
	require.Error(t, err)
	require.ErrorIs(t, err, corePersistence.ErrUserNotFound)
}
