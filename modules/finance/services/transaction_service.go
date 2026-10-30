// Package services provides this package.
package services

import (
	"context"

	"github.com/google/uuid"
	transaction2 "github.com/iota-uz/iota-sdk/modules/finance/domain/entities/transaction"
	"github.com/iota-uz/iota-sdk/modules/logging/history"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/eventbus"
)

type TransactionService struct {
	repo           transaction2.Repository
	eventPublisher eventbus.EventBus
	historyWriter  history.Writer
}

func NewTransactionService(repo transaction2.Repository, eventPublisher eventbus.EventBus, historyWriter history.Writer) *TransactionService {
	return &TransactionService{
		repo:           repo,
		eventPublisher: eventPublisher,
		historyWriter:  historyWriter,
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

func (s *TransactionService) Create(ctx context.Context, data transaction2.Transaction) (transaction2.Transaction, error) {
	var entity transaction2.Transaction
	err := composables.InTx(ctx, func(txCtx context.Context) error {
		var err error
		entity, err = s.repo.Create(txCtx, data)
		if err != nil {
			return err
		}
		return history.RecordMutation(txCtx, s.historyWriter, history.Mutation{
			Type: TransactionCreated, Version: 1, Resource: TransactionResource, ID: entity.ID().String(),
			After: transactionSnapshot(entity),
		})
	})
	if err != nil {
		return nil, err
	}
	s.eventPublisher.Publish("transaction.created", entity)
	return entity, nil
}

func (s *TransactionService) Update(ctx context.Context, data transaction2.Transaction) (transaction2.Transaction, error) {
	var entity transaction2.Transaction
	err := composables.InTx(ctx, func(txCtx context.Context) error {
		before, err := s.repo.GetByID(txCtx, data.ID())
		if err != nil {
			return err
		}
		entity, err = s.repo.Update(txCtx, data)
		if err != nil {
			return err
		}
		return history.RecordMutation(txCtx, s.historyWriter, history.Mutation{
			Type: TransactionUpdated, Version: 1, Resource: TransactionResource, ID: data.ID().String(),
			Before: transactionSnapshot(before), After: transactionSnapshot(entity),
		})
	})
	if err != nil {
		return nil, err
	}
	s.eventPublisher.Publish("transaction.updated", entity)
	return entity, nil
}

func (s *TransactionService) Delete(ctx context.Context, id uuid.UUID) error {
	err := composables.InTx(ctx, func(txCtx context.Context) error {
		entity, err := s.repo.GetByID(txCtx, id)
		if err != nil {
			return err
		}
		if err := s.repo.Delete(txCtx, id); err != nil {
			return err
		}
		return history.RecordMutation(txCtx, s.historyWriter, history.Mutation{
			Type: TransactionDeleted, Version: 1, Resource: TransactionResource, ID: id.String(),
			Before: transactionSnapshot(entity),
		})
	})
	if err != nil {
		return err
	}
	s.eventPublisher.Publish("transaction.deleted", id)
	return nil
}
