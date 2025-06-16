package services

import (
	"context"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/modules/finance/domain/entities/counterparty"
	"github.com/iota-uz/iota-sdk/pkg/composables"
)

type CounterpartyService struct {
	repo counterparty.Repository
}

func NewCounterpartyService(repo counterparty.Repository) *CounterpartyService {
	return &CounterpartyService{
		repo: repo,
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
		return createErr
	})
	return result, err
}

func (s *CounterpartyService) Update(ctx context.Context, entity counterparty.Counterparty) (counterparty.Counterparty, error) {
	var result counterparty.Counterparty
	err := composables.InTx(ctx, func(txCtx context.Context) error {
		var updateErr error
		result, updateErr = s.repo.Update(txCtx, entity)
		return updateErr
	})
	return result, err
}

func (s *CounterpartyService) Delete(ctx context.Context, id uuid.UUID) (counterparty.Counterparty, error) {
	entity, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	err = composables.InTx(ctx, func(txCtx context.Context) error {
		return s.repo.Delete(txCtx, id)
	})
	if err != nil {
		return nil, err
	}
	return entity, nil
}

func (s *CounterpartyService) Count(ctx context.Context, params *counterparty.FindParams) (int64, error) {
	return s.repo.Count(ctx, params)
}
