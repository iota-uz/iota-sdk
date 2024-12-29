package services

import (
	"context"
	currency2 "github.com/iota-uz/iota-sdk/modules/core/domain/entities/currency"

	"github.com/iota-agency/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/event"
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

func (s *CurrencyService) GetByCode(ctx context.Context, id string) (*currency2.Currency, error) {
	return s.Repo.GetByCode(ctx, id)
}

func (s *CurrencyService) GetAll(ctx context.Context) ([]*currency2.Currency, error) {
	return s.Repo.GetAll(ctx)
}

func (s *CurrencyService) GetPaginated(
	ctx context.Context, params *currency.FindParams,
) ([]*currency2.Currency, error) {
	return s.Repo.GetPaginated(ctx, params)
}

func (s *CurrencyService) Create(ctx context.Context, data *currency2.CreateDTO) error {
	tx, err := composables.UsePoolTx(ctx)
	if err != nil {
		return err
	}
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
	return tx.Commit(ctx)
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

func (s *CurrencyService) Delete(ctx context.Context, code string) (*currency2.Currency, error) {
	tx, err := composables.UsePoolTx(ctx)
	if err != nil {
		return nil, err
	}
	deletedEvent, err := currency2.NewDeletedEvent(ctx)
	if err != nil {
		return nil, err
	}
	entity, err := s.Repo.GetByCode(ctx, code)
	if err != nil {
		return nil, err
	}
	if err := s.Repo.Delete(ctx, code); err != nil {
		return nil, err
	}
	deletedEvent.Result = *entity
	s.Publisher.Publish(deletedEvent)
	return entity, tx.Commit(ctx)
}
