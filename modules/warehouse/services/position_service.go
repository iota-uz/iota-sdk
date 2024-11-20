package services

import (
	"context"
	position2 "github.com/iota-agency/iota-sdk/modules/warehouse/domain/entities/position"
	"github.com/iota-agency/iota-sdk/modules/warehouse/permissions"
	"github.com/iota-agency/iota-sdk/pkg/composables"
	"github.com/iota-agency/iota-sdk/pkg/event"
)

type PositionService struct {
	repo      position2.Repository
	publisher event.Publisher
}

func NewPositionService(
	repo position2.Repository,
	publisher event.Publisher,
) *PositionService {
	return &PositionService{
		repo:      repo,
		publisher: publisher,
	}
}

func (s *PositionService) GetByID(ctx context.Context, id uint) (*position2.Position, error) {
	if err := composables.CanUser(ctx, permissions.PositionRead); err != nil {
		return nil, err
	}
	return s.repo.GetByID(ctx, id)
}

func (s *PositionService) GetAll(ctx context.Context) ([]*position2.Position, error) {
	if err := composables.CanUser(ctx, permissions.PositionRead); err != nil {
		return nil, err
	}
	return s.repo.GetAll(ctx)
}

func (s *PositionService) GetPaginated(ctx context.Context, params *position2.FindParams) ([]*position2.Position, error) {
	if err := composables.CanUser(ctx, permissions.PositionRead); err != nil {
		return nil, err
	}
	return s.repo.GetPaginated(ctx, params)
}

func (s *PositionService) Create(ctx context.Context, data *position2.CreateDTO) error {
	if err := composables.CanUser(ctx, permissions.PositionCreate); err != nil {
		return err
	}
	entity, err := data.ToEntity()
	if err != nil {
		return err
	}
	if err := s.repo.Create(ctx, entity); err != nil {
		return err
	}
	createdEvent, err := position2.NewCreatedEvent(ctx, *data, *entity)
	if err != nil {
		return err
	}
	s.publisher.Publish(createdEvent)
	return nil
}

func (s *PositionService) Update(ctx context.Context, id uint, data *position2.UpdateDTO) error {
	if err := composables.CanUser(ctx, permissions.PositionUpdate); err != nil {
		return err
	}
	entity, err := data.ToEntity(id)
	if err != nil {
		return err
	}
	if err := s.repo.Update(ctx, entity); err != nil {
		return err
	}
	updatedEvent, err := position2.NewUpdatedEvent(ctx, *data, *entity)
	if err != nil {
		return err
	}
	s.publisher.Publish(updatedEvent)
	return nil
}

func (s *PositionService) Delete(ctx context.Context, id uint) (*position2.Position, error) {
	if err := composables.CanUser(ctx, permissions.PositionDelete); err != nil {
		return nil, err
	}
	entity, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if err := s.repo.Delete(ctx, id); err != nil {
		return nil, err
	}
	deletedEvent, err := position2.NewDeletedEvent(ctx, *entity)
	if err != nil {
		return nil, err
	}
	s.publisher.Publish(deletedEvent)
	return entity, nil
}
