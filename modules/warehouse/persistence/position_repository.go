package persistence

import (
	"context"
	position2 "github.com/iota-agency/iota-sdk/modules/warehouse/domain/entities/position"
	"github.com/iota-agency/iota-sdk/modules/warehouse/persistence/models"
	"github.com/iota-agency/iota-sdk/pkg/composables"
	"github.com/iota-agency/iota-sdk/pkg/graphql/helpers"
	"github.com/iota-agency/iota-sdk/pkg/mapping"
	"github.com/iota-agency/iota-sdk/pkg/service"
)

type GormPositionRepository struct{}

func NewPositionRepository() position2.Repository {
	return &GormPositionRepository{}
}

func (g *GormPositionRepository) GetPaginated(
	ctx context.Context, params *position2.FindParams,
) ([]*position2.Position, error) {
	tx, ok := composables.UseTx(ctx)
	if !ok {
		return nil, service.ErrNoTx
	}
	q := tx.Limit(params.Limit).Offset(params.Offset)
	q, err := helpers.ApplySort(q, params.SortBy)
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

func (g *GormPositionRepository) GetAll(ctx context.Context) ([]*position2.Position, error) {
	tx, ok := composables.UseTx(ctx)
	if !ok {
		return nil, service.ErrNoTx
	}
	var entities []*models.WarehousePosition
	if err := tx.Find(&entities).Error; err != nil {
		return nil, err
	}
	return mapping.MapDbModels(entities, toDomainPosition)
}

func (g *GormPositionRepository) GetByID(ctx context.Context, id uint) (*position2.Position, error) {
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

func (g *GormPositionRepository) CreateOrUpdate(ctx context.Context, data *position2.Position) error {
	tx, ok := composables.UseTx(ctx)
	if !ok {
		return service.ErrNoTx
	}
	return tx.Save(toDBPosition(data)).Error
}

func (g *GormPositionRepository) Create(ctx context.Context, data *position2.Position) error {
	tx, ok := composables.UseTx(ctx)
	if !ok {
		return service.ErrNoTx
	}
	if err := tx.Create(toDBPosition(data)).Error; err != nil {
		return err
	}
	return nil
}

func (g *GormPositionRepository) Update(ctx context.Context, data *position2.Position) error {
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
