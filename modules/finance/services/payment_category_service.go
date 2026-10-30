// Package services provides this package.
package services

import (
	"context"

	"github.com/google/uuid"
	paymentcategory "github.com/iota-uz/iota-sdk/modules/finance/domain/aggregates/payment_category"
	"github.com/iota-uz/iota-sdk/modules/logging/history"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/eventbus"
)

type PaymentCategoryService struct {
	repo          paymentcategory.Repository
	publisher     eventbus.EventBus
	historyWriter history.Writer
}

func NewPaymentCategoryService(repo paymentcategory.Repository, publisher eventbus.EventBus, historyWriter history.Writer) *PaymentCategoryService {
	return &PaymentCategoryService{
		repo:          repo,
		publisher:     publisher,
		historyWriter: historyWriter,
	}
}

func (s *PaymentCategoryService) GetByID(ctx context.Context, id uuid.UUID) (paymentcategory.PaymentCategory, error) {
	return s.repo.GetByID(ctx, id)
}

func (s *PaymentCategoryService) Count(ctx context.Context, params *paymentcategory.FindParams) (uint, error) {
	count, err := s.repo.Count(ctx, params)
	if err != nil {
		return 0, err
	}
	return uint(count), nil
}

func (s *PaymentCategoryService) GetAll(ctx context.Context) ([]paymentcategory.PaymentCategory, error) {
	return s.repo.GetAll(ctx)
}

func (s *PaymentCategoryService) GetPaginated(
	ctx context.Context, params *paymentcategory.FindParams,
) ([]paymentcategory.PaymentCategory, error) {
	return s.repo.GetPaginated(ctx, params)
}

func (s *PaymentCategoryService) Create(ctx context.Context, entity paymentcategory.PaymentCategory) (paymentcategory.PaymentCategory, error) {
	createdEvent, err := paymentcategory.NewCreatedEvent(ctx, entity)
	if err != nil {
		return nil, err
	}
	var created paymentcategory.PaymentCategory
	err = composables.InTx(ctx, func(txCtx context.Context) error {
		var createErr error
		created, createErr = s.repo.Create(txCtx, entity)
		if createErr != nil {
			return createErr
		}
		return history.RecordMutation(txCtx, s.historyWriter, history.Mutation{
			Type: PaymentCategoryCreated, Version: 1, Resource: PaymentCategoryResource, ID: created.ID().String(),
			After: paymentCategorySnapshot(created),
		})
	})
	if err != nil {
		return nil, err
	}
	createdEvent.Result = created
	s.publisher.Publish(createdEvent)
	return created, nil
}

func (s *PaymentCategoryService) Update(ctx context.Context, entity paymentcategory.PaymentCategory) (paymentcategory.PaymentCategory, error) {
	updatedEvent, err := paymentcategory.NewUpdatedEvent(ctx, entity)
	if err != nil {
		return nil, err
	}
	var updated paymentcategory.PaymentCategory
	err = composables.InTx(ctx, func(txCtx context.Context) error {
		before, err := s.repo.GetByID(txCtx, entity.ID())
		if err != nil {
			return err
		}
		updated, err = s.repo.Update(txCtx, entity)
		if err != nil {
			return err
		}
		return history.RecordMutation(txCtx, s.historyWriter, history.Mutation{
			Type: PaymentCategoryUpdated, Version: 1, Resource: PaymentCategoryResource, ID: entity.ID().String(),
			Before: paymentCategorySnapshot(before), After: paymentCategorySnapshot(updated),
		})
	})
	if err != nil {
		return nil, err
	}
	updatedEvent.Result = updated
	s.publisher.Publish(updatedEvent)
	return updated, nil
}

func (s *PaymentCategoryService) Delete(ctx context.Context, id uuid.UUID) (paymentcategory.PaymentCategory, error) {
	deletedEvent, err := paymentcategory.NewDeletedEvent(ctx)
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
			Type: PaymentCategoryDeleted, Version: 1, Resource: PaymentCategoryResource, ID: id.String(),
			Before: paymentCategorySnapshot(entity),
		})
	})
	if err != nil {
		return nil, err
	}
	deletedEvent.Result = entity
	s.publisher.Publish(deletedEvent)
	return entity, nil
}
