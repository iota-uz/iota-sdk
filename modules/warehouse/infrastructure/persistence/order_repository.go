package persistence

import (
	"context"
	"errors"
	"fmt"

	"github.com/iota-uz/iota-sdk/modules/warehouse/domain/aggregates/order"
	"github.com/iota-uz/iota-sdk/modules/warehouse/domain/aggregates/product"
	"github.com/iota-uz/iota-sdk/modules/warehouse/infrastructure/persistence/mappers"
	"github.com/iota-uz/iota-sdk/modules/warehouse/infrastructure/persistence/models"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/repo"
)

var (
	ErrOrderNotFound = errors.New("order not found")
)

const (
	orderFindQuery = `
		SELECT id, tenant_id, type, status, created_at
		FROM warehouse_orders wo`

	orderCountQuery = `
		SELECT COUNT(*) as count
		FROM warehouse_orders`

	orderInsertQuery = `
		INSERT INTO warehouse_orders (tenant_id, type, status, created_at)
		VALUES ($1, $2, $3, $4)
		RETURNING id`

	orderItemInsertQuery = `
		INSERT INTO warehouse_order_items (warehouse_order_id, warehouse_product_id)
		VALUES ($1, $2)
		ON CONFLICT DO NOTHING`

	orderUpdateQuery = `
		UPDATE warehouse_orders wo
		SET
		type = COALESCE(NULLIF($1, ''), wo.type),
		status = COALESCE(NULLIF($2, ''), wo.status)
		WHERE wo.id = $3 AND wo.tenant_id = $4`

	orderItemsDeleteQuery = `
		DELETE FROM warehouse_order_items
		WHERE warehouse_order_id = $1`

	orderDeleteQuery = `
		DELETE FROM warehouse_orders
		WHERE id = $1 AND tenant_id = $2`

	selectOrderProductsQuery = `
		SELECT
			wp.id,
			wp.tenant_id,
			wp.position_id,
			wp.rfid,
			wp.status,
			wp.created_at,
			wp.updated_at,
			p.id,
			p.tenant_id,
			p.title,
			p.barcode,
			p.unit_id,
			p.created_at,
			p.updated_at,
			wu.id,
			wu.tenant_id,
			wu.title,
			wu.short_title,
			wu.created_at,
			wu.updated_at
		FROM warehouse_products wp
		LEFT JOIN warehouse_positions p ON p.id = wp.position_id
		LEFT JOIN warehouse_units wu ON wu.id = p.unit_id`

	insertOrderProductsQuery = `
		INSERT INTO warehouse_products (tenant_id, position_id, rfid, status, created_at)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id`

	updateOrderProductsQuery = `
		UPDATE warehouse_products
		SET position_id = $1, rfid = $2, status = $3
		WHERE id = $4 AND tenant_id = $5`
)

type GormOrderRepository struct {
	productRepo product.Repository
}

func NewOrderRepository(productRepo product.Repository) order.Repository {
	return &GormOrderRepository{
		productRepo: productRepo,
	}
}

func (g *GormOrderRepository) GetPaginated(ctx context.Context, params *order.FindParams) ([]order.Order, error) {
	tenantID, err := composables.UseTenantID(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get tenant from context: %w", err)
	}

	where, args := []string{"wo.tenant_id = $1"}, []interface{}{tenantID}
	if params.CreatedAt.To != "" && params.CreatedAt.From != "" {
		where, args = append(where, fmt.Sprintf("wo.created_at BETWEEN $%d and $%d", len(args)+1, len(args)+2)), append(args, params.CreatedAt.From, params.CreatedAt.To)
	}
	if params.Query != "" && params.Field != "" {
		where, args = append(where, fmt.Sprintf("wo.%s::VARCHAR ILIKE $%d", params.Field, len(args)+1)), append(args, "%"+params.Query+"%")
	}
	if params.Status != "" {
		where, args = append(where, fmt.Sprintf("wo.status = $%d", len(args)+1)), append(args, params.Status)
	}
	if params.Type != "" {
		where, args = append(where, fmt.Sprintf("wo.type = $%d", len(args)+1)), append(args, params.Type)
	}

	q := repo.Join(
		orderFindQuery,
		repo.JoinWhere(where...),
		repo.FormatLimitOffset(params.Limit, params.Offset),
	)

	return g.queryOrders(ctx, q, args...)
}

func (g *GormOrderRepository) Count(ctx context.Context) (int64, error) {
	tenantID, err := composables.UseTenantID(ctx)
	if err != nil {
		return 0, fmt.Errorf("failed to get tenant from context: %w", err)
	}

	tx, err := composables.UseTx(ctx)
	if err != nil {
		return 0, err
	}
	var count int64
	if err := tx.QueryRow(ctx, orderCountQuery+" WHERE tenant_id = $1", tenantID).Scan(&count); err != nil {
		return 0, err
	}
	return count, nil
}

func (g *GormOrderRepository) GetAll(ctx context.Context) ([]order.Order, error) {
	tenantID, err := composables.UseTenantID(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get tenant from context: %w", err)
	}
	return g.queryOrders(ctx, orderFindQuery+" WHERE wo.tenant_id = $1", tenantID)
}

func (g *GormOrderRepository) GetByID(ctx context.Context, id uint) (order.Order, error) {
	tenantID, err := composables.UseTenantID(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get tenant from context: %w", err)
	}
	orders, err := g.queryOrders(ctx, orderFindQuery+" WHERE wo.id = $1 AND wo.tenant_id = $2", id, tenantID)
	if err != nil {
		return nil, err
	}
	if len(orders) == 0 {
		return nil, ErrOrderNotFound
	}
	return orders[0], nil
}

