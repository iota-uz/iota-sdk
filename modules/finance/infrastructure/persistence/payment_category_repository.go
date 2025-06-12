package persistence

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	paymentcategory "github.com/iota-uz/iota-sdk/modules/finance/domain/aggregates/payment_category"
	"github.com/iota-uz/iota-sdk/modules/finance/infrastructure/persistence/models"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/repo"
)

var (
	ErrPaymentCategoryNotFound = errors.New("payment category not found")
)

const (
	selectPaymentCategoryQuery = `
		SELECT
			pc.id,
			pc.tenant_id,
			pc.name,
			pc.description,
			pc.created_at,
			pc.updated_at
		FROM payment_categories pc
	`
	countPaymentCategoryQuery  = `SELECT COUNT(*) as count FROM payment_categories pc`
	insertPaymentCategoryQuery = `
	INSERT INTO payment_categories (
		tenant_id,
		name,
		description
	)
	VALUES ($1, $2, $3) RETURNING id`
	updatePaymentCategoryQuery = `UPDATE payment_categories SET name = $1, description = $2 WHERE id = $3 AND tenant_id = $4`
	deletePaymentCategoryQuery = `DELETE FROM payment_categories WHERE id = $1 AND tenant_id = $2`
)

type GormPaymentCategoryRepository struct {
	fieldMap map[paymentcategory.Field]string
}

func NewPaymentCategoryRepository() paymentcategory.Repository {
	return &GormPaymentCategoryRepository{
		fieldMap: map[paymentcategory.Field]string{
			paymentcategory.ID:          "pc.id",
			paymentcategory.Name:        "pc.name",
			paymentcategory.Description: "pc.description",
			paymentcategory.CreatedAt:   "pc.created_at",
			paymentcategory.UpdatedAt:   "pc.updated_at",
		},
	}
}

func (g *GormPaymentCategoryRepository) buildCategoryFilters(ctx context.Context, params *paymentcategory.FindParams) ([]string, []interface{}, error) {
	tenantID, err := composables.UseTenantID(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get tenant from context: %w", err)
	}

	where := []string{"pc.tenant_id = $1"}
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
				"(pc.name ILIKE $%d OR pc.description ILIKE $%d)",
				index, index,
			),
		)
		args = append(args, "%"+params.Search+"%")
	}

	return where, args, nil
}

func (g *GormPaymentCategoryRepository) queryCategories(ctx context.Context, query string, args ...interface{}) ([]paymentcategory.PaymentCategory, error) {
	pool, err := composables.UseTx(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get transaction: %w", err)
	}
	rows, err := pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to execute query: %w", err)
	}
	defer rows.Close()
	categories := make([]paymentcategory.PaymentCategory, 0)

	for rows.Next() {
		var pc models.PaymentCategory
		if err := rows.Scan(
			&pc.ID,
			&pc.TenantID,
			&pc.Name,
			&pc.Description,
			&pc.CreatedAt,
			&pc.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan payment category row: %w", err)
		}
		entity, err := ToDomainPaymentCategory(&pc)
		if err != nil {
			return nil, fmt.Errorf("failed to convert to domain payment category: %w", err)
		}
		categories = append(categories, entity)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("row iteration error: %w", err)
	}
	return categories, nil
}

func (g *GormPaymentCategoryRepository) GetPaginated(
	ctx context.Context, params *paymentcategory.FindParams,
) ([]paymentcategory.PaymentCategory, error) {
	where, args, err := g.buildCategoryFilters(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("failed to build filters: %w", err)
	}

	query := repo.Join(
		selectPaymentCategoryQuery,
		repo.JoinWhere(where...),
		params.SortBy.ToSQL(g.fieldMap),
		repo.FormatLimitOffset(params.Limit, params.Offset),
	)

	return g.queryCategories(ctx, query, args...)
}

