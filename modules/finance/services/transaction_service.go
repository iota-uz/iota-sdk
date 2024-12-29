package services

import (
	"context"

	transaction2 "github.com/iota-uz/iota-sdk/modules/finance/domain/entities/transaction"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/event"
)

type TransactionService struct {
	repo           transaction2.Repository
	eventPublisher event.Publisher
}

func NewTransactionService(repo transaction2.Repository, eventPublisher *event.Publisher) *TransactionService {
	return &TransactionService{
		repo:           repo,
		eventPublisher: *eventPublisher,
	}
}

func (s *TransactionService) Count(ctx context.Context) (int64, error) {
	return s.repo.Count(ctx)
}

func (s *TransactionService) GetAll(ctx context.Context) ([]*transaction2.Transaction, error) {
	return s.repo.GetAll(ctx)
}

func (s *TransactionService) GetByID(ctx context.Context, id uint) (*transaction2.Transaction, error) {
	return s.repo.GetByID(ctx, id)
}

func (s *TransactionService) GetPaginated(
	ctx context.Context, params *transaction2.FindParams,
) ([]*transaction2.Transaction, error) {
	return s.repo.GetPaginated(ctx, params)
}

func (s *TransactionService) Create(ctx context.Context, data *transaction2.Transaction) error {
	tx, err := composables.UsePoolTx(ctx)
	if err != nil {
		return err
	}
	if err := s.repo.Create(ctx, data); err != nil {
		return err
	}
	s.eventPublisher.Publish("transaction.created", data)
	return tx.Commit(ctx)
}

func (s *TransactionService) Update(ctx context.Context, data *transaction2.Transaction) error {
	tx, err := composables.UsePoolTx(ctx)
	if err != nil {
		return err
	}
	if err := s.repo.Update(ctx, data); err != nil {
		return err
	}
	s.eventPublisher.Publish("transaction.updated", data)
	return tx.Commit(ctx)
}

func (s *TransactionService) Delete(ctx context.Context, id uint) error {
	tx, err := composables.UsePoolTx(ctx)
	if err != nil {
		return err
	}
	if err := s.repo.Delete(ctx, id); err != nil {
		return err
	}
	s.eventPublisher.Publish("transaction.deleted", id)
	return tx.Commit(ctx)
}