func (g *GormOrderRepository) Create(ctx context.Context, data order.Order) error {
	tenantID, err := composables.UseTenantID(ctx)
	if err != nil {
		return fmt.Errorf("failed to get tenant from context: %w", err)
	}

	tx, err := composables.UseTx(ctx)
	if err != nil {
		return err
	}

	// Set tenant ID in domain entity
	data.SetTenantID(tenantID)

	dbOrder, dbProducts, err := mappers.ToDBOrder(data)
	if err != nil {
		return err
	}

	// Make sure tenant ID is set in DB model
	dbOrder.TenantID = tenantID.String()

	if err := tx.QueryRow(
		ctx,
		orderInsertQuery,
		dbOrder.TenantID,
		dbOrder.Type,
		dbOrder.Status,
		dbOrder.CreatedAt,
	).Scan(&dbOrder.ID); err != nil {
		return err
	}

	for _, p := range dbProducts {
		// Set tenant ID in product
		p.TenantID = tenantID.String()

		if err := tx.QueryRow(
			ctx,
			insertOrderProductsQuery,
			p.TenantID,
			p.PositionID,
			p.Rfid,
			p.Status,
			p.CreatedAt,
		).Scan(&p.ID); err != nil {
			return err
		}
	}

	for _, item := range dbProducts {
		if _, err := tx.Exec(
			ctx,
			orderItemInsertQuery,
			dbOrder.ID,
			item.ID,
		); err != nil {
			return err
		}
	}

	return nil
}

func (g *GormOrderRepository) Update(ctx context.Context, data order.Order) error {
	tenantID, err := composables.UseTenantID(ctx)
	if err != nil {
		return fmt.Errorf("failed to get tenant from context: %w", err)
	}

	tx, err := composables.UseTx(ctx)
	if err != nil {
		return err
	}

	// Set tenant ID in domain entity
	data.SetTenantID(tenantID)

	dbOrder, dbProducts, err := mappers.ToDBOrder(data)
	if err != nil {
		return err
	}

	// Make sure tenant ID is set in DB model
	dbOrder.TenantID = tenantID.String()

	if _, err := tx.Exec(
		ctx,
		orderUpdateQuery,
		dbOrder.Type,
		dbOrder.Status,
		dbOrder.ID,
		dbOrder.TenantID,
	); err != nil {
		return err
	}

	if _, err := tx.Exec(ctx, orderItemsDeleteQuery, dbOrder.ID); err != nil {
		return err
	}

	for _, item := range dbProducts {
		if _, err := tx.Exec(
			ctx,
			orderItemInsertQuery,
			dbOrder.ID,
			item.ID,
		); err != nil {
			return err
		}
	}

	for _, product := range dbProducts {
		// Set tenant ID
		product.TenantID = tenantID.String()

		if _, err := tx.Exec(
			ctx,
			updateOrderProductsQuery,
			product.PositionID,
			product.Rfid,
			product.Status,
			product.ID,
			product.TenantID,
		); err != nil {
			return err
		}
	}

	return nil
}

func (g *GormOrderRepository) Delete(ctx context.Context, id uint) error {
	tenantID, err := composables.UseTenantID(ctx)
	if err != nil {
		return fmt.Errorf("failed to get tenant from context: %w", err)
	}

	tx, err := composables.UseTx(ctx)
	if err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, orderDeleteQuery, id, tenantID); err != nil {
		return err
	}
	return nil
}

func (g *GormOrderRepository) queryProducts(ctx context.Context, query string, args ...interface{}) ([]*product.Product, error) {
	tx, err := composables.UseTx(ctx)
	if err != nil {
		return nil, err
	}

	rows, err := tx.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var products []*product.Product

	for rows.Next() {
		var wp models.WarehouseProduct
		var pos models.WarehousePosition
		var wu models.WarehouseUnit

		if err := rows.Scan(
			&wp.ID,
			&wp.TenantID,
			&wp.PositionID,
			&wp.Rfid,
			&wp.Status,
			&wp.CreatedAt,
			&wp.UpdatedAt,
			&pos.ID,
			&pos.TenantID,
			&pos.Title,
			&pos.Barcode,
			&pos.UnitID,
			&pos.CreatedAt,
			&pos.UpdatedAt,
			&wu.ID,
			&wu.TenantID,
			&wu.Title,
			&wu.ShortTitle,
			&wu.CreatedAt,
			&wu.UpdatedAt,
		); err != nil {
			return nil, err
		}

		entity, err := mappers.ToDomainProduct(&wp, &pos, &wu)
		if err != nil {
			return nil, err
		}
		products = append(products, entity)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return products, nil
}

func (g *GormOrderRepository) queryOrders(ctx context.Context, query string, args ...interface{}) ([]order.Order, error) {
	pool, err := composables.UseTx(ctx)
	if err != nil {
		return nil, err
	}

	rows, err := pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	orders := make([]order.Order, 0)
	for rows.Next() {
		var o models.WarehouseOrder
		if err := rows.Scan(
			&o.ID,
			&o.TenantID,
			&o.Type,
			&o.Status,
			&o.CreatedAt,
		); err != nil {
			return nil, err
		}

		domainOrder, err := mappers.ToDomainOrder(&o)
		if err != nil {
			return nil, err
		}
		orders = append(orders, domainOrder)
	}

	for _, domainOrder := range orders {
		domainProducts, err := g.queryProducts(ctx,
			repo.Join(
				selectOrderProductsQuery,
				"WHERE wp.id IN (SELECT warehouse_product_id FROM warehouse_order_items WHERE warehouse_order_id = $1)",
			),
			domainOrder.ID(),
		)
		if err != nil {
			return nil, err
		}
		for _, p := range domainProducts {
			if err := domainOrder.AddItem(p.Position, p); err != nil {
				return nil, err
			}
		}
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return orders, nil
}