func (g *GormPaymentCategoryRepository) Count(ctx context.Context, params *paymentcategory.FindParams) (int64, error) {
	tx, err := composables.UseTx(ctx)
	if err != nil {
		return 0, fmt.Errorf("failed to get transaction: %w", err)
	}

	where, args, err := g.buildCategoryFilters(ctx, params)
	if err != nil {
		return 0, fmt.Errorf("failed to build filters: %w", err)
	}

	query := repo.Join(
		countPaymentCategoryQuery,
		repo.JoinWhere(where...),
	)

	var count int64
	err = tx.QueryRow(ctx, query, args...).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to count payment categories: %w", err)
	}
	return count, nil
}

func (g *GormPaymentCategoryRepository) GetAll(ctx context.Context) ([]paymentcategory.PaymentCategory, error) {
	tenantID, err := composables.UseTenantID(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get tenant from context: %w", err)
	}

	query := repo.Join(
		selectPaymentCategoryQuery,
		repo.JoinWhere("pc.tenant_id = $1"),
	)

	return g.queryCategories(ctx, query, tenantID.String())
}

func (g *GormPaymentCategoryRepository) GetByID(ctx context.Context, id uuid.UUID) (paymentcategory.PaymentCategory, error) {
	tenantID, err := composables.UseTenantID(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get tenant from context: %w", err)
	}

	query := repo.Join(
		selectPaymentCategoryQuery,
		repo.JoinWhere("pc.id = $1 AND pc.tenant_id = $2"),
	)

	categories, err := g.queryCategories(ctx, query, id, tenantID.String())
	if err != nil {
		return nil, fmt.Errorf("failed to get payment category with ID: %s: %w", id, err)
	}
	if len(categories) == 0 {
		return nil, fmt.Errorf("%s: id: %s", ErrPaymentCategoryNotFound.Error(), id)
	}
	return categories[0], nil
}

func (g *GormPaymentCategoryRepository) Create(ctx context.Context, data paymentcategory.PaymentCategory) (paymentcategory.PaymentCategory, error) {
	tx, err := composables.UseTx(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get transaction: %w", err)
	}

	tenantID, err := composables.UseTenantID(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get tenant from context: %w", err)
	}

	dbRow := ToDBPaymentCategory(data)
	dbRow.TenantID = tenantID.String()

	var id uuid.UUID
	if err := tx.QueryRow(
		ctx,
		insertPaymentCategoryQuery,
		dbRow.TenantID,
		dbRow.Name,
		dbRow.Description,
	).Scan(&id); err != nil {
		return nil, fmt.Errorf("failed to create payment category: %w", err)
	}
	return g.GetByID(ctx, id)
}

func (g *GormPaymentCategoryRepository) Update(ctx context.Context, data paymentcategory.PaymentCategory) (paymentcategory.PaymentCategory, error) {
	tx, err := composables.UseTx(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get transaction: %w", err)
	}

	tenantID, err := composables.UseTenantID(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get tenant from context: %w", err)
	}

	dbRow := ToDBPaymentCategory(data)
	dbRow.TenantID = tenantID.String()

	if _, err := tx.Exec(
		ctx,
		updatePaymentCategoryQuery,
		dbRow.Name,
		dbRow.Description,
		data.ID(),
		dbRow.TenantID,
	); err != nil {
		return nil, fmt.Errorf("failed to update payment category with ID: %s: %w", data.ID(), err)
	}
	return g.GetByID(ctx, data.ID())
}

func (g *GormPaymentCategoryRepository) Delete(ctx context.Context, id uuid.UUID) error {
	tx, err := composables.UseTx(ctx)
	if err != nil {
		return fmt.Errorf("failed to get transaction: %w", err)
	}

	tenantID, err := composables.UseTenantID(ctx)
	if err != nil {
		return fmt.Errorf("failed to get tenant from context: %w", err)
	}

	if _, err := tx.Exec(ctx, deletePaymentCategoryQuery, id, tenantID.String()); err != nil {
		return fmt.Errorf("failed to delete payment category with ID: %s: %w", id, err)
	}
	return nil
}
