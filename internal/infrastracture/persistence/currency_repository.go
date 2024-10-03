package persistence

import (
	"context"
	"github.com/iota-agency/iota-erp/internal/domain/entities/currency"
	"github.com/iota-agency/iota-erp/internal/infrastracture/persistence/models"
	"github.com/iota-agency/iota-erp/sdk/composables"
	"github.com/iota-agency/iota-erp/sdk/service"
)

type GormCurrencyRepository struct {
}

func NewCurrencyRepository() currency.Repository {
	return &GormCurrencyRepository{}
}

func (g *GormCurrencyRepository) GetPaginated(ctx context.Context, limit, offset int, sortBy []string) ([]*currency.Currency, error) {
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
	var entities []*currency.Currency
	for _, r := range rows {
		entities = append(entities, toDomainCurrency(r))
	}
	return entities, nil
}

func (g *GormCurrencyRepository) Count(ctx context.Context) (uint, error) {
	tx, ok := composables.UseTx(ctx)
	if !ok {
		return 0, service.ErrNoTx
	}
	var count int64
	if err := tx.Model(&models.Currency{}).Count(&count).Error; err != nil {
		return 0, err
	}
	return uint(count), nil
}

func (g *GormCurrencyRepository) GetAll(ctx context.Context) ([]*currency.Currency, error) {
	tx, ok := composables.UseTx(ctx)
	if !ok {
		return nil, service.ErrNoTx
	}
	var rows []*models.Currency
	if err := tx.Find(&rows).Error; err != nil {
		return nil, err
	}
	var entities []*currency.Currency
	for _, r := range rows {
		entities = append(entities, toDomainCurrency(r))
	}
	return entities, nil
}

func (g *GormCurrencyRepository) GetByID(ctx context.Context, id uint) (*currency.Currency, error) {
	tx, ok := composables.UseTx(ctx)
	if !ok {
		return nil, service.ErrNoTx
	}
	var entity models.Currency
	if err := tx.First(&entity, id).Error; err != nil {
		return nil, err
	}
	return toDomainCurrency(&entity), nil
}

func (g *GormCurrencyRepository) Create(ctx context.Context, entity *currency.Currency) error {
	tx, ok := composables.UseTx(ctx)
	if !ok {
		return service.ErrNoTx
	}
	row := toDbCurrency(entity)
	if err := tx.Create(row).Error; err != nil {
		return err
	}
	return nil
}

func (g *GormCurrencyRepository) Update(ctx context.Context, entity *currency.Currency) error {
	tx, ok := composables.UseTx(ctx)
	if !ok {
		return service.ErrNoTx
	}
	row := toDbCurrency(entity)
	if err := tx.Save(row).Error; err != nil {
		return err
	}
	return nil
}

func (g *GormCurrencyRepository) Delete(ctx context.Context, id uint) error {
	tx, ok := composables.UseTx(ctx)
	if !ok {
		return service.ErrNoTx
	}
	if err := tx.Delete(&currency.Currency{}, id).Error; err != nil {
		return err
	}
	return nil
}
