package persistence

import (
	"context"
	"github.com/iota-agency/iota-erp/internal/infrastracture/persistence/models"

	"github.com/iota-agency/iota-erp/internal/domain/entities/product"
	"github.com/iota-agency/iota-erp/sdk/composables"
	"github.com/iota-agency/iota-erp/sdk/graphql/helpers"
	"github.com/iota-agency/iota-erp/sdk/service"
)

type GormProductRepository struct{}

func NewProductRepository() product.Repository {
	return &GormProductRepository{}
}

func (g *GormProductRepository) GetPaginated(ctx context.Context, limit, offset int, sortBy []string) ([]*product.Product, error) {
	tx, ok := composables.UseTx(ctx)
	if !ok {
		return nil, service.ErrNoTx
	}
	q := tx.Limit(limit).Offset(offset)
	q, err := helpers.ApplySort(q, sortBy, &product.Product{})
	if err != nil {
		return nil, err
	}
	var entities []*models.WarehouseProduct
	if err := q.Find(&entities).Error; err != nil {
		return nil, err
	}
	var products []*product.Product
	for _, entity := range entities {
		p, err := toDomainProduct(entity)
		if err != nil {
			return nil, err
		}
		products = append(products, p)
	}
	return products, nil
}

func (g *GormProductRepository) Count(ctx context.Context) (int64, error) {
	tx, ok := composables.UseTx(ctx)
	if !ok {
		return 0, service.ErrNoTx
	}
	var count int64
	if err := tx.Model(&models.WarehouseProduct{}).Count(&count).Error; err != nil {
		return 0, err
	}
	return count, nil
}

func (g *GormProductRepository) GetAll(ctx context.Context) ([]*product.Product, error) {
	tx, ok := composables.UseTx(ctx)
	if !ok {
		return nil, service.ErrNoTx
	}
	var entities []*models.WarehouseProduct
	if err := tx.Find(&entities).Error; err != nil {
		return nil, err
	}
	var products []*product.Product
	for _, entity := range entities {
		p, err := toDomainProduct(entity)
		if err != nil {
			return nil, err
		}
		products = append(products, p)
	}
	return products, nil
}

func (g *GormProductRepository) GetByID(ctx context.Context, id int64) (*product.Product, error) {
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

func (g *GormProductRepository) Create(ctx context.Context, data *product.Product) error {
	tx, ok := composables.UseTx(ctx)
	if !ok {
		return service.ErrNoTx
	}
	if err := tx.Create(toDBProduct(data)).Error; err != nil {
		return err
	}
	return nil
}

func (g *GormProductRepository) Update(ctx context.Context, data *product.Product) error {
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
	if err := tx.Where("id = ?", id).Delete(&product.Product{}).Error; err != nil {
		return err
	}
	return nil
}
