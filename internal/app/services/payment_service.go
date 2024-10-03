package services

import (
	"context"

	"github.com/iota-agency/iota-erp/internal/domain/entities/payment"
	"github.com/iota-agency/iota-erp/pkg/composables"
	"github.com/iota-agency/iota-erp/sdk/event"
)

type PaymentService struct {
	Repo      payment.Repository
	Publisher *event.Publisher
}

func NewPaymentService(repo payment.Repository, app *Application) *PaymentService {
	return &PaymentService{
		Repo:      repo,
		Publisher: app.EventPublisher,
	}
}

func (s *PaymentService) GetByID(ctx context.Context, id uint) (*payment.Payment, error) {
	return s.Repo.GetByID(ctx, id)
}

func (s *PaymentService) GetAll(ctx context.Context) ([]*payment.Payment, error) {
	return s.Repo.GetAll(ctx)
}

func (s *PaymentService) GetPaginated(ctx context.Context, limit, offset int, sortBy []string) ([]*payment.Payment, error) {
	return s.Repo.GetPaginated(ctx, limit, offset, sortBy)
}

func (s *PaymentService) Create(ctx context.Context, data *payment.CreateDTO) error {
	ev := &payment.Created{
		Data: &(*data),
	}
	if u, err := composables.UseUser(ctx); err == nil {
		ev.Sender = u
	}
	if sess, err := composables.UseSession(ctx); err == nil {
		ev.Session = sess
	}
	entity := data.ToEntity()
	if err := s.Repo.Create(ctx, entity); err != nil {
		return err
	}
	ev.Result = &(*entity)
	s.Publisher.Publish(ev)
	return nil
}

func (s *PaymentService) Update(ctx context.Context, id uint, data *payment.UpdateDTO) error {
	evt := &payment.Updated{
		Data: &(*data),
	}
	if u, err := composables.UseUser(ctx); err == nil {
		evt.Sender = u
	}
	if sess, err := composables.UseSession(ctx); err == nil {
		evt.Session = sess
	}
	entity := data.ToEntity(id)
	if err := s.Repo.Update(ctx, entity); err != nil {
		return err
	}
	evt.Result = &(*entity)
	s.Publisher.Publish(evt)
	return nil
}

func (s *PaymentService) Delete(ctx context.Context, id uint) (*payment.Payment, error) {
	evt := &payment.Deleted{}
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
