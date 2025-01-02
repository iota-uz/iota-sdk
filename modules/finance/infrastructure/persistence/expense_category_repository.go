package persistence

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"github.com/iota-uz/iota-sdk/modules/finance/infrastructure/persistence/models"
	"github.com/iota-uz/iota-sdk/pkg/utils/repo"
	"strings"

	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/currency"
	category "github.com/iota-uz/iota-sdk/modules/finance/domain/aggregates/expense_category"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/mapping"
)

var (
	ErrExpenseCategoryNotFound = errors.New("expense category not found")
)

type GormExpenseCategoryRepository struct {
	currencyRepo currency.Repository
}

func NewExpenseCategoryRepository(currencyRepo currency.Repository) category.Repository {
	return &GormExpenseCategoryRepository{
		currencyRepo: currencyRepo,
	}
}

func (g *GormExpenseCategoryRepository) GetPaginated(
	ctx context.Context, params *category.FindParams,
) ([]*category.ExpenseCategory, error) {
	pool, err := composables.UsePool(ctx)
	if err != nil {
		return nil, err
	}

	where, args := []string{"1 = 1"}, []interface{}{}
	if params.ID != 0 {
		where, args = append(where, fmt.Sprintf("ec.id = $%d", len(args)+1)), append(args, params.ID)
	}
	if params.CreatedAt.To != "" && params.CreatedAt.From != "" {
		where, args = append(where, fmt.Sprintf("ec.created_at BETWEEN $%d and $%d", len(args)+1, len(args)+2)), append(args, params.CreatedAt.From, params.CreatedAt.To)
	}
	if params.Query != "" && params.Field != "" {
		where, args = append(where, fmt.Sprintf("ec.%s::VARCHAR ILIKE $%d", params.Field, len(args)+1)), append(args, "%"+params.Query+"%")
	}

	rows, err := pool.Query(ctx, `
		SELECT ec.id, ec.name, ec.description, ec.amount_currency_id, ec.amount, ec.created_at, ec.updated_at
		FROM expense_categories ec
		WHERE `+strings.Join(where, " AND ")+`
		ORDER BY id DESC
		`+repo.FormatLimitOffset(params.Limit, params.Offset),
		args...,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	categories := make([]*category.ExpenseCategory, 0)

	for rows.Next() {
		var c models.ExpenseCategory
		var description sql.NullString
		if err := rows.Scan(
			&c.ID,
			&c.Name,
			&description,
			&c.AmountCurrencyID,
			&c.Amount,
			&c.CreatedAt,
			&c.UpdatedAt,
		); err != nil {
			return nil, err
		}
		if description.Valid {
			c.Description = mapping.Pointer(description.String)
		}
		domainCategory, err := toDomainExpenseCategory(&c)
		if err != nil {
			return nil, err
		}

		currency, err := g.currencyRepo.GetByCode(ctx, c.AmountCurrencyID)
		if err != nil {
			return nil, err
		}
		domainCategory.Currency = *currency
		categories = append(categories, domainCategory)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}
	return categories, nil
}

func (g *GormExpenseCategoryRepository) Count(ctx context.Context) (uint, error) {
	pool, err := composables.UsePool(ctx)
	if err != nil {
		return 0, err
	}
	var count uint
	if err := pool.QueryRow(ctx, `
		SELECT COUNT(*) as count FROM expense_categories
	`).Scan(&count); err != nil {
		return 0, err
	}
	return count, nil
}

func (g *GormExpenseCategoryRepository) GetAll(ctx context.Context) ([]*category.ExpenseCategory, error) {
	return g.GetPaginated(ctx, &category.FindParams{
		Limit: 100000,
	})
}

func (g *GormExpenseCategoryRepository) GetByID(ctx context.Context, id uint) (*category.ExpenseCategory, error) {
	categories, err := g.GetPaginated(ctx, &category.FindParams{
		ID: id,
	})
	if err != nil {
		return nil, err
	}
	if len(categories) == 0 {
		return nil, ErrExpenseCategoryNotFound
	}
	return categories[0], nil
}

func (g *GormExpenseCategoryRepository) Create(ctx context.Context, data *category.ExpenseCategory) error {
	tx, err := composables.UsePoolTx(ctx)
	if err != nil {
		return err
	}
	dbRow := toDBExpenseCategory(data)
	if err := tx.QueryRow(ctx, `
		INSERT INTO expense_categories (name, description, amount, amount_currency_id)
		VALUES ($1, $2, $3, $4)
		RETURNING id
	`, dbRow.Name, dbRow.Description, dbRow.Amount, dbRow.AmountCurrencyID).Scan(&data.ID); err != nil {
		return err
	}
	return nil
}

func (g *GormExpenseCategoryRepository) Update(ctx context.Context, data *category.ExpenseCategory) error {
	tx, err := composables.UsePoolTx(ctx)
	if err != nil {
		return err
	}
	dbRow := toDBExpenseCategory(data)
	if _, err := tx.Exec(ctx, `
		UPDATE expense_categories ec
		SET 
		name = $1,
		description = $2,
		amount = $3,
		amount_currency_id = $4
		WHERE id = $5
	`, dbRow.Name, dbRow.Description, dbRow.Amount, dbRow.AmountCurrencyID, data.ID); err != nil {
		return err
	}
	return nil
}

func (g *GormExpenseCategoryRepository) Delete(ctx context.Context, id uint) error {
	tx, err := composables.UsePoolTx(ctx)
	if err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `
		DELETE from expense_categories WHERE id = $1
	`, id); err != nil {
		return err
	}
	return nil
}
