package services

import (
	"context"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/modules/finance/domain/entities/counterparty"
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
	return s.repo.Create(ctx, entity)
}

func (s *CounterpartyService) Update(ctx context.Context, entity counterparty.Counterparty) error {
	_, err := s.repo.Update(ctx, entity)
	return err
}

func (s *CounterpartyService) Delete(ctx context.Context, id uuid.UUID) (counterparty.Counterparty, error) {
	entity, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if err := s.repo.Delete(ctx, id); err != nil {
		return nil, err
	}
	return entity, nil
}

func (s *CounterpartyService) Count(ctx context.Context, params *counterparty.FindParams) (int64, error) {
	return s.repo.Count(ctx, params)
}
