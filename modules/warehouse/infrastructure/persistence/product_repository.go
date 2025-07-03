package persistence

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/iota-uz/iota-sdk/modules/warehouse/infrastructure/persistence/mappers"

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

	productCountQuery = `
		SELECT COUNT(DISTINCT wp.id) FROM warehouse_products wp`

	productInsertQuery = `
		INSERT INTO warehouse_products (tenant_id, position_id, rfid, status, created_at)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id`

	productUpdateQuery = `
		UPDATE warehouse_products
		SET position_id = $1, rfid = $2, status = $3
		WHERE id = $4 AND tenant_id = $5`

	productUpdateStatusQuery = `
		UPDATE warehouse_products
		SET status = $1
		WHERE id = ANY($2) AND tenant_id = $3`

	productDeleteQuery = `
		DELETE FROM warehouse_products
		WHERE id = $1 AND tenant_id = $2`

	productBulkDeleteQuery = `
		DELETE FROM warehouse_products
		WHERE id = ANY($1) AND tenant_id = $2`
)

type GormProductRepository struct {
}

func NewProductRepository() product.Repository {
	return &GormProductRepository{}
}

func (g *GormProductRepository) GetPaginated(ctx context.Context, params *product.FindParams) ([]*product.Product, error) {
	tenantID, err := composables.UseTenantID(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get tenant from context: %w", err)
	}

	where, args := []string{"wp.tenant_id = $1"}, []interface{}{tenantID}

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
	tenantID, err := composables.UseTenantID(ctx)
	if err != nil {
		return 0, fmt.Errorf("failed to get tenant from context: %w", err)
	}

	where, args := []string{"tenant_id = $1"}, []interface{}{tenantID}

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
	tenantID, err := composables.UseTenantID(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get tenant from context: %w", err)
	}
	return g.queryProducts(ctx, productFindQuery+" WHERE wp.tenant_id = $1", tenantID)
}

func (g *GormProductRepository) GetByID(ctx context.Context, id uint) (*product.Product, error) {
	tenantID, err := composables.UseTenantID(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get tenant from context: %w", err)
	}

	products, err := g.queryProducts(ctx, productFindQuery+" WHERE wp.id = $1 AND wp.tenant_id = $2", id, tenantID)
	if err != nil {
		return nil, err
	}
	if len(products) == 0 {
		return nil, ErrProductNotFound
	}
	return products[0], nil
}

func (g *GormProductRepository) GetByRfid(ctx context.Context, rfid string) (*product.Product, error) {
	tenantID, err := composables.UseTenantID(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get tenant from context: %w", err)
	}

	products, err := g.queryProducts(ctx, productFindQuery+" WHERE wp.rfid = $1 AND wp.tenant_id = $2", rfid, tenantID)
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

	tenantID, err := composables.UseTenantID(ctx)
	if err != nil {
		return fmt.Errorf("failed to get tenant from context: %w", err)
	}

	dbProduct, err := mappers.ToDBProduct(data)
	if err != nil {
		return err
	}

	dbProduct.TenantID = tenantID.String()
	data.TenantID = tenantID

	if err := tx.QueryRow(
		ctx,
		productInsertQuery,
		dbProduct.TenantID,
		dbProduct.PositionID,
		dbProduct.Rfid,
		dbProduct.Status,
		dbProduct.CreatedAt,
	).Scan(&data.ID); err != nil {
		return err
	}
	return nil
}

func (g *GormProductRepository) BulkCreate(ctx context.Context, data []product.Product) error {
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
	tenantID, err := composables.UseTenantID(ctx)
	if err != nil {
		return fmt.Errorf("failed to get tenant from context: %w", err)
	}

	dbProduct, err := mappers.ToDBProduct(data)
	if err != nil {
		return err
	}

	dbProduct.TenantID = tenantID.String()
	data.TenantID = tenantID

	return g.execQuery(
		ctx,
		productUpdateQuery,
		dbProduct.PositionID,
		dbProduct.Rfid,
		dbProduct.Status,
		dbProduct.ID,
		dbProduct.TenantID,
	)
}

func (g *GormProductRepository) UpdateStatus(ctx context.Context, ids []uint, status product.Status) error {
	tenantID, err := composables.UseTenantID(ctx)
	if err != nil {
		return fmt.Errorf("failed to get tenant from context: %w", err)
	}

	return g.execQuery(ctx, productUpdateStatusQuery, status, ids, tenantID)
}

func (g *GormProductRepository) Delete(ctx context.Context, id uint) error {
	tenantID, err := composables.UseTenantID(ctx)
	if err != nil {
		return fmt.Errorf("failed to get tenant from context: %w", err)
	}

	return g.execQuery(ctx, productDeleteQuery, id, tenantID)
}

func (g *GormProductRepository) BulkDelete(ctx context.Context, ids []uint) error {
	tenantID, err := composables.UseTenantID(ctx)
	if err != nil {
		return fmt.Errorf("failed to get tenant from context: %w", err)
	}

	return g.execQuery(ctx, productBulkDeleteQuery, ids, tenantID)
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

func (g *GormProductRepository) execQuery(ctx context.Context, query string, args ...interface{}) error {
	tx, err := composables.UseTx(ctx)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, query, args...)
	return err
}
