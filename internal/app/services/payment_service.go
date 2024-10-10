package services

import (
	"context"

	"github.com/iota-agency/iota-erp/internal/domain/entities/payment"
	"github.com/iota-agency/iota-erp/sdk/event"
)

type PaymentService struct {
	repo      payment.Repository
	publisher event.Publisher
}

func NewPaymentService(repo payment.Repository, publisher event.Publisher) *PaymentService {
	return &PaymentService{
		repo:      repo,
		publisher: publisher,
	}
}

func (s *PaymentService) GetByID(ctx context.Context, id uint) (*payment.Payment, error) {
	return s.repo.GetByID(ctx, id)
}

func (s *PaymentService) GetAll(ctx context.Context) ([]*payment.Payment, error) {
	return s.repo.GetAll(ctx)
}

func (s *PaymentService) GetPaginated(
	ctx context.Context,
	limit, offset int,
	sortBy []string,
) ([]*payment.Payment, error) {
	return s.repo.GetPaginated(ctx, limit, offset, sortBy)
}

func (s *PaymentService) Create(ctx context.Context, data *payment.CreateDTO) error {
	createdEvent, err := payment.NewCreatedEvent(ctx, *data)
	if err != nil {
		return err
	}
	entity := data.ToEntity()
	if err := s.repo.Create(ctx, entity); err != nil {
		return err
	}
	createdEvent.Result = *entity
	s.publisher.Publish(createdEvent)
	return nil
}

func (s *PaymentService) Update(ctx context.Context, id uint, data *payment.UpdateDTO) error {
	updatedEvent, err := payment.NewUpdatedEvent(ctx, *data)
	if err != nil {
		return err
	}
	entity := data.ToEntity(id)
	if err := s.repo.Update(ctx, entity); err != nil {
		return err
	}
	updatedEvent.Result = *entity
	s.publisher.Publish(updatedEvent)
	return nil
}

func (s *PaymentService) Delete(ctx context.Context, id uint) (*payment.Payment, error) {
	deletedEvent, err := payment.NewDeletedEvent(ctx)
	if err != nil {
		return nil, err
	}
	entity, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if err := s.repo.Delete(ctx, id); err != nil {
		return nil, err
	}
	deletedEvent.Result = *entity
	s.publisher.Publish(deletedEvent)
	return entity, nil
}
