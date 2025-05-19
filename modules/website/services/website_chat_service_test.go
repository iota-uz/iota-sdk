package services_test

import (
	"testing"

	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/user"
	"github.com/iota-uz/iota-sdk/modules/core/domain/value_objects/country"
	"github.com/iota-uz/iota-sdk/modules/core/domain/value_objects/internet"
	"github.com/iota-uz/iota-sdk/modules/core/domain/value_objects/phone"
	corePersistence "github.com/iota-uz/iota-sdk/modules/core/infrastructure/persistence"
	"github.com/iota-uz/iota-sdk/modules/crm/domain/aggregates/chat"
	"github.com/iota-uz/iota-sdk/modules/crm/domain/aggregates/client"
	crmPersistence "github.com/iota-uz/iota-sdk/modules/crm/infrastructure/persistence"
	"github.com/iota-uz/iota-sdk/modules/website/infrastructure/persistence"
	"github.com/iota-uz/iota-sdk/modules/website/services"
	"github.com/iota-uz/iota-sdk/pkg/composables"
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
	aiconfigRepo := persistence.NewAIChatConfigRepository()

	// Create the website chat service
	websiteChatService := services.NewWebsiteChatService(services.WebsiteChatServiceConfig{
		AIConfigRepo: aiconfigRepo,
		UserRepo:     userRepo,
		ClientRepo:   clientRepo,
		ChatRepo:     chatRepo,
		AIUserEmail:  internet.MustParseEmail("ai@example.com"),
	})

	return fixtures, websiteChatService, clientRepo
}

// Email-based test removed as the service now only supports phone numbers

func TestWebsiteChatService_CreateThread_WithPhone(t *testing.T) {
	t.Parallel()
	chatRepo := crmPersistence.NewChatRepository()
	fixtures, sut, clientRepo := setupChatTest(t)

	// Test phone contact
	phoneStr := "+12126647665" // Valid US number format

	// Create thread
	thread, err := sut.CreateThread(fixtures.ctx, services.CreateThreadDTO{
		Phone:   phoneStr,
		Country: country.UnitedStates,
	})
	require.NoError(t, err)
	require.NotNil(t, thread)

	// Verify thread
	assert.NotZero(t, thread.ID())
	chatEntity, err := chatRepo.GetByID(fixtures.ctx, thread.ChatID())
	require.NoError(t, err)
	assert.NotEmpty(t, chatEntity.Members())

	// Verify client was created
	p, _ := phone.NewFromE164(phoneStr)
	client, err := clientRepo.GetByPhone(fixtures.ctx, p.Value())
	require.NoError(t, err)
	assert.Equal(t, p.Value(), client.Phone().Value())

	// Verify thread has correct client ID
	assert.Equal(t, client.ID(), chatEntity.ClientID())
}

func TestWebsiteChatService_CreateThread_ExistingClient(t *testing.T) {
	t.Parallel()
	chatRepo := crmPersistence.NewChatRepository()
	fixtures, sut, clientRepo := setupChatTest(t)

	// Get tenant ID for the client
	tenant, err := composables.UseTenant(fixtures.ctx)
	require.NoError(t, err)

	// Create an existing client first with phone
	phoneStr := "+12126647668" // Valid US number format
	p, err := phone.Parse(phoneStr, country.UnitedStates)
	require.NoError(t, err)

	existingClient, err := client.New(
		phoneStr,
		client.WithPhone(p),
		client.WithTenantID(tenant.ID),
	)
	require.NoError(t, err)

	savedClient, err := clientRepo.Save(fixtures.ctx, existingClient)
	require.NoError(t, err)

	// Create thread with existing client's phone
	thread, err := sut.CreateThread(fixtures.ctx, services.CreateThreadDTO{
		Phone:   phoneStr,
		Country: country.UnitedStates,
	})
	require.NoError(t, err)
	require.NotNil(t, thread)

	chatEntity, err := chatRepo.GetByID(fixtures.ctx, thread.ChatID())
	require.NoError(t, err)
	assert.Equal(t, savedClient.ID(), chatEntity.ClientID())
}

