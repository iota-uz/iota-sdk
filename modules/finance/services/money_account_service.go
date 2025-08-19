package services

import (
	"context"

	"github.com/go-faster/errors"
	"github.com/google/uuid"

	moneyaccount "github.com/iota-uz/iota-sdk/modules/finance/domain/aggregates/money_account"
	"github.com/iota-uz/iota-sdk/modules/finance/domain/entities/transaction"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/eventbus"
)

type MoneyAccountService struct {
	repo            moneyaccount.Repository
	transactionRepo transaction.Repository
	publisher       eventbus.EventBus
}

func NewMoneyAccountService(
	repo moneyaccount.Repository,
	transactionRepo transaction.Repository,
	publisher eventbus.EventBus,
) *MoneyAccountService {
	return &MoneyAccountService{
		repo:            repo,
		transactionRepo: transactionRepo,
		publisher:       publisher,
	}
}

func (s *MoneyAccountService) GetByID(ctx context.Context, id uuid.UUID) (moneyaccount.Account, error) {
	return s.repo.GetByID(ctx, id)
}

func (s *MoneyAccountService) GetAll(ctx context.Context) ([]moneyaccount.Account, error) {
	return s.repo.GetAll(ctx)
}

func (s *MoneyAccountService) GetPaginated(
	ctx context.Context, params *moneyaccount.FindParams,
) ([]moneyaccount.Account, error) {
	return s.repo.GetPaginated(ctx, params)
}

func (s *MoneyAccountService) RecalculateBalance(ctx context.Context, id uuid.UUID) error {
	return s.repo.RecalculateBalance(ctx, id)
}

func (s *MoneyAccountService) Create(ctx context.Context, entity moneyaccount.Account) (moneyaccount.Account, error) {
	createdEvent, err := moneyaccount.NewCreatedEvent(ctx, entity, entity)
	if err != nil {
		return nil, err
	}

	var createdEntity moneyaccount.Account
	err = composables.InTx(ctx, func(txCtx context.Context) error {
		createdEntity, err = s.repo.Create(txCtx, entity)
		if err != nil {
			return errors.Wrap(err, "accountRepo.Create")
		}
		if _, err := s.transactionRepo.Create(txCtx, createdEntity.InitialTransaction()); err != nil {
			return errors.Wrap(err, "transactionRepo.Create")
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	createdEvent.Result = createdEntity
	s.publisher.Publish(createdEvent)
	return createdEntity, nil
}

func (s *MoneyAccountService) Update(ctx context.Context, entity moneyaccount.Account) (moneyaccount.Account, error) {
	updatedEvent, err := moneyaccount.NewUpdatedEvent(ctx, entity, entity)
	if err != nil {
		return nil, err
	}

	var updatedEntity moneyaccount.Account
	err = composables.InTx(ctx, func(txCtx context.Context) error {
		updatedEntity, err = s.repo.Update(txCtx, entity)
		if err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	updatedEvent.Result = updatedEntity
	s.publisher.Publish(updatedEvent)
	return updatedEntity, nil
}

func (s *MoneyAccountService) Delete(ctx context.Context, id uuid.UUID) (moneyaccount.Account, error) {
	entity, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	deletedEvent, err := moneyaccount.NewDeletedEvent(ctx, entity)
	if err != nil {
		return nil, err
	}

	err = composables.InTx(ctx, func(txCtx context.Context) error {
		if err := s.repo.Delete(txCtx, id); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	s.publisher.Publish(deletedEvent)
	return entity, nil
}

func (s *MoneyAccountService) Count(ctx context.Context, params *moneyaccount.FindParams) (int64, error) {
	return s.repo.Count(ctx, params)
}
