package persistence

import (
	"context"
	"errors"
	"fmt"
	"github.com/iota-uz/iota-sdk/modules/warehouse/domain/aggregates/order"
	"github.com/iota-uz/iota-sdk/modules/warehouse/domain/aggregates/product"
	"github.com/iota-uz/iota-sdk/modules/warehouse/persistence/models"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/mapping"
	"github.com/iota-uz/iota-sdk/pkg/utils/repo"
)

var (
	ErrOrderNotFound = errors.New("order not found")
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
	pool, err := composables.UsePool(ctx)
	if err != nil {
		return nil, err
	}
	where, args := []string{"1 = 1"}, []interface{}{}
	if params.ID != 0 {
		where, args = append(where, fmt.Sprintf("wo.id = $%d", len(args)+1)), append(args, params.ID)
	}
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
	sql := "SELECT id, type, status, created_at FROM warehouse_orders wo"
	rows, err := pool.Query(
		ctx,
		repo.Join(
			sql,
			repo.JoinWhere(where...),
			repo.FormatLimitOffset(params.Limit, params.Offset),
		),
		args...,
	)
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
		products, err := g.productRepo.GetPaginated(ctx, &product.FindParams{
			OrderID: o.ID,
		})
		if err != nil {
			return nil, err
		}
		// FIXME: better fix ToDomainOrder function than converting back to db model
		if o.Products, err = mapping.MapDbModels(products, toDBProduct); err != nil {
			return nil, err
		}
		domainOrder, err := ToDomainOrder(&o)
		if err != nil {
			return nil, err
		}
		orders = append(orders, domainOrder)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return orders, nil
}

func (g *GormOrderRepository) Count(ctx context.Context) (int64, error) {
	pool, err := composables.UsePool(ctx)
	if err != nil {
		return 0, err
	}
	var count int64
	if err := pool.QueryRow(ctx, `
		SELECT COUNT(*) as count FROM warehouse_orders
	`).Scan(&count); err != nil {
		return 0, err
	}
	return count, nil
}

func (g *GormOrderRepository) GetAll(ctx context.Context) ([]order.Order, error) {
	return g.GetPaginated(ctx, &order.FindParams{
		Limit: 100000,
	})
}

func (g *GormOrderRepository) GetByID(ctx context.Context, id uint) (order.Order, error) {
	orders, err := g.GetPaginated(ctx, &order.FindParams{
		ID: id,
	})
	if err != nil {
		return nil, err
	}
	if len(orders) == 0 {
		return nil, ErrOrderNotFound
	}
	return orders[0], nil
}

func (g *GormOrderRepository) Create(ctx context.Context, data order.Order) error {
	tx, err := composables.UsePoolTx(ctx)
	if err != nil {
		return err
	}
	dbOrder, err := ToDBOrder(data)
	if err != nil {
		return err
	}
	if err := tx.QueryRow(ctx, `
		INSERT INTO warehouse_orders (type, status) VALUES ($1, $2) RETURNING id
	`, dbOrder.Type, dbOrder.Status).Scan(&dbOrder.ID); err != nil {
		return err
	}

	for _, item := range dbOrder.Products {
		if _, err := tx.Exec(ctx, `
			INSERT INTO warehouse_order_items (warehouse_order_id, warehouse_product_id) VALUES ($1, $2) ON CONFLICT DO NOTHING
		`, dbOrder.ID, item.ID); err != nil {
			return err
		}
	}
	return nil
}

func (g *GormOrderRepository) Update(ctx context.Context, data order.Order) error {
	tx, err := composables.UsePoolTx(ctx)
	if err != nil {
		return err
	}
	dbOrder, err := ToDBOrder(data)
	if err != nil {
		return err
	}

	if _, err := tx.Exec(ctx, `
		UPDATE warehouse_orders wo 
		SET 
		type = COALESCE(NULLIF($1, ''), wo.type),
		status = COALESCE(NULLIF($2, ''), wo.status)
		WHERE wo.id = $3
	`, dbOrder.Type, dbOrder.Status, dbOrder.ID); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `
			DELETE FROM warehouse_order_items WHERE warehouse_order_id = $1
		`, dbOrder.ID); err != nil {
		return err
	}
	for _, product := range dbOrder.Products {
		if _, err := tx.Exec(ctx, `
			INSERT INTO warehouse_order_items (warehouse_order_id, warehouse_product_id) VALUES ($1, $2) ON CONFLICT DO NOTHING
		`, dbOrder.ID, product.ID); err != nil {
			return err
		}
	}
	return nil
}

func (g *GormOrderRepository) Delete(ctx context.Context, id uint) error {
	tx, err := composables.UsePoolTx(ctx)
	if err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `DELETE FROM warehouse_orders WHERE id = $1`, id); err != nil {
		return err
	}
	return nil
}
