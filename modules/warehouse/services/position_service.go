package services

import (
	"context"
	"errors"
	"github.com/iota-agency/iota-sdk/modules/warehouse/domain/aggregates/position"
	"github.com/iota-agency/iota-sdk/modules/warehouse/domain/entities/unit"
	"github.com/iota-agency/iota-sdk/modules/warehouse/permissions"
	"github.com/iota-agency/iota-sdk/pkg/application"
	"github.com/iota-agency/iota-sdk/pkg/composables"
	"github.com/iota-agency/iota-sdk/pkg/event"
	"github.com/iota-agency/iota-sdk/pkg/services"
	"github.com/xuri/excelize/v2"
	"gorm.io/gorm"
)

type PositionService struct {
	repo          position.Repository
	publisher     event.Publisher
	uploadService *services.UploadService
	unitService   *UnitService
}

func NewPositionService(
	repo position.Repository,
	publisher event.Publisher,
	app application.Application,
) *PositionService {
	return &PositionService{
		repo:          repo,
		publisher:     publisher,
		uploadService: app.Service(services.UploadService{}).(*services.UploadService),
		unitService:   app.Service(UnitService{}).(*UnitService),
	}
}

func (s *PositionService) GetByID(ctx context.Context, id uint) (*position.Position, error) {
	if err := composables.CanUser(ctx, permissions.PositionRead); err != nil {
		return nil, err
	}
	return s.repo.GetByID(ctx, id)
}

func (s *PositionService) GetAll(ctx context.Context) ([]*position.Position, error) {
	if err := composables.CanUser(ctx, permissions.PositionRead); err != nil {
		return nil, err
	}
	return s.repo.GetAll(ctx)
}

func (s *PositionService) GetPaginated(ctx context.Context, params *position.FindParams) ([]*position.Position, error) {
	if err := composables.CanUser(ctx, permissions.PositionRead); err != nil {
		return nil, err
	}
	return s.repo.GetPaginated(ctx, params)
}

func (s *PositionService) findOrCreateUnit(ctx context.Context, unitName string) (*unit.Unit, error) {
	u, err := s.unitService.GetByTitleOrShortTitle(ctx, unitName)
	if err == nil {
		return u, nil
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return s.unitService.Create(ctx, &unit.CreateDTO{
			Title:      unitName,
			ShortTitle: unitName,
		})
	}
	return nil, err
}

func (s *PositionService) createPosition(ctx context.Context, posRow *position.XlsRow, unitId uint) error {
	pos := &position.CreateDTO{
		Title:   posRow.Title,
		Barcode: posRow.Barcode,
		UnitID:  unitId,
	}
	return s.Create(ctx, pos)
}

func (s *PositionService) UploadFromFile(ctx context.Context, fileID uint) error {
	if err := composables.CanUser(ctx, permissions.PositionCreate); err != nil {
		return err
	}
	uploadEntity, err := s.uploadService.GetByID(ctx, fileID)
	if err != nil {
		return err
	}
	f, err := excelize.OpenFile(uploadEntity.Path)
	if err != nil {
		return err
	}
	defer f.Close()
	sheets := f.GetSheetList()
	rows, err := f.GetRows(sheets[0])
	if err != nil {
		return err
	}
	for _, row := range rows {
		posRow, err := position.XlsRowFromStrings(row)
		if err != nil && errors.Is(err, position.ErrInvalidRow) {
			continue
		}
		if err != nil {
			return err
		}
		unitEntity, err := s.findOrCreateUnit(ctx, posRow.Unit)
		if err != nil {
			return err
		}
		entity, err := s.repo.GetByBarcode(ctx, posRow.Barcode)
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		if errors.Is(err, gorm.ErrRecordNotFound) {
			if err := s.createPosition(ctx, posRow, unitEntity.ID); err != nil {
				return err
			}
		} else {
			pos := &position.UpdateDTO{
				Title:   posRow.Title,
				UnitID:  unitEntity.ID,
				Barcode: posRow.Barcode,
			}
			if err := s.Update(ctx, entity.ID, pos); err != nil {
				return err
			}
		}
	}
	return nil
}

func (s *PositionService) Create(ctx context.Context, data *position.CreateDTO) error {
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
	createdEvent, err := position.NewCreatedEvent(ctx, *data, *entity)
	if err != nil {
		return err
	}
	s.publisher.Publish(createdEvent)
	return nil
}

func (s *PositionService) Update(ctx context.Context, id uint, data *position.UpdateDTO) error {
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
	updatedEvent, err := position.NewUpdatedEvent(ctx, *data, *entity)
	if err != nil {
		return err
	}
	s.publisher.Publish(updatedEvent)
	return nil
}

func (s *PositionService) Delete(ctx context.Context, id uint) (*position.Position, error) {
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
	deletedEvent, err := position.NewDeletedEvent(ctx, *entity)
	if err != nil {
		return nil, err
	}
	s.publisher.Publish(deletedEvent)
	return entity, nil
}

func (s *PositionService) Count(ctx context.Context) (int64, error) {
	return s.repo.Count(ctx)
}
