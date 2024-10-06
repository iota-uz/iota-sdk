package services

import (
	"context"

	"github.com/iota-agency/iota-erp/internal/domain/entities/payment"
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
	createdEvent, err := payment.NewCreatedEvent(ctx, *data)
	if err != nil {
		return err
	}
	entity := data.ToEntity()
	if err := s.Repo.Create(ctx, entity); err != nil {
		return err
	}
	createdEvent.Result = *entity
	s.Publisher.Publish(createdEvent)
	return nil
}

func (s *PaymentService) Update(ctx context.Context, id uint, data *payment.UpdateDTO) error {
	updatedEvent, err := payment.NewUpdatedEvent(ctx, *data)
	if err != nil {
		return err
	}
	entity := data.ToEntity(id)
	if err := s.Repo.Update(ctx, entity); err != nil {
		return err
	}
	updatedEvent.Result = *entity
	s.Publisher.Publish(updatedEvent)
	return nil
}

func (s *PaymentService) Delete(ctx context.Context, id uint) (*payment.Payment, error) {
	deletedEvent, err := payment.NewDeletedEvent(ctx)
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
