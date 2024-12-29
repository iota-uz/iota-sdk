package persistence

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/iota-uz/iota-sdk/modules/finance/domain/aggregates/expense"
	category "github.com/iota-agency/iota-sdk/modules/finance/domain/aggregates/expense_category"
	"github.com/iota-agency/iota-sdk/modules/finance/domain/entities/transaction"
	"github.com/iota-uz/iota-sdk/modules/finance/persistence/models"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/utils/repo"
	"gorm.io/gorm"
)

var (
	ErrExpenseNotFound = errors.New("expense not found")
)

type GormExpenseRepository struct {
	categoryRepo    category.Repository
	transactionRepo transaction.Repository
}

func NewExpenseRepository(categoryRepo category.Repository, transactionRepo transaction.Repository) expense.Repository {
	return &GormExpenseRepository{
		categoryRepo:    categoryRepo,
		transactionRepo: transactionRepo,
	}
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
	pool, err := composables.UsePool(ctx)
	if err != nil {
		return nil, err
	}
	where, args := []string{"1 = 1"}, []interface{}{}
	if params.ID != 0 {
		where, args = append(where, fmt.Sprintf("ex.id = $%d", len(args)+1)), append(args, params.ID)
	}

	if params.CreatedAt.To != "" && params.CreatedAt.From != "" {
		where, args = append(where, fmt.Sprintf("ex.created_at BETWEEN $%d and $%d", len(args)+1, len(args)+2)), append(args, params.CreatedAt.From, params.CreatedAt.To)
	}

	if params.Query != "" && params.Field != "" {
		if params.Field == "category" {
		} else if params.Field == "amount" {

		} else {
			where, args = append(where, fmt.Sprintf("ex.%s::VARCHAR ILIKE $%d", params.Field, len(args)+1)), append(args, "%"+params.Query+"%")
		}
	}

	rows, err := pool.Query(ctx, `
		SELECT ex.id, ex.transaction_id, ex.category_id, ex.created_at, ex.updated_at
		FROM expenses ex
		WHERE `+strings.Join(where, " AND ")+`
		ORDER BY id DESC
		`+repo.FormatLimitOffset(params.Limit, params.Offset),
		args...,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	expenses := make([]*expense.Expense, 0)
	for rows.Next() {
		var expense models.Expense
		if err := rows.Scan(
			&expense.ID,
			&expense.TransactionID,
			&expense.CategoryID,
			&expense.CreatedAt,
			&expense.UpdatedAt,
		); err != nil {
			return nil, err
		}

		domainExpense, err := toDomainExpense(&expense)
		if err != nil {
			return nil, err
		}
		category, err := g.categoryRepo.GetByID(ctx, expense.CategoryID)
		if err != nil {
			return nil, err
		}
		domainExpense.Category = *category
		expenses = append(expenses, domainExpense)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}
	return expenses, nil
}

func (g *GormExpenseRepository) Count(ctx context.Context) (uint, error) {
	pool, err := composables.UsePool(ctx)
	if err != nil {
		return 0, err
	}
	var count uint
	if err := pool.QueryRow(ctx, `
		SELECT COUNT(*) as count FROM expenses
	`).Scan(&count); err != nil {
		return 0, err
	}
	return count, nil
}

func (g *GormExpenseRepository) GetAll(ctx context.Context) ([]*expense.Expense, error) {
	return g.GetPaginated(ctx, &expense.FindParams{
		Limit: 100000,
	})
}

func (g *GormExpenseRepository) GetByID(ctx context.Context, id uint) (*expense.Expense, error) {
	expenses, err := g.GetPaginated(ctx, &expense.FindParams{
		ID: id,
	})
	if err != nil {
		return nil, err
	}
	if len(expenses) == 0 {
		return nil, ErrExpenseNotFound
	}
	return expenses[0], nil
}

func (g *GormExpenseRepository) Create(ctx context.Context, data *expense.Expense) error {
	tx, err := composables.UsePoolTx(ctx)
	if err != nil {
		return err
	}
	expenseRow, transaction := toDBExpense(data)
	if err := g.transactionRepo.Create(ctx, transaction); err != nil {
		return err
	}
	if err := tx.QueryRow(ctx, `
		INSERT INTO expenses (transaction_id, category_id)
		VALUES ($1, $2)
	`, transaction.ID, expenseRow.CategoryID).Scan(&data.ID); err != nil {
		return err
	}
	expenseRow.TransactionID = transaction.ID
	return nil
}

func (g *GormExpenseRepository) Update(ctx context.Context, data *expense.Expense) error {
	tx, err := composables.UsePoolTx(ctx)
	if err != nil {
		return err
	}
	expenseRow, transaction := toDBExpense(data)
	if err := g.transactionRepo.Update(ctx, transaction); err != nil {
		return err
	}
	expenseRow.TransactionID = transaction.ID
	if _, err := tx.Exec(ctx, `
		UPDATE expenses
		SET transaction_id = $1, category_id = 2
		WHERE id = $3
	`, expenseRow.TransactionID, expenseRow.CategoryID); err != nil {
		return err
	}
	return nil
}

func (g *GormExpenseRepository) Delete(ctx context.Context, id uint) error {
	tx, err := composables.UsePoolTx(ctx)
	if err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `DELETE FROM expenses where id = $1`, id); err != nil {
		return err
	}
	return nil
}
