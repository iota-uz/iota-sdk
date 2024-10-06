package services

import (
	"context"
	"github.com/iota-agency/iota-erp/internal/domain/aggregates/expense_category"

	"github.com/iota-agency/iota-erp/sdk/event"
)

type ExpenseCategoryService struct {
	Repo      category.Repository
	Publisher *event.Publisher
}

func NewExpenseCategoryService(repo category.Repository, app *Application) *ExpenseCategoryService {
	return &ExpenseCategoryService{
		Repo:      repo,
		Publisher: app.EventPublisher,
	}
}

func (s *ExpenseCategoryService) GetByID(ctx context.Context, id uint) (*category.ExpenseCategory, error) {
	return s.Repo.GetByID(ctx, id)
}

func (s *ExpenseCategoryService) Count(ctx context.Context) (uint, error) {
	return s.Repo.Count(ctx)
}

func (s *ExpenseCategoryService) GetAll(ctx context.Context) ([]*category.ExpenseCategory, error) {
	return s.Repo.GetAll(ctx)
}

func (s *ExpenseCategoryService) GetPaginated(ctx context.Context, limit, offset int, sortBy []string) ([]*category.ExpenseCategory, error) {
	return s.Repo.GetPaginated(ctx, limit, offset, sortBy)
}

func (s *ExpenseCategoryService) Create(ctx context.Context, data *category.CreateDTO) error {
	createdEvent, err := category.NewCreatedEvent(ctx, *data)
	if err != nil {
		return err
	}
	entity, err := data.ToEntity()
	if err != nil {
		return err
	}
	if err := s.Repo.Create(ctx, entity); err != nil {
		return err
	}
	createdEvent.Result = *entity
	s.Publisher.Publish(createdEvent)
	return nil
}

func (s *ExpenseCategoryService) Update(ctx context.Context, id uint, data *category.UpdateDTO) error {
	updatedEvent, err := category.NewUpdatedEvent(ctx, *data)
	if err != nil {
		return err
	}
	entity, err := data.ToEntity(id)
	if err != nil {
		return err
	}
	if err := s.Repo.Update(ctx, entity); err != nil {
		return err
	}
	updatedEvent.Result = *entity
	s.Publisher.Publish(updatedEvent)
	return nil
}

func (s *ExpenseCategoryService) Delete(ctx context.Context, id uint) (*category.ExpenseCategory, error) {
	deletedEvent, err := category.NewDeletedEvent(ctx)
	if err != nil {
		return nil, err
	}
	entity, err := s.Repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if err := s.Repo.Delete(ctx, id); err != nil {
		return nil, err
	}
	deletedEvent.Result = *entity
	s.Publisher.Publish(deletedEvent)
	return entity, nil
}
