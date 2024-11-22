package services

import (
	"context"
	payment2 "github.com/iota-agency/iota-sdk/modules/finance/domain/aggregates/payment"
	"github.com/iota-agency/iota-sdk/pkg/composables"
	"github.com/iota-agency/iota-sdk/pkg/domain/entities/permission"
	"github.com/iota-agency/iota-sdk/pkg/event"
)

type PaymentService struct {
	repo           payment2.Repository
	publisher      event.Publisher
	accountService *MoneyAccountService
}

func NewPaymentService(
	repo payment2.Repository,
	publisher event.Publisher,
	accountService *MoneyAccountService,
) *PaymentService {
	return &PaymentService{
		repo:           repo,
		publisher:      publisher,
		accountService: accountService,
	}
}

func (s *PaymentService) GetByID(ctx context.Context, id uint) (*payment2.Payment, error) {
	if err := composables.CanUser(ctx, permission.PaymentRead); err != nil {
		return nil, err
	}
	return s.repo.GetByID(ctx, id)
}

func (s *PaymentService) GetAll(ctx context.Context) ([]*payment2.Payment, error) {
	if err := composables.CanUser(ctx, permission.PaymentRead); err != nil {
		return nil, err
	}
	return s.repo.GetAll(ctx)
}

func (s *PaymentService) GetPaginated(
	ctx context.Context,
	limit, offset int,
	sortBy []string,
) ([]*payment2.Payment, error) {
	if err := composables.CanUser(ctx, permission.PaymentRead); err != nil {
		return nil, err
	}
	return s.repo.GetPaginated(ctx, limit, offset, sortBy)
}

func (s *PaymentService) Create(ctx context.Context, data *payment2.CreateDTO) error {
	if err := composables.CanUser(ctx, permission.PaymentCreate); err != nil {
		return err
	}
	entity := data.ToEntity()
	if err := s.repo.Create(ctx, entity); err != nil {
		return err
	}
	createdEvent, err := payment2.NewCreatedEvent(ctx, *data, *entity)
	if err != nil {
		return err
	}
	if err := s.accountService.RecalculateBalance(ctx, entity.Account.ID); err != nil {
		return err
	}
	s.publisher.Publish(createdEvent)
	return nil
}

func (s *PaymentService) Update(ctx context.Context, id uint, data *payment2.UpdateDTO) error {
	if err := composables.CanUser(ctx, permission.PaymentUpdate); err != nil {
		return err
	}
	entity := data.ToEntity(id)
	if err := s.repo.Update(ctx, entity); err != nil {
		return err
	}
	updatedEvent, err := payment2.NewUpdatedEvent(ctx, *data, *entity)
	if err != nil {
		return err
	}
	if err := s.accountService.RecalculateBalance(ctx, entity.Account.ID); err != nil {
		return err
	}
	s.publisher.Publish(updatedEvent)
	return nil
}

func (s *PaymentService) Delete(ctx context.Context, id uint) (*payment2.Payment, error) {
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
	deletedEvent, err := payment2.NewDeletedEvent(ctx, *entity)
	if err != nil {
		return nil, err
	}
	s.publisher.Publish(deletedEvent)
	return entity, nil
}
