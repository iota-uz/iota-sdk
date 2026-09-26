// Package services provides this package.
package services

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/modules/finance/domain/aggregates/debt"
	moneyaccount "github.com/iota-uz/iota-sdk/modules/finance/domain/aggregates/money_account"
	"github.com/iota-uz/iota-sdk/modules/finance/permissions"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/eventbus"
	"github.com/iota-uz/iota-sdk/pkg/money"
)

var (
	// ErrDebtAccountCurrency means the debt and the account it names hold different currencies.
	ErrDebtAccountCurrency = errors.New("debt currency differs from its account currency")
	// ErrDebtNotOpen means the debt is already settled, written off or cancelled.
	ErrDebtNotOpen = errors.New("debt is not open")
)

type DebtService struct {
	repo        debt.Repository
	accountRepo moneyaccount.Repository
	publisher   eventbus.EventBus
}

func NewDebtService(
	repo debt.Repository,
	accountRepo moneyaccount.Repository,
	publisher eventbus.EventBus,
) *DebtService {
	return &DebtService{
		repo:        repo,
		accountRepo: accountRepo,
		publisher:   publisher,
	}
}

func (s *DebtService) checkAccount(ctx context.Context, entity debt.Debt) error {
	accountID := entity.MoneyAccountID()
	if accountID == nil {
		return nil
	}
	account, err := s.accountRepo.GetByID(ctx, *accountID)
	if err != nil {
		return err
	}
	if !account.Balance().SameCurrency(entity.OriginalAmount()) {
		return ErrDebtAccountCurrency
	}
	return nil
}

func (s *DebtService) GetByID(ctx context.Context, id uuid.UUID) (debt.Debt, error) {
	if err := composables.CanUser(ctx, permissions.DebtRead); err != nil {
		return nil, err
	}
	return s.repo.GetByID(ctx, id)
}

func (s *DebtService) GetAll(ctx context.Context) ([]debt.Debt, error) {
	if err := composables.CanUser(ctx, permissions.DebtRead); err != nil {
		return nil, err
	}
	return s.repo.GetAll(ctx)
}

func (s *DebtService) GetPaginated(
	ctx context.Context, params *debt.FindParams,
) ([]debt.Debt, error) {
	if err := composables.CanUser(ctx, permissions.DebtRead); err != nil {
		return nil, err
	}
	return s.repo.GetPaginated(ctx, params)
}

func (s *DebtService) GetByCounterpartyID(ctx context.Context, counterpartyID uuid.UUID) ([]debt.Debt, error) {
	if err := composables.CanUser(ctx, permissions.DebtRead); err != nil {
		return nil, err
	}
	return s.repo.GetByCounterpartyID(ctx, counterpartyID)
}

func (s *DebtService) GetCounterpartyAggregates(ctx context.Context) ([]debt.CounterpartyAggregate, error) {
	if err := composables.CanUser(ctx, permissions.DebtRead); err != nil {
		return nil, err
	}
	return s.repo.GetCounterpartyAggregates(ctx)
}

func (s *DebtService) Create(ctx context.Context, entity debt.Debt) (debt.Debt, error) {
	if err := composables.CanUser(ctx, permissions.DebtCreate); err != nil {
		return nil, err
	}
	if err := s.checkAccount(ctx, entity); err != nil {
		return nil, err
	}

	createdEvent, err := debt.NewDebtCreatedEvent(ctx, entity, entity)
	if err != nil {
		return nil, err
	}

	var createdEntity debt.Debt
	err = composables.InTx(ctx, func(txCtx context.Context) error {
		var err error
		createdEntity, err = s.repo.Create(txCtx, entity)
		return err
	})
	if err != nil {
		return nil, err
	}

	createdEvent.Result = createdEntity
	s.publisher.Publish(createdEvent)
	return createdEntity, nil
}

func (s *DebtService) Update(ctx context.Context, entity debt.Debt) (debt.Debt, error) {
	if err := composables.CanUser(ctx, permissions.DebtUpdate); err != nil {
		return nil, err
	}
	if err := s.checkAccount(ctx, entity); err != nil {
		return nil, err
	}

	updatedEvent, err := debt.NewDebtUpdatedEvent(ctx, entity, entity)
	if err != nil {
		return nil, err
	}

	var updatedEntity debt.Debt
	err = composables.InTx(ctx, func(txCtx context.Context) error {
		var err error
		updatedEntity, err = s.repo.Update(txCtx, entity)
		return err
	})
	if err != nil {
		return nil, err
	}

	updatedEvent.Result = updatedEntity
	s.publisher.Publish(updatedEvent)
	return updatedEntity, nil
}

