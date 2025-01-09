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
		SELECT id, type, status, created_at 
		FROM warehouse_orders wo`

	orderCountQuery = `
		SELECT COUNT(*) as count 
		FROM warehouse_orders`

	orderInsertQuery = `
		INSERT INTO warehouse_orders (type, status, created_at) 
		VALUES ($1, $2, $3) 
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
		WHERE wo.id = $3`

	orderItemsDeleteQuery = `
		DELETE FROM warehouse_order_items 
		WHERE warehouse_order_id = $1`

	orderDeleteQuery = `
		DELETE FROM warehouse_orders 
		WHERE id = $1`

	selectOrderProductsQuery = `
		SELECT 
			wp.id, 
			wp.position_id,
			wp.rfid,
			wp.status,
			wp.created_at, 
			wp.updated_at,
			p.id,
			p.title,
			p.barcode,
			p.unit_id,
			p.created_at,
			p.updated_at,
			wu.id,
			wu.title,
			wu.short_title,
			wu.created_at,
			wu.updated_at
		FROM warehouse_products wp
		LEFT JOIN warehouse_positions p ON p.id = wp.position_id
		LEFT JOIN warehouse_units wu ON wu.id = p.unit_id`

	insertOrderProductsQuery = `
		INSERT INTO warehouse_products (position_id, rfid, status, created_at)
		VALUES ($1, $2, $3, $4)
		RETURNING id`

	updateOrderProductsQuery = `
		UPDATE warehouse_products 
		SET position_id = $1, rfid = $2, status = $3
		WHERE id = $4`
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
	where, args := []string{"1 = 1"}, []interface{}{}
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
	tx, err := composables.UseTx(ctx)
	if err != nil {
		return 0, err
	}
	var count int64
	if err := tx.QueryRow(ctx, orderCountQuery).Scan(&count); err != nil {
		return 0, err
	}
	return count, nil
}

func (g *GormOrderRepository) GetAll(ctx context.Context) ([]order.Order, error) {
	return g.queryOrders(ctx, orderFindQuery)
}

func (g *GormOrderRepository) GetByID(ctx context.Context, id uint) (order.Order, error) {
	orders, err := g.queryOrders(ctx, orderFindQuery+" WHERE wo.id = $1", id)
	if err != nil {
		return nil, err
	}
	if len(orders) == 0 {
		return nil, ErrOrderNotFound
	}
	return orders[0], nil
}

func (g *GormOrderRepository) Create(ctx context.Context, data order.Order) error {
	tx, err := composables.UseTx(ctx)
	if err != nil {
		return err
	}
	dbOrder, dbProducts, err := mappers.ToDBOrder(data)
	if err != nil {
		return err
	}

	if err := tx.QueryRow(
		ctx,
		orderInsertQuery,
		dbOrder.Type,
		dbOrder.Status,
		dbOrder.CreatedAt,
	).Scan(&dbOrder.ID); err != nil {
		return err
	}

	for _, p := range dbProducts {
		if err := tx.QueryRow(
			ctx,
			insertOrderProductsQuery,
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
	tx, err := composables.UseTx(ctx)
	if err != nil {
		return err
	}
	dbOrder, dbProducts, err := mappers.ToDBOrder(data)
	if err != nil {
		return err
	}

	if _, err := tx.Exec(
		ctx,
		orderUpdateQuery,
		dbOrder.Type,
		dbOrder.Status,
		dbOrder.ID,
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
		if _, err := tx.Exec(
			ctx,
			updateOrderProductsQuery,
			product.PositionID,
			product.Rfid,
			product.Status,
			product.ID,
		); err != nil {
			return err
		}
	}

	return nil
}

func (g *GormOrderRepository) Delete(ctx context.Context, id uint) error {
	tx, err := composables.UseTx(ctx)
	if err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, orderDeleteQuery, id); err != nil {
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
			&wp.PositionID,
			&wp.Rfid,
			&wp.Status,
			&wp.CreatedAt,
			&wp.UpdatedAt,
			&pos.ID,
			&pos.Title,
			&pos.Barcode,
			&pos.UnitID,
			&pos.CreatedAt,
			&pos.UpdatedAt,
			&wu.ID,
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
