package services

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/upload"
	"github.com/iota-uz/iota-sdk/modules/finance/domain/aggregates/payment"
	"github.com/iota-uz/iota-sdk/modules/finance/permissions"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/eventbus"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
)

type PaymentService struct {
	repo           payment.Repository
	publisher      eventbus.EventBus
	accountService *MoneyAccountService
	uploadRepo     upload.Repository
}

func NewPaymentService(
	repo payment.Repository,
	publisher eventbus.EventBus,
	accountService *MoneyAccountService,
	uploadRepo upload.Repository,
) *PaymentService {
	return &PaymentService{
		repo:           repo,
		publisher:      publisher,
		accountService: accountService,
		uploadRepo:     uploadRepo,
	}
}

func (s *PaymentService) GetByID(ctx context.Context, id uuid.UUID) (payment.Payment, error) {
	if err := composables.CanUser(ctx, permissions.PaymentRead); err != nil {
		return nil, err
	}
	return s.repo.GetByID(ctx, id)
}

func (s *PaymentService) GetAll(ctx context.Context) ([]payment.Payment, error) {
	if err := composables.CanUser(ctx, permissions.PaymentRead); err != nil {
		return nil, err
	}
	return s.repo.GetAll(ctx)
}

func (s *PaymentService) GetPaginated(
	ctx context.Context, params *payment.FindParams,
) ([]payment.Payment, error) {
	if err := composables.CanUser(ctx, permissions.PaymentRead); err != nil {
		return nil, err
	}
	return s.repo.GetPaginated(ctx, params)
}

func (s *PaymentService) Create(ctx context.Context, entity payment.Payment) (payment.Payment, error) {
	if err := composables.CanUser(ctx, permissions.PaymentCreate); err != nil {
		return nil, err
	}

	createdEvent, err := payment.NewCreatedEvent(ctx, entity, entity)
	if err != nil {
		return nil, err
	}

	var createdEntity payment.Payment
	err = composables.InTx(ctx, func(txCtx context.Context) error {
		var err error
		createdEntity, err = s.repo.Create(txCtx, entity)
		if err != nil {
			return err
		}
		return s.accountService.RecalculateBalance(txCtx, createdEntity.Account().ID())
	})
	if err != nil {
		return nil, err
	}

	createdEvent.Result = createdEntity
	s.publisher.Publish(createdEvent)
	return createdEntity, nil
}

func (s *PaymentService) Update(ctx context.Context, entity payment.Payment) (payment.Payment, error) {
	if err := composables.CanUser(ctx, permissions.PaymentUpdate); err != nil {
		return nil, err
	}

	updatedEvent, err := payment.NewUpdatedEvent(ctx, entity, entity)
	if err != nil {
		return nil, err
	}

	var updatedEntity payment.Payment
	err = composables.InTx(ctx, func(txCtx context.Context) error {
		var err error
		updatedEntity, err = s.repo.Update(txCtx, entity)
		if err != nil {
			return err
		}
		return s.accountService.RecalculateBalance(txCtx, entity.Account().ID())
	})
	if err != nil {
		return nil, err
	}

	updatedEvent.Result = updatedEntity
	s.publisher.Publish(updatedEvent)
	return updatedEntity, nil
}

func (s *PaymentService) Delete(ctx context.Context, id uuid.UUID) (payment.Payment, error) {
	if err := composables.CanUser(ctx, permissions.PaymentDelete); err != nil {
		return nil, err
	}

	entity, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	deletedEvent, err := payment.NewDeletedEvent(ctx, entity)
	if err != nil {
		return nil, err
	}

	err = composables.InTx(ctx, func(txCtx context.Context) error {
		if err := s.repo.Delete(txCtx, id); err != nil {
			return err
		}
		return s.accountService.RecalculateBalance(txCtx, entity.Account().ID())
	})
	if err != nil {
		return nil, err
	}

	s.publisher.Publish(deletedEvent)
	return entity, nil
}

func (s *PaymentService) Count(ctx context.Context) (int64, error) {
	return s.repo.Count(ctx)
}

// AttachFileToPayment attaches an upload to a payment
func (s *PaymentService) AttachFileToPayment(ctx context.Context, paymentID uuid.UUID, uploadID uint) error {
	if err := composables.CanUser(ctx, permissions.PaymentUpdate); err != nil {
		return err
	}

	// Validate payment exists and user has access
	_, err := s.repo.GetByID(ctx, paymentID)
	if err != nil {
		return fmt.Errorf("failed to find payment: %w", err)
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
		return s.repo.AttachFile(txCtx, paymentID, uploadID)
	})
}

// DetachFileFromPayment detaches an upload from a payment
func (s *PaymentService) DetachFileFromPayment(ctx context.Context, paymentID uuid.UUID, uploadID uint) error {
	if err := composables.CanUser(ctx, permissions.PaymentUpdate); err != nil {
		return err
	}

	// Validate payment exists and user has access
	_, err := s.repo.GetByID(ctx, paymentID)
	if err != nil {
		return fmt.Errorf("failed to find payment: %w", err)
	}

	return composables.InTx(ctx, func(txCtx context.Context) error {
		return s.repo.DetachFile(txCtx, paymentID, uploadID)
	})
}

// GetPaymentAttachments returns all upload IDs attached to a payment
func (s *PaymentService) GetPaymentAttachments(ctx context.Context, paymentID uuid.UUID) ([]uint, error) {
	if err := composables.CanUser(ctx, permissions.PaymentRead); err != nil {
		return nil, err
	}

	// Validate payment exists and user has access
	_, err := s.repo.GetByID(ctx, paymentID)
	if err != nil {
		return nil, fmt.Errorf("failed to find payment: %w", err)
	}

	return s.repo.GetAttachments(ctx, paymentID)
}
