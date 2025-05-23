package services

import (
	"context"

	"github.com/go-faster/errors"

	moneyaccount "github.com/iota-uz/iota-sdk/modules/finance/domain/aggregates/money_account"
	"github.com/iota-uz/iota-sdk/modules/finance/domain/entities/transaction"
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

func (s *MoneyAccountService) GetByID(ctx context.Context, id uint) (*moneyaccount.Account, error) {
	return s.repo.GetByID(ctx, id)
}

func (s *MoneyAccountService) GetAll(ctx context.Context) ([]*moneyaccount.Account, error) {
	return s.repo.GetAll(ctx)
}

func (s *MoneyAccountService) GetPaginated(
	ctx context.Context, params *moneyaccount.FindParams,
) ([]*moneyaccount.Account, error) {
	return s.repo.GetPaginated(ctx, params)
}

func (s *MoneyAccountService) RecalculateBalance(ctx context.Context, id uint) error {
	return s.repo.RecalculateBalance(ctx, id)
}

func (s *MoneyAccountService) Create(ctx context.Context, data *moneyaccount.CreateDTO) error {
	entity, err := data.ToEntity()
	if err != nil {
		return err
	}
	createdEntity, err := s.repo.Create(ctx, entity)
	if err != nil {
		return errors.Wrap(err, "accountRepo.Create")
	}
	if err := s.transactionRepo.Create(ctx, createdEntity.InitialTransaction()); err != nil {
		return errors.Wrap(err, "transactionRepo.Create")
	}
	createdEvent, err := moneyaccount.NewCreatedEvent(ctx, *data, *createdEntity)
	if err != nil {
		return err
	}
	s.publisher.Publish(createdEvent)
	return nil
}

func (s *MoneyAccountService) Update(ctx context.Context, id uint, data *moneyaccount.UpdateDTO) error {
	entity, err := data.ToEntity(id)
	if err != nil {
		return err
	}
	if err := s.repo.Update(ctx, entity); err != nil {
		return err
	}
	updatedEvent, err := moneyaccount.NewUpdatedEvent(ctx, *data, *entity)
	if err != nil {
		return err
	}
	s.publisher.Publish(updatedEvent)
	return nil
}

func (s *MoneyAccountService) Delete(ctx context.Context, id uint) (*moneyaccount.Account, error) {
	entity, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if err := s.repo.Delete(ctx, id); err != nil {
		return nil, err
	}
	deletedEvent, err := moneyaccount.NewDeletedEvent(ctx, *entity)
	if err != nil {
		return nil, err
	}
	s.publisher.Publish(deletedEvent)
	return entity, nil
}

func (s *MoneyAccountService) Count(ctx context.Context) (int64, error) {
	return s.repo.Count(ctx)
}
