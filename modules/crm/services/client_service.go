package services

import (
	"context"

	"github.com/iota-uz/iota-sdk/modules/crm/domain/aggregates/chat"
	"github.com/iota-uz/iota-sdk/modules/crm/domain/aggregates/client"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/eventbus"
)

type ClientService struct {
	repo        client.Repository
	chatService *ChatService
	publisher   eventbus.EventBus
}

func NewClientService(
	repo client.Repository,
	chatService *ChatService,
	publisher eventbus.EventBus,
) *ClientService {
	return &ClientService{
		repo:        repo,
		chatService: chatService,
		publisher:   publisher,
	}
}

func (s *ClientService) Count(ctx context.Context) (int64, error) {
	return s.repo.Count(ctx)
}

func (s *ClientService) GetAll(ctx context.Context) ([]client.Client, error) {
	return s.repo.GetAll(ctx)
}

func (s *ClientService) GetByID(ctx context.Context, id uint) (client.Client, error) {
	return s.repo.GetByID(ctx, id)
}

func (s *ClientService) GetPaginated(ctx context.Context, params *client.FindParams) ([]client.Client, error) {
	return s.repo.GetPaginated(ctx, params)
}

func (s *ClientService) Create(ctx context.Context, data *client.CreateDTO) (client.Client, error) {
	entity, err := data.ToEntity()
	if err != nil {
		return nil, err
	}
	tx, err := composables.BeginTx(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	ctx = composables.WithTx(ctx, tx)
	createdEntity, err := s.repo.Create(ctx, entity)
	if err != nil {
		return nil, err
	}
	_, err = s.chatService.Create(ctx, &chat.CreateDTO{
		ClientID: createdEntity.ID(),
	})
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return createdEntity, nil
}

func (s *ClientService) Save(ctx context.Context, entity client.Client) error {
	tx, err := composables.BeginTx(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	ctx = composables.WithTx(ctx, tx)
	
	if _, err := s.repo.Update(ctx, entity); err != nil {
		return err
	}
	
	if err := tx.Commit(ctx); err != nil {
		return err
	}
	return nil
}

func (s *ClientService) Delete(ctx context.Context, id uint) (client.Client, error) {
	tx, err := composables.BeginTx(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	ctx = composables.WithTx(ctx, tx)
	entity, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if err := s.repo.Delete(ctx, id); err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return entity, nil
}
