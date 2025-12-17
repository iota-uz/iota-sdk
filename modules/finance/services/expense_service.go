package services

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/upload"
	"github.com/iota-uz/iota-sdk/modules/finance/domain/aggregates/expense"
	"github.com/iota-uz/iota-sdk/modules/finance/permissions"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/eventbus"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
)

type ExpenseService struct {
	repo           expense.Repository
	publisher      eventbus.EventBus
	accountService *MoneyAccountService
	uploadRepo     upload.Repository
}

func NewExpenseService(
	repo expense.Repository,
	publisher eventbus.EventBus,
	accountService *MoneyAccountService,
	uploadRepo upload.Repository,
) *ExpenseService {
	return &ExpenseService{
		repo:           repo,
		publisher:      publisher,
		accountService: accountService,
		uploadRepo:     uploadRepo,
	}
}

func (s *ExpenseService) GetByID(ctx context.Context, id uuid.UUID) (expense.Expense, error) {
	if err := composables.CanUser(ctx, permissions.ExpenseRead); err != nil {
		return nil, err
	}
	return s.repo.GetByID(ctx, id)
}

func (s *ExpenseService) GetAll(ctx context.Context) ([]expense.Expense, error) {
	if err := composables.CanUser(ctx, permissions.ExpenseRead); err != nil {
		return nil, err
	}
	return s.repo.GetAll(ctx)
}

func (s *ExpenseService) GetPaginated(
	ctx context.Context, params *expense.FindParams,
) ([]expense.Expense, error) {
	if err := composables.CanUser(ctx, permissions.ExpenseRead); err != nil {
		return nil, err
	}
	return s.repo.GetPaginated(ctx, params)
}

func (s *ExpenseService) Create(ctx context.Context, entity expense.Expense) (expense.Expense, error) {
	if err := composables.CanUser(ctx, permissions.ExpenseCreate); err != nil {
		return nil, err
	}

	createdEvent, err := expense.NewCreatedEvent(ctx, entity)
	if err != nil {
		return nil, err
	}

	var created expense.Expense
	err = composables.InTx(ctx, func(txCtx context.Context) error {
		created, err = s.repo.Create(txCtx, entity)
		if err != nil {
			return err
		}
		if err := s.accountService.RecalculateBalance(txCtx, entity.Account().ID()); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	s.publisher.Publish(createdEvent)
	return created, nil
}

func (s *ExpenseService) Update(ctx context.Context, entity expense.Expense) (expense.Expense, error) {
	if err := composables.CanUser(ctx, permissions.ExpenseUpdate); err != nil {
		return nil, err
	}

	updatedEvent, err := expense.NewUpdatedEvent(ctx, entity)
	if err != nil {
		return nil, err
	}

	var updated expense.Expense
	err = composables.InTx(ctx, func(txCtx context.Context) error {
		updated, err = s.repo.Update(txCtx, entity)
		if err != nil {
			return err
		}
		if err := s.accountService.RecalculateBalance(txCtx, entity.Account().ID()); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	s.publisher.Publish(updatedEvent)
	return updated, nil
}

func (s *ExpenseService) Delete(ctx context.Context, id uuid.UUID) (expense.Expense, error) {
	if err := composables.CanUser(ctx, permissions.ExpenseDelete); err != nil {
		return nil, err
	}

	entity, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	deletedEvent, err := expense.NewDeletedEvent(ctx, entity)
	if err != nil {
		return nil, err
	}

	err = composables.InTx(ctx, func(txCtx context.Context) error {
		if err := s.repo.Delete(txCtx, id); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	s.publisher.Publish(deletedEvent)
	return entity, nil
}

func (s *ExpenseService) Count(ctx context.Context, params *expense.FindParams) (uint, error) {
	count, err := s.repo.Count(ctx, params)
	if err != nil {
		return 0, err
	}
	return uint(count), nil
}

// AttachFileToExpense attaches an upload to an expense
func (s *ExpenseService) AttachFileToExpense(ctx context.Context, expenseID uuid.UUID, uploadID uint) error {
	if err := composables.CanUser(ctx, permissions.ExpenseUpdate); err != nil {
		return err
	}

	// Validate expense exists and user has access
	_, err := s.repo.GetByID(ctx, expenseID)
	if err != nil {
		return fmt.Errorf("failed to find expense: %w", err)
	}

	// Validate upload exists and belongs to same tenant
	upload, err := s.uploadRepo.GetByID(ctx, uploadID)
	if err != nil {
		return fmt.Errorf("failed to find upload: %w", err)
	}

	// Check tenant isolation
	tenantID, err := composables.UseTenantID(ctx)
	if err != nil {
		return fmt.Errorf("failed to get tenant ID: %w", err)
	}
	if upload.TenantID() != tenantID {
		return serrors.NewError("TENANT_MISMATCH", "upload does not belong to this tenant", "upload.tenant_mismatch")
	}

	return composables.InTx(ctx, func(txCtx context.Context) error {
		return s.repo.AttachFile(txCtx, expenseID, uploadID)
	})
}

// DetachFileFromExpense detaches an upload from an expense
func (s *ExpenseService) DetachFileFromExpense(ctx context.Context, expenseID uuid.UUID, uploadID uint) error {
	if err := composables.CanUser(ctx, permissions.ExpenseUpdate); err != nil {
		return err
	}

	// Validate expense exists and user has access
	_, err := s.repo.GetByID(ctx, expenseID)
	if err != nil {
		return fmt.Errorf("failed to find expense: %w", err)
	}

	return composables.InTx(ctx, func(txCtx context.Context) error {
		return s.repo.DetachFile(txCtx, expenseID, uploadID)
	})
}

// GetExpenseAttachments returns all upload IDs attached to an expense
func (s *ExpenseService) GetExpenseAttachments(ctx context.Context, expenseID uuid.UUID) ([]uint, error) {
	if err := composables.CanUser(ctx, permissions.ExpenseRead); err != nil {
		return nil, err
	}

	// Validate expense exists and user has access
	_, err := s.repo.GetByID(ctx, expenseID)
	if err != nil {
		return nil, fmt.Errorf("failed to find expense: %w", err)
	}

	return s.repo.GetAttachments(ctx, expenseID)
}
