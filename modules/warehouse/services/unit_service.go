package services

import (
	"context"

	unit2 "github.com/iota-agency/iota-sdk/modules/warehouse/domain/entities/unit"
	"github.com/iota-agency/iota-sdk/modules/warehouse/permissions"
	"github.com/iota-agency/iota-sdk/pkg/composables"
	"github.com/iota-agency/iota-sdk/pkg/event"
)

type UnitService struct {
	repo      unit2.Repository
	publisher event.Publisher
}

func NewUnitService(
	repo unit2.Repository,
	publisher event.Publisher,
) *UnitService {
	return &UnitService{
		repo:      repo,
		publisher: publisher,
	}
}

func (s *UnitService) GetByID(ctx context.Context, id uint) (*unit2.Unit, error) {
	if err := composables.CanUser(ctx, permissions.UnitRead); err != nil {
		return nil, err
	}
	return s.repo.GetByID(ctx, id)
}

func (s *UnitService) GetAll(ctx context.Context) ([]*unit2.Unit, error) {
	if err := composables.CanUser(ctx, permissions.UnitRead); err != nil {
		return nil, err
	}
	return s.repo.GetAll(ctx)
}

func (s *UnitService) GetPaginated(
	ctx context.Context,
	limit, offset int,
	sortBy []string,
) ([]*unit2.Unit, error) {
	if err := composables.CanUser(ctx, permissions.ProductRead); err != nil {
		return nil, err
	}
	return s.repo.GetPaginated(ctx, limit, offset, sortBy)
}

func (s *UnitService) Create(ctx context.Context, data *unit2.CreateDTO) error {
	if err := composables.CanUser(ctx, permissions.UnitCreate); err != nil {
		return err
	}
	entity, err := data.ToEntity()
	if err != nil {
		return err
	}
	if err := s.repo.Create(ctx, entity); err != nil {
		return err
	}
	createdEvent, err := unit2.NewCreatedEvent(ctx, *data, *entity)
	if err != nil {
		return err
	}
	s.publisher.Publish(createdEvent)
	return nil
}

func (s *UnitService) Update(ctx context.Context, id uint, data *unit2.UpdateDTO) error {
	if err := composables.CanUser(ctx, permissions.UnitUpdate); err != nil {
		return err
	}
	entity, err := data.ToEntity(id)
	if err != nil {
		return err
	}
	if err := s.repo.Update(ctx, entity); err != nil {
		return err
	}
	updatedEvent, err := unit2.NewUpdatedEvent(ctx, *data, *entity)
	if err != nil {
		return err
	}
	s.publisher.Publish(updatedEvent)
	return nil
}

func (s *UnitService) Delete(ctx context.Context, id uint) (*unit2.Unit, error) {
	if err := composables.CanUser(ctx, permissions.UnitDelete); err != nil {
		return nil, err
	}
	entity, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if err := s.repo.Delete(ctx, id); err != nil {
		return nil, err
	}
	deletedEvent, err := unit2.NewDeletedEvent(ctx, *entity)
	if err != nil {
		return nil, err
	}
	s.publisher.Publish(deletedEvent)
	return entity, nil
}
func (s *UnitService) Count(ctx context.Context) (uint, error) {
	return s.repo.Count(ctx)
}
