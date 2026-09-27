// Package services provides this package.
package services

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/modules/billing/domain/aggregates/billing"
	"github.com/iota-uz/iota-sdk/modules/billing/domain/aggregates/details"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/eventbus"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
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
	callback  billing.TransactionCallback
	mu        sync.RWMutex
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
		// If provider exists (Click, Payme, Octo, Stripe), use it
		if provider != nil {
			providedTransaction, err := provider.Create(txCtx, entity)
			if err != nil {
				return err
			}
			createdTransaction, err = s.repo.Save(txCtx, providedTransaction)
			return err
		}

		// For Details-only gateways (Cash, Integrator), save directly
		createdTransaction, err = s.repo.Save(txCtx, entity)
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

// ErrTransactionNotCancellable is returned when a cancellation is asked for a
// transaction that has already left the active set: refunded, partially
// refunded, expired, failed, completed, or cancelled once already.
//
// Cancelling one of those describes nothing that can still happen to the money.
// The record would be rewritten to say the payment was never taken, and for a
// refunded transaction that is worse than wrong — the refund disappears from
// the history, leaving money that went back to the customer recorded as money
// that never arrived.
//
// The gateway providers each refuse a *completed* transaction in their own
// protocol's terms (ErrPaymeCancelAfterCompletion, ErrOctoCancelAfterCapture,
// ErrClickCancelAfterPayment). This refuses every other terminal state, and
// refuses it for every gateway — including Cash and Integrator, which have no
// provider at all and whose cancellation is a bare status write with nothing
// standing in front of it.
var ErrTransactionNotCancellable = errors.New("transaction is not active; only a created or pending transaction can be cancelled")

// Cancel voids an unpaid transaction.
//
// A transaction that is no longer active is refused (ErrTransactionNotCancellable)
// before the provider is called and before the updated event is built: a
// cancellation that must not happen should neither reach the gateway nor
// announce a status change that did not occur.
//
// The refusal is an error rather than a silent no-op, including for a
// transaction that is already cancelled. Nothing in this SDK cancels twice — a
// gateway-initiated cancellation (Payme's CancelTransaction) is handled by
// PaymeController.cancel through Save and never arrives here — so the only way
// to reach this method with a dead transaction is a caller that believes the
// transaction is alive, and that caller is better told than humoured.
func (s *BillingService) Cancel(ctx context.Context, cmd *CancelTransactionCommand) (billing.Transaction, error) {
	const op serrors.Op = "BillingService.Cancel"

	entity, err := s.repo.GetByID(ctx, cmd.TransactionID)
	if err != nil {
		return nil, serrors.E(op, err)
	}

	if !entity.Status().IsActive() {
		return nil, serrors.E(op, fmt.Errorf("%w: status %q", ErrTransactionNotCancellable, entity.Status()))
	}

	provider := s.providers[entity.Gateway()]

	updatedEvent, err := billing.NewUpdatedEvent(ctx, entity)
	if err != nil {
		return nil, serrors.E(op, err)
	}

	var updatedTransaction billing.Transaction
	err = composables.InTx(ctx, func(txCtx context.Context) error {
		// If provider exists, use it
		if provider != nil {
			providedTransaction, err := provider.Cancel(txCtx, entity)
			if err != nil {
				return err
			}
			updatedTransaction, err = s.repo.Save(txCtx, providedTransaction)
			return err
		}

		// For Details-only gateways, just update status to Canceled
		entity = entity.SetStatus(billing.Canceled)
		updatedTransaction, err = s.repo.Save(txCtx, entity)
		return err
	})
	if err != nil {
		return nil, serrors.E(op, err)
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
		// If provider exists, use it
		if provider != nil {
			providedTransaction, err := provider.Refund(txCtx, entity, cmd.Quantity)
			if err != nil {
				return err
			}
			updatedTransaction, err = s.repo.Save(txCtx, providedTransaction)
			return err
		}

		// For Details-only gateways, update status based on refund amount
		if cmd.Quantity >= entity.Amount().Quantity() {
			entity = entity.SetStatus(billing.Refunded)
		} else {
			entity = entity.SetStatus(billing.PartiallyRefunded)
		}
		updatedTransaction, err = s.repo.Save(txCtx, entity)
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

// RegisterCallback registers a callback function to be invoked during transaction processing.
// This method is thread-safe and can be called concurrently.
func (s *BillingService) RegisterCallback(callback billing.TransactionCallback) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.callback = callback
}

// InvokeCallback safely invokes the registered callback if it exists.
// This method is thread-safe and can be called concurrently.
// If the callback panics, the panic is recovered and returned as an error.
func (s *BillingService) InvokeCallback(ctx context.Context, transaction billing.Transaction) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("callback panic: %v", r)
		}
	}()

	s.mu.RLock()
	callback := s.callback
	s.mu.RUnlock()

	if callback == nil {
		return nil
	}
	return callback(ctx, transaction)
}
