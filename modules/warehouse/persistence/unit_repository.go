package persistence

import (
	"context"
	"fmt"

	"github.com/iota-uz/iota-sdk/modules/warehouse/domain/entities/unit"
	"github.com/iota-uz/iota-sdk/modules/warehouse/persistence/models"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/graphql/helpers"
)

type GormUnitRepository struct{}

func NewUnitRepository() unit.Repository {
	return &GormUnitRepository{}
}

func (g *GormUnitRepository) GetPaginated(
	ctx context.Context, params *unit.FindParams,
) ([]*unit.Unit, error) {
	tx, ok := composables.UseTx(ctx)
	if !ok {
		return nil, composables.ErrNoTx
	}
	tx = tx.Limit(params.Limit).Offset(params.Offset)
	if params.Query != "" && params.Field != "" {
		tx = tx.Where(fmt.Sprintf("%s::varchar ILIKE ?", params.Field), "%"+params.Query+"%")
	}

	if params.CreatedAt.To != "" && params.CreatedAt.From != "" {
		tx = tx.Where("created_at BETWEEN ? and ?", params.CreatedAt.From, params.CreatedAt.To)
	}
	tx, err := helpers.ApplySort(tx, params.SortBy)
	if err != nil {
		return nil, err
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

func (g *GormUnitRepository) Count(ctx context.Context) (uint, error) {
	tx, ok := composables.UseTx(ctx)
	if !ok {
		return 0, composables.ErrNoTx
	}
	var count int64
	if err := tx.Model(&models.WarehouseUnit{}).Count(&count).Error; err != nil { //nolint:exhaustruct
		return 0, err
	}
	return uint(count), nil
}

func (g *GormUnitRepository) GetAll(ctx context.Context) ([]*unit.Unit, error) {
	tx, ok := composables.UseTx(ctx)
	if !ok {
		return nil, composables.ErrNoTx
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
		return nil, composables.ErrNoTx
	}
	var entity models.WarehouseUnit
	if err := tx.Where("id = ?", id).First(&entity).Error; err != nil {
		return nil, err
	}
	return toDomainUnit(&entity), nil
}

func (g *GormUnitRepository) GetByTitleOrShortTitle(ctx context.Context, name string) (*unit.Unit, error) {
	tx, ok := composables.UseTx(ctx)
	if !ok {
		return nil, composables.ErrNoTx
	}
	var entity models.WarehouseUnit
	if err := tx.Where("title = ? OR short_title = ?", name, name).First(&entity).Error; err != nil {
		return nil, err
	}
	return toDomainUnit(&entity), nil
}

func (g *GormUnitRepository) Create(ctx context.Context, data *unit.Unit) error {
	tx, ok := composables.UseTx(ctx)
	if !ok {
		return composables.ErrNoTx
	}
	dbRow := toDBUnit(data)
	if err := tx.Create(dbRow).Error; err != nil {
		return err
	}
	data.ID = dbRow.ID
	return nil
}

func (g *GormUnitRepository) CreateOrUpdate(ctx context.Context, data *unit.Unit) error {
	tx, ok := composables.UseTx(ctx)
	if !ok {
		return composables.ErrNoTx
	}
	return tx.Save(toDBUnit(data)).Error
}

func (g *GormUnitRepository) Update(ctx context.Context, data *unit.Unit) error {
	tx, ok := composables.UseTx(ctx)
	if !ok {
		return composables.ErrNoTx
	}
	return tx.Updates(toDBUnit(data)).Error
}

func (g *GormUnitRepository) Delete(ctx context.Context, id uint) error {
	tx, ok := composables.UseTx(ctx)
	if !ok {
		return composables.ErrNoTx
	}
	return tx.Where("id = ?", id).Delete(&models.WarehouseUnit{}).Error //nolint:exhaustruct
}
