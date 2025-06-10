package services

import (
	"context"

	"github.com/iota-uz/iota-sdk/modules/finance/domain/aggregates/payment"
	"github.com/iota-uz/iota-sdk/modules/finance/permissions"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/eventbus"
)

type PaymentService struct {
	repo           payment.Repository
	publisher      eventbus.EventBus
	accountService *MoneyAccountService
}

func NewPaymentService(
	repo payment.Repository,
	publisher eventbus.EventBus,
	accountService *MoneyAccountService,
) *PaymentService {
	return &PaymentService{
		repo:           repo,
		publisher:      publisher,
		accountService: accountService,
	}
}

func (s *PaymentService) GetByID(ctx context.Context, id uint) (payment.Payment, error) {
	if err := composables.CanUser(ctx, permissions.PaymentRead); err != nil {
		return nil, err
	}
	return s.repo.GetByID(ctx, id)
}

func (s *PaymentService) GetAll(ctx context.Context) ([]payment.Payment, error) {
	if err := composables.CanUser(ctx, permissions.PaymentRead); err != nil {
		return nil, err
	}
	return s.repo.GetAll(ctx)
}

func (s *PaymentService) GetPaginated(
	ctx context.Context, params *payment.FindParams,
) ([]payment.Payment, error) {
	if err := composables.CanUser(ctx, permissions.PaymentRead); err != nil {
		return nil, err
	}
	return s.repo.GetPaginated(ctx, params)
}

func (s *PaymentService) Create(ctx context.Context, entity payment.Payment) error {
	if err := composables.CanUser(ctx, permissions.PaymentCreate); err != nil {
		return err
	}

	createdEvent, err := payment.NewCreatedEvent(ctx, entity, entity)
	if err != nil {
		return err
	}

	var createdEntity payment.Payment
	err = composables.InTx(ctx, func(txCtx context.Context) error {
		var err error
		createdEntity, err = s.repo.Create(txCtx, entity)
		if err != nil {
			return err
		}
		return s.accountService.RecalculateBalance(txCtx, createdEntity.Account().ID())
	})
	if err != nil {
		return err
	}

	createdEvent.Result = createdEntity
	s.publisher.Publish(createdEvent)
	return nil
}

func (s *PaymentService) Update(ctx context.Context, entity payment.Payment) error {
	if err := composables.CanUser(ctx, permissions.PaymentUpdate); err != nil {
		return err
	}

	updatedEvent, err := payment.NewUpdatedEvent(ctx, entity, entity)
	if err != nil {
		return err
	}

	err = composables.InTx(ctx, func(txCtx context.Context) error {
		if err := s.repo.Update(txCtx, entity); err != nil {
			return err
		}
		return s.accountService.RecalculateBalance(txCtx, entity.Account().ID())
	})
	if err != nil {
		return err
	}

	s.publisher.Publish(updatedEvent)
	return nil
}

func (s *PaymentService) Delete(ctx context.Context, id uint) (payment.Payment, error) {
	if err := composables.CanUser(ctx, permissions.PaymentDelete); err != nil {
		return nil, err
	}

	entity, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	deletedEvent, err := payment.NewDeletedEvent(ctx, entity)
	if err != nil {
		return nil, err
	}

	err = composables.InTx(ctx, func(txCtx context.Context) error {
		if err := s.repo.Delete(txCtx, id); err != nil {
			return err
		}
		return s.accountService.RecalculateBalance(txCtx, entity.Account().ID())
	})
	if err != nil {
		return nil, err
	}

	s.publisher.Publish(deletedEvent)
	return entity, nil
}

func (s *PaymentService) Count(ctx context.Context) (int64, error) {
	return s.repo.Count(ctx)
}