func TestWebsiteChatService_CreateThread_NewThreadEachTime(t *testing.T) {
	t.Parallel()
	chatRepo := crmPersistence.NewChatRepository()
	fixtures, sut, clientRepo := setupChatTest(t)

	// Get tenant ID for the client
	tenant, err := composables.UseTenant(fixtures.ctx)
	require.NoError(t, err)

	// 1. Create a client and get the client ID
	phoneStr := "+12126647669" // Valid US number format
	p, err := phone.Parse(phoneStr, country.UnitedStates)
	require.NoError(t, err)

	existingClient, err := client.New(phoneStr,
		client.WithPhone(p),
		client.WithTenantID(tenant.ID),
	)
	require.NoError(t, err)

	savedClient, err := clientRepo.Save(fixtures.ctx, existingClient)
	require.NoError(t, err)

	// 2. Create first thread with the client's phone
	firstThread, err := sut.CreateThread(fixtures.ctx, services.CreateThreadDTO{
		Phone:   phoneStr,
		Country: country.UnitedStates,
	})
	require.NoError(t, err)
	require.NotNil(t, firstThread)

	// 3. Create a second thread with the same phone
	secondThread, err := sut.CreateThread(fixtures.ctx, services.CreateThreadDTO{
		Phone:   phoneStr,
		Country: country.UnitedStates,
	})
	require.NoError(t, err)
	require.NotNil(t, secondThread)

	chatEntity, err := chatRepo.GetByID(fixtures.ctx, secondThread.ChatID())
	require.NoError(t, err)
	// 4. Verify both threads have different IDs but same client ID
	assert.NotEqual(t, firstThread.ID(), secondThread.ID(), "Creating a thread with the same phone should create a new thread")
	assert.Equal(t, savedClient.ID(), chatEntity.ClientID(), "Thread should be associated with the correct client")
}

func TestWebsiteChatService_CreateThread_InvalidPhone(t *testing.T) {
	t.Parallel()
	fixtures, sut, _ := setupChatTest(t)

	// Test invalid phone
	invalidPhone := "not-a-phone-number"

	// Should fail
	_, err := sut.CreateThread(fixtures.ctx, services.CreateThreadDTO{
		Phone:   invalidPhone,
		Country: country.UnitedStates,
	})
	require.Error(t, err)
	assert.ErrorIs(t, err, phone.ErrInvalidPhoneNumber)
}