func (s *DebtService) Settle(ctx context.Context, debtID uuid.UUID, settlementAmount float64, transactionID *uuid.UUID) (debt.Debt, error) {
	if err := composables.CanUser(ctx, permissions.DebtUpdate); err != nil {
		return nil, err
	}

	entity, err := s.repo.GetByID(ctx, debtID)
	if err != nil {
		return nil, err
	}

	// Calculate new outstanding amount
	currentOutstanding := entity.OutstandingAmount()
	settlementMoney := money.NewFromFloat(settlementAmount, currentOutstanding.Currency().Code)
	newOutstanding, err := currentOutstanding.Subtract(settlementMoney)
	if err != nil {
		return nil, fmt.Errorf("invalid settlement amount: %w", err)
	}

	// Determine status based on remaining outstanding amount
	var newStatus debt.DebtStatus
	if newOutstanding.Amount() <= 0 {
		newStatus = debt.DebtStatusSettled
		newOutstanding = money.New(0, currentOutstanding.Currency().Code)
	} else {
		newStatus = debt.DebtStatusPartial
	}

	// Update debt with new status and outstanding amount
	settledDebt := entity.
		UpdateStatus(newStatus).
		UpdateOutstandingAmount(newOutstanding)

	// Link to settlement transaction if provided
	if transactionID != nil {
		settledDebt = settledDebt.UpdateSettlementTransactionID(transactionID)
	}

	var updatedEntity debt.Debt
	err = composables.InTx(ctx, func(txCtx context.Context) error {
		updatedEntity, err = s.repo.Update(txCtx, settledDebt)
		return err
	})
	if err != nil {
		return nil, err
	}

	// Publish debt settled event
	settledEvent, err := debt.NewDebtSettledEvent(ctx, updatedEntity)
	if err != nil {
		return nil, err
	}
	s.publisher.Publish(settledEvent)
	return updatedEntity, nil
}

func (s *DebtService) WriteOff(ctx context.Context, debtID uuid.UUID) (debt.Debt, error) {
	if err := composables.CanUser(ctx, permissions.DebtUpdate); err != nil {
		return nil, err
	}

	entity, err := s.repo.GetByID(ctx, debtID)
	if err != nil {
		return nil, err
	}

	// Mark debt as written off
	zeroCurrency := entity.OriginalAmount().Currency().Code
	writtenOffDebt := entity.
		UpdateStatus(debt.DebtStatusWrittenOff).
		UpdateOutstandingAmount(money.New(0, zeroCurrency))

	var updatedEntity debt.Debt
	err = composables.InTx(ctx, func(txCtx context.Context) error {
		updatedEntity, err = s.repo.Update(txCtx, writtenOffDebt)
		return err
	})
	if err != nil {
		return nil, err
	}

	// Publish debt written off event
	writtenOffEvent, err := debt.NewDebtWrittenOffEvent(ctx, updatedEntity)
	if err != nil {
		return nil, err
	}
	s.publisher.Publish(writtenOffEvent)
	return updatedEntity, nil
}

// Cancel closes an open debt that is no longer owed. Nothing stays
// outstanding, so a cancelled payable debt stops reserving money.
func (s *DebtService) Cancel(ctx context.Context, debtID uuid.UUID) (debt.Debt, error) {
	if err := composables.CanUser(ctx, permissions.DebtUpdate); err != nil {
		return nil, err
	}

	entity, err := s.repo.GetByID(ctx, debtID)
	if err != nil {
		return nil, err
	}
	if !entity.Status().IsOpen() {
		return nil, ErrDebtNotOpen
	}

	cancelled := entity.
		UpdateStatus(debt.DebtStatusCancelled).
		UpdateOutstandingAmount(money.New(0, entity.OutstandingAmount().Currency().Code))

	var updatedEntity debt.Debt
	err = composables.InTx(ctx, func(txCtx context.Context) error {
		updatedEntity, err = s.repo.Update(txCtx, cancelled)
		return err
	})
	if err != nil {
		return nil, err
	}

	cancelledEvent, err := debt.NewDebtCancelledEvent(ctx, updatedEntity)
	if err != nil {
		return nil, err
	}
	s.publisher.Publish(cancelledEvent)
	return updatedEntity, nil
}

func (s *DebtService) Delete(ctx context.Context, id uuid.UUID) (debt.Debt, error) {
	if err := composables.CanUser(ctx, permissions.DebtDelete); err != nil {
		return nil, err
	}

	entity, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	deletedEvent, err := debt.NewDebtDeletedEvent(ctx, entity)
	if err != nil {
		return nil, err
	}

	err = composables.InTx(ctx, func(txCtx context.Context) error {
		return s.repo.Delete(txCtx, id)
	})
	if err != nil {
		return nil, err
	}

	s.publisher.Publish(deletedEvent)
	return entity, nil
}

func (s *DebtService) Count(ctx context.Context) (int64, error) {
	return s.repo.Count(ctx)
}
