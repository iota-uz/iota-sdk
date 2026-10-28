// Package services provides this package.
package services

import (
	"context"
	"strconv"

	"github.com/iota-uz/iota-sdk/modules/hrm/domain/entities/position"
	"github.com/iota-uz/iota-sdk/modules/logging/history"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/eventbus"
)

type PositionService struct {
	repo          position.Repository
	publisher     eventbus.EventBus
	historyWriter history.Writer
}

func NewPositionService(repo position.Repository, publisher eventbus.EventBus, historyWriter history.Writer) *PositionService {
	return &PositionService{
		repo:          repo,
		publisher:     publisher,
		historyWriter: historyWriter,
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
	err := composables.InTx(ctx, func(txCtx context.Context) error {
		if err := s.repo.Create(txCtx, data); err != nil {
			return err
		}
		return history.RecordMutation(txCtx, s.historyWriter, history.Mutation{
			Type: PositionCreated, Version: 1, Resource: PositionResource, ID: positionID(int64(data.ID)),
			After: positionSnapshot(data),
		})
	})
	if err != nil {
		return err
	}
	s.publisher.Publish("position.created", data)
	return nil
}

func (s *PositionService) Update(ctx context.Context, data *position.Position) error {
	err := composables.InTx(ctx, func(txCtx context.Context) error {
		before, err := s.repo.GetByID(txCtx, int64(data.ID))
		if err != nil {
			return err
		}
		if err := s.repo.Update(txCtx, data); err != nil {
			return err
		}
		return history.RecordMutation(txCtx, s.historyWriter, history.Mutation{
			Type: PositionUpdated, Version: 1, Resource: PositionResource, ID: positionID(int64(data.ID)),
			Before: positionSnapshot(before), After: positionSnapshot(data),
		})
	})
	if err != nil {
		return err
	}
	s.publisher.Publish("position.updated", data)
	return nil
}

func (s *PositionService) Delete(ctx context.Context, id int64) error {
	err := composables.InTx(ctx, func(txCtx context.Context) error {
		before, err := s.repo.GetByID(txCtx, id)
		if err != nil {
			return err
		}
		if err := s.repo.Delete(txCtx, id); err != nil {
			return err
		}
		return history.RecordMutation(txCtx, s.historyWriter, history.Mutation{
			Type: PositionDeleted, Version: 1, Resource: PositionResource, ID: positionID(id),
			Before: positionSnapshot(before),
		})
	})
	if err != nil {
		return err
	}
	s.publisher.Publish("position.deleted", id)
	return nil
}

func positionID(id int64) string {
	return strconv.FormatInt(id, 10)
}
