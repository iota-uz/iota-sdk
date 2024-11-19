package persistence

import (
	"context"
	"github.com/iota-agency/iota-sdk/pkg/composables"
	"github.com/iota-agency/iota-sdk/pkg/graphql/helpers"
	"github.com/iota-agency/iota-sdk/pkg/modules/warehouse/domain/entities/unit"
	"github.com/iota-agency/iota-sdk/pkg/service"

	"github.com/iota-agency/iota-sdk/pkg/modules/warehouse/persistence/models"
)

type GormUnitRepository struct{}

func NewUnitRepository() unit.Repository {
	return &GormUnitRepository{}
}

func (g *GormUnitRepository) GetPaginated(
	ctx context.Context,
	limit, offset int,
	sortBy []string,
) ([]*unit.Unit, error) {
	tx, ok := composables.UseTx(ctx)
	if !ok {
		return nil, service.ErrNoTx
	}
	q := tx.Limit(limit).Offset(offset)
	q, err := helpers.ApplySort(q, sortBy)
	if err != nil {
		return nil, err
	}
	var entities []*models.WarehouseUnit
	if err := q.Find(&entities).Error; err != nil {
		return nil, err
	}
	units := make([]*unit.Unit, len(entities))
	for i, entity := range entities {
		units[i] = toDomainUnit(entity)
	}
	return units, nil
}

func (g *GormUnitRepository) Count(ctx context.Context) (int64, error) {
	tx, ok := composables.UseTx(ctx)
	if !ok {
		return 0, service.ErrNoTx
	}
	var count int64
	if err := tx.Model(&models.WarehouseUnit{}).Count(&count).Error; err != nil { //nolint:exhaustruct
		return 0, err
	}
	return count, nil
}

func (g *GormUnitRepository) GetAll(ctx context.Context) ([]*unit.Unit, error) {
	tx, ok := composables.UseTx(ctx)
	if !ok {
		return nil, service.ErrNoTx
	}
	var entities []*models.WarehouseUnit
	if err := tx.Find(&entities).Error; err != nil {
		return nil, err
	}
	units := make([]*unit.Unit, len(entities))
	for i, entity := range entities {
		units[i] = toDomainUnit(entity)
	}
	return units, nil
}

func (g *GormUnitRepository) GetByID(ctx context.Context, id uint) (*unit.Unit, error) {
	tx, ok := composables.UseTx(ctx)
	if !ok {
		return nil, service.ErrNoTx
	}
	var entity models.WarehouseUnit
	if err := tx.Where("id = ?", id).First(&entity).Error; err != nil {
		return nil, err
	}
	return toDomainUnit(&entity), nil
}

func (g *GormUnitRepository) Create(ctx context.Context, data *unit.Unit) error {
	tx, ok := composables.UseTx(ctx)
	if !ok {
		return service.ErrNoTx
	}
	if err := tx.Create(toDBUnit(data)).Error; err != nil {
		return err
	}
	return nil
}

func (g *GormUnitRepository) CreateOrUpdate(ctx context.Context, data *unit.Unit) error {
	tx, ok := composables.UseTx(ctx)
	if !ok {
		return service.ErrNoTx
	}
	return tx.Save(toDBUnit(data)).Error
}

func (g *GormUnitRepository) Update(ctx context.Context, data *unit.Unit) error {
	tx, ok := composables.UseTx(ctx)
	if !ok {
		return service.ErrNoTx
	}
	return tx.Updates(toDBUnit(data)).Error
}

func (g *GormUnitRepository) Delete(ctx context.Context, id uint) error {
	tx, ok := composables.UseTx(ctx)
	if !ok {
		return service.ErrNoTx
	}
	return tx.Where("id = ?", id).Delete(&models.WarehouseUnit{}).Error //nolint:exhaustruct
}
