package services

import (
	"context"
	"errors"

	"github.com/iota-uz/iota-sdk/modules/core/domain/value_objects/phone"

	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/upload"
	"github.com/iota-uz/iota-sdk/modules/crm/domain/aggregates/chat"
	"github.com/iota-uz/iota-sdk/modules/crm/domain/aggregates/client"
	cpassproviders "github.com/iota-uz/iota-sdk/modules/crm/infrastructure/cpass-providers"
	"github.com/iota-uz/iota-sdk/modules/crm/infrastructure/persistence"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/configuration"
	"github.com/iota-uz/iota-sdk/pkg/eventbus"
)

// MessageMedia represents media attached to a message
type MessageMedia struct {
	MinioTempPath string
	Filename      string
	MimeType      string
}

// SendMessageDTO represents the data needed to send a message
type SendMessageDTO struct {
	ChatID      uint
	Message     string
	Attachments []*upload.Upload
}

type ChatService struct {
	repo       chat.Repository
	clientRepo client.Repository
	providers  map[chat.Transport]chat.Provider
	publisher  eventbus.EventBus
}

func NewChatService(
	repo chat.Repository,
	clientRepo client.Repository,
	providers []chat.Provider,
	publisher eventbus.EventBus,
) *ChatService {
	providerMap := make(map[chat.Transport]chat.Provider)
	for _, provider := range providers {
		providerMap[provider.Source()] = provider
	}

	service := &ChatService{
		repo:       repo,
		clientRepo: clientRepo,
		providers:  providerMap,
		publisher:  publisher,
	}

	for _, provider := range providers {
		provider.OnReceived(service.onMessageReceived)
	}
	return service
}

func (s *ChatService) Count(ctx context.Context) (int64, error) {
	return s.repo.Count(ctx)
}

func (s *ChatService) GetAll(ctx context.Context) ([]chat.Chat, error) {
	return s.repo.GetAll(ctx)
}

func (s *ChatService) GetByID(ctx context.Context, id uint) (chat.Chat, error) {
	return s.repo.GetByID(ctx, id)
}

func (s *ChatService) GetPaginated(ctx context.Context, params *chat.FindParams) ([]chat.Chat, error) {
	return s.repo.GetPaginated(ctx, params)
}

func (s *ChatService) GetByClientID(ctx context.Context, clientID uint) (chat.Chat, error) {
	return s.repo.GetByClientID(ctx, clientID)
}

// GetByClientIDOrCreate retrieves a chat by client ID or creates a new one if it doesn't exist
func (s *ChatService) GetByClientIDOrCreate(ctx context.Context, clientID uint) (chat.Chat, error) {
	// First try to get without transaction
	chatEntity, err := s.repo.GetByClientID(ctx, clientID)
	if err == nil {
		return chatEntity, nil
	}
	if !errors.Is(err, persistence.ErrChatNotFound) {
		return nil, err
	}

	var createdEntity chat.Chat
	var createDTO *chat.CreateDTO

	// Need to create chat, start a transaction
	err = composables.InTx(ctx, func(txCtx context.Context) error {
		client, err := s.clientRepo.GetByID(txCtx, clientID)
		if err != nil {
			return err
		}

		createDTO = &chat.CreateDTO{
			ClientID: client.ID(),
		}

		entity, err := createDTO.ToEntity()
		if err != nil {
			return err
		}

		createdEntity, err = s.repo.Create(txCtx, entity)
		return err
	})
	if err != nil {
		return nil, err
	}

	// Publish event after successful commit
	event, err := chat.NewCreatedEvent(ctx, *createDTO, createdEntity)
	if err != nil {
		return nil, err
	}
	s.publisher.Publish(event)

	return createdEntity, nil
}

// Create creates a new chat
func (s *ChatService) Create(ctx context.Context, data *chat.CreateDTO) (chat.Chat, error) {
	entity, err := data.ToEntity()
	if err != nil {
		return nil, err
	}
	createdEntity, err := s.repo.Create(ctx, entity)
	if err != nil {
		return nil, err
	}
	event, err := chat.NewCreatedEvent(ctx, *data, createdEntity)
	if err != nil {
		return nil, err
	}
	s.publisher.Publish(event)
	return createdEntity, nil
}

// Update updates a chat entity
func (s *ChatService) Update(ctx context.Context, entity chat.Chat) (chat.Chat, error) {
	return s.repo.Update(ctx, entity)
}

// AddMessageToChat adds a message to a chat and handles the transaction
func (s *ChatService) AddMessageToChat(
	ctx context.Context,
	chatID uint,
	message string,
	sender chat.Sender,
	source chat.Transport,
) (chat.Chat, error) {
	var updatedChat chat.Chat

	err := composables.InTx(ctx, func(txCtx context.Context) error {
		// Get chat entity
		chatEntity, err := s.repo.GetByID(txCtx, chatID)
		if err != nil {
			return err
		}

		// Add message
		_, err = chatEntity.AddMessage(message, sender, source)
		if err != nil {
			return err
		}

		// Update chat
		updatedChat, err = s.repo.Update(txCtx, chatEntity)
		return err
	})
	if err != nil {
		return nil, err
	}

	// Publish event after successful commit
	event, err := chat.NewMessageAddedEvent(ctx, updatedChat)
	if err != nil {
		return nil, err
	}
	s.publisher.Publish(event)

	return updatedChat, nil
}