func TestWebsiteChatService_CreateThread_WithDifferentPhones(t *testing.T) {
	t.Parallel()
	fixtures, sut, _ := setupChatTest(t)

	// Test multiple phone formats
	tests := []struct {
		name      string
		phone     string
		expectErr bool
	}{
		{
			name:      "Valid US phone with plus",
			phone:     "+12126647667",
			expectErr: false,
		},
		{
			name:      "Valid US phone without plus",
			phone:     "12126647667",
			expectErr: false, // This implementation accepts without plus
		},
		{
			name:      "Valid phone with different format",
			phone:     "+1-212-664-7667",
			expectErr: false, // This implementation accepts different formats
		},
		{
			name:      "Invalid phone",
			phone:     "invalid-phone",
			expectErr: true,
		},
		{
			name:      "Empty phone",
			phone:     "",
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			thread, err := sut.CreateThread(fixtures.ctx, services.CreateThreadDTO{
				Phone:   tt.phone,
				Country: country.UnitedStates,
			})

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
	phoneStr := "+12126647670" // Valid US number
	thread, err := sut.CreateThread(fixtures.ctx, services.CreateThreadDTO{
		Phone:   phoneStr,
		Country: country.UnitedStates,
	})
	require.NoError(t, err)
	require.NotNil(t, thread)

	// Create message DTO
	dto := services.SendMessageToThreadDTO{
		ThreadID: thread.ID(),
		Message:  "Hello from client",
	}

	// Send message
	updatedThread, err := sut.SendMessageToThread(fixtures.ctx, dto)
	require.NoError(t, err)
	require.NotNil(t, updatedThread)

	// Verify message was added
	messages := updatedThread.Messages()
	require.NotEmpty(t, messages)
	lastMsg := messages[len(messages)-1]
	require.NoError(t, err)
	assert.Equal(t, "Hello from client", lastMsg.Message())

	// Verify sender is a client
	sender := lastMsg.Sender().Sender()
	_, ok := sender.(chat.ClientSender)
	require.True(t, ok, "Message sender should be a ClientSender")
	assert.Equal(t, chat.WebsiteTransport, lastMsg.Sender().Transport())
}

func TestWebsiteChatService_SendMessageToThread_EmptyMessage(t *testing.T) {
	t.Parallel()
	fixtures, sut, _ := setupChatTest(t)

	// Create a thread first
	phoneStr := "+12126647671" // Valid US number
	thread, err := sut.CreateThread(fixtures.ctx, services.CreateThreadDTO{
		Phone:   phoneStr,
		Country: country.UnitedStates,
	})
	require.NoError(t, err)

	// Try to send empty message
	dto := services.SendMessageToThreadDTO{
		ThreadID: thread.ID(),
		Message:  "",
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
	phoneStr := "+12126647672" // Valid US number
	thread, err := sut.CreateThread(fixtures.ctx, services.CreateThreadDTO{
		Phone:   phoneStr,
		Country: country.UnitedStates,
	})
	require.NoError(t, err)
	require.NotNil(t, thread)

	createdUser, err := userRepo.Create(fixtures.ctx, user.New(
		"Support",
		"Agent",
		internet.MustParseEmail("test@gmail.com"),
		user.UILanguageEN,
		user.WithTenantID(fixtures.tenant.ID),
	))
	require.NoError(t, err)

	// Create reply DTO
	dto := services.ReplyToThreadDTO{
		ThreadID: thread.ID(),
		UserID:   createdUser.ID(),
		Message:  "Reply from support agent",
	}

	// Send reply
	repliedThread, err := sut.ReplyToThread(fixtures.ctx, dto)
	require.NoError(t, err)
	require.NotNil(t, repliedThread)

	// Verify message was added
	messages := repliedThread.Messages()
	require.NotEmpty(t, messages)

	lastMsg := messages[len(messages)-1]
	require.NoError(t, err)
	assert.Equal(t, "Reply from support agent", lastMsg.Message())

	// Verify sender is a user
	sender := lastMsg.Sender().Sender()
	userSender, ok := sender.(chat.UserSender)
	require.True(t, ok, "Message sender should be a UserSender")
	assert.Equal(t, createdUser.ID(), userSender.UserID())
	// Now we get the transport from the member, not the sender
	assert.Equal(t, chat.WebsiteTransport, lastMsg.Sender().Transport())
}

func TestWebsiteChatService_ReplyToThread_EmptyMessage(t *testing.T) {
	t.Parallel()
	fixtures, sut, _ := setupChatTest(t)

	phoneStr := "+12126647673" // Valid US number
	thread, err := sut.CreateThread(fixtures.ctx, services.CreateThreadDTO{
		Phone:   phoneStr,
		Country: country.UnitedStates,
	})
	require.NoError(t, err)

	// Should fail
	_, err = sut.ReplyToThread(fixtures.ctx, services.ReplyToThreadDTO{
		ThreadID: thread.ID(),
		UserID:   1,
		Message:  "",
	})
	require.Error(t, err)
	require.ErrorIs(t, err, chat.ErrEmptyMessage)
}

func TestWebsiteChatService_ReplyToThread_UserNotFound(t *testing.T) {
	t.Parallel()
	fixtures, sut, _ := setupChatTest(t)

	// Create a thread first
	phoneStr := "+12126647674" // Valid US number
	thread, err := sut.CreateThread(fixtures.ctx, services.CreateThreadDTO{
		Phone:   phoneStr,
		Country: country.UnitedStates,
	})
	require.NoError(t, err)

	// Try to reply with non-existent user
	dto := services.ReplyToThreadDTO{
		ThreadID: thread.ID(),
		UserID:   999, // Non-existent user
		Message:  "This should fail",
	}

	// Should fail because user is not a member
	_, err = sut.ReplyToThread(fixtures.ctx, dto)
	require.Error(t, err)
	require.ErrorIs(t, err, corePersistence.ErrUserNotFound)
}
