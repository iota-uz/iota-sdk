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
	countExpenseCategoryQuery  = `SELECT COUNT(*) as count FROM expense_categories ec`
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
	fieldMap map[category.Field]string
}

func NewExpenseCategoryRepository() category.Repository {
	return &GormExpenseCategoryRepository{
		fieldMap: map[category.Field]string{
			category.ID:          "ec.id",
			category.Name:        "ec.name",
			category.Description: "ec.description",
			category.Amount:      "ec.amount",
			category.CurrencyID:  "ec.amount_currency_id",
			category.CreatedAt:   "ec.created_at",
			category.UpdatedAt:   "ec.updated_at",
		},
	}
}

func (g *GormExpenseCategoryRepository) buildCategoryFilters(params *category.FindParams) ([]string, []interface{}, error) {
	where := []string{"1 = 1"}
	args := []interface{}{}

	for _, filter := range params.Filters {
		column, ok := g.fieldMap[filter.Column]
		if !ok {
			return nil, nil, fmt.Errorf("invalid filter: unknown filter field: %v", filter.Column)
		}

		where = append(where, filter.Filter.String(column, len(args)+1))
		args = append(args, filter.Filter.Value()...)
	}

	// Search support
	if params.Search != "" {
		index := len(args) + 1
		where = append(
			where,
			fmt.Sprintf(
				"(ec.name ILIKE $%d OR ec.description ILIKE $%d)",
				index, index,
			),
		)
		args = append(args, "%"+params.Search+"%")
	}

	return where, args, nil
}

func (g *GormExpenseCategoryRepository) queryCategories(ctx context.Context, query string, args ...interface{}) ([]category.ExpenseCategory, error) {
	pool, err := composables.UseTx(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get transaction: %w", err)
	}
	rows, err := pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to execute query: %w", err)
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
			return nil, fmt.Errorf("failed to scan expense category row: %w", err)
		}
		entity, err := toDomainExpenseCategory(&ec, &c)
		if err != nil {
			return nil, fmt.Errorf("failed to convert to domain expense category: %w", err)
		}
		categories = append(categories, entity)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("row iteration error: %w", err)
	}
	return categories, nil
}

func (g *GormExpenseCategoryRepository) GetPaginated(
	ctx context.Context, params *category.FindParams,
) ([]category.ExpenseCategory, error) {
	where, args, err := g.buildCategoryFilters(params)
	if err != nil {
		return nil, fmt.Errorf("failed to build filters: %w", err)
	}

	query := repo.Join(
		selectExpenseCategoryQuery,
		repo.JoinWhere(where...),
		params.SortBy.ToSQL(g.fieldMap),
		repo.FormatLimitOffset(params.Limit, params.Offset),
	)

	return g.queryCategories(ctx, query, args...)
}

func (g *GormExpenseCategoryRepository) Count(ctx context.Context, params *category.FindParams) (int64, error) {
	tx, err := composables.UseTx(ctx)
	if err != nil {
		return 0, fmt.Errorf("failed to get transaction: %w", err)
	}

	where, args, err := g.buildCategoryFilters(params)
	if err != nil {
		return 0, fmt.Errorf("failed to build filters: %w", err)
	}

	query := repo.Join(
		countExpenseCategoryQuery,
		repo.JoinWhere(where...),
	)

	var count int64
	err = tx.QueryRow(ctx, query, args...).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to count expense categories: %w", err)
	}
	return count, nil
}

func (g *GormExpenseCategoryRepository) GetAll(ctx context.Context) ([]category.ExpenseCategory, error) {
	query := selectExpenseCategoryQuery
	return g.queryCategories(ctx, query)
}

func (g *GormExpenseCategoryRepository) GetByID(ctx context.Context, id uint) (category.ExpenseCategory, error) {
	query := repo.Join(
		selectExpenseCategoryQuery,
		repo.JoinWhere("ec.id = $1"),
	)

	categories, err := g.queryCategories(ctx, query, id)
	if err != nil {
		return nil, fmt.Errorf("failed to get expense category with ID: %d: %w", id, err)
	}
	if len(categories) == 0 {
		return nil, fmt.Errorf("%s: id: %d", ErrExpenseCategoryNotFound.Error(), id)
	}
	return categories[0], nil
}

func (g *GormExpenseCategoryRepository) Create(ctx context.Context, data category.ExpenseCategory) (category.ExpenseCategory, error) {
	tx, err := composables.UseTx(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get transaction: %w", err)
	}
	dbRow := toDBExpenseCategory(data)
	var id uint
	if err := tx.QueryRow(
		ctx,
		insertExpenseCategoryQuery,
		dbRow.Name,
		dbRow.Description,
		dbRow.Amount,
		dbRow.AmountCurrencyID,
	).Scan(&id); err != nil {
		return nil, fmt.Errorf("failed to create expense category: %w", err)
	}
	return g.GetByID(ctx, id)
}

func (g *GormExpenseCategoryRepository) Update(ctx context.Context, data category.ExpenseCategory) (category.ExpenseCategory, error) {
	tx, err := composables.UseTx(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get transaction: %w", err)
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
		return nil, fmt.Errorf("failed to update expense category with ID: %d: %w", data.ID(), err)
	}
	return g.GetByID(ctx, data.ID())
}

func (g *GormExpenseCategoryRepository) Delete(ctx context.Context, id uint) error {
	tx, err := composables.UseTx(ctx)
	if err != nil {
		return fmt.Errorf("failed to get transaction: %w", err)
	}
	if _, err := tx.Exec(ctx, deleteExpenseCategoryQuery, id); err != nil {
		return fmt.Errorf("failed to delete expense category with ID: %d: %w", id, err)
	}
	return nil
}
