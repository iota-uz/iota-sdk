package persistence

import (
	"context"
	category "github.com/iota-agency/iota-erp/internal/domain/aggregates/expense_category"
	"github.com/iota-agency/iota-erp/internal/infrastructure/persistence/models"
	"github.com/iota-agency/iota-erp/sdk/composables"
	"github.com/iota-agency/iota-erp/sdk/service"
)

type GormExpenseCategoryRepository struct{}

func NewExpenseCategoryRepository() category.Repository {
	return &GormExpenseCategoryRepository{}
}

func (g *GormExpenseCategoryRepository) GetPaginated(
	ctx context.Context, limit, offset int,
	sortBy []string,
) ([]*category.ExpenseCategory, error) {
	tx, ok := composables.UseTx(ctx)
	if !ok {
		return nil, service.ErrNoTx
	}
	q := tx.Limit(limit).Offset(offset)
	for _, s := range sortBy {
		q = q.Order(s)
	}
	var rows []*models.ExpenseCategory
	if err := q.Preload("AmountCurrency").Find(&rows).Error; err != nil {
		return nil, err
	}
	categories := make([]*category.ExpenseCategory, len(rows))
	for i, row := range rows {
		e, err := toDomainExpenseCategory(row)
		if err != nil {
			return nil, err
		}
		categories[i] = e
	}
	return categories, nil
}

func (g *GormExpenseCategoryRepository) Count(ctx context.Context) (uint, error) {
	tx, ok := composables.UseTx(ctx)
	if !ok {
		return 0, service.ErrNoTx
	}
	var count int64
	if err := tx.Model(&models.ExpenseCategory{}).Count(&count).Error; err != nil { //nolint:exhaustruct
		return 0, err
	}
	return uint(count), nil
}

func (g *GormExpenseCategoryRepository) GetAll(ctx context.Context) ([]*category.ExpenseCategory, error) {
	tx, ok := composables.UseTx(ctx)
	if !ok {
		return nil, service.ErrNoTx
	}
	var rows []*models.ExpenseCategory
	if err := tx.Preload("AmountCurrency").Find(&rows).Error; err != nil {
		return nil, err
	}
	entities := make([]*category.ExpenseCategory, len(rows))
	for i, row := range rows {
		e, err := toDomainExpenseCategory(row)
		if err != nil {
			return nil, err
		}
		entities[i] = e
	}
	return entities, nil
}

func (g *GormExpenseCategoryRepository) GetByID(ctx context.Context, id uint) (*category.ExpenseCategory, error) {
	tx, ok := composables.UseTx(ctx)
	if !ok {
		return nil, service.ErrNoTx
	}
	var entity models.ExpenseCategory
	if err := tx.Preload("AmountCurrency").First(&entity, id).Error; err != nil {
		return nil, err
	}
	return toDomainExpenseCategory(&entity)
}

func (g *GormExpenseCategoryRepository) Create(ctx context.Context, data *category.ExpenseCategory) error {
	tx, ok := composables.UseTx(ctx)
	if !ok {
		return service.ErrNoTx
	}
	return tx.Create(toDBExpenseCategory(data)).Error
}

func (g *GormExpenseCategoryRepository) Update(ctx context.Context, data *category.ExpenseCategory) error {
	tx, ok := composables.UseTx(ctx)
	if !ok {
		return service.ErrNoTx
	}
	return tx.Updates(toDBExpenseCategory(data)).Error
}

func (g *GormExpenseCategoryRepository) Delete(ctx context.Context, id uint) error {
	tx, ok := composables.UseTx(ctx)
	if !ok {
		return service.ErrNoTx
	}
	return tx.Delete(&models.ExpenseCategory{}, id).Error //nolint:exhaustruct
}
