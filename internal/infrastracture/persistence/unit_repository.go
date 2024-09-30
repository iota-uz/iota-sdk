package persistence

import (
	"context"
	"github.com/iota-agency/iota-erp/internal/domain/entities/unit"
	"github.com/iota-agency/iota-erp/internal/infrastracture/persistence/models"
	"github.com/iota-agency/iota-erp/sdk/composables"
	"github.com/iota-agency/iota-erp/sdk/graphql/helpers"
	"github.com/iota-agency/iota-erp/sdk/service"
)

type GormUnitsRepository struct {
}

func NewUnitsRepository() unit.Repository {
	return &GormUnitsRepository{}
}

func (g *GormUnitsRepository) GetPaginated(ctx context.Context, limit, offset int, sortBy []string) ([]*unit.Unit, error) {
	tx, ok := composables.UseTx(ctx)
	if !ok {
		return nil, service.ErrNoTx
	}
	q := tx.Limit(limit).Offset(offset)
	q, err := helpers.ApplySort(q, sortBy, &unit.Unit{})
	if err != nil {
		return nil, err
	}
	var entities []*models.WarehouseUnit
	if err := q.Find(&entities).Error; err != nil {
		return nil, err
	}
	var units []*unit.Unit
	for _, entity := range entities {
		units = append(units, toDomainUnit(entity))
	}
	return units, nil
}

func (g *GormUnitsRepository) Count(ctx context.Context) (int64, error) {
	tx, ok := composables.UseTx(ctx)
	if !ok {
		return 0, service.ErrNoTx
	}
	var count int64
	if err := tx.Model(&models.WarehouseUnit{}).Count(&count).Error; err != nil {
		return 0, err
	}
	return count, nil
}

func (g *GormUnitsRepository) GetAll(ctx context.Context) ([]*unit.Unit, error) {
	tx, ok := composables.UseTx(ctx)
	if !ok {
		return nil, service.ErrNoTx
	}
	var entities []*models.WarehouseUnit
	if err := tx.Find(&entities).Error; err != nil {
		return nil, err
	}
	var units []*unit.Unit
	for _, entity := range entities {
		units = append(units, toDomainUnit(entity))
	}
	return units, nil
}

func (g *GormUnitsRepository) GetByID(ctx context.Context, id int64) (*unit.Unit, error) {
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

func (g *GormUnitsRepository) Create(ctx context.Context, data *unit.Unit) error {
	tx, ok := composables.UseTx(ctx)
	if !ok {
		return service.ErrNoTx
	}
	if err := tx.Create(toDBUnit(data)).Error; err != nil {
		return err
	}
	return nil
}

func (g *GormUnitsRepository) Update(ctx context.Context, data *unit.Unit) error {
	tx, ok := composables.UseTx(ctx)
	if !ok {
		return service.ErrNoTx
	}
	if err := tx.Model(&models.WarehouseUnit{}).Updates(toDBUnit(data)).Error; err != nil {
		return err
	}
	return nil
}

func (g *GormUnitsRepository) Delete(ctx context.Context, id int64) error {
	tx, ok := composables.UseTx(ctx)
	if !ok {
		return service.ErrNoTx
	}
	if err := tx.Where("id = ?", id).Delete(&models.WarehouseUnit{}).Error; err != nil {
		return err
	}
	return nil
}
