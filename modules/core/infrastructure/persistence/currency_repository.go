package persistence

import (
	"context"
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/currency"
	"github.com/iota-uz/iota-sdk/modules/core/infrastructure/persistence/models"

	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/mapping"
)

type GormCurrencyRepository struct{}

func NewCurrencyRepository() currency.Repository {
	return &GormCurrencyRepository{}
}

func (g *GormCurrencyRepository) GetPaginated(
	ctx context.Context,
	limit, offset int,
	sortBy []string,
) ([]*currency.Currency, error) {
	tx, ok := composables.UseTx(ctx)
	if !ok {
		return nil, composables.ErrNoTx
	}
	var rows []*models.Currency
	q := tx.Limit(limit).Offset(offset)
	for _, s := range sortBy {
		q = q.Order(s)
	}
	if err := q.Find(&rows).Error; err != nil {
		return nil, err
	}
	return mapping.MapDbModels(rows, ToDomainCurrency)
}

func (g *GormCurrencyRepository) Count(ctx context.Context) (uint, error) {
	tx, ok := composables.UseTx(ctx)
	if !ok {
		return 0, composables.ErrNoTx
	}
	var count int64
	if err := tx.Model(&models.Currency{}).Count(&count).Error; err != nil { //nolint:exhaustruct
		return 0, err
	}
	return uint(count), nil
}

func (g *GormCurrencyRepository) GetAll(ctx context.Context) ([]*currency.Currency, error) {
	tx, ok := composables.UseTx(ctx)
	if !ok {
		return nil, composables.ErrNoTx
	}
	var rows []*models.Currency
	if err := tx.Find(&rows).Error; err != nil {
		return nil, err
	}
	return mapping.MapDbModels(rows, ToDomainCurrency)
}

func (g *GormCurrencyRepository) GetByID(ctx context.Context, id uint) (*currency.Currency, error) {
	tx, ok := composables.UseTx(ctx)
	if !ok {
		return nil, composables.ErrNoTx
	}
	var entity models.Currency
	if err := tx.First(&entity, id).Error; err != nil {
		return nil, err
	}
	return ToDomainCurrency(&entity)
}

func (g *GormCurrencyRepository) Create(ctx context.Context, entity *currency.Currency) error {
	tx, ok := composables.UseTx(ctx)
	if !ok {
		return composables.ErrNoTx
	}
	row := ToDBCurrency(entity)
	return tx.Create(row).Error
}

func (g *GormCurrencyRepository) Update(ctx context.Context, entity *currency.Currency) error {
	tx, ok := composables.UseTx(ctx)
	if !ok {
		return composables.ErrNoTx
	}
	row := ToDBCurrency(entity)
	return tx.Save(row).Error
}

func (g *GormCurrencyRepository) CreateOrUpdate(ctx context.Context, currency *currency.Currency) error {
	tx, ok := composables.UseTx(ctx)
	if !ok {
		return composables.ErrNoTx
	}
	row := ToDBCurrency(currency)
	return tx.Save(row).Error
}

func (g *GormCurrencyRepository) Delete(ctx context.Context, id uint) error {
	tx, ok := composables.UseTx(ctx)
	if !ok {
		return composables.ErrNoTx
	}
	if err := tx.Delete(&currency.Currency{}, id).Error; err != nil { //nolint:exhaustruct
		return err
	}
	return nil
}
