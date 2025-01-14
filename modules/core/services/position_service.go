package services

import (
	"context"

	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/position"
	"github.com/iota-uz/iota-sdk/pkg/eventbus"
)

type PositionService struct {
	repo      position.Repository
	publisher eventbus.EventBus
}

func NewPositionService(repo position.Repository, publisher eventbus.EventBus) *PositionService {
	return &PositionService{
		repo:      repo,
		publisher: publisher,
	}
}

func (s *PositionService) Count(ctx context.Context) (int64, error) {
	return s.repo.Count(ctx)
}

func (s *PositionService) GetAll(ctx context.Context) ([]*position.Position, error) {
	return s.repo.GetAll(ctx)
}

func (s *PositionService) GetByID(ctx context.Context, id int64) (*position.Position, error) {
	return s.repo.GetByID(ctx, id)
}

func (s *PositionService) GetPaginated(
	ctx context.Context, params *position.FindParams,
) ([]*position.Position, error) {
	return s.repo.GetPaginated(ctx, params)
}

func (s *PositionService) Create(ctx context.Context, data *position.Position) error {
	if err := s.repo.Create(ctx, data); err != nil {
		return err
	}
	s.publisher.Publish("position.created", data)
	return nil
}

func (s *PositionService) Update(ctx context.Context, data *position.Position) error {
	if err := s.repo.Update(ctx, data); err != nil {
		return err
	}
	s.publisher.Publish("position.updated", data)
	return nil
}

func (s *PositionService) Delete(ctx context.Context, id int64) error {
	if err := s.repo.Delete(ctx, id); err != nil {
		return err
	}
	s.publisher.Publish("position.deleted", id)
	return nil
}
