package services

import (
	"context"
	"github.com/iota-uz/iota-sdk/modules/crm/domain/aggregates/client"
	"github.com/iota-uz/iota-sdk/pkg/eventbus"
)

type ClientService struct {
	repo      client.Repository
	publisher eventbus.EventBus
}

func NewClientService(repo client.Repository, publisher eventbus.EventBus) *ClientService {
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

func (s *ClientService) Create(ctx context.Context, data *client.CreateDTO) (client.Client, error) {
	entity, err := data.ToEntity()
	if err != nil {
		return nil, err
	}
	createdEntity, err := s.repo.Create(ctx, entity)
	if err != nil {
		return nil, err
	}
	return createdEntity, nil
}

func (s *ClientService) Update(ctx context.Context, id uint, data *client.UpdateDTO) error {
	entity, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return err
	}
	modified, err := data.Apply(entity)
	if err != nil {
		return err
	}
	if _, err := s.repo.Update(ctx, modified); err != nil {
		return err
	}
	return nil
}

func (s *ClientService) Delete(ctx context.Context, id uint) (client.Client, error) {
	if err := s.repo.Delete(ctx, id); err != nil {
		return nil, err
	}
	entity, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	return entity, nil
}
