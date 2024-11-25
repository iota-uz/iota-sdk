package services

import (
	"context"
	"github.com/iota-agency/iota-sdk/modules/finance/domain/aggregates/expense"
	"github.com/iota-agency/iota-sdk/modules/finance/permissions"
	"github.com/iota-agency/iota-sdk/pkg/composables"
	"github.com/iota-agency/iota-sdk/pkg/event"
)

type ExpenseService struct {
	repo           expense.Repository
	publisher      event.Publisher
	accountService *MoneyAccountService
}

func NewExpenseService(
	repo expense.Repository,
	publisher event.Publisher,
	accountService *MoneyAccountService,
) *ExpenseService {
	return &ExpenseService{
		repo:           repo,
		publisher:      publisher,
		accountService: accountService,
	}
}

func (s *ExpenseService) GetByID(ctx context.Context, id uint) (*expense.Expense, error) {
	if err := composables.CanUser(ctx, permissions.ExpenseRead); err != nil {
		return nil, err
	}
	return s.repo.GetByID(ctx, id)
}

func (s *ExpenseService) GetAll(ctx context.Context) ([]*expense.Expense, error) {
	if err := composables.CanUser(ctx, permissions.ExpenseRead); err != nil {
		return nil, err
	}
	return s.repo.GetAll(ctx)
}

func (s *ExpenseService) GetPaginated(
	ctx context.Context,
	limit, offset int,
	sortBy []string,
) ([]*expense.Expense, error) {
	if err := composables.CanUser(ctx, permissions.ExpenseRead); err != nil {
		return nil, err
	}
	return s.repo.GetPaginated(ctx, limit, offset, sortBy)
}

func (s *ExpenseService) Create(ctx context.Context, data *expense.CreateDTO) error {
	if err := composables.CanUser(ctx, permissions.ExpenseCreate); err != nil {
		return err
	}
	entity, err := data.ToEntity()
	if err != nil {
		return err
	}
	if err := s.repo.Create(ctx, entity); err != nil {
		return err
	}
	createdEvent, err := expense.NewCreatedEvent(ctx, *data, *entity)
	if err != nil {
		return err
	}
	if err := s.accountService.RecalculateBalance(ctx, entity.Account.ID); err != nil {
		return err
	}
	s.publisher.Publish(createdEvent)
	return nil
}

func (s *ExpenseService) Update(ctx context.Context, id uint, data *expense.UpdateDTO) error {
	if err := composables.CanUser(ctx, permissions.ExpenseUpdate); err != nil {
		return err
	}
	entity, err := data.ToEntity(id)
	if err != nil {
		return err
	}
	if err := s.repo.Update(ctx, entity); err != nil {
		return err
	}
	updatedEvent, err := expense.NewUpdatedEvent(ctx, *data, *entity)
	if err != nil {
		return err
	}
	if err := s.accountService.RecalculateBalance(ctx, entity.Account.ID); err != nil {
		return err
	}
	s.publisher.Publish(updatedEvent)
	return nil
}

func (s *ExpenseService) Delete(ctx context.Context, id uint) (*expense.Expense, error) {
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
	deletedEvent, err := expense.NewDeletedEvent(ctx, *entity)
	if err != nil {
		return nil, err
	}
	s.publisher.Publish(deletedEvent)
	return entity, nil
}