package services

import (
	"context"
	"errors"

	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/phone"
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
	repo          chat.Repository
	clientRepo    client.Repository
	cpassProvider cpassproviders.Provider
	publisher     eventbus.EventBus
}

func NewChatService(
	repo chat.Repository,
	clientRepo client.Repository,
	cpassProvider cpassproviders.Provider,
	publisher eventbus.EventBus,
) *ChatService {
	return &ChatService{
		repo:          repo,
		clientRepo:    clientRepo,
		cpassProvider: cpassProvider,
		publisher:     publisher,
	}
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

func (s *ChatService) GetByClientIDOrCreate(ctx context.Context, clientID uint) (chat.Chat, error) {
	chatEntity, err := s.repo.GetByClientID(ctx, clientID)
	if err == nil {
		return chatEntity, nil
	}
	if !errors.Is(err, persistence.ErrChatNotFound) {
		return nil, err
	}

	client, err := s.clientRepo.GetByID(ctx, clientID)
	if err != nil {
		return nil, err
	}
	return s.Create(ctx, &chat.CreateDTO{
		ClientID: client.ID(),
	})
}

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

func (s *ChatService) Update(ctx context.Context, entity chat.Chat) (chat.Chat, error) {
	return s.repo.Update(ctx, entity)
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
	pool, err := composables.UsePool(ctx)
	if err != nil {
		return nil, err
	}
	tx, err := pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	ctx = composables.WithTx(ctx, tx)
	chatEntity, err := s.GetByClientIDOrCreate(ctx, clientEntity.ID())
	if err != nil {
		return nil, err
	}

	if _, err := chatEntity.AddMessage(
		params.Body,
		chat.NewClientSender(
			clientEntity.ID(),
			clientEntity.FirstName(),
			clientEntity.LastName(),
		),
	); err != nil {
		return nil, err
	}

	updatedChat, err := s.repo.Update(ctx, chatEntity)
	if err != nil {
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}

	event, err := chat.NewMessageAddedEvent(ctx, chatEntity)
	if err != nil {
		return nil, err
	}
	s.publisher.Publish(event)
	return updatedChat, nil
}

func (s *ChatService) SendMessage(ctx context.Context, dto SendMessageDTO) (chat.Chat, error) {
	user, err := composables.UseUser(ctx)
	if err != nil {
		return nil, err
	}
	pool, err := composables.UsePool(ctx)
	if err != nil {
		return nil, err
	}
	tx, err := pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	ctx = composables.WithTx(ctx, tx)
	chatEntity, err := s.GetByID(ctx, dto.ChatID)
	if err != nil {
		return nil, err
	}
	_, err = chatEntity.AddMessage(dto.Message, chat.NewUserSender(user.ID(), user.FirstName(), user.LastName()))
	if err != nil {
		return nil, err
	}
	updatedChat, err := s.repo.Update(ctx, chatEntity)
	if err != nil {
		return nil, err
	}
	clientEntity, err := s.clientRepo.GetByID(ctx, chatEntity.ClientID())
	if err != nil {
		return nil, err
	}
	if err := s.cpassProvider.SendMessage(ctx, cpassproviders.SendMessageDTO{
		From:    configuration.Use().TwilioPhoneNumber,
		To:      clientEntity.Phone().Value(),
		Message: dto.Message,
	}); err != nil {
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
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
