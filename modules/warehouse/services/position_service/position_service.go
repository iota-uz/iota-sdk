package positionservice

import (
	"context"
	"errors"
	"github.com/iota-agency/iota-sdk/modules/warehouse/domain/aggregates/position"
	"github.com/iota-agency/iota-sdk/modules/warehouse/domain/aggregates/product"
	"github.com/iota-agency/iota-sdk/modules/warehouse/domain/entities/unit"
	"github.com/iota-agency/iota-sdk/modules/warehouse/permissions"
	"github.com/iota-agency/iota-sdk/modules/warehouse/services"
	"github.com/iota-agency/iota-sdk/modules/warehouse/services/product_service"
	"github.com/iota-agency/iota-sdk/pkg/application"
	"github.com/iota-agency/iota-sdk/pkg/composables"
	"github.com/iota-agency/iota-sdk/pkg/event"
	coreservices "github.com/iota-agency/iota-sdk/pkg/services"
	"gorm.io/gorm"
)

type PositionService struct {
	repo           position.Repository
	publisher      event.Publisher
	uploadService  *coreservices.UploadService
	unitService    *services.UnitService
	productService *productservice.ProductService
}

func NewPositionService(
	repo position.Repository,
	publisher event.Publisher,
	app application.Application,
) *PositionService {
	return &PositionService{
		repo:           repo,
		publisher:      publisher,
		uploadService:  app.Service(coreservices.UploadService{}).(*coreservices.UploadService),
		unitService:    app.Service(services.UnitService{}).(*services.UnitService),
		productService: app.Service(productservice.ProductService{}).(*productservice.ProductService),
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

func (s *PositionService) GetByIDs(ctx context.Context, ids []uint) ([]*position.Position, error) {
	if err := composables.CanUser(ctx, permissions.PositionRead); err != nil {
		return nil, err
	}
	return s.repo.GetByIDs(ctx, ids)
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

func (s *PositionService) createPosition(ctx context.Context, posRow *XlsRow, unitId uint) error {
	data := &position.CreateDTO{
		Title:   posRow.Title,
		Barcode: posRow.Barcode,
		UnitID:  unitId,
	}
	pos, err := s.Create(ctx, data)
	if err != nil {
		return err
	}
	if posRow.Quantity == 0 {
		return nil
	}
	products := make([]*product.CreateDTO, 0)
	for i := 0; i < posRow.Quantity; i++ {
		products = append(products, &product.CreateDTO{
			PositionID: pos.ID,
			Status:     string(product.InStock),
		})
	}
	return s.productService.BulkCreate(ctx, products)
}

func (s *PositionService) LoadFromFilePath(ctx context.Context, path string) error {
	rows, err := positionRowsFromFile(path)
	if err != nil {
		return err
	}
	unitNameToID := make(map[string]uint)
	for _, u := range uniqueUnits(rows) {
		entity, err := s.findOrCreateUnit(ctx, u)
		if err != nil {
			return err
		}
		unitNameToID[u] = entity.ID
	}

	for _, row := range rows {
		unitID := unitNameToID[row.Unit]
		entity, err := s.repo.GetByBarcode(ctx, row.Barcode)
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		if errors.Is(err, gorm.ErrRecordNotFound) {
			if err := s.createPosition(ctx, row, unitID); err != nil {
				return err
			}
		} else {
			pos := &position.UpdateDTO{
				Title:   row.Title,
				UnitID:  unitID,
				Barcode: row.Barcode,
			}
			if err := s.Update(ctx, entity.ID, pos); err != nil {
				return err
			}
		}
	}
	return nil
}

func (s *PositionService) UpdateWithFile(ctx context.Context, fileID uint) error {
	if err := composables.CanUser(ctx, permissions.PositionCreate); err != nil {
		return err
	}
	uploadEntity, err := s.uploadService.GetByID(ctx, fileID)
	if err != nil {
		return err
	}
	return s.LoadFromFilePath(ctx, uploadEntity.Path)
}

func (s *PositionService) Create(ctx context.Context, data *position.CreateDTO) (*position.Position, error) {
	if err := composables.CanUser(ctx, permissions.PositionCreate); err != nil {
		return nil, err
	}
	entity, err := data.ToEntity()
	if err != nil {
		return nil, err
	}
	if err := s.repo.Create(ctx, entity); err != nil {
		return nil, err
	}
	createdEvent, err := position.NewCreatedEvent(ctx, *data, *entity)
	if err != nil {
		return nil, err
	}
	s.publisher.Publish(createdEvent)
	return entity, nil
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
