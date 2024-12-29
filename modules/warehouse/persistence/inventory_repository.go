package persistence

import (
	"context"
	"fmt"

	"github.com/iota-uz/iota-sdk/modules/warehouse/domain/entities/inventory"
	"github.com/iota-uz/iota-sdk/modules/warehouse/persistence/models"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/graphql/helpers"
	"github.com/iota-uz/iota-sdk/pkg/mapping"
)

type GormInventoryRepository struct{}

func NewInventoryRepository() inventory.Repository {
	return &GormInventoryRepository{}
}

func (g *GormInventoryRepository) GetPaginated(
	ctx context.Context, params *inventory.FindParams,
) ([]*inventory.Check, error) {
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

	if params.Status != "" {
		tx = tx.Where("status = ?", params.Status)
	}

	if params.Type != "" {
		tx = tx.Where("type = ?", params.Type)
	}

	tx, err := helpers.ApplySort(tx, params.SortBy)
	if err != nil {
		return nil, err
	}
	var entities []*models.InventoryCheck
	if err := tx.Preload("CreatedBy").Find(&entities).Error; err != nil {
		return nil, err
	}
	return mapping.MapDbModels(entities, toDomainInventoryCheck)
}

func (g *GormInventoryRepository) Positions(ctx context.Context) ([]*inventory.Position, error) {
	tx, ok := composables.UseTx(ctx)
	if !ok {
		return nil, composables.ErrNoTx
	}
	var entities []*models.InventoryPosition
	sql := `
	SELECT warehouse_positions.id, warehouse_positions.title, COUNT(warehouse_products.id) quantity, array_agg(warehouse_products.rfid) rfid_tags
	FROM warehouse_positions JOIN warehouse_products ON warehouse_positions.id = warehouse_products.position_id
	GROUP BY warehouse_positions.id;
	`
	if err := tx.Raw(sql).Scan(&entities).Error; err != nil {
		return nil, err
	}
	return mapping.MapDbModels(entities, toDomainInventoryPosition)
}

func (g *GormInventoryRepository) Count(ctx context.Context) (uint, error) {
	tx, ok := composables.UseTx(ctx)
	if !ok {
		return 0, composables.ErrNoTx
	}
	var count int64
	if err := tx.Model(&models.InventoryCheck{}).Count(&count).Error; err != nil { //nolint:exhaustruct
		return 0, err
	}
	return uint(count), nil
}

func (g *GormInventoryRepository) GetAll(ctx context.Context) ([]*inventory.Check, error) {
	tx, ok := composables.UseTx(ctx)
	if !ok {
		return nil, composables.ErrNoTx
	}
	var entities []*models.InventoryCheck
	if err := tx.Find(&entities).Error; err != nil {
		return nil, err
	}
	return mapping.MapDbModels(entities, toDomainInventoryCheck)
}

func (g *GormInventoryRepository) GetByID(ctx context.Context, id uint) (*inventory.Check, error) {
	tx, ok := composables.UseTx(ctx)
	if !ok {
		return nil, composables.ErrNoTx
	}
	var entity models.InventoryCheck
	if err := tx.Preload("Results.Position.Unit").Where("id = ?", id).First(&entity).Error; err != nil {
		return nil, err
	}
	return toDomainInventoryCheck(&entity)
}

func (g *GormInventoryRepository) GetByIDWithDifference(ctx context.Context, id uint) (*inventory.Check, error) {
	tx, ok := composables.UseTx(ctx)
	if !ok {
		return nil, composables.ErrNoTx
	}
	var entity models.InventoryCheck
	if err := tx.Preload("Results", "actual_quantity != expected_quantity").Preload("Results.Position.Unit").Where("id = ?", id).First(&entity).Error; err != nil {
		return nil, err
	}
	return toDomainInventoryCheck(&entity)
}

func (g *GormInventoryRepository) Create(ctx context.Context, data *inventory.Check) error {
	tx, ok := composables.UseTx(ctx)
	if !ok {
		return composables.ErrNoTx
	}
	dbRow, err := toDBInventoryCheck(data)
	if err != nil {
		return err
	}
	if err := tx.Create(dbRow).Error; err != nil {
		return err
	}
	data.ID = dbRow.ID
	return nil
}

func (g *GormInventoryRepository) Update(ctx context.Context, data *inventory.Check) error {
	tx, ok := composables.UseTx(ctx)
	if !ok {
		return composables.ErrNoTx
	}
	dbRow, err := toDBInventoryCheck(data)
	if err != nil {
		return err
	}
	return tx.Updates(dbRow).Error
}

func (g *GormInventoryRepository) Delete(ctx context.Context, id uint) error {
	tx, ok := composables.UseTx(ctx)
	if !ok {
		return composables.ErrNoTx
	}
	return tx.Where("id = ?", id).Delete(&models.InventoryCheck{}).Error //nolint:exhaustruct
}
