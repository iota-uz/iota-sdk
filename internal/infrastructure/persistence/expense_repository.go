package persistence

import (
	"context"
	"github.com/iota-agency/iota-erp/internal/domain/aggregates/expense"
	"github.com/iota-agency/iota-erp/internal/infrastructure/persistence/models"
	"github.com/iota-agency/iota-erp/pkg/composables"
	"github.com/iota-agency/iota-erp/sdk/service"
	"gorm.io/gorm"
)

type GormExpenseRepository struct{}

func NewExpenseRepository() expense.Repository {
	return &GormExpenseRepository{}
}

func (g *GormExpenseRepository) txWithPreloads(ctx context.Context) (*gorm.DB, error) {
	tx, ok := composables.UseTx(ctx)
	if !ok {
		return nil, service.ErrNoTx
	}
	return tx.Preload("Transaction").Preload("Category").Preload("Category.AmountCurrency"), nil
}

func (g *GormExpenseRepository) GetPaginated(
	ctx context.Context, limit, offset int,
	sortBy []string,
) ([]*expense.Expense, error) {
	tx, err := g.txWithPreloads(ctx)
	if err != nil {
		return nil, err
	}
	q := tx.Limit(limit).Offset(offset)
	for _, s := range sortBy {
		q = q.Order(s)
	}
	var rows []*models.Expense
	if err := q.Find(&rows).Error; err != nil {
		return nil, err
	}
	entities := make([]*expense.Expense, len(rows))
	for i, row := range rows {
		e, err := toDomainExpense(row)
		if err != nil {
			return nil, err
		}
		entities[i] = e
	}
	return entities, nil
}

func (g *GormExpenseRepository) Count(ctx context.Context) (uint, error) {
	tx, ok := composables.UseTx(ctx)
	if !ok {
		return 0, service.ErrNoTx
	}
	var count int64
	if err := tx.Model(&models.Expense{}).Count(&count).Error; err != nil { //nolint:exhaustruct
		return 0, err
	}
	return uint(count), nil
}

func (g *GormExpenseRepository) GetAll(ctx context.Context) ([]*expense.Expense, error) {
	tx, err := g.txWithPreloads(ctx)
	if err != nil {
		return nil, err
	}
	var rows []*models.Expense
	if err := tx.Find(&rows).Error; err != nil {
		return nil, err
	}
	entities := make([]*expense.Expense, len(rows))
	for i, row := range rows {
		e, err := toDomainExpense(row)
		if err != nil {
			return nil, err
		}
		entities[i] = e
	}
	return entities, nil
}

func (g *GormExpenseRepository) GetByID(ctx context.Context, id uint) (*expense.Expense, error) {
	tx, err := g.txWithPreloads(ctx)
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
		return service.ErrNoTx
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
		return service.ErrNoTx
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
		return service.ErrNoTx
	}
	return tx.Delete(&models.Expense{}, id).Error //nolint:exhaustruct
}
