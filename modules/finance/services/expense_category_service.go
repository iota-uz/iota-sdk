package services

import (
	"context"

	category2 "github.com/iota-uz/iota-sdk/modules/finance/domain/aggregates/expense_category"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/event"
)

type ExpenseCategoryService struct {
	repo      category2.Repository
	publisher event.Publisher
}

func NewExpenseCategoryService(repo category2.Repository, publisher event.Publisher) *ExpenseCategoryService {
	return &ExpenseCategoryService{
		repo:      repo,
		publisher: publisher,
	}
}

func (s *ExpenseCategoryService) GetByID(ctx context.Context, id uint) (*category2.ExpenseCategory, error) {
	return s.repo.GetByID(ctx, id)
}

func (s *ExpenseCategoryService) Count(ctx context.Context) (uint, error) {
	return s.repo.Count(ctx)
}

func (s *ExpenseCategoryService) GetAll(ctx context.Context) ([]*category2.ExpenseCategory, error) {
	return s.repo.GetAll(ctx)
}

func (s *ExpenseCategoryService) GetPaginated(
	ctx context.Context, params *category2.FindParams,
) ([]*category2.ExpenseCategory, error) {
	return s.repo.GetPaginated(ctx, params)
}

func (s *ExpenseCategoryService) Create(ctx context.Context, data *category2.CreateDTO) error {
	tx, err := composables.UsePoolTx(ctx)
	if err != nil {
		return err
	}
	createdEvent, err := category2.NewCreatedEvent(ctx, *data)
	if err != nil {
		return err
	}
	entity, err := data.ToEntity()
	if err != nil {
		return err
	}
	if err := s.repo.Create(ctx, entity); err != nil {
		return err
	}
	createdEvent.Result = *entity
	s.publisher.Publish(createdEvent)
	return tx.Commit(ctx)
}

func (s *ExpenseCategoryService) Update(ctx context.Context, id uint, data *category2.UpdateDTO) error {
	tx, err := composables.UsePoolTx(ctx)
	updatedEvent, err := category2.NewUpdatedEvent(ctx, *data)
	if err != nil {
		return err
	}
	entity, err := data.ToEntity(id)
	if err != nil {
		return err
	}
	if err := s.repo.Update(ctx, entity); err != nil {
		return err
	}
	updatedEvent.Result = *entity
	s.publisher.Publish(updatedEvent)
	return tx.Commit(ctx)
}

func (s *ExpenseCategoryService) Delete(ctx context.Context, id uint) (*category2.ExpenseCategory, error) {
	tx, err := composables.UsePoolTx(ctx)
	deletedEvent, err := category2.NewDeletedEvent(ctx)
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
	deletedEvent.Result = *entity
	s.publisher.Publish(deletedEvent)
	return entity, tx.Commit(ctx)
}
