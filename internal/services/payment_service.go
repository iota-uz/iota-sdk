package services

import (
	"context"
	"github.com/iota-agency/iota-sdk/internal/domain/aggregates/payment"
	"github.com/iota-agency/iota-sdk/internal/domain/entities/permission"
	"github.com/iota-agency/iota-sdk/pkg/composables"
	"github.com/iota-agency/iota-sdk/pkg/event"
)

type PaymentService struct {
	repo           payment.Repository
	publisher      event.Publisher
	accountService *MoneyAccountService
}

func NewPaymentService(
	repo payment.Repository,
	publisher event.Publisher,
	accountService *MoneyAccountService,
) *PaymentService {
	return &PaymentService{
		repo:           repo,
		publisher:      publisher,
		accountService: accountService,
	}
}

func (s *PaymentService) GetByID(ctx context.Context, id uint) (*payment.Payment, error) {
	if err := composables.CanUser(ctx, permission.PaymentRead); err != nil {
		return nil, err
	}
	return s.repo.GetByID(ctx, id)
}

func (s *PaymentService) GetAll(ctx context.Context) ([]*payment.Payment, error) {
	if err := composables.CanUser(ctx, permission.PaymentRead); err != nil {
		return nil, err
	}
	return s.repo.GetAll(ctx)
}

func (s *PaymentService) GetPaginated(
	ctx context.Context,
	limit, offset int,
	sortBy []string,
) ([]*payment.Payment, error) {
	if err := composables.CanUser(ctx, permission.PaymentRead); err != nil {
		return nil, err
	}
	return s.repo.GetPaginated(ctx, limit, offset, sortBy)
}

func (s *PaymentService) Create(ctx context.Context, data *payment.CreateDTO) error {
	if err := composables.CanUser(ctx, permission.PaymentCreate); err != nil {
		return err
	}
	entity := data.ToEntity()
	if err := s.repo.Create(ctx, entity); err != nil {
		return err
	}
	createdEvent, err := payment.NewCreatedEvent(ctx, *data, *entity)
	if err != nil {
		return err
	}
	if err := s.accountService.RecalculateBalance(ctx, entity.Account.ID); err != nil {
		return err
	}
	s.publisher.Publish(createdEvent)
	return nil
}

func (s *PaymentService) Update(ctx context.Context, id uint, data *payment.UpdateDTO) error {
	if err := composables.CanUser(ctx, permission.PaymentUpdate); err != nil {
		return err
	}
	entity := data.ToEntity(id)
	if err := s.repo.Update(ctx, entity); err != nil {
		return err
	}
	updatedEvent, err := payment.NewUpdatedEvent(ctx, *data, *entity)
	if err != nil {
		return err
	}
	if err := s.accountService.RecalculateBalance(ctx, entity.Account.ID); err != nil {
		return err
	}
	s.publisher.Publish(updatedEvent)
	return nil
}

func (s *PaymentService) Delete(ctx context.Context, id uint) (*payment.Payment, error) {
	if err := composables.CanUser(ctx, permission.PaymentDelete); err != nil {
		return nil, err
	}
	entity, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if err := s.repo.Delete(ctx, id); err != nil {
		return nil, err
	}
	deletedEvent, err := payment.NewDeletedEvent(ctx, *entity)
	if err != nil {
		return nil, err
	}
	s.publisher.Publish(deletedEvent)
	return entity, nil
}
