package services

import (
	"context"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/modules/billing/domain/aggregates/billing"
	"github.com/iota-uz/iota-sdk/modules/billing/domain/aggregates/details"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/eventbus"
)

type CreateTransactionCommand struct {
	TenantID uuid.UUID
	Quantity float64
	Currency billing.Currency
	Gateway  billing.Gateway
	Details  details.Details
}

type CancelTransactionCommand struct {
	TransactionID uuid.UUID
}

type RefundTransactionCommand struct {
	TransactionID uuid.UUID
	Quantity      float64
}

type BillingService struct {
	repo      billing.Repository
	providers map[billing.Gateway]billing.Provider
	publisher eventbus.EventBus
}

func NewBillingService(
	repo billing.Repository,
	providers []billing.Provider,
	publisher eventbus.EventBus,
) *BillingService {
	providerMap := make(map[billing.Gateway]billing.Provider)
	for _, provider := range providers {
		providerMap[provider.Gateway()] = provider
	}

	return &BillingService{
		repo:      repo,
		providers: providerMap,
		publisher: publisher,
	}
}

func (s *BillingService) Count(ctx context.Context, params *billing.FindParams) (int64, error) {
	return s.repo.Count(ctx, params)
}

func (s *BillingService) GetByID(ctx context.Context, id uuid.UUID) (billing.Transaction, error) {
	return s.repo.GetByID(ctx, id)
}

func (s *BillingService) GetByDetailsFields(
	ctx context.Context,
	gateway billing.Gateway,
	filters []billing.DetailsFieldFilter,
) ([]billing.Transaction, error) {
	return s.repo.GetByDetailsFields(ctx, gateway, filters)
}

func (s *BillingService) GetPaginated(ctx context.Context, params *billing.FindParams) ([]billing.Transaction, error) {
	return s.repo.GetPaginated(ctx, params)
}

func (s *BillingService) Create(ctx context.Context, cmd *CreateTransactionCommand) (billing.Transaction, error) {
	entity := billing.New(
		cmd.Quantity,
		cmd.Currency,
		cmd.Gateway,
		cmd.Details,
		billing.WithTenantID(cmd.TenantID),
	)

	provider := s.providers[entity.Gateway()]

	createdEvent, err := billing.NewCreatedEvent(ctx, entity)
	if err != nil {
		return nil, err
	}

	var createdTransaction billing.Transaction
	err = composables.InTx(ctx, func(txCtx context.Context) error {
		providedTransaction, err := provider.Create(txCtx, entity)
		if err != nil {
			return err
		}
		createdTransaction, err = s.repo.Save(txCtx, providedTransaction)
		return err
	})
	if err != nil {
		return nil, err
	}

	createdEvent.Result = createdTransaction
	s.publisher.Publish(createdEvent)

	return createdTransaction, nil
}

func (s *BillingService) Save(ctx context.Context, entity billing.Transaction) (billing.Transaction, error) {
	var (
		createdEvent *billing.CreatedEvent
		updatedEvent *billing.UpdatedEvent
		err          error
	)

	isCreate := entity.ID() == uuid.Nil

	if isCreate {
		createdEvent, err = billing.NewCreatedEvent(ctx, entity)
		if err != nil {
			return nil, err
		}
	} else {
		updatedEvent, err = billing.NewUpdatedEvent(ctx, entity)
		if err != nil {
			return nil, err
		}
	}

	var savedTransaction billing.Transaction
	if err := composables.InTx(ctx, func(txCtx context.Context) error {
		savedTransaction, err = s.repo.Save(txCtx, entity)
		return err
	}); err != nil {
		return nil, err
	}

	if isCreate {
		createdEvent.Result = savedTransaction
		s.publisher.Publish(createdEvent)
	} else {
		updatedEvent.Result = savedTransaction
		s.publisher.Publish(updatedEvent)
	}

	for _, e := range savedTransaction.Events() {
		s.publisher.Publish(e)
	}

	return savedTransaction, nil
}

func (s *BillingService) Cancel(ctx context.Context, cmd *CancelTransactionCommand) (billing.Transaction, error) {
	entity, err := s.repo.GetByID(ctx, cmd.TransactionID)
	if err != nil {
		return nil, err
	}

	provider := s.providers[entity.Gateway()]

	updatedEvent, err := billing.NewUpdatedEvent(ctx, entity)
	if err != nil {
		return nil, err
	}

	var updatedTransaction billing.Transaction
	err = composables.InTx(ctx, func(txCtx context.Context) error {
		providedTransaction, err := provider.Cancel(txCtx, entity)
		if err != nil {
			return err
		}
		updatedTransaction, err = s.repo.Save(txCtx, providedTransaction)
		return err
	})
	if err != nil {
		return nil, err
	}

	updatedEvent.Result = updatedTransaction
	s.publisher.Publish(updatedEvent)
	for _, e := range updatedTransaction.Events() {
		s.publisher.Publish(e)
	}

	return updatedTransaction, nil
}

func (s *BillingService) Refund(ctx context.Context, cmd *RefundTransactionCommand) (billing.Transaction, error) {
	entity, err := s.repo.GetByID(ctx, cmd.TransactionID)
	if err != nil {
		return nil, err
	}

	provider := s.providers[entity.Gateway()]

	updatedEvent, err := billing.NewUpdatedEvent(ctx, entity)
	if err != nil {
		return nil, err
	}

	var updatedTransaction billing.Transaction
	err = composables.InTx(ctx, func(txCtx context.Context) error {
		providedTransaction, err := provider.Refund(txCtx, entity, cmd.Quantity)
		if err != nil {
			return err
		}
		updatedTransaction, err = s.repo.Save(txCtx, providedTransaction)
		return err
	})
	if err != nil {
		return nil, err
	}

	updatedEvent.Result = updatedTransaction
	s.publisher.Publish(updatedEvent)
	for _, e := range updatedTransaction.Events() {
		s.publisher.Publish(e)
	}

	return updatedTransaction, nil
}

func (s *BillingService) Delete(ctx context.Context, id uuid.UUID) (billing.Transaction, error) {
	entity, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	deletedEvent, err := billing.NewDeletedEvent(ctx, entity)
	if err != nil {
		return nil, err
	}

	var deletedTransaction billing.Transaction
	err = composables.InTx(ctx, func(txCtx context.Context) error {
		if err := s.repo.Delete(txCtx, id); err != nil {
			return err
		} else {
			deletedTransaction = entity
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	deletedEvent.Result = deletedTransaction

	s.publisher.Publish(deletedEvent)

	return deletedTransaction, nil
}
