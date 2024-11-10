package persistence

import (
	"context"
	product2 "github.com/iota-agency/iota-erp/internal/modules/warehouse/domain/entities/product"
	"github.com/iota-agency/iota-erp/pkg/composables"

	"github.com/iota-agency/iota-erp/internal/infrastructure/persistence/models"
	"github.com/iota-agency/iota-erp/sdk/graphql/helpers"
	"github.com/iota-agency/iota-erp/sdk/service"
)

type GormProductRepository struct{}

func NewProductRepository() product2.Repository {
	return &GormProductRepository{}
}

func (g *GormProductRepository) GetPaginated(
	ctx context.Context, limit, offset int,
	sortBy []string,
) ([]*product2.Product, error) {
	tx, ok := composables.UseTx(ctx)
	if !ok {
		return nil, service.ErrNoTx
	}
	q := tx.Limit(limit).Offset(offset)
	q, err := helpers.ApplySort(q, sortBy, &product2.Product{}) //nolint:exhaustruct
	if err != nil {
		return nil, err
	}
	var entities []*models.WarehouseProduct
	if err := q.Find(&entities).Error; err != nil {
		return nil, err
	}
	products := make([]*product2.Product, len(entities))
	for i, entity := range entities {
		p, err := toDomainProduct(entity)
		if err != nil {
			return nil, err
		}
		products[i] = p
	}
	return products, nil
}

func (g *GormProductRepository) Count(ctx context.Context) (int64, error) {
	tx, ok := composables.UseTx(ctx)
	if !ok {
		return 0, service.ErrNoTx
	}
	var count int64
	if err := tx.Model(&models.WarehouseProduct{}).Count(&count).Error; err != nil { //nolint:exhaustruct
		return 0, err
	}
	return count, nil
}

func (g *GormProductRepository) GetAll(ctx context.Context) ([]*product2.Product, error) {
	tx, ok := composables.UseTx(ctx)
	if !ok {
		return nil, service.ErrNoTx
	}
	var entities []*models.WarehouseProduct
	if err := tx.Find(&entities).Error; err != nil {
		return nil, err
	}
	products := make([]*product2.Product, len(entities))
	for i, entity := range entities {
		p, err := toDomainProduct(entity)
		if err != nil {
			return nil, err
		}
		products[i] = p
	}
	return products, nil
}

func (g *GormProductRepository) GetByID(ctx context.Context, id int64) (*product2.Product, error) {
	tx, ok := composables.UseTx(ctx)
	if !ok {
		return nil, service.ErrNoTx
	}
	var entity models.WarehouseProduct
	if err := tx.Where("id = ?", id).First(&entity).Error; err != nil {
		return nil, err
	}
	return toDomainProduct(&entity)
}

func (g *GormProductRepository) Create(ctx context.Context, data *product2.Product) error {
	tx, ok := composables.UseTx(ctx)
	if !ok {
		return service.ErrNoTx
	}
	if err := tx.Create(toDBProduct(data)).Error; err != nil {
		return err
	}
	return nil
}

func (g *GormProductRepository) Update(ctx context.Context, data *product2.Product) error {
	tx, ok := composables.UseTx(ctx)
	if !ok {
		return service.ErrNoTx
	}
	if err := tx.Save(toDBProduct(data)).Error; err != nil {
		return err
	}
	return nil
}

func (g *GormProductRepository) Delete(ctx context.Context, id int64) error {
	tx, ok := composables.UseTx(ctx)
	if !ok {
		return service.ErrNoTx
	}
	if err := tx.Where("id = ?", id).Delete(&models.WarehouseProduct{}).Error; err != nil { //nolint:exhaustruct
		return err
	}
	return nil
}
