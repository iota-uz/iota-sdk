package services

import (
	"context"
	moneyAccount "github.com/iota-agency/iota-erp/internal/domain/aggregates/money_account"
	"github.com/iota-agency/iota-erp/internal/domain/entities/permission"
	"github.com/iota-agency/iota-erp/pkg/composables"
	"github.com/iota-agency/iota-erp/pkg/event"
)

type MoneyAccountService struct {
	repo      moneyAccount.Repository
	publisher event.Publisher
}

func NewMoneyAccountService(repo moneyAccount.Repository, publisher event.Publisher) *MoneyAccountService {
	return &MoneyAccountService{
		repo:      repo,
		publisher: publisher,
	}
}

func (s *MoneyAccountService) GetByID(ctx context.Context, id uint) (*moneyAccount.Account, error) {
	if err := composables.CanUser(ctx, permission.AccountRead); err != nil {
		return nil, err
	}
	return s.repo.GetByID(ctx, id)
}

func (s *MoneyAccountService) GetAll(ctx context.Context) ([]*moneyAccount.Account, error) {
	if err := composables.CanUser(ctx, permission.AccountRead); err != nil {
		return nil, err
	}
	return s.repo.GetAll(ctx)
}

func (s *MoneyAccountService) GetPaginated(
	ctx context.Context,
	limit, offset int,
	sortBy []string,
) ([]*moneyAccount.Account, error) {
	if err := composables.CanUser(ctx, permission.AccountRead); err != nil {
		return nil, err
	}
	return s.repo.GetPaginated(ctx, limit, offset, sortBy)
}

// TODO: what the hell am i supposed to do with this?

func (s *MoneyAccountService) RecalculateBalance(ctx context.Context, id uint) error {
	return s.repo.RecalculateBalance(ctx, id)
}

func (s *MoneyAccountService) Create(ctx context.Context, data *moneyAccount.CreateDTO) error {
	if err := composables.CanUser(ctx, permission.AccountCreate); err != nil {
		return err
	}
	entity, err := data.ToEntity()
	if err != nil {
		return err
	}
	if err := s.repo.Create(ctx, entity); err != nil {
		return err
	}
	createdEvent, err := moneyAccount.NewCreatedEvent(ctx, *data, *entity)
	if err != nil {
		return err
	}
	s.publisher.Publish(createdEvent)
	return nil
}

func (s *MoneyAccountService) Update(ctx context.Context, id uint, data *moneyAccount.UpdateDTO) error {
	if err := composables.CanUser(ctx, permission.AccountUpdate); err != nil {
		return err
	}
	entity, err := data.ToEntity(id)
	if err != nil {
		return err
	}
	if err := s.repo.Update(ctx, entity); err != nil {
		return err
	}
	updatedEvent, err := moneyAccount.NewUpdatedEvent(ctx, *data, *entity)
	if err != nil {
		return err
	}
	s.publisher.Publish(updatedEvent)
	return nil
}

func (s *MoneyAccountService) Delete(ctx context.Context, id uint) (*moneyAccount.Account, error) {
	if err := composables.CanUser(ctx, permission.AccountDelete); err != nil {
		return nil, err
	}
	entity, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if err := s.repo.Delete(ctx, id); err != nil {
		return nil, err
	}
	deletedEvent, err := moneyAccount.NewDeletedEvent(ctx, *entity)
	if err != nil {
		return nil, err
	}
	s.publisher.Publish(deletedEvent)
	return entity, nil
}
