package persistence

import (
	"context"
	"errors"
	"fmt"
	coremodels "github.com/iota-uz/iota-sdk/modules/core/infrastructure/persistence/models"
	category "github.com/iota-uz/iota-sdk/modules/finance/domain/aggregates/expense_category"
	"github.com/iota-uz/iota-sdk/modules/finance/infrastructure/persistence/models"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/repo"
)

var (
	ErrExpenseCategoryNotFound = errors.New("expense category not found")
)

const (
	selectExpenseCategoryQuery = `
		SELECT 
			ec.id,
			ec.name,
			ec.description,
			ec.amount_currency_id,
			ec.amount,
			ec.created_at,
			ec.updated_at,
			c.code,
			c.name,
			c.symbol,
			c.created_at,
			c.updated_at
		FROM expense_categories ec LEFT JOIN currencies c ON ec.amount_currency_id = c.code
	`
	countExpenseCategoryQuery  = `SELECT COUNT(*) as count FROM expense_categories`
	insertExpenseCategoryQuery = `
	INSERT INTO expense_categories (
		name, 
		description, 
		amount, 
		amount_currency_id
	)
	VALUES ($1, $2, $3, $4) RETURNING id`
	updateExpenseCategoryQuery = `UPDATE expense_categories SET name = $1, description = $2, amount = $3, amount_currency_id = $4 WHERE id = $5`
	deleteExpenseCategoryQuery = `DELETE FROM expense_categories WHERE id = $1`
)

type GormExpenseCategoryRepository struct {
}

func NewExpenseCategoryRepository() category.Repository {
	return &GormExpenseCategoryRepository{}
}

func (g *GormExpenseCategoryRepository) queryCategories(ctx context.Context, query string, args ...interface{}) ([]category.ExpenseCategory, error) {
	pool, err := composables.UseTx(ctx)
	if err != nil {
		return nil, err
	}
	rows, err := pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	categories := make([]category.ExpenseCategory, 0)

	for rows.Next() {
		var ec models.ExpenseCategory
		var c coremodels.Currency
		if err := rows.Scan(
			&ec.ID,
			&ec.Name,
			&ec.Description,
			&ec.AmountCurrencyID,
			&ec.Amount,
			&ec.CreatedAt,
			&ec.UpdatedAt,
			&c.Code,
			&c.Name,
			&c.Symbol,
			&c.CreatedAt,
			&c.UpdatedAt,
		); err != nil {
			return nil, err
		}
		entity, err := toDomainExpenseCategory(&ec, &c)
		if err != nil {
			return nil, err
		}
		categories = append(categories, entity)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}
	return categories, nil
}

func (g *GormExpenseCategoryRepository) GetPaginated(
	ctx context.Context, params *category.FindParams,
) ([]category.ExpenseCategory, error) {
	where, args := []string{"1 = 1"}, []interface{}{}
	if params.CreatedAt.To != "" && params.CreatedAt.From != "" {
		where, args = append(where, fmt.Sprintf("ec.created_at BETWEEN $%d and $%d", len(args)+1, len(args)+2)), append(args, params.CreatedAt.From, params.CreatedAt.To)
	}
	if params.Query != "" && params.Field != "" {
		where, args = append(where, fmt.Sprintf("ec.%s::VARCHAR ILIKE $%d", params.Field, len(args)+1)), append(args, "%"+params.Query+"%")
	}
	return g.queryCategories(
		ctx,
		repo.Join(
			selectExpenseCategoryQuery,
			repo.JoinWhere(where...),
			repo.FormatLimitOffset(params.Limit, params.Offset),
		),
		args...,
	)
}

func (g *GormExpenseCategoryRepository) Count(ctx context.Context) (uint, error) {
	pool, err := composables.UseTx(ctx)
	if err != nil {
		return 0, err
	}
	var count uint
	if err := pool.QueryRow(ctx, countExpenseCategoryQuery).Scan(&count); err != nil {
		return 0, err
	}
	return count, nil
}

func (g *GormExpenseCategoryRepository) GetAll(ctx context.Context) ([]category.ExpenseCategory, error) {
	return g.queryCategories(ctx, selectExpenseCategoryQuery)
}

func (g *GormExpenseCategoryRepository) GetByID(ctx context.Context, id uint) (category.ExpenseCategory, error) {
	categories, err := g.queryCategories(ctx, selectExpenseCategoryQuery+" WHERE ec.id = $1", id)
	if err != nil {
		return nil, err
	}
	if len(categories) == 0 {
		return nil, ErrExpenseCategoryNotFound
	}
	return categories[0], nil
}

func (g *GormExpenseCategoryRepository) Create(ctx context.Context, data category.ExpenseCategory) (category.ExpenseCategory, error) {
	tx, err := composables.UseTx(ctx)
	if err != nil {
		return nil, err
	}
	dbRow := toDBExpenseCategory(data)
	if err := tx.QueryRow(
		ctx,
		insertExpenseCategoryQuery,
		dbRow.Name,
		dbRow.Description,
		dbRow.Amount,
		dbRow.AmountCurrencyID,
	).Scan(&dbRow.ID); err != nil {
		return nil, err
	}
	return g.GetByID(ctx, dbRow.ID)
}

func (g *GormExpenseCategoryRepository) Update(ctx context.Context, data category.ExpenseCategory) (category.ExpenseCategory, error) {
	tx, err := composables.UseTx(ctx)
	if err != nil {
		return nil, err
	}
	dbRow := toDBExpenseCategory(data)
	if _, err := tx.Exec(
		ctx,
		updateExpenseCategoryQuery,
		dbRow.Name,
		dbRow.Description,
		dbRow.Amount,
		dbRow.AmountCurrencyID,
		data.ID(),
	); err != nil {
		return nil, err
	}
	return g.GetByID(ctx, data.ID())
}

func (g *GormExpenseCategoryRepository) Delete(ctx context.Context, id uint) error {
	tx, err := composables.UseTx(ctx)
	if err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, deleteExpenseCategoryQuery, id); err != nil {
		return err
	}
	return nil
}
