package services

import (
	"context"
	"github.com/iota-agency/iota-sdk/pkg/event"

	"github.com/iota-agency/iota-sdk/pkg/domain/entities/transaction"
)

type TransactionService struct {
	repo           transaction.Repository
	eventPublisher event.Publisher
}

func NewTransactionService(repo transaction.Repository, eventPublisher *event.Publisher) *TransactionService {
	return &TransactionService{
		repo:           repo,
		eventPublisher: *eventPublisher,
	}
}

func (s *TransactionService) Count(ctx context.Context) (int64, error) {
	return s.repo.Count(ctx)
}

func (s *TransactionService) GetAll(ctx context.Context) ([]*transaction.Transaction, error) {
	return s.repo.GetAll(ctx)
}

func (s *TransactionService) GetByID(ctx context.Context, id int64) (*transaction.Transaction, error) {
	return s.repo.GetByID(ctx, id)
}

func (s *TransactionService) GetPaginated(
	ctx context.Context,
	limit, offset int,
	sortBy []string,
) ([]*transaction.Transaction, error) {
	return s.repo.GetPaginated(ctx, limit, offset, sortBy)
}

func (s *TransactionService) Create(ctx context.Context, data *transaction.Transaction) error {
	if err := s.repo.Create(ctx, data); err != nil {
		return err
	}
	s.eventPublisher.Publish("transaction.created", data)
	return nil
}

func (s *TransactionService) Update(ctx context.Context, data *transaction.Transaction) error {
	if err := s.repo.Update(ctx, data); err != nil {
		return err
	}
	s.eventPublisher.Publish("transaction.updated", data)
	return nil
}

func (s *TransactionService) Delete(ctx context.Context, id int64) error {
	if err := s.repo.Delete(ctx, id); err != nil {
		return err
	}
	s.eventPublisher.Publish("transaction.deleted", id)
	return nil
}
