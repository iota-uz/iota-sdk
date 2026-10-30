// Package services provides this package.
package services

import (
	"context"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/modules/finance/domain/entities/counterparty"
	"github.com/iota-uz/iota-sdk/modules/logging/history"
	"github.com/iota-uz/iota-sdk/pkg/composables"
)

type CounterpartyService struct {
	repo          counterparty.Repository
	historyWriter history.Writer
}

func NewCounterpartyService(repo counterparty.Repository, historyWriter history.Writer) *CounterpartyService {
	return &CounterpartyService{
		repo:          repo,
		historyWriter: historyWriter,
	}
}

func (s *CounterpartyService) GetByID(ctx context.Context, id uuid.UUID) (counterparty.Counterparty, error) {
	return s.repo.GetByID(ctx, id)
}

func (s *CounterpartyService) GetAll(ctx context.Context) ([]counterparty.Counterparty, error) {
	return s.repo.GetAll(ctx)
}

func (s *CounterpartyService) GetPaginated(ctx context.Context, params *counterparty.FindParams) ([]counterparty.Counterparty, error) {
	return s.repo.GetPaginated(ctx, params)
}

func (s *CounterpartyService) Create(ctx context.Context, entity counterparty.Counterparty) (counterparty.Counterparty, error) {
	var result counterparty.Counterparty
	err := composables.InTx(ctx, func(txCtx context.Context) error {
		var createErr error
		result, createErr = s.repo.Create(txCtx, entity)
		if createErr != nil {
			return createErr
		}
		return history.RecordMutation(txCtx, s.historyWriter, history.Mutation{
			Type: CounterpartyCreated, Version: 1, Resource: CounterpartyResource, ID: result.ID().String(),
			After: counterpartySnapshot(result),
		})
	})
	return result, err
}

func (s *CounterpartyService) Update(ctx context.Context, entity counterparty.Counterparty) (counterparty.Counterparty, error) {
	var result counterparty.Counterparty
	err := composables.InTx(ctx, func(txCtx context.Context) error {
		before, err := s.repo.GetByID(txCtx, entity.ID())
		if err != nil {
			return err
		}
		result, err = s.repo.Update(txCtx, entity)
		if err != nil {
			return err
		}
		return history.RecordMutation(txCtx, s.historyWriter, history.Mutation{
			Type: CounterpartyUpdated, Version: 1, Resource: CounterpartyResource, ID: entity.ID().String(),
			Before: counterpartySnapshot(before), After: counterpartySnapshot(result),
		})
	})
	return result, err
}

func (s *CounterpartyService) Delete(ctx context.Context, id uuid.UUID) (counterparty.Counterparty, error) {
	entity, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	err = composables.InTx(ctx, func(txCtx context.Context) error {
		if err := s.repo.Delete(txCtx, id); err != nil {
			return err
		}
		return history.RecordMutation(txCtx, s.historyWriter, history.Mutation{
			Type: CounterpartyDeleted, Version: 1, Resource: CounterpartyResource, ID: id.String(),
			Before: counterpartySnapshot(entity),
		})
	})
	if err != nil {
		return nil, err
	}
	return entity, nil
}

func (s *CounterpartyService) Count(ctx context.Context, params *counterparty.FindParams) (int64, error) {
	return s.repo.Count(ctx, params)
}
