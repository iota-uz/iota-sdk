package services

import (
	"context"
	position2 "github.com/iota-uz/iota-sdk/modules/core/domain/entities/position"
	"github.com/iota-uz/iota-sdk/pkg/event"
)

type PositionService struct {
	repo      position2.Repository
	publisher event.Publisher
}

func NewPositionService(repo position2.Repository, publisher event.Publisher) *PositionService {
	return &PositionService{
		repo:      repo,
		publisher: publisher,
	}
}

func (s *PositionService) Count(ctx context.Context) (int64, error) {
	return s.repo.Count(ctx)
}

func (s *PositionService) GetAll(ctx context.Context) ([]*position2.Position, error) {
	return s.repo.GetAll(ctx)
}

func (s *PositionService) GetByID(ctx context.Context, id int64) (*position2.Position, error) {
	return s.repo.GetByID(ctx, id)
}

func (s *PositionService) GetPaginated(
	ctx context.Context, params *position.FindParams,
) ([]*position2.Position, error) {
	return s.repo.GetPaginated(ctx, params)
}

func (s *PositionService) Create(ctx context.Context, data *position2.Position) error {
	tx, err := composables.UsePoolTx(ctx)
	if err != nil {
		return err
	}
	if err := s.repo.Create(ctx, data); err != nil {
		return err
	}
	s.publisher.Publish("position.created", data)
	return tx.Commit(ctx)
}

func (s *PositionService) Update(ctx context.Context, data *position2.Position) error {
	tx, err := composables.UsePoolTx(ctx)
	if err != nil {
		return err
	}
	if err := s.repo.Update(ctx, data); err != nil {
		return err
	}
	s.publisher.Publish("position.updated", data)
	return tx.Commit(ctx)
}

func (s *PositionService) Delete(ctx context.Context, id int64) error {
	tx, err := composables.UsePoolTx(ctx)
	if err != nil {
		return err
	}
	if err := s.repo.Delete(ctx, id); err != nil {
		return err
	}
	s.publisher.Publish("position.deleted", id)
	return tx.Commit(ctx)
}
