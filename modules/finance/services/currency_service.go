package services

import (
	"context"
	currency2 "github.com/iota-agency/iota-sdk/modules/finance/domain/entities/currency"
	"github.com/iota-agency/iota-sdk/pkg/event"
)

type CurrencyService struct {
	Repo      currency2.Repository
	Publisher event.Publisher
}

func NewCurrencyService(repo currency2.Repository, publisher event.Publisher) *CurrencyService {
	return &CurrencyService{
		Repo:      repo,
		Publisher: publisher,
	}
}

func (s *CurrencyService) GetByID(ctx context.Context, id uint) (*currency2.Currency, error) {
	return s.Repo.GetByID(ctx, id)
}

func (s *CurrencyService) GetAll(ctx context.Context) ([]*currency2.Currency, error) {
	return s.Repo.GetAll(ctx)
}

func (s *CurrencyService) GetPaginated(
	ctx context.Context,
	limit, offset int,
	sortBy []string,
) ([]*currency2.Currency, error) {
	return s.Repo.GetPaginated(ctx, limit, offset, sortBy)
}

func (s *CurrencyService) Create(ctx context.Context, data *currency2.CreateDTO) error {
	createdEvent, err := currency2.NewCreatedEvent(ctx, *data)
	if err != nil {
		return err
	}
	entity, err := data.ToEntity()
	if err != nil {
		return err
	}
	if err := s.Repo.Create(ctx, entity); err != nil {
		return err
	}
	createdEvent.Result = *entity
	s.Publisher.Publish(createdEvent)
	return nil
}

func (s *CurrencyService) Update(ctx context.Context, data *currency2.UpdateDTO) error {
	updatedEvent, err := currency2.NewUpdatedEvent(ctx, *data)
	if err != nil {
		return err
	}
	entity, err := data.ToEntity()
	if err != nil {
		return err
	}
	if err := s.Repo.Update(ctx, entity); err != nil {
		return err
	}
	updatedEvent.Result = *entity
	s.Publisher.Publish(updatedEvent)
	return nil
}

func (s *CurrencyService) Delete(ctx context.Context, id uint) (*currency2.Currency, error) {
	deletedEvent, err := currency2.NewDeletedEvent(ctx)
	if err != nil {
		return nil, err
	}
	entity, err := s.Repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if err := s.Repo.Delete(ctx, id); err != nil {
		return nil, err
	}
	deletedEvent.Result = *entity
	s.Publisher.Publish(deletedEvent)
	return entity, nil
}
