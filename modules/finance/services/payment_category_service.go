package services

import (
	"context"

	"github.com/google/uuid"
	paymentcategory "github.com/iota-uz/iota-sdk/modules/finance/domain/aggregates/payment_category"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/eventbus"
)

type PaymentCategoryService struct {
	repo      paymentcategory.Repository
	publisher eventbus.EventBus
}

func NewPaymentCategoryService(repo paymentcategory.Repository, publisher eventbus.EventBus) *PaymentCategoryService {
	return &PaymentCategoryService{
		repo:      repo,
		publisher: publisher,
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

func (s *PaymentCategoryService) Create(ctx context.Context, entity paymentcategory.PaymentCategory) error {
	createdEvent, err := paymentcategory.NewCreatedEvent(ctx, entity)
	if err != nil {
		return err
	}
	err = composables.InTx(ctx, func(txCtx context.Context) error {
		_, createErr := s.repo.Create(txCtx, entity)
		return createErr
	})
	if err != nil {
		return err
	}
	createdEvent.Result = entity
	s.publisher.Publish(createdEvent)
	return nil
}

func (s *PaymentCategoryService) Update(ctx context.Context, entity paymentcategory.PaymentCategory) error {
	updatedEvent, err := paymentcategory.NewUpdatedEvent(ctx, entity)
	if err != nil {
		return err
	}
	err = composables.InTx(ctx, func(txCtx context.Context) error {
		_, updateErr := s.repo.Update(txCtx, entity)
		return updateErr
	})
	if err != nil {
		return err
	}
	updatedEvent.Result = entity
	s.publisher.Publish(updatedEvent)
	return nil
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
		return s.repo.Delete(txCtx, id)
	})
	if err != nil {
		return nil, err
	}
	deletedEvent.Result = entity
	s.publisher.Publish(deletedEvent)
	return entity, nil
}
