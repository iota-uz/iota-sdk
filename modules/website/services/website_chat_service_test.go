package services_test

import (
	"testing"

	"github.com/iota-uz/iota-sdk/modules/core/domain/value_objects/internet"
	"github.com/iota-uz/iota-sdk/modules/core/domain/value_objects/phone"
	corePersistence "github.com/iota-uz/iota-sdk/modules/core/infrastructure/persistence"
	"github.com/iota-uz/iota-sdk/modules/crm/domain/aggregates/client"
	crmPersistence "github.com/iota-uz/iota-sdk/modules/crm/infrastructure/persistence"
	crmServices "github.com/iota-uz/iota-sdk/modules/crm/services"
	"github.com/iota-uz/iota-sdk/modules/website/services"
	"github.com/iota-uz/iota-sdk/pkg/eventbus"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// setupChatTest extends the setupTest with WebsiteChatService
func setupChatTest(t *testing.T) (*testFixtures, *services.WebsiteChatService, client.Repository, *crmServices.ChatService) {
	t.Helper()

	fixtures := setupTest(t)

	// Create repositories
	passportRepo := corePersistence.NewPassportRepository()
	clientRepo := crmPersistence.NewClientRepository(passportRepo)
	chatRepo := crmPersistence.NewChatRepository()

	// Create the event publisher
	bus := eventbus.NewEventPublisher(logrus.New())

	// Create the chat service
	chatService := crmServices.NewChatService(chatRepo, clientRepo, nil, bus)

	// Create the website chat service
	websiteChatService := services.NewWebsiteChatService(chatService, clientRepo)

	return fixtures, websiteChatService, clientRepo, chatService
}

func TestWebsiteChatService_CreateThread_WithEmail(t *testing.T) {
	t.Parallel()
	fixtures, sut, clientRepo, _ := setupChatTest(t)

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
	fixtures, sut, clientRepo, _ := setupChatTest(t)

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
	fixtures, sut, clientRepo, _ := setupChatTest(t)

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
	fixtures, sut, _, _ := setupChatTest(t)

	// Test invalid contact
	invalidContact := "not-an-email-or-phone"

	// Should fail
	_, err := sut.CreateThread(fixtures.ctx, invalidContact)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid contact")
}

func TestWebsiteChatService_CreateThread_WithEmailAndPhone(t *testing.T) {
	t.Parallel()
	fixtures, sut, _, _ := setupChatTest(t)

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
				assert.NoError(t, err)
				assert.NotNil(t, thread)
				assert.NotZero(t, thread.ID())
			}
		})
	}
}
