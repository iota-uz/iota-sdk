package services

import (
	"context"

	"github.com/google/uuid"
	transaction2 "github.com/iota-uz/iota-sdk/modules/finance/domain/entities/transaction"
	"github.com/iota-uz/iota-sdk/pkg/eventbus"
)

type TransactionService struct {
	repo           transaction2.Repository
	eventPublisher eventbus.EventBus
}

func NewTransactionService(repo transaction2.Repository, eventPublisher *eventbus.EventBus) *TransactionService {
	return &TransactionService{
		repo:           repo,
		eventPublisher: *eventPublisher,
	}
}

func (s *TransactionService) Count(ctx context.Context) (int64, error) {
	return s.repo.Count(ctx)
}

func (s *TransactionService) GetAll(ctx context.Context) ([]transaction2.Transaction, error) {
	return s.repo.GetAll(ctx)
}

func (s *TransactionService) GetByID(ctx context.Context, id uuid.UUID) (transaction2.Transaction, error) {
	return s.repo.GetByID(ctx, id)
}

func (s *TransactionService) GetPaginated(
	ctx context.Context, params *transaction2.FindParams,
) ([]transaction2.Transaction, error) {
	return s.repo.GetPaginated(ctx, params)
}

func (s *TransactionService) Create(ctx context.Context, data transaction2.Transaction) error {
	if _, err := s.repo.Create(ctx, data); err != nil {
		return err
	}
	s.eventPublisher.Publish("transaction.created", data)
	return nil
}

func (s *TransactionService) Update(ctx context.Context, data transaction2.Transaction) error {
	if _, err := s.repo.Update(ctx, data); err != nil {
		return err
	}
	s.eventPublisher.Publish("transaction.updated", data)
	return nil
}

func (s *TransactionService) Delete(ctx context.Context, id uuid.UUID) error {
	if err := s.repo.Delete(ctx, id); err != nil {
		return err
	}
	s.eventPublisher.Publish("transaction.deleted", id)
	return nil
}
