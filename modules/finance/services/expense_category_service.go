package services

import (
	"context"

	"github.com/google/uuid"
	category "github.com/iota-uz/iota-sdk/modules/finance/domain/aggregates/expense_category"
	"github.com/iota-uz/iota-sdk/pkg/eventbus"
)

type ExpenseCategoryService struct {
	repo      category.Repository
	publisher eventbus.EventBus
}

func NewExpenseCategoryService(repo category.Repository, publisher eventbus.EventBus) *ExpenseCategoryService {
	return &ExpenseCategoryService{
		repo:      repo,
		publisher: publisher,
	}
}

func (s *ExpenseCategoryService) GetByID(ctx context.Context, id uuid.UUID) (category.ExpenseCategory, error) {
	return s.repo.GetByID(ctx, id)
}

func (s *ExpenseCategoryService) Count(ctx context.Context, params *category.FindParams) (uint, error) {
	count, err := s.repo.Count(ctx, params)
	if err != nil {
		return 0, err
	}
	return uint(count), nil
}

func (s *ExpenseCategoryService) GetAll(ctx context.Context) ([]category.ExpenseCategory, error) {
	return s.repo.GetAll(ctx)
}

func (s *ExpenseCategoryService) GetPaginated(
	ctx context.Context, params *category.FindParams,
) ([]category.ExpenseCategory, error) {
	return s.repo.GetPaginated(ctx, params)
}

func (s *ExpenseCategoryService) Create(ctx context.Context, entity category.ExpenseCategory) (category.ExpenseCategory, error) {
	createdEvent, err := category.NewCreatedEvent(ctx, entity)
	if err != nil {
		return nil, err
	}
	createdEntity, err := s.repo.Create(ctx, entity)
	if err != nil {
		return nil, err
	}
	createdEvent.Result = createdEntity
	s.publisher.Publish(createdEvent)
	return createdEntity, nil
}

func (s *ExpenseCategoryService) Update(ctx context.Context, entity category.ExpenseCategory) error {
	updatedEvent, err := category.NewUpdatedEvent(ctx, entity)
	if err != nil {
		return err
	}
	if _, err := s.repo.Update(ctx, entity); err != nil {
		return err
	}
	updatedEvent.Result = entity
	s.publisher.Publish(updatedEvent)
	return nil
}

func (s *ExpenseCategoryService) Delete(ctx context.Context, id uuid.UUID) (category.ExpenseCategory, error) {
	deletedEvent, err := category.NewDeletedEvent(ctx)
	if err != nil {
		return nil, err
	}
	entity, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if err := s.repo.Delete(ctx, id); err != nil {
		return nil, err
	}
	deletedEvent.Result = entity
	s.publisher.Publish(deletedEvent)
	return entity, nil
}
