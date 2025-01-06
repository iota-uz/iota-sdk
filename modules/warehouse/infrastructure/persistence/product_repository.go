package persistence

import (
	"context"
	"errors"
	"fmt"
	"github.com/iota-uz/iota-sdk/modules/warehouse/infrastructure/persistence/mappers"
	"strings"

	"github.com/iota-uz/iota-sdk/modules/warehouse/domain/aggregates/position"
	"github.com/iota-uz/iota-sdk/modules/warehouse/domain/aggregates/product"
	"github.com/iota-uz/iota-sdk/modules/warehouse/infrastructure/persistence/models"
	"github.com/iota-uz/iota-sdk/pkg/composables"
)

var (
	ErrProductNotFound = errors.New("product not found")
)

const (
	productFindQuery = `
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

	productCountQuery = `
		SELECT COUNT(DISTINCT wp.id) FROM warehouse_products wp`

	productInsertQuery = `
		INSERT INTO warehouse_products (position_id, rfid, status) 
		VALUES ($1, $2, $3)
		RETURNING id`

	productUpdateQuery = `
		UPDATE warehouse_products 
		SET position_id = $1, rfid = $2, status = $3
		WHERE id = $4`

	productUpdateStatusQuery = `
		UPDATE warehouse_products 
		SET status = $1
		WHERE id = ANY($2)`

	productDeleteQuery = `
		DELETE FROM warehouse_products 
		WHERE id = $1`

	productBulkDeleteQuery = `
		DELETE FROM warehouse_products 
		WHERE id = ANY($1)`
)

type GormProductRepository struct {
	positionRepo position.Repository
}

func NewProductRepository(positionRepo position.Repository) product.Repository {
	return &GormProductRepository{
		positionRepo: positionRepo,
	}
}

func (g *GormProductRepository) GetPaginated(ctx context.Context, params *product.FindParams) ([]*product.Product, error) {
	where, args := []string{"1 = 1"}, []interface{}{}

	if params.OrderID != 0 {
		where = append(where, fmt.Sprintf(
			"EXISTS (SELECT FROM warehouse_order_items WHERE warehouse_product_id = wp.id AND warehouse_order_id = $%d)",
			len(args)+1,
		))
		args = append(args, params.OrderID)
	}

	if params.Status != "" {
		where = append(where, fmt.Sprintf("wp.status = $%d", len(args)+1))
		args = append(args, params.Status)
	}

	if params.CreatedAt.To != "" && params.CreatedAt.From != "" {
		where = append(where, fmt.Sprintf(
			"wp.created_at BETWEEN $%d and $%d",
			len(args)+1, len(args)+2,
		))
		args = append(args, params.CreatedAt.From, params.CreatedAt.To)
	}

	if len(params.Rfids) > 0 {
		where = append(where, fmt.Sprintf("wp.rfid = ANY($%d)", len(args)+1))
		args = append(args, params.Rfids)
	}

	if params.Query != "" && params.Field != "" {
		if params.Field == "position" {
			where = append(where, fmt.Sprintf(
				"EXISTS (SELECT FROM warehouse_positions WHERE id = wp.position_id AND title ILIKE $%d)",
				len(args)+1,
			))
			args = append(args, params.Query)
		} else {
			where = append(where, fmt.Sprintf("wp.%s::VARCHAR ILIKE $%d", params.Field, len(args)+1))
			args = append(args, "%"+params.Query+"%")
		}
	}

	query := productFindQuery + "\n" +
		"WHERE " + strings.Join(where, " AND ") + "\n" +
		"ORDER BY wp.id DESC"

	if params.Limit > 0 {
		query += fmt.Sprintf(" LIMIT %d", params.Limit)
	}
	if params.Offset > 0 {
		query += fmt.Sprintf(" OFFSET %d", params.Offset)
	}

	return g.queryProducts(ctx, query, args...)
}

func (g *GormProductRepository) Count(ctx context.Context, opts *product.CountParams) (int64, error) {
	where, args := []string{"1 = 1"}, []interface{}{}

	if opts.PositionID != 0 {
		where = append(where, fmt.Sprintf("position_id = $%d", len(args)+1))
		args = append(args, opts.PositionID)
	}

	if opts.Status.IsValid() {
		where = append(where, fmt.Sprintf("status = $%d", len(args)+1))
		args = append(args, opts.Status)
	}

	query := productCountQuery + "\nWHERE " + strings.Join(where, " AND ")

	tx, err := composables.UseTx(ctx)
	if err != nil {
		return 0, err
	}

	var count int64
	if err := tx.QueryRow(ctx, query, args...).Scan(&count); err != nil {
		return 0, err
	}
	return count, nil
}

func (g *GormProductRepository) FindByPositionID(ctx context.Context, opts *product.FindByPositionParams) ([]*product.Product, error) {
	return g.GetPaginated(ctx, &product.FindParams{
		PositionID: opts.PositionID,
		Status:     string(opts.Status),
		SortBy:     opts.SortBy,
	})
}

func (g *GormProductRepository) GetAll(ctx context.Context) ([]*product.Product, error) {
	return g.queryProducts(ctx, productFindQuery)
}

func (g *GormProductRepository) GetByID(ctx context.Context, id uint) (*product.Product, error) {
	products, err := g.queryProducts(ctx, productFindQuery+" WHERE wp.id = $1", id)
	if err != nil {
		return nil, err
	}
	if len(products) == 0 {
		return nil, ErrProductNotFound
	}
	return products[0], nil
}

func (g *GormProductRepository) GetByRfid(ctx context.Context, rfid string) (*product.Product, error) {
	products, err := g.queryProducts(ctx, productFindQuery+" WHERE wp.rfid = $1", rfid)
	if err != nil {
		return nil, err
	}
	if len(products) == 0 {
		return nil, ErrProductNotFound
	}
	return products[0], nil
}

func (g *GormProductRepository) GetByRfidMany(ctx context.Context, tags []string) ([]*product.Product, error) {
	return g.GetPaginated(ctx, &product.FindParams{
		Rfids: tags,
	})
}

func (g *GormProductRepository) Create(ctx context.Context, data *product.Product) error {
	tx, err := composables.UseTx(ctx)
	if err != nil {
		return err
	}

	if err := tx.QueryRow(ctx, productInsertQuery,
		data.PositionID,
		data.Rfid,
		data.Status,
	).Scan(&data.ID); err != nil {
		return err
	}
	return nil
}

func (g *GormProductRepository) BulkCreate(ctx context.Context, data []*product.Product) error {
	for _, p := range data {
		if err := g.Create(ctx, p); err != nil {
			return err
		}
	}
	return nil
}

func (g *GormProductRepository) CreateOrUpdate(ctx context.Context, data *product.Product) error {
	p, err := g.GetByID(ctx, data.ID)
	if err != nil && !errors.Is(err, ErrProductNotFound) {
		return err
	}
	if p != nil {
		return g.Update(ctx, data)
	}
	return g.Create(ctx, data)
}

func (g *GormProductRepository) Update(ctx context.Context, data *product.Product) error {
	return g.execQuery(ctx, productUpdateQuery,
		data.PositionID,
		data.Rfid,
		data.Status,
		data.ID,
	)
}

func (g *GormProductRepository) UpdateStatus(ctx context.Context, ids []uint, status product.Status) error {
	return g.execQuery(ctx, productUpdateStatusQuery, status, ids)
}

func (g *GormProductRepository) Delete(ctx context.Context, id uint) error {
	return g.execQuery(ctx, productDeleteQuery, id)
}

func (g *GormProductRepository) BulkDelete(ctx context.Context, ids []uint) error {
	return g.execQuery(ctx, productBulkDeleteQuery, ids)
}

func (g *GormProductRepository) queryProducts(ctx context.Context, query string, args ...interface{}) ([]*product.Product, error) {
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

func (g *GormProductRepository) execQuery(ctx context.Context, query string, args ...interface{}) error {
	tx, err := composables.UseTx(ctx)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, query, args...)
	return err
}
