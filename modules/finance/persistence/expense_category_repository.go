package persistence

import (
	"context"
	"fmt"

	category "github.com/iota-agency/iota-sdk/modules/finance/domain/aggregates/expense_category"
	"github.com/iota-agency/iota-sdk/modules/finance/persistence/models"
	"github.com/iota-agency/iota-sdk/pkg/composables"
	"github.com/iota-agency/iota-sdk/pkg/mapping"
)

type GormExpenseCategoryRepository struct{}

func NewExpenseCategoryRepository() category.Repository {
	return &GormExpenseCategoryRepository{}
}

func (g *GormExpenseCategoryRepository) GetPaginated(
	ctx context.Context, params *category.FindParams,
) ([]*category.ExpenseCategory, error) {
	tx, ok := composables.UseTx(ctx)
	if !ok {
		return nil, composables.ErrNoTx
	}
	q := tx.Limit(params.Limit).Offset(params.Offset)
	if params.Query != "" && params.Field != "" {
		q = q.Where(fmt.Sprintf("%s::varchar ILIKE ?", params.Field), "%"+params.Query+"%")
	}
	if params.CreatedAt.To != "" && params.CreatedAt.From != "" {
		q = q.Where("created_at BETWEEN ? and ?", params.CreatedAt.From, params.CreatedAt.To)
	}
	for _, s := range params.SortBy {
		q = q.Order(s)
	}
	var rows []*models.ExpenseCategory
	if err := q.Preload("AmountCurrency").Find(&rows).Error; err != nil {
		return nil, err
	}
	return mapping.MapDbModels(rows, toDomainExpenseCategory)
}

func (g *GormExpenseCategoryRepository) Count(ctx context.Context) (uint, error) {
	tx, ok := composables.UseTx(ctx)
	if !ok {
		return 0, composables.ErrNoTx
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
		return nil, composables.ErrNoTx
	}
	var rows []*models.ExpenseCategory
	if err := tx.Preload("AmountCurrency").Find(&rows).Error; err != nil {
		return nil, err
	}
	return mapping.MapDbModels(rows, toDomainExpenseCategory)
}

func (g *GormExpenseCategoryRepository) GetByID(ctx context.Context, id uint) (*category.ExpenseCategory, error) {
	tx, ok := composables.UseTx(ctx)
	if !ok {
		return nil, composables.ErrNoTx
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
		return composables.ErrNoTx
	}
	return tx.Create(toDBExpenseCategory(data)).Error
}

func (g *GormExpenseCategoryRepository) Update(ctx context.Context, data *category.ExpenseCategory) error {
	tx, ok := composables.UseTx(ctx)
	if !ok {
		return composables.ErrNoTx
	}
	return tx.Updates(toDBExpenseCategory(data)).Error
}

func (g *GormExpenseCategoryRepository) Delete(ctx context.Context, id uint) error {
	tx, ok := composables.UseTx(ctx)
	if !ok {
		return composables.ErrNoTx
	}
	return tx.Delete(&models.ExpenseCategory{}, id).Error //nolint:exhaustruct
}
