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
	"github.com/iota-uz/iota-sdk/modules/logging/history"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/eventbus"
	"github.com/iota-uz/iota-sdk/pkg/money"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
)

var (
	// ErrDebtAccountCurrency means the debt and the account it names hold different currencies.
	ErrDebtAccountCurrency = errors.New("debt currency differs from its account currency")
	// ErrDebtNotOpen means the debt is already settled, written off or cancelled.
	ErrDebtNotOpen = errors.New("debt is not open")
	// ErrDebtProject means the debt names a project the tenant does not have.
	ErrDebtProject = errors.New("debt project not found")
	// ErrDebtAmountBelowPaid means the new amount of an open debt is less than what was already paid on it.
	ErrDebtAmountBelowPaid = errors.New("debt amount is less than already paid")
)

type DebtService struct {
	repo          debt.Repository
	accountRepo   moneyaccount.Repository
	projects      ProjectDirectory
	publisher     eventbus.EventBus
	historyWriter history.Writer
}

func NewDebtService(
	repo debt.Repository,
	accountRepo moneyaccount.Repository,
	projects ProjectDirectory,
	publisher eventbus.EventBus,
	historyWriter history.Writer,
) *DebtService {
	return &DebtService{
		repo:          repo,
		accountRepo:   accountRepo,
		projects:      projects,
		publisher:     publisher,
		historyWriter: historyWriter,
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

func (s *DebtService) checkProject(ctx context.Context, entity debt.Debt) error {
	const op serrors.Op = "DebtService.checkProject"
	projectID := entity.ProjectID()
	if projectID == nil {
		return nil
	}
	ok, err := s.projects.Has(ctx, *projectID)
	if err != nil {
		return serrors.Wrap(op, err)
	}
	if !ok {
		return ErrDebtProject
	}
	return nil
}

// checkChange keeps a closed debt closed and an open debt's amount at least
// what was already paid on it.
func checkChange(existing, entity debt.Debt) error {
	if !existing.Status().IsOpen() {
		if entity.Status() != existing.Status() {
			return ErrDebtNotOpen
		}
		return nil
	}
	paid := existing.OriginalAmount().Amount() - existing.OutstandingAmount().Amount()
	if entity.OriginalAmount().Amount() < paid {
		return ErrDebtAmountBelowPaid
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
	if err := s.checkProject(ctx, entity); err != nil {
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
		if err != nil {
			return err
		}
		return history.RecordMutation(txCtx, s.historyWriter, history.Mutation{
			Type: DebtCreated, Version: 1, Resource: DebtResource, ID: createdEntity.ID().String(),
			After: debtSnapshot(createdEntity),
		})
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
	existing, err := s.repo.GetByID(ctx, entity.ID())
	if err != nil {
		return nil, err
	}
	if err := checkChange(existing, entity); err != nil {
		return nil, err
	}
	if err := s.checkAccount(ctx, entity); err != nil {
		return nil, err
	}
	if err := s.checkProject(ctx, entity); err != nil {
		return nil, err
	}

	updatedEvent, err := debt.NewDebtUpdatedEvent(ctx, entity, entity)
	if err != nil {
		return nil, err
	}

	var updatedEntity debt.Debt
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
			Type: DebtUpdated, Version: 1, Resource: DebtResource, ID: entity.ID().String(),
			Before: debtSnapshot(before), After: debtSnapshot(updatedEntity),
		})
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

	var updatedEntity debt.Debt
	err := composables.InTx(ctx, func(txCtx context.Context) error {
		entity, err := s.repo.GetByID(txCtx, debtID)
		if err != nil {
			return err
		}

		// Calculate new outstanding amount
		currentOutstanding := entity.OutstandingAmount()
		settlementMoney := money.NewFromFloat(settlementAmount, currentOutstanding.Currency().Code)
		newOutstanding, err := currentOutstanding.Subtract(settlementMoney)
		if err != nil {
			return fmt.Errorf("invalid settlement amount: %w", err)
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

		updatedEntity, err = s.repo.Update(txCtx, settledDebt)
		if err != nil {
			return err
		}
		return s.recordDebtChange(txCtx, DebtSettled, entity, updatedEntity)
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

	var updatedEntity debt.Debt
	err := composables.InTx(ctx, func(txCtx context.Context) error {
		entity, err := s.repo.GetByID(txCtx, debtID)
		if err != nil {
			return err
		}

		// Mark debt as written off
		zeroCurrency := entity.OriginalAmount().Currency().Code
		writtenOffDebt := entity.
			UpdateStatus(debt.DebtStatusWrittenOff).
			UpdateOutstandingAmount(money.New(0, zeroCurrency))

		updatedEntity, err = s.repo.Update(txCtx, writtenOffDebt)
		if err != nil {
			return err
		}
		return s.recordDebtChange(txCtx, DebtWrittenOff, entity, updatedEntity)
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

	var updatedEntity debt.Debt
	err := composables.InTx(ctx, func(txCtx context.Context) error {
		entity, err := s.repo.GetByID(txCtx, debtID)
		if err != nil {
			return err
		}
		if !entity.Status().IsOpen() {
			return ErrDebtNotOpen
		}

		cancelled := entity.
			UpdateStatus(debt.DebtStatusCancelled).
			UpdateOutstandingAmount(money.New(0, entity.OutstandingAmount().Currency().Code))

		updatedEntity, err = s.repo.Update(txCtx, cancelled)
		if err != nil {
			return err
		}
		return s.recordDebtChange(txCtx, DebtCancelled, entity, updatedEntity)
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
		if err := s.repo.Delete(txCtx, id); err != nil {
			return err
		}
		return history.RecordMutation(txCtx, s.historyWriter, history.Mutation{
			Type: DebtDeleted, Version: 1, Resource: DebtResource, ID: id.String(),
			Before: debtSnapshot(entity),
		})
	})
	if err != nil {
		return nil, err
	}

	s.publisher.Publish(deletedEvent)
	return entity, nil
}

func (s *DebtService) recordDebtChange(ctx context.Context, eventType string, before, after debt.Debt) error {
	return history.RecordMutation(ctx, s.historyWriter, history.Mutation{
		Type: eventType, Version: 1, Resource: DebtResource, ID: after.ID().String(),
		Before: debtSnapshot(before), After: debtSnapshot(after),
	})
}

func (s *DebtService) Count(ctx context.Context) (int64, error) {
	return s.repo.Count(ctx)
}