// CreateOrGetClientByPhone creates a new client or gets an existing one by phone number
func (s *ChatService) CreateOrGetClientByPhone(ctx context.Context, phoneNumber string) (client.Client, error) {
	logger := composables.UseLogger(ctx)

	// Validate and normalize the phone number
	phoneValue, err := phone.NewFromE164(phoneNumber)
	if err != nil {
		logger.WithError(err).Error("invalid phone number")
		return nil, err
	}

	// Try to find existing client without transaction first
	clientEntity, err := s.clientRepo.GetByPhone(ctx, phoneValue.Value())
	if err == nil {
		return clientEntity, nil
	}

	// Only proceed with creation if client not found
	if err != persistence.ErrClientNotFound {
		logger.WithError(err).Error("error getting client by phone")
		return nil, err
	}

	var newClientEntity client.Client

	// Create client in a transaction
	err = composables.InTx(ctx, func(txCtx context.Context) error {
		// Create a new client with the provided phone number
		newClient, err := client.New(
			"Guest", // Default first name
			"User",  // Default last name
			"",      // No middle name
			client.WithPhone(phoneValue),
		)
		if err != nil {
			logger.WithError(err).Error("failed to create client entity")
			return err
		}

		// Save the new client
		newClientEntity, err = s.clientRepo.Create(txCtx, newClient)
		return err
	})
	if err != nil {
		return nil, err
	}

	return newClientEntity, nil
}

// GetOrCreateChatByPhone creates a chat for a client based on phone number
func (s *ChatService) GetOrCreateChatByPhone(ctx context.Context, phoneNumber string) (chat.Chat, client.Client, error) {
	// Get or create the client
	clientEntity, err := s.CreateOrGetClientByPhone(ctx, phoneNumber)
	if err != nil {
		return nil, nil, err
	}

	// Get or create a chat for this client
	chatEntity, err := s.GetByClientIDOrCreate(ctx, clientEntity.ID())
	if err != nil {
		return nil, nil, err
	}

	return chatEntity, clientEntity, nil
}

func (s *ChatService) onMessageReceived(msg chat.Message) error {
	// Get or create a chat for the client
	chatEntity, clientEntity, err := s.GetOrCreateChatByPhone(ctx, msg.Sender())
	if err != nil {
		return err
	}

	// Add the message to the chat
	_, err = chatEntity.AddMessage(
		msg.Message(),
		chat.NewClientSender(
			clientEntity.ID(),
			clientEntity.FirstName(),
			clientEntity.LastName(),
		),
		chat.SMSTransport,
	)
	if err != nil {
		return err
	}

	return nil
}

func (s *ChatService) RegisterClientMessage(
	ctx context.Context,
	params *cpassproviders.ReceivedMessageEvent,
) (chat.Chat, error) {
	p, err := phone.NewFromE164(params.From)
	if err != nil {
		return nil, err
	}

	clientEntity, err := s.clientRepo.GetByPhone(ctx, p.Value())
	if err != nil {
		return nil, err
	}

	var updatedChat chat.Chat

	err = composables.InTx(ctx, func(txCtx context.Context) error {
		chatEntity, err := s.GetByClientIDOrCreate(txCtx, clientEntity.ID())
		if err != nil {
			return err
		}

		if _, err := chatEntity.AddMessage(
			params.Body,
			chat.NewClientSender(
				clientEntity.ID(),
				clientEntity.FirstName(),
				clientEntity.LastName(),
			),
			chat.SMSTransport,
		); err != nil {
			return err
		}

		updatedChat, err = s.repo.Update(txCtx, chatEntity)
		return err
	})
	if err != nil {
		return nil, err
	}

	event, err := chat.NewMessageAddedEvent(ctx, updatedChat)
	if err != nil {
		return nil, err
	}
	s.publisher.Publish(event)

	return updatedChat, nil
}

func (s *ChatService) SendMessage(ctx context.Context, dto SendMessageDTO) (chat.Chat, error) {
	var updatedChat chat.Chat

	err := composables.InTx(ctx, func(txCtx context.Context) error {
		// Get the chat by ID
		chatEntity, err := s.GetByID(txCtx, dto.ChatID)
		if err != nil {
			return err
		}

		// Add message as system user (for AI responses)
		_, err = chatEntity.AddMessage(
			dto.Message,
			chat.NewUserSender(1, "AI", "Assistant"), // System user ID 1 for AI
			chat.SMSTransport,
		)
		if err != nil {
			return err
		}

		updatedChat, err = s.repo.Update(txCtx, chatEntity)
		if err != nil {
			return err
		}

		clientEntity, err := s.clientRepo.GetByID(txCtx, chatEntity.ClientID())
		if err != nil {
			return err
		}

		return s.cpassProvider.SendMessage(txCtx, cpassproviders.SendMessageDTO{
			From:    configuration.Use().Twilio.PhoneNumber,
			To:      clientEntity.Phone().Value(),
			Message: dto.Message,
		})
	})
	if err != nil {
		return nil, err
	}

	event, err := chat.NewMessageAddedEvent(ctx, updatedChat)
	if err != nil {
		return nil, err
	}
	s.publisher.Publish(event)

	return updatedChat, nil
}

func (s *ChatService) Delete(ctx context.Context, id uint) (chat.Chat, error) {
	entity, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if err := s.repo.Delete(ctx, id); err != nil {
		return nil, err
	}
	return entity, nil
}
