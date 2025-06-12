package persistence

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
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
			ec.tenant_id,
			ec.name,
			ec.description,
			ec.created_at,
			ec.updated_at
		FROM expense_categories ec
	`
	countExpenseCategoryQuery  = `SELECT COUNT(*) as count FROM expense_categories ec`
	insertExpenseCategoryQuery = `
	INSERT INTO expense_categories (
		tenant_id,
		name,
		description
	)
	VALUES ($1, $2, $3) RETURNING id`
	updateExpenseCategoryQuery = `UPDATE expense_categories SET name = $1, description = $2 WHERE id = $3 AND tenant_id = $4`
	deleteExpenseCategoryQuery = `DELETE FROM expense_categories WHERE id = $1 AND tenant_id = $2`
)

type PgExpenseCategoryRepository struct {
	fieldMap map[category.Field]string
}

func NewExpenseCategoryRepository() category.Repository {
	return &PgExpenseCategoryRepository{
		fieldMap: map[category.Field]string{
			category.ID:          "ec.id",
			category.Name:        "ec.name",
			category.Description: "ec.description",
			category.CreatedAt:   "ec.created_at",
			category.UpdatedAt:   "ec.updated_at",
		},
	}
}

func (g *PgExpenseCategoryRepository) buildCategoryFilters(ctx context.Context, params *category.FindParams) ([]string, []interface{}, error) {
	tenantID, err := composables.UseTenantID(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get tenant from context: %w", err)
	}

	where := []string{"ec.tenant_id = $1"}
	args := []interface{}{tenantID.String()}

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

func (g *PgExpenseCategoryRepository) queryCategories(ctx context.Context, query string, args ...interface{}) ([]category.ExpenseCategory, error) {
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
		if err := rows.Scan(
			&ec.ID,
			&ec.TenantID,
			&ec.Name,
			&ec.Description,
			&ec.CreatedAt,
			&ec.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan expense category row: %w", err)
		}
		entity, err := ToDomainExpenseCategory(&ec)
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

func (g *PgExpenseCategoryRepository) GetPaginated(
	ctx context.Context, params *category.FindParams,
) ([]category.ExpenseCategory, error) {
	where, args, err := g.buildCategoryFilters(ctx, params)
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

func (g *PgExpenseCategoryRepository) Count(ctx context.Context, params *category.FindParams) (int64, error) {
	tx, err := composables.UseTx(ctx)
	if err != nil {
		return 0, fmt.Errorf("failed to get transaction: %w", err)
	}

	where, args, err := g.buildCategoryFilters(ctx, params)
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

func (g *PgExpenseCategoryRepository) GetAll(ctx context.Context) ([]category.ExpenseCategory, error) {
	tenantID, err := composables.UseTenantID(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get tenant from context: %w", err)
	}

	query := repo.Join(
		selectExpenseCategoryQuery,
		repo.JoinWhere("ec.tenant_id = $1"),
	)

	return g.queryCategories(ctx, query, tenantID.String())
}

func (g *PgExpenseCategoryRepository) GetByID(ctx context.Context, id uuid.UUID) (category.ExpenseCategory, error) {
	tenantID, err := composables.UseTenantID(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get tenant from context: %w", err)
	}

	query := repo.Join(
		selectExpenseCategoryQuery,
		repo.JoinWhere("ec.id = $1 AND ec.tenant_id = $2"),
	)

	categories, err := g.queryCategories(ctx, query, id, tenantID.String())
	if err != nil {
		return nil, fmt.Errorf("failed to get expense category with ID: %d: %w", id, err)
	}
	if len(categories) == 0 {
		return nil, fmt.Errorf("%s: id: %d", ErrExpenseCategoryNotFound.Error(), id)
	}
	return categories[0], nil
}

func (g *PgExpenseCategoryRepository) Create(ctx context.Context, data category.ExpenseCategory) (category.ExpenseCategory, error) {
	tx, err := composables.UseTx(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get transaction: %w", err)
	}

	tenantID, err := composables.UseTenantID(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get tenant from context: %w", err)
	}

	dbRow := ToDBExpenseCategory(data)
	dbRow.TenantID = tenantID.String()

	var id uuid.UUID
	if err := tx.QueryRow(
		ctx,
		insertExpenseCategoryQuery,
		dbRow.TenantID,
		dbRow.Name,
		dbRow.Description,
	).Scan(&id); err != nil {
		return nil, fmt.Errorf("failed to create expense category: %w", err)
	}
	return g.GetByID(ctx, id)
}

func (g *PgExpenseCategoryRepository) Update(ctx context.Context, data category.ExpenseCategory) (category.ExpenseCategory, error) {
	tx, err := composables.UseTx(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get transaction: %w", err)
	}

	tenantID, err := composables.UseTenantID(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get tenant from context: %w", err)
	}

	dbRow := ToDBExpenseCategory(data)
	dbRow.TenantID = tenantID.String()

	if _, err := tx.Exec(
		ctx,
		updateExpenseCategoryQuery,
		dbRow.Name,
		dbRow.Description,
		data.ID(),
		dbRow.TenantID,
	); err != nil {
		return nil, fmt.Errorf("failed to update expense category with ID: %d: %w", data.ID(), err)
	}
	return g.GetByID(ctx, data.ID())
}

func (g *PgExpenseCategoryRepository) Delete(ctx context.Context, id uuid.UUID) error {
	tx, err := composables.UseTx(ctx)
	if err != nil {
		return fmt.Errorf("failed to get transaction: %w", err)
	}

	tenantID, err := composables.UseTenantID(ctx)
	if err != nil {
		return fmt.Errorf("failed to get tenant from context: %w", err)
	}

	if _, err := tx.Exec(ctx, deleteExpenseCategoryQuery, id, tenantID.String()); err != nil {
		return fmt.Errorf("failed to delete expense category with ID: %d: %w", id, err)
	}
	return nil
}
