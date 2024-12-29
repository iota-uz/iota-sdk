package services

import (
	"context"

	moneyaccount "github.com/iota-uz/iota-sdk/modules/finance/domain/aggregates/money_account"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/domain/entities/permission"
	"github.com/iota-uz/iota-sdk/pkg/event"
)

type MoneyAccountService struct {
	repo      moneyaccount.Repository
	publisher event.Publisher
}

func NewMoneyAccountService(repo moneyaccount.Repository, publisher event.Publisher) *MoneyAccountService {
	return &MoneyAccountService{
		repo:      repo,
		publisher: publisher,
	}
}

func (s *MoneyAccountService) GetByID(ctx context.Context, id uint) (*moneyaccount.Account, error) {
	if err := composables.CanUser(ctx, permission.AccountRead); err != nil {
		return nil, err
	}
	return s.repo.GetByID(ctx, id)
}

func (s *MoneyAccountService) GetAll(ctx context.Context) ([]*moneyaccount.Account, error) {
	if err := composables.CanUser(ctx, permission.AccountRead); err != nil {
		return nil, err
	}
	return s.repo.GetAll(ctx)
}

func (s *MoneyAccountService) GetPaginated(
	ctx context.Context, params *moneyaccount.FindParams,
) ([]*moneyaccount.Account, error) {
	if err := composables.CanUser(ctx, permission.AccountRead); err != nil {
		return nil, err
	}
	return s.repo.GetPaginated(ctx, params)
}

// TODO: what the hell am i supposed to do with this?

func (s *MoneyAccountService) RecalculateBalance(ctx context.Context, id uint) error {
	return s.repo.RecalculateBalance(ctx, id)
}

func (s *MoneyAccountService) Create(ctx context.Context, data *moneyaccount.CreateDTO) error {
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
	createdEvent, err := moneyaccount.NewCreatedEvent(ctx, *data, *entity)
	if err != nil {
		return err
	}
	s.publisher.Publish(createdEvent)
	return nil
}

func (s *MoneyAccountService) Update(ctx context.Context, id uint, data *moneyaccount.UpdateDTO) error {
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
	updatedEvent, err := moneyaccount.NewUpdatedEvent(ctx, *data, *entity)
	if err != nil {
		return err
	}
	s.publisher.Publish(updatedEvent)
	return nil
}

func (s *MoneyAccountService) Delete(ctx context.Context, id uint) (*moneyaccount.Account, error) {
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
	deletedEvent, err := moneyaccount.NewDeletedEvent(ctx, *entity)
	if err != nil {
		return nil, err
	}
	s.publisher.Publish(deletedEvent)
	return entity, nil
}

func (s *MoneyAccountService) Count(ctx context.Context) (uint, error) {
	return s.repo.Count(ctx)
}
