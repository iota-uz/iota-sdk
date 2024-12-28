package services

import (
	"context"

	unit "github.com/iota-agency/iota-sdk/modules/warehouse/domain/entities/unit"
	"github.com/iota-agency/iota-sdk/modules/warehouse/permissions"
	"github.com/iota-agency/iota-sdk/pkg/composables"
	"github.com/iota-agency/iota-sdk/pkg/event"
)

type UnitService struct {
	repo      unit.Repository
	publisher event.Publisher
}

func NewUnitService(
	repo unit.Repository,
	publisher event.Publisher,
) *UnitService {
	return &UnitService{
		repo:      repo,
		publisher: publisher,
	}
}

func (s *UnitService) GetByID(ctx context.Context, id uint) (*unit.Unit, error) {
	if err := composables.CanUser(ctx, permissions.UnitRead); err != nil {
		return nil, err
	}
	return s.repo.GetByID(ctx, id)
}

func (s *UnitService) GetByTitleOrShortTitle(ctx context.Context, name string) (*unit.Unit, error) {
	if err := composables.CanUser(ctx, permissions.UnitRead); err != nil {
		return nil, err
	}
	return s.repo.GetByTitleOrShortTitle(ctx, name)
}

func (s *UnitService) GetAll(ctx context.Context) ([]*unit.Unit, error) {
	if err := composables.CanUser(ctx, permissions.UnitRead); err != nil {
		return nil, err
	}
	return s.repo.GetAll(ctx)
}

func (s *UnitService) GetPaginated(
	ctx context.Context, params *unit.FindParams,
) ([]*unit.Unit, error) {
	if err := composables.CanUser(ctx, permissions.ProductRead); err != nil {
		return nil, err
	}
	return s.repo.GetPaginated(ctx, params)
}

func (s *UnitService) Create(ctx context.Context, data *unit.CreateDTO) (*unit.Unit, error) {
	tx, err := composables.UsePoolTx(ctx)
	if err != nil {
		return nil, err
	}
	if err := composables.CanUser(ctx, permissions.UnitCreate); err != nil {
		return nil, err
	}
	entity, err := data.ToEntity()
	if err != nil {
		return nil, err
	}
	if err := s.repo.Create(ctx, entity); err != nil {
		return nil, err
	}
	createdEvent, err := unit.NewCreatedEvent(ctx, *data, *entity)
	if err != nil {
		return nil, err
	}
	s.publisher.Publish(createdEvent)
	return entity, tx.Commit(ctx)
}

func (s *UnitService) Update(ctx context.Context, id uint, data *unit.UpdateDTO) error {
	tx, err := composables.UsePoolTx(ctx)
	if err != nil {
		return err
	}
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
	updatedEvent, err := unit.NewUpdatedEvent(ctx, *data, *entity)
	if err != nil {
		return err
	}
	s.publisher.Publish(updatedEvent)
	return tx.Commit(ctx)
}

func (s *UnitService) Delete(ctx context.Context, id uint) (*unit.Unit, error) {
	tx, err := composables.UsePoolTx(ctx)
	if err != nil {
		return nil, err
	}
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
	deletedEvent, err := unit.NewDeletedEvent(ctx, *entity)
	if err != nil {
		return nil, err
	}
	s.publisher.Publish(deletedEvent)
	return entity, tx.Commit(ctx)
}
func (s *UnitService) Count(ctx context.Context) (uint, error) {
	return s.repo.Count(ctx)
}
