package persistence

import (
	"context"
	"github.com/iota-agency/iota-sdk/modules/warehouse/domain/aggregates/position"
	"github.com/iota-agency/iota-sdk/modules/warehouse/persistence/models"
	"github.com/iota-agency/iota-sdk/pkg/composables"
	"github.com/iota-agency/iota-sdk/pkg/graphql/helpers"
	"github.com/iota-agency/iota-sdk/pkg/mapping"
	"github.com/iota-agency/iota-sdk/pkg/service"
	"gorm.io/gorm"
)

type GormPositionRepository struct{}

func NewPositionRepository() position.Repository {
	return &GormPositionRepository{}
}

func (g *GormPositionRepository) tx(ctx context.Context) (*gorm.DB, error) {
	tx, ok := composables.UseTx(ctx)
	if !ok {
		return nil, service.ErrNoTx
	}
	return tx.Preload("Unit"), nil
}

func (g *GormPositionRepository) GetPaginated(
	ctx context.Context, params *position.FindParams,
) ([]*position.Position, error) {
	tx, err := g.tx(ctx)
	if err != nil {
		return nil, err
	}
	q := tx.Limit(params.Limit).Offset(params.Offset)
	q, err = helpers.ApplySort(q, params.SortBy)
	if err != nil {
		return nil, err
	}
	if params.Search != "" {
		q = q.Where("title ILIKE ?", "%"+params.Search+"%")
	}
	var entities []*models.WarehousePosition
	if err := q.Find(&entities).Error; err != nil {
		return nil, err
	}
	return mapping.MapDbModels(entities, toDomainPosition)
}

func (g *GormPositionRepository) Count(ctx context.Context) (int64, error) {
	tx, ok := composables.UseTx(ctx)
	if !ok {
		return 0, service.ErrNoTx
	}
	var count int64
	if err := tx.Model(&models.WarehousePosition{}).Count(&count).Error; err != nil { //nolint:exhaustruct
		return 0, err
	}
	return count, nil
}

func (g *GormPositionRepository) GetAll(ctx context.Context) ([]*position.Position, error) {
	tx, err := g.tx(ctx)
	if err != nil {
		return nil, err
	}
	var entities []*models.WarehousePosition
	if err := tx.Find(&entities).Error; err != nil {
		return nil, err
	}
	return mapping.MapDbModels(entities, toDomainPosition)
}

func (g *GormPositionRepository) GetByID(ctx context.Context, id uint) (*position.Position, error) {
	tx, ok := composables.UseTx(ctx)
	if !ok {
		return nil, service.ErrNoTx
	}
	var entity models.WarehousePosition
	if err := tx.Where("id = ?", id).First(&entity).Error; err != nil {
		return nil, err
	}
	return toDomainPosition(&entity)
}

func (g *GormPositionRepository) CreateOrUpdate(ctx context.Context, data *position.Position) error {
	tx, ok := composables.UseTx(ctx)
	if !ok {
		return service.ErrNoTx
	}
	return tx.Save(toDBPosition(data)).Error
}

func (g *GormPositionRepository) Create(ctx context.Context, data *position.Position) error {
	tx, ok := composables.UseTx(ctx)
	if !ok {
		return service.ErrNoTx
	}
	if err := tx.Create(toDBPosition(data)).Error; err != nil {
		return err
	}
	return nil
}

func (g *GormPositionRepository) Update(ctx context.Context, data *position.Position) error {
	tx, ok := composables.UseTx(ctx)
	if !ok {
		return service.ErrNoTx
	}
	return tx.Save(toDBPosition(data)).Error
}

func (g *GormPositionRepository) Delete(ctx context.Context, id uint) error {
	tx, ok := composables.UseTx(ctx)
	if !ok {
		return service.ErrNoTx
	}
	return tx.Where("id = ?", id).Delete(&models.WarehousePosition{}).Error //nolint:exhaustruct
}
