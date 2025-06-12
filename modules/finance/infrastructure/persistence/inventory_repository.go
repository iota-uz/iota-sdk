package persistence

import (
	"context"
	"fmt"

	"github.com/go-faster/errors"
	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/modules/finance/infrastructure/persistence/models"
	"github.com/iota-uz/iota-sdk/pkg/repo"

	"github.com/iota-uz/iota-sdk/modules/finance/domain/entities/inventory"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/mapping"
)

var (
	ErrInventoryNotFound = errors.New("inventory not found")
)

const (
	inventoryFindQuery = `
		SELECT i.id,
			i.tenant_id,
			i.name,
			i.description,
			i.currency_id,
			i.price,
			i.quantity,
			i.created_at,
			i.updated_at,
			COALESCE(c.code, ''),
			COALESCE(c.name, ''),
			COALESCE(c.symbol, ''),
			COALESCE(c.created_at, '1970-01-01 00:00:00'::timestamp),
			COALESCE(c.updated_at, '1970-01-01 00:00:00'::timestamp)
		FROM inventory i LEFT JOIN currencies c ON c.code = i.currency_id`
	inventoryCountQuery  = `SELECT COUNT(*) as count FROM inventory WHERE tenant_id = $1`
	inventoryInsertQuery = `
		INSERT INTO inventory (
			tenant_id,
			name,
			description,
			currency_id,
			price,
			quantity,
			created_at,
			updated_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8) RETURNING id`
	inventoryUpdateQuery = `
		UPDATE inventory
		SET name = $1, description = $2, currency_id = $3, price = $4, quantity = $5, updated_at = $6
		WHERE id = $7 AND tenant_id = $8`
	inventoryDeleteQuery = `DELETE FROM inventory WHERE id = $1 AND tenant_id = $2`
)

type InventoryRepository struct{}

func NewInventoryRepository() inventory.Repository {
	return &InventoryRepository{}
}

func (r *InventoryRepository) Create(ctx context.Context, inv inventory.Inventory) (inventory.Inventory, error) {
	tenantID, err := composables.UseTenantID(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get tenant from context: %w", err)
	}

	inv = inv.UpdateTenantID(tenantID)
	entity := ToDBInventory(inv)
	tx, err := composables.UseTx(ctx)
	if err != nil {
		return nil, err
	}
	args := []interface{}{
		entity.TenantID,
		entity.Name,
		entity.Description,
		entity.CurrencyID,
		entity.Price,
		entity.Quantity,
		entity.CreatedAt,
		entity.UpdatedAt,
	}
	row := tx.QueryRow(ctx, inventoryInsertQuery, args...)
	var id uuid.UUID
	if err := row.Scan(&id); err != nil {
		return nil, err
	}
	return r.GetByID(ctx, id)
}

func (r *InventoryRepository) GetByID(ctx context.Context, id uuid.UUID) (inventory.Inventory, error) {
	tenantID, err := composables.UseTenantID(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get tenant from context: %w", err)
	}

	items, err := r.queryInventory(ctx, repo.Join(inventoryFindQuery, "WHERE i.id = $1 AND i.tenant_id = $2"), id, tenantID)
	if err != nil {
		return nil, err
	}
	if len(items) == 0 {
		return nil, ErrInventoryNotFound
	}
	return items[0], nil
}

func (r *InventoryRepository) Count(ctx context.Context) (int64, error) {
	return r.count(ctx)
}

func (r *InventoryRepository) GetAll(ctx context.Context) ([]inventory.Inventory, error) {
	tenantID, err := composables.UseTenantID(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get tenant from context: %w", err)
	}

	q := repo.Join(inventoryFindQuery, "WHERE i.tenant_id = $1")
	return r.queryInventory(ctx, q, tenantID)
}

func (r *InventoryRepository) GetPaginated(ctx context.Context, params *inventory.FindParams) ([]inventory.Inventory, error) {
	tenantID, err := composables.UseTenantID(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get tenant from context: %w", err)
	}

	where := []string{"i.tenant_id = $1"}
	args := []interface{}{tenantID}

	if params.Query != "" && params.Field != "" {
		where = append(where, fmt.Sprintf("i.%s::VARCHAR ILIKE $%d", params.Field, len(args)+1))
		args = append(args, "%"+params.Query+"%")
	}

	limit := params.Limit
	if limit == 0 {
		limit = 20
	}

	q := repo.Join(
		inventoryFindQuery,
		repo.JoinWhere(where...),
		repo.FormatLimitOffset(limit, params.Offset),
	)

	return r.queryInventory(ctx, q, args...)
}

func (r *InventoryRepository) Update(ctx context.Context, inv inventory.Inventory) (inventory.Inventory, error) {
	tenantID, err := composables.UseTenantID(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get tenant from context: %w", err)
	}

	inv = inv.UpdateTenantID(tenantID)
	dbInventory := ToDBInventory(inv)
	args := []interface{}{
		dbInventory.Name,
		dbInventory.Description,
		dbInventory.CurrencyID,
		dbInventory.Price,
		dbInventory.Quantity,
		dbInventory.UpdatedAt,
		dbInventory.ID,
		dbInventory.TenantID,
	}
	if err := r.execQuery(ctx, inventoryUpdateQuery, args...); err != nil {
		return nil, err
	}
	return r.GetByID(ctx, inv.ID())
}

func (r *InventoryRepository) Delete(ctx context.Context, id uuid.UUID) error {
	tenantID, err := composables.UseTenantID(ctx)
	if err != nil {
		return fmt.Errorf("failed to get tenant from context: %w", err)
	}

	return r.execQuery(ctx, inventoryDeleteQuery, id, tenantID)
}

func (r *InventoryRepository) count(ctx context.Context) (int64, error) {
	tenantID, err := composables.UseTenantID(ctx)
	if err != nil {
		return 0, fmt.Errorf("failed to get tenant from context: %w", err)
	}

	tx, err := composables.UseTx(ctx)
	if err != nil {
		return 0, err
	}
	var count int64
	if err := tx.QueryRow(ctx, inventoryCountQuery, tenantID).Scan(&count); err != nil {
		return 0, err
	}
	return count, nil
}

func (r *InventoryRepository) queryInventory(ctx context.Context, query string, args ...interface{}) ([]inventory.Inventory, error) {
	tx, err := composables.UseTx(ctx)
	if err != nil {
		return nil, err
	}
	rows, err := tx.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var dbRows []*models.Inventory
	for rows.Next() {
		r := &models.Inventory{}
		var currencyCode, currencyName, currencySymbol string
		var currencyCreatedAt, currencyUpdatedAt interface{}
		if err := rows.Scan(
			&r.ID,
			&r.TenantID,
			&r.Name,
			&r.Description,
			&r.CurrencyID,
			&r.Price,
			&r.Quantity,
			&r.CreatedAt,
			&r.UpdatedAt,
			&currencyCode,
			&currencyName,
			&currencySymbol,
			&currencyCreatedAt,
			&currencyUpdatedAt,
		); err != nil {
			return nil, err
		}
		dbRows = append(dbRows, r)
	}
	return mapping.MapDBModels(dbRows, ToDomainInventory)
}

func (r *InventoryRepository) execQuery(ctx context.Context, query string, args ...interface{}) error {
	tx, err := composables.UseTx(ctx)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, query, args...)
	return err
}
