// Package services provides this package.
package services

import (
	"context"

	"github.com/google/uuid"
	category "github.com/iota-uz/iota-sdk/modules/finance/domain/aggregates/expense_category"
	"github.com/iota-uz/iota-sdk/modules/logging/history"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/eventbus"
)

type ExpenseCategoryService struct {
	repo          category.Repository
	publisher     eventbus.EventBus
	historyWriter history.Writer
}

func NewExpenseCategoryService(repo category.Repository, publisher eventbus.EventBus, historyWriter history.Writer) *ExpenseCategoryService {
	return &ExpenseCategoryService{
		repo:          repo,
		publisher:     publisher,
		historyWriter: historyWriter,
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
	var createdEntity category.ExpenseCategory
	err = composables.InTx(ctx, func(txCtx context.Context) error {
		var createErr error
		createdEntity, createErr = s.repo.Create(txCtx, entity)
		if createErr != nil {
			return createErr
		}
		return history.RecordMutation(txCtx, s.historyWriter, history.Mutation{
			Type: ExpenseCategoryCreated, Version: 1, Resource: ExpenseCategoryResource, ID: createdEntity.ID().String(),
			After: expenseCategorySnapshot(createdEntity),
		})
	})
	if err != nil {
		return nil, err
	}
	createdEvent.Result = createdEntity
	s.publisher.Publish(createdEvent)
	return createdEntity, nil
}

func (s *ExpenseCategoryService) Update(ctx context.Context, entity category.ExpenseCategory) (category.ExpenseCategory, error) {
	updatedEvent, err := category.NewUpdatedEvent(ctx, entity)
	if err != nil {
		return nil, err
	}
	var updatedEntity category.ExpenseCategory
	err = composables.InTx(ctx, func(txCtx context.Context) error {
		before, err := s.repo.GetByID(txCtx, entity.ID())
		if err != nil {
			return err
		}
		updatedEntity, err = s.repo.Update(txCtx, entity)
		if err != nil {
			return err
		}
		return history.RecordMutation(txCtx, s.historyWriter, history.Mutation{
			Type: ExpenseCategoryUpdated, Version: 1, Resource: ExpenseCategoryResource, ID: entity.ID().String(),
			Before: expenseCategorySnapshot(before), After: expenseCategorySnapshot(updatedEntity),
		})
	})
	if err != nil {
		return nil, err
	}
	updatedEvent.Result = updatedEntity
	s.publisher.Publish(updatedEvent)
	return updatedEntity, nil
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
	err = composables.InTx(ctx, func(txCtx context.Context) error {
		if err := s.repo.Delete(txCtx, id); err != nil {
			return err
		}
		return history.RecordMutation(txCtx, s.historyWriter, history.Mutation{
			Type: ExpenseCategoryDeleted, Version: 1, Resource: ExpenseCategoryResource, ID: id.String(),
			Before: expenseCategorySnapshot(entity),
		})
	})
	if err != nil {
		return nil, err
	}
	deletedEvent.Result = entity
	s.publisher.Publish(deletedEvent)
	return entity, nil
}
