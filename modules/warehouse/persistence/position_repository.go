package persistence

import (
	"context"
	"fmt"

	"github.com/iota-agency/iota-sdk/modules/warehouse/domain/aggregates/position"
	"github.com/iota-agency/iota-sdk/modules/warehouse/persistence/models"
	"github.com/iota-agency/iota-sdk/pkg/composables"
	"github.com/iota-agency/iota-sdk/pkg/graphql/helpers"
	"github.com/iota-agency/iota-sdk/pkg/mapping"
	"gorm.io/gorm"
)

type GormPositionRepository struct{}

func NewPositionRepository() position.Repository {
	return &GormPositionRepository{}
}

func (g *GormPositionRepository) tx(ctx context.Context) (*gorm.DB, error) {
	tx, ok := composables.UseTx(ctx)
	if !ok {
		return nil, composables.ErrNoTx
	}
	return tx.Preload("Unit").Preload("Images"), nil
}

func (g *GormPositionRepository) GetPaginated(
	ctx context.Context, params *position.FindParams,
) ([]*position.Position, error) {
	tx, err := g.tx(ctx)
	if err != nil {
		return nil, err
	}
	tx = tx.Limit(params.Limit).Offset(params.Offset)
	if params.CreatedAt.To != "" && params.CreatedAt.From != "" {
		tx = tx.Where("warehouse_positions.created_at BETWEEN ? and ?", params.CreatedAt.From, params.CreatedAt.To)
	}
	if params.Query != "" && params.Field != "" {
		tx = tx.Where(fmt.Sprintf("%s::varchar ILIKE ?", params.Field), "%"+params.Query+"%")
	}
	if params.UnitID != "" {
		tx = tx.Where("unit_id = ?", params.UnitID)
	}
	tx, err = helpers.ApplySort(tx, params.SortBy)
	if err != nil {
		return nil, err
	}
	var entities []*models.WarehousePosition
	if err := tx.Find(&entities).Error; err != nil {
		return nil, err
	}
	return mapping.MapDbModels(entities, toDomainPosition)
}

func (g *GormPositionRepository) Count(ctx context.Context) (int64, error) {
	tx, ok := composables.UseTx(ctx)
	if !ok {
		return 0, composables.ErrNoTx
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
	tx, err := g.tx(ctx)
	if err != nil {
		return nil, err
	}
	var entity models.WarehousePosition
	if err := tx.Where("id = ?", id).First(&entity).Error; err != nil {
		return nil, err
	}
	return toDomainPosition(&entity)
}

func (g *GormPositionRepository) GetByBarcode(ctx context.Context, barcode string) (*position.Position, error) {
	tx, err := g.tx(ctx)
	if err != nil {
		return nil, err
	}
	var entity models.WarehousePosition
	if err := tx.Where("barcode = ?", barcode).First(&entity).Error; err != nil {
		return nil, err
	}
	return toDomainPosition(&entity)
}

func (g *GormPositionRepository) CreateOrUpdate(ctx context.Context, data *position.Position) error {
	tx, ok := composables.UseTx(ctx)
	if !ok {
		return composables.ErrNoTx
	}
	positionRow, uploadRows := toDBPosition(data)
	if err := tx.Save(positionRow).Error; err != nil {
		return err
	}
	if len(uploadRows) == 0 {
		return nil
	}
	return tx.Save(uploadRows).Error
}

func (g *GormPositionRepository) Create(ctx context.Context, data *position.Position) error {
	tx, ok := composables.UseTx(ctx)
	if !ok {
		return composables.ErrNoTx
	}
	positionRow, junctionRows := toDBPosition(data)
	if err := tx.Create(positionRow).Error; err != nil {
		return err
	}
	data.ID = positionRow.ID
	if len(junctionRows) == 0 {
		return nil
	}
	for _, junctionRow := range junctionRows {
		// TODO: this feels like a hack
		junctionRow.WarehousePositionID = positionRow.ID
	}
	if err := tx.Create(junctionRows).Error; err != nil {
		return err
	}
	return nil
}

func (g *GormPositionRepository) Update(ctx context.Context, data *position.Position) error {
	tx, ok := composables.UseTx(ctx)
	if !ok {
		return composables.ErrNoTx
	}
	positionRow, uploadRows := toDBPosition(data)
	if err := tx.Updates(positionRow).Error; err != nil {
		return err
	}
	if err := tx.Delete(&models.WarehousePositionImage{}, "warehouse_position_id = ?", positionRow.ID).Error; err != nil {
		return err
	}
	for _, uploadRow := range uploadRows {
		if err := tx.Create(uploadRow).Error; err != nil {
			return err
		}
	}
	return nil
}

func (g *GormPositionRepository) Delete(ctx context.Context, id uint) error {
	tx, ok := composables.UseTx(ctx)
	if !ok {
		return composables.ErrNoTx
	}
	return tx.Where("id = ?", id).Delete(&models.WarehousePosition{}).Error //nolint:exhaustruct
}
