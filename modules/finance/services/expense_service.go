package services

import (
	"context"

	"github.com/iota-uz/iota-sdk/modules/finance/domain/aggregates/expense"
	"github.com/iota-uz/iota-sdk/modules/finance/permissions"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/eventbus"
)

type ExpenseService struct {
	repo           expense.Repository
	publisher      eventbus.EventBus
	accountService *MoneyAccountService
}

func NewExpenseService(
	repo expense.Repository,
	publisher eventbus.EventBus,
	accountService *MoneyAccountService,
) *ExpenseService {
	return &ExpenseService{
		repo:           repo,
		publisher:      publisher,
		accountService: accountService,
	}
}

func (s *ExpenseService) GetByID(ctx context.Context, id uint) (expense.Expense, error) {
	if err := composables.CanUser(ctx, permissions.ExpenseRead); err != nil {
		return nil, err
	}
	return s.repo.GetByID(ctx, id)
}

func (s *ExpenseService) GetAll(ctx context.Context) ([]expense.Expense, error) {
	if err := composables.CanUser(ctx, permissions.ExpenseRead); err != nil {
		return nil, err
	}
	return s.repo.GetAll(ctx)
}

func (s *ExpenseService) GetPaginated(
	ctx context.Context, params *expense.FindParams,
) ([]expense.Expense, error) {
	if err := composables.CanUser(ctx, permissions.ExpenseRead); err != nil {
		return nil, err
	}
	return s.repo.GetPaginated(ctx, params)
}

func (s *ExpenseService) Create(ctx context.Context, entity expense.Expense) error {
	if err := composables.CanUser(ctx, permissions.ExpenseCreate); err != nil {
		return err
	}
	if err := s.repo.Create(ctx, entity); err != nil {
		return err
	}
	createdEvent, err := expense.NewCreatedEvent(ctx, entity)
	if err != nil {
		return err
	}
	if err := s.accountService.RecalculateBalance(ctx, entity.Account().ID); err != nil {
		return err
	}
	s.publisher.Publish(createdEvent)
	return nil
}

func (s *ExpenseService) Update(ctx context.Context, entity expense.Expense) error {
	if err := composables.CanUser(ctx, permissions.ExpenseUpdate); err != nil {
		return err
	}
	if err := s.repo.Update(ctx, entity); err != nil {
		return err
	}
	updatedEvent, err := expense.NewUpdatedEvent(ctx, entity)
	if err != nil {
		return err
	}
	if err := s.accountService.RecalculateBalance(ctx, entity.Account().ID); err != nil {
		return err
	}
	s.publisher.Publish(updatedEvent)
	return nil
}

func (s *ExpenseService) Delete(ctx context.Context, id uint) (expense.Expense, error) {
	if err := composables.CanUser(ctx, permissions.ExpenseDelete); err != nil {
		return nil, err
	}
	entity, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if err := s.repo.Delete(ctx, id); err != nil {
		return nil, err
	}
	deletedEvent, err := expense.NewDeletedEvent(ctx, entity)
	if err != nil {
		return nil, err
	}
	s.publisher.Publish(deletedEvent)
	return entity, nil
}

func (s *ExpenseService) Count(ctx context.Context, params *expense.FindParams) (uint, error) {
	count, err := s.repo.Count(ctx, params)
	if err != nil {
		return 0, err
	}
	return uint(count), nil
}
