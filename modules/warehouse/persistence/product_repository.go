package persistence

import (
	"context"

	"github.com/iota-agency/iota-sdk/modules/warehouse/domain/aggregates/product"
	"github.com/iota-agency/iota-sdk/modules/warehouse/persistence/models"
	"github.com/iota-agency/iota-sdk/pkg/composables"
	"github.com/iota-agency/iota-sdk/pkg/graphql/helpers"
	"github.com/iota-agency/iota-sdk/pkg/mapping"
	"gorm.io/gorm"
)

type GormProductRepository struct{}

func NewProductRepository() product.Repository {
	return &GormProductRepository{}
}

func (g *GormProductRepository) tx(ctx context.Context, params *product.FindParams) (*gorm.DB, error) {
	tx, ok := composables.UseTx(ctx)
	if !ok {
		return nil, composables.ErrNoTx
	}
	var positionArgs []interface{}
	if params.Query != "" && params.Field != "" {
		if params.Field == "position" {
			positionArgs = append(positionArgs, tx.Where("title ILIKE ?", "%"+params.Query+"%"))
		}
	}
	return tx.InnerJoins("Position", positionArgs...), nil
}

func (g *GormProductRepository) GetPaginated(
	ctx context.Context, params *product.FindParams,
) ([]*product.Product, error) {
	tx, err := g.tx(ctx, params)
	if err != nil {
		return nil, err
	}
	tx = tx.Limit(params.Limit).Offset(params.Offset)
	if params.CreatedAt.To != "" && params.CreatedAt.From != "" {
		tx = tx.Where("warehouse_products.created_at BETWEEN ? and ?", params.CreatedAt.From, params.CreatedAt.To)
	}
	if params.Status != "" {
		tx = tx.Where("status = ?", params.Status)
	}
	tx, err = helpers.ApplySort(tx, params.SortBy)
	if err != nil {
		return nil, err
	}
	var entities []*models.WarehouseProduct
	if err := tx.Find(&entities).Error; err != nil {
		return nil, err
	}
	return mapping.MapDbModels(entities, toDomainProduct)
}

func (g *GormProductRepository) Count(ctx context.Context) (int64, error) {
	tx, ok := composables.UseTx(ctx)
	if !ok {
		return 0, composables.ErrNoTx
	}
	var count int64
	if err := tx.Model(&models.WarehouseProduct{}).Count(&count).Error; err != nil { //nolint:exhaustruct
		return 0, err
	}
	return count, nil
}

func (g *GormProductRepository) CountInStock(ctx context.Context, positionID uint) (int64, error) {
	tx, ok := composables.UseTx(ctx)
	if !ok {
		return 0, composables.ErrNoTx
	}
	var count int64
	q := tx.Model(&models.WarehouseProduct{}).Where("position_id = ?", positionID).Where("status = ?", string(product.InStock))
	if err := q.Count(&count).Error; err != nil {
		return 0, err
	}
	return count, nil
}

func (g *GormProductRepository) GetByPositionID(ctx context.Context, positionID uint, opts *product.QueryOptions) ([]*product.Product, error) {
	tx, err := g.tx(ctx, &product.FindParams{})
	if err != nil {
		return nil, err
	}
	q := tx.Where("position_id = ?", positionID)
	q, err = helpers.ApplySort(q, opts.SortBy)
	if err != nil {
		return nil, err
	}
	var entities []*models.WarehouseProduct
	if err := q.Find(&entities).Error; err != nil {
		return nil, err
	}
	return mapping.MapDbModels(entities, toDomainProduct)
}

func (g *GormProductRepository) GetAll(ctx context.Context) ([]*product.Product, error) {
	tx, err := g.tx(ctx, &product.FindParams{})
	if err != nil {
		return nil, err
	}
	var entities []*models.WarehouseProduct
	if err := tx.Find(&entities).Error; err != nil {
		return nil, err
	}
	return mapping.MapDbModels(entities, toDomainProduct)
}

func (g *GormProductRepository) GetByID(ctx context.Context, id uint) (*product.Product, error) {
	tx, err := g.tx(ctx, &product.FindParams{})
	if err != nil {
		return nil, err
	}
	var entity models.WarehouseProduct
	if err := tx.Where("warehouse_products.id = ?", id).First(&entity).Error; err != nil {
		return nil, err
	}
	return toDomainProduct(&entity)
}

func (g *GormProductRepository) GetByRfid(ctx context.Context, rfid string) (*product.Product, error) {
	tx, err := g.tx(ctx, &product.FindParams{})
	if err != nil {
		return nil, err
	}
	var entity models.WarehouseProduct
	if err := tx.Where("rfid = ?", rfid).First(&entity).Error; err != nil {
		return nil, err
	}
	return toDomainProduct(&entity)
}

func (g *GormProductRepository) Create(ctx context.Context, data *product.Product) error {
	tx, ok := composables.UseTx(ctx)
	if !ok {
		return composables.ErrNoTx
	}
	dbRow, err := toDBProduct(data)
	if err != nil {
		return err
	}
	return tx.Create(dbRow).Error
}

func (g *GormProductRepository) BulkCreate(ctx context.Context, data []*product.Product) error {
	tx, ok := composables.UseTx(ctx)
	if !ok {
		return composables.ErrNoTx
	}
	dbRows, err := mapping.MapDbModels(data, toDBProduct)
	if err != nil {
		return err
	}
	maxParams := 1000
	for i := 0; i < len(dbRows); i += maxParams {
		end := i + maxParams
		if end > len(dbRows) {
			end = len(dbRows)
		}
		if err := tx.Create(dbRows[i:end]).Error; err != nil {
			return err
		}
	}
	return nil
}

func (g *GormProductRepository) CreateOrUpdate(ctx context.Context, data *product.Product) error {
	tx, ok := composables.UseTx(ctx)
	if !ok {
		return composables.ErrNoTx
	}
	dbRow, err := toDBProduct(data)
	if err != nil {
		return err
	}
	return tx.Save(dbRow).Error
}

func (g *GormProductRepository) Update(ctx context.Context, data *product.Product) error {
	tx, ok := composables.UseTx(ctx)
	if !ok {
		return composables.ErrNoTx
	}
	dbRow, err := toDBProduct(data)
	if err != nil {
		return err
	}
	return tx.Save(dbRow).Error
}

func (g *GormProductRepository) Delete(ctx context.Context, id uint) error {
	tx, ok := composables.UseTx(ctx)
	if !ok {
		return composables.ErrNoTx
	}
	return tx.Where("id = ?", id).Delete(&models.WarehouseProduct{}).Error //nolint:exhaustruct
}
