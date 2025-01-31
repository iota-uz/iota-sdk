package services

import (
	"context"

	"github.com/iota-uz/iota-sdk/modules/crm/domain/aggregates/chat"
	"github.com/iota-uz/iota-sdk/modules/crm/domain/aggregates/client"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/eventbus"
)

type ChatService struct {
	repo       chat.Repository
	clientRepo client.Repository
	publisher  eventbus.EventBus
}

func NewChatService(
	repo chat.Repository, clientRepo client.Repository,
	publisher eventbus.EventBus,
) *ChatService {
	return &ChatService{
		repo:       repo,
		clientRepo: clientRepo,
		publisher:  publisher,
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

func (s *ChatService) Create(ctx context.Context, data *chat.CreateDTO) (chat.Chat, error) {
	user, err := composables.UseUser(ctx)
	if err != nil {
		return nil, err
	}
	client, err := s.clientRepo.GetByID(ctx, data.ClientID)
	if err != nil {
		return nil, err
	}
	entity, err := data.ToEntity(user.ID(), client)
	if err != nil {
		return nil, err
	}
	createdEntity, err := s.repo.Create(ctx, entity)
	if err != nil {
		return nil, err
	}
	return createdEntity, nil
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
