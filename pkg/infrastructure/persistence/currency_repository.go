package persistence

import (
	"context"
	currency2 "github.com/iota-agency/iota-sdk/modules/finance/domain/entities/currency"
	"github.com/iota-agency/iota-sdk/pkg/composables"
	"github.com/iota-agency/iota-sdk/pkg/service"

	"github.com/iota-agency/iota-sdk/pkg/infrastructure/persistence/models"
)

type GormCurrencyRepository struct{}

func NewCurrencyRepository() currency2.Repository {
	return &GormCurrencyRepository{}
}

func (g *GormCurrencyRepository) GetPaginated(
	ctx context.Context,
	limit, offset int,
	sortBy []string,
) ([]*currency2.Currency, error) {
	tx, ok := composables.UseTx(ctx)
	if !ok {
		return nil, service.ErrNoTx
	}
	var rows []*models.Currency
	q := tx.Limit(limit).Offset(offset)
	for _, s := range sortBy {
		q = q.Order(s)
	}
	if err := q.Find(&rows).Error; err != nil {
		return nil, err
	}
	entities := make([]*currency2.Currency, 0, len(rows))
	for _, r := range rows {
		c, err := toDomainCurrency(r)
		if err != nil {
			return nil, err
		}
		entities = append(entities, c)
	}
	return entities, nil
}

func (g *GormCurrencyRepository) Count(ctx context.Context) (uint, error) {
	tx, ok := composables.UseTx(ctx)
	if !ok {
		return 0, service.ErrNoTx
	}
	var count int64
	if err := tx.Model(&models.Currency{}).Count(&count).Error; err != nil { //nolint:exhaustruct
		return 0, err
	}
	return uint(count), nil
}

func (g *GormCurrencyRepository) GetAll(ctx context.Context) ([]*currency2.Currency, error) {
	tx, ok := composables.UseTx(ctx)
	if !ok {
		return nil, service.ErrNoTx
	}
	var rows []*models.Currency
	if err := tx.Find(&rows).Error; err != nil {
		return nil, err
	}
	entities := make([]*currency2.Currency, 0, len(rows))
	for _, r := range rows {
		c, err := toDomainCurrency(r)
		if err != nil {
			return nil, err
		}
		entities = append(entities, c)
	}
	return entities, nil
}

func (g *GormCurrencyRepository) GetByID(ctx context.Context, id uint) (*currency2.Currency, error) {
	tx, ok := composables.UseTx(ctx)
	if !ok {
		return nil, service.ErrNoTx
	}
	var entity models.Currency
	if err := tx.First(&entity, id).Error; err != nil {
		return nil, err
	}
	return toDomainCurrency(&entity)
}

func (g *GormCurrencyRepository) Create(ctx context.Context, entity *currency2.Currency) error {
	tx, ok := composables.UseTx(ctx)
	if !ok {
		return service.ErrNoTx
	}
	row := toDBCurrency(entity)
	return tx.Create(row).Error
}

func (g *GormCurrencyRepository) Update(ctx context.Context, entity *currency2.Currency) error {
	tx, ok := composables.UseTx(ctx)
	if !ok {
		return service.ErrNoTx
	}
	row := toDBCurrency(entity)
	return tx.Save(row).Error
}

func (g *GormCurrencyRepository) CreateOrUpdate(ctx context.Context, currency *currency2.Currency) error {
	tx, ok := composables.UseTx(ctx)
	if !ok {
		return service.ErrNoTx
	}
	row := toDBCurrency(currency)
	return tx.Save(row).Error
}

func (g *GormCurrencyRepository) Delete(ctx context.Context, id uint) error {
	tx, ok := composables.UseTx(ctx)
	if !ok {
		return service.ErrNoTx
	}
	if err := tx.Delete(&currency2.Currency{}, id).Error; err != nil { //nolint:exhaustruct
		return err
	}
	return nil
}
