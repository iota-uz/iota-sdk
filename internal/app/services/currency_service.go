package services

import (
	"context"
	"github.com/iota-agency/iota-erp/internal/domain/entities/currency"

	"github.com/iota-agency/iota-erp/pkg/composables"
	"github.com/iota-agency/iota-erp/sdk/event"
)

type CurrencyService struct {
	Repo      currency.Repository
	Publisher *event.Publisher
}

func NewCurrencyService(repo currency.Repository, app *Application) *CurrencyService {
	return &CurrencyService{
		Repo:      repo,
		Publisher: app.EventPublisher,
	}
}

func (s *CurrencyService) GetByID(ctx context.Context, id uint) (*currency.Currency, error) {
	return s.Repo.GetByID(ctx, id)
}

func (s *CurrencyService) GetAll(ctx context.Context) ([]*currency.Currency, error) {
	return s.Repo.GetAll(ctx)
}

func (s *CurrencyService) GetPaginated(ctx context.Context, limit, offset int, sortBy []string) ([]*currency.Currency, error) {
	return s.Repo.GetPaginated(ctx, limit, offset, sortBy)
}

func (s *CurrencyService) Create(ctx context.Context, data *currency.CreateDTO) error {
	ev := &currency.Created{
		Data: &(*data),
	}
	if u, err := composables.UseUser(ctx); err == nil {
		ev.Sender = u
	}
	if sess, err := composables.UseSession(ctx); err == nil {
		ev.Session = sess
	}
	entity, err := data.ToEntity()
	if err != nil {
		return err
	}
	if err := s.Repo.Create(ctx, entity); err != nil {
		return err
	}
	ev.Result = &(*entity)
	s.Publisher.Publish(ev)
	return nil
}

// TODO: deal with id

func (s *CurrencyService) Update(ctx context.Context, id string, data *currency.UpdateDTO) error {
	evt := &currency.Updated{
		Data: &(*data),
	}
	if u, err := composables.UseUser(ctx); err == nil {
		evt.Sender = u
	}
	if sess, err := composables.UseSession(ctx); err == nil {
		evt.Session = sess
	}
	entity, err := data.ToEntity()
	if err != nil {
		return err
	}
	if err := s.Repo.Update(ctx, entity); err != nil {
		return err
	}
	evt.Result = &(*entity)
	s.Publisher.Publish(evt)
	return nil
}

func (s *CurrencyService) Delete(ctx context.Context, id uint) (*currency.Currency, error) {
	evt := &currency.Deleted{}
	if u, err := composables.UseUser(ctx); err == nil {
		evt.Sender = u
	}
	if sess, err := composables.UseSession(ctx); err == nil {
		evt.Session = sess
	}
	entity, err := s.Repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if err := s.Repo.Delete(ctx, id); err != nil {
		return nil, err
	}
	evt.Result = entity
	s.Publisher.Publish(evt)
	return entity, nil
}
