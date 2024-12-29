package persistence

import (
	"context"

	"github.com/iota-uz/iota-sdk/modules/finance/domain/aggregates/expense"
	"github.com/iota-uz/iota-sdk/modules/finance/persistence/models"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/mapping"
	"gorm.io/gorm"
)

type GormExpenseRepository struct{}

func NewExpenseRepository() expense.Repository {
	return &GormExpenseRepository{}
}

func (g *GormExpenseRepository) tx(ctx context.Context, params *expense.FindParams) (*gorm.DB, error) {
	tx, ok := composables.UseTx(ctx)
	if !ok {
		return nil, composables.ErrNoTx
	}
	categoryArgs := []interface{}{}
	transactionArgs := []interface{}{}
	if params.Query != "" && params.Field != "" {
		if params.Field == "category" {
			categoryArgs = append(categoryArgs, tx.Where("name ILIKE ?", "%"+params.Query+"%"))
		}
		if params.Field == "amount" {
			transactionArgs = append(transactionArgs, tx.Where("amount::varchar ILIKE ?", "%"+params.Query+"%"))
		}
	}
	return tx.InnerJoins("Transaction", transactionArgs...).InnerJoins("Category", categoryArgs...).InnerJoins("Category.AmountCurrency"), nil
}

func (g *GormExpenseRepository) GetPaginated(
	ctx context.Context, params *expense.FindParams,
) ([]*expense.Expense, error) {
	tx, err := g.tx(ctx, params)
	if err != nil {
		return nil, err
	}
	tx = tx.Limit(params.Limit).Offset(params.Offset)
	if params.CreatedAt.To != "" && params.CreatedAt.From != "" {
		tx = tx.Where("expenses.created_at BETWEEN ? and ?", params.CreatedAt.From, params.CreatedAt.To)
	}
	for _, s := range params.SortBy {
		tx = tx.Order(s)
	}
	var rows []*models.Expense
	if err := tx.Find(&rows).Error; err != nil {
		return nil, err
	}
	return mapping.MapDbModels(rows, toDomainExpense)
}

func (g *GormExpenseRepository) Count(ctx context.Context) (uint, error) {
	tx, ok := composables.UseTx(ctx)
	if !ok {
		return 0, composables.ErrNoTx
	}
	var count int64
	if err := tx.Model(&models.Expense{}).Count(&count).Error; err != nil { //nolint:exhaustruct
		return 0, err
	}
	return uint(count), nil
}

func (g *GormExpenseRepository) GetAll(ctx context.Context) ([]*expense.Expense, error) {
	tx, err := g.tx(ctx, &expense.FindParams{})
	if err != nil {
		return nil, err
	}
	var rows []*models.Expense
	if err := tx.Find(&rows).Error; err != nil {
		return nil, err
	}
	return mapping.MapDbModels(rows, toDomainExpense)
}

func (g *GormExpenseRepository) GetByID(ctx context.Context, id uint) (*expense.Expense, error) {
	tx, err := g.tx(ctx, &expense.FindParams{})
	if err != nil {
		return nil, err
	}
	var entity models.Expense
	if err := tx.First(&entity, id).Error; err != nil {
		return nil, err
	}
	return toDomainExpense(&entity)
}

func (g *GormExpenseRepository) Create(ctx context.Context, data *expense.Expense) error {
	tx, ok := composables.UseTx(ctx)
	if !ok {
		return composables.ErrNoTx
	}
	expenseRow, transactionRow := toDBExpense(data)
	if err := tx.Create(transactionRow).Error; err != nil {
		return err
	}
	expenseRow.TransactionID = transactionRow.ID
	return tx.Create(expenseRow).Error
}

func (g *GormExpenseRepository) Update(ctx context.Context, data *expense.Expense) error {
	tx, ok := composables.UseTx(ctx)
	if !ok {
		return composables.ErrNoTx
	}
	expenseRow, transactionRow := toDBExpense(data)
	if err := tx.Save(transactionRow).Error; err != nil {
		return err
	}
	expenseRow.TransactionID = transactionRow.ID
	return tx.Save(expenseRow).Error
}

func (g *GormExpenseRepository) Delete(ctx context.Context, id uint) error {
	tx, ok := composables.UseTx(ctx)
	if !ok {
		return composables.ErrNoTx
	}
	return tx.Delete(&models.Expense{}, id).Error //nolint:exhaustruct
}
