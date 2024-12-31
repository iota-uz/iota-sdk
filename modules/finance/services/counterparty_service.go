package services

import (
	"context"

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

func (s *CounterpartyService) GetByID(ctx context.Context, id uint) (counterparty.Counterparty, error) {
	return s.repo.GetByID(ctx, id)
}

func (s *CounterpartyService) GetAll(ctx context.Context) ([]counterparty.Counterparty, error) {
	return s.repo.GetAll(ctx)
}

func (s *CounterpartyService) GetPaginated(ctx context.Context, params *counterparty.FindParams) ([]counterparty.Counterparty, error) {
	return s.repo.GetPaginated(ctx, params)
}

//
//func (s *CounterpartyService) Create(ctx context.Context, data counterparty.CreateDTO) error {
//	entity, err := data.ToEntity()
//	if err != nil {
//		return err
//	}
//	return s.repo.Create(ctx, entity)
//}
//
//func (s *CounterpartyService) Update(ctx context.Context, id uint, data counterparty.UpdateDTO) error {
//	entity, err := data.ToEntity(id)
//	if err != nil {
//		return err
//	}
//	return s.repo.Update(ctx, entity)
//}

func (s *CounterpartyService) Delete(ctx context.Context, id uint) (counterparty.Counterparty, error) {
	entity, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if err := s.repo.Delete(ctx, id); err != nil {
		return nil, err
	}
	return entity, nil
}

func (s *CounterpartyService) Count(ctx context.Context) (int64, error) {
	return s.repo.Count(ctx)
}
