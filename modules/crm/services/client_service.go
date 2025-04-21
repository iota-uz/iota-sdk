package services

import (
	"context"

	"github.com/iota-uz/iota-sdk/modules/crm/domain/aggregates/client"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/eventbus"
)

type ClientService struct {
	repo      client.Repository
	publisher eventbus.EventBus
}

func NewClientService(
	repo client.Repository,
	publisher eventbus.EventBus,
) *ClientService {
	return &ClientService{
		repo:      repo,
		publisher: publisher,
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

func (s *ClientService) Create(ctx context.Context, data client.Client) error {
	var err error
	var createdClient client.Client
	err = composables.InTx(ctx, func(txCtx context.Context) error {
		created, err := s.repo.Create(ctx, data)
		if err != nil {
			return err
		}
		createdClient = created
		return nil
	})
	if err != nil {
		return err
	}
	createdEvent, err := client.NewCreatedEvent(ctx, data)
	if err != nil {
		return err
	}
	createdEvent.Result = createdClient
	s.publisher.Publish(createdEvent)
	return nil
}

func (s *ClientService) Update(ctx context.Context, data client.Client) error {
	var err error
	var updatedClient client.Client
	err = composables.InTx(ctx, func(txCtx context.Context) error {
		updated, err := s.repo.Update(ctx, data)
		if err != nil {
			return err
		}
		updatedClient = updated
		return nil
	})
	if err != nil {
		return err
	}
	updatedEvent, err := client.NewUpdatedEvent(ctx, data)
	if err != nil {
		return err
	}
	updatedEvent.Result = updatedClient
	s.publisher.Publish(updatedEvent)
	return nil
}

func (s *ClientService) Delete(ctx context.Context, id uint) (client.Client, error) {
	entity, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	var deletedClient client.Client
	err = composables.InTx(ctx, func(txCtx context.Context) error {
		if err := s.repo.Delete(ctx, id); err != nil {
			return err
		}
		deletedClient = entity
		return nil
	})
	deletedEvent, err := client.NewDeletedEvent(ctx, entity)
	if err != nil {
		return nil, err
	}
	deletedEvent.Result = deletedClient
	s.publisher.Publish(deletedEvent)
	return entity, nil
}
