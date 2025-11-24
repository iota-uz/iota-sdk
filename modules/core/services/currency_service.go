package services

import (
	"context"

	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/currency"
	"github.com/iota-uz/iota-sdk/pkg/eventbus"
)

type CurrencyService struct {
	repo      currency.Repository
	publisher eventbus.EventBus
}

func NewCurrencyService(repo currency.Repository, publisher eventbus.EventBus) *CurrencyService {
	return &CurrencyService{
		repo:      repo,
		publisher: publisher,
	}
}

func (s *CurrencyService) GetByCode(ctx context.Context, id string) (currency.Currency, error) {
	return s.repo.GetByCode(ctx, id)
}

func (s *CurrencyService) GetAll(ctx context.Context) ([]currency.Currency, error) {
	return s.repo.GetAll(ctx)
}

func (s *CurrencyService) GetPaginated(
	ctx context.Context, params *currency.FindParams,
) ([]currency.Currency, error) {
	return s.repo.GetPaginated(ctx, params)
}

func (s *CurrencyService) Create(ctx context.Context, data *currency.CreateDTO) error {
	createdEvent, err := currency.NewCreatedEvent(ctx, *data)
	if err != nil {
		return err
	}
	entity, err := data.ToEntity()
	if err != nil {
		return err
	}
	if err := s.repo.Create(ctx, entity); err != nil {
		return err
	}
	createdEvent.Result = entity
	s.publisher.Publish(createdEvent)
	return nil
}

func (s *CurrencyService) Update(ctx context.Context, data *currency.UpdateDTO) error {
	updatedEvent, err := currency.NewUpdatedEvent(ctx, *data)
	if err != nil {
		return err
	}
	entity, err := data.ToEntity()
	if err != nil {
		return err
	}
	if err := s.repo.Update(ctx, entity); err != nil {
		return err
	}
	updatedEvent.Result = entity
	s.publisher.Publish(updatedEvent)
	return nil
}

func (s *CurrencyService) Delete(ctx context.Context, code string) (currency.Currency, error) {
	deletedEvent, err := currency.NewDeletedEvent(ctx)
	if err != nil {
		return nil, err
	}
	entity, err := s.repo.GetByCode(ctx, code)
	if err != nil {
		return nil, err
	}
	if err := s.repo.Delete(ctx, code); err != nil {
		return nil, err
	}
	deletedEvent.Result = entity
	s.publisher.Publish(deletedEvent)
	return entity, nil
}
