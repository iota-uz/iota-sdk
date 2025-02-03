package services

import (
	"context"
	"errors"

	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/phone"
	"github.com/iota-uz/iota-sdk/modules/crm/domain/aggregates/chat"
	"github.com/iota-uz/iota-sdk/modules/crm/domain/aggregates/client"
	cpassproviders "github.com/iota-uz/iota-sdk/modules/crm/infrastructure/cpass-providers"
	"github.com/iota-uz/iota-sdk/modules/crm/infrastructure/persistence"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/eventbus"
)

// MessageMedia represents media attached to a message
type MessageMedia struct {
	MinioTempPath string
	Filename      string
	MimeType      string
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
	client, err := s.clientRepo.GetByID(ctx, data.ClientID)
	if err != nil {
		return nil, err
	}
	entity, err := data.ToEntity(client)
	if err != nil {
		return nil, err
	}
	createdEntity, err := s.repo.Create(ctx, entity)
	if err != nil {
		return nil, err
	}
	return createdEntity, nil
}

func (s *ChatService) RegisterClientMessage(
	ctx context.Context,
	params *cpassproviders.ReceivedMessageEvent,
) error {
	p, err := phone.NewFromE164(params.From)
	if err != nil {
		return err
	}
	clientEntity, err := s.clientRepo.GetByPhone(ctx, p.Value())
	if err != nil {
		return err
	}

	chatEntity, err := s.GetByClientIDOrCreate(ctx, clientEntity.ID())
	if err != nil {
		return err
	}

	if _, err := s.repo.Update(
		ctx,
		chatEntity.RegisterClientMessage(params.Body, clientEntity.ID()),
	); err != nil {
		return err
	}

	return nil
}

func (s *ChatService) SendMessage(ctx context.Context, chatID uint, msg string) (chat.Chat, error) {
	user, err := composables.UseUser(ctx)
	if err != nil {
		return nil, err
	}
	entity, err := s.repo.GetByID(ctx, chatID)
	if err != nil {
		return nil, err
	}
	updatedEntity, err := s.repo.Update(ctx, entity.SendMessage(msg, user.ID()))
	if err != nil {
		return nil, err
	}
	if err := s.cpassProvider.SendMessage(ctx, cpassproviders.SendMessageDTO{
		From:    "+18449090114",
		To:      updatedEntity.Client().Phone().Value(),
		Message: msg,
	}); err != nil {
		return nil, err
	}
	return updatedEntity, nil
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
