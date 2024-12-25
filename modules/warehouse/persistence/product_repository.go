package persistence

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/iota-agency/iota-sdk/modules/warehouse/domain/aggregates/position"
	"github.com/iota-agency/iota-sdk/modules/warehouse/domain/aggregates/product"
	"github.com/iota-agency/iota-sdk/modules/warehouse/persistence/models"
	"github.com/iota-agency/iota-sdk/pkg/composables"
	"github.com/iota-agency/iota-sdk/pkg/mapping"
	"github.com/iota-agency/iota-sdk/pkg/utils/repo"
	"gorm.io/gorm"
)

var (
	ErrProductNotFound = errors.New("product not found")
)

type GormProductRepository struct {
	positionRepo position.Repository
}

func NewProductRepository(positionRepo position.Repository) product.Repository {
	return &GormProductRepository{
		positionRepo: positionRepo,
	}
}

func (g *GormProductRepository) tx(ctx context.Context, params *product.FindParams) (*gorm.DB, error) {
	tx, ok := composables.UseTx(ctx)
	if !ok {
		return nil, composables.ErrNoTx
	}
	var positionArgs []interface{}
	if params.Query != "" && params.Field != "" {
		if params.Field == "position" {
			positionArgs = append(positionArgs, tx.Where("title ILIKE ?", "%"+params.Query+"%"))
		}
	}
	return tx.InnerJoins("Position", positionArgs...), nil
}

func (g *GormProductRepository) GetPaginated(
	ctx context.Context, params *product.FindParams,
) ([]*product.Product, error) {
	pool, err := composables.UsePool(ctx)
	if err != nil {
		return nil, err
	}
	where, args := []string{"1 = 1"}, []interface{}{}
	if params.ID != 0 {
		where, args = append(where, fmt.Sprintf("wp.id = $%d", len(args)+1)), append(args, params.ID)
	}

	if params.Status != "" {
		where, args = append(where, fmt.Sprintf("wp.status = $%d", len(args)+1)), append(args, params.Status)
	}

	if params.CreatedAt.To != "" && params.CreatedAt.From != "" {
		where, args = append(where, fmt.Sprintf("wp.created_at BETWEEN $%d and $%d", len(args)+1, len(args)+2)), append(args, params.CreatedAt.From, params.CreatedAt.To)
	}

	if len(params.Rfids) > 0 {
		where, args = append(where, fmt.Sprintf("wp.rfid = ANY($%d)", len(args)+1)), append(args, params.Rfids)
	}

	if params.Query != "" && params.Field != "" {
		if params.Field == "position" {
			where, args = append(where, fmt.Sprintf("EXISTS (SELECT warehouse_positions WHERE id = wp.position_id AND title ILIKE $%d)", len(args)+1)), append(args, params.Query)
		} else {
			where, args = append(where, fmt.Sprintf("wp.%s::VARCHAR ILIKE $%d", params.Field, len(args)+1)), append(args, "%"+params.Query+"%")
		}
	}

	rows, err := pool.Query(ctx, `
		SELECT wp.id, wp.status, wp.position_id, wp.rfid, wp.created_at, wp.updated_at
		FROM warehouse_products wp
		WHERE `+strings.Join(where, " AND ")+`
		ORDER BY id DESC
		`+repo.FormatLimitOffset(params.Limit, params.Offset),
		args...,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	products := make([]*product.Product, 0)
	for rows.Next() {
		var product models.WarehouseProduct
		if err := rows.Scan(
			&product.ID,
			&product.Status,
			&product.PositionID,
			&product.Rfid,
			&product.CreatedAt,
			&product.UpdatedAt,
		); err != nil {
			return nil, err
		}
		domainProduct, err := toDomainProduct(&product)
		if err != nil {
			return nil, err
		}
		if domainProduct.Position, err = g.positionRepo.GetByID(ctx, domainProduct.PositionID); err != nil {
			return nil, err
		}
		products = append(products, domainProduct)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return products, nil
}

func (g *GormProductRepository) Count(ctx context.Context) (int64, error) {
	pool, err := composables.UsePool(ctx)
	if err != nil {
		return 0, err
	}
	var count int64
	if err := pool.QueryRow(ctx, `
		SELECT COUNT(*) as count FROM warehouse_products
	`).Scan(&count); err != nil {
		return 0, err
	}
	return count, nil
}

func (g *GormProductRepository) CountWithFilters(ctx context.Context, opts *product.CountParams) (int64, error) {
	pool, err := composables.UsePool(ctx)
	if err != nil {
		return 0, err
	}
	where, args := []string{"1 = 1"}, []interface{}{}
	if opts.PositionID != 0 {
		where, args = append(where, fmt.Sprintf("position_id = $%d", len(args)+1)), append(args, opts.PositionID)
	}

	if opts.Status.IsValid() {
		where, args = append(where, fmt.Sprintf("status = $%d", len(args)+1)), append(args, opts.Status)
	}

	var count int64
	if err := pool.QueryRow(ctx, `
		SELECT COUNT(*) as count FROM warehouse_products
		WHERE `+strings.Join(where, " AND ")+`
	`).Scan(&count); err != nil {
		return 0, err
	}
	return count, nil
}

func (g *GormProductRepository) FindByPositionID(ctx context.Context, opts *product.FindByPositionParams) ([]*product.Product, error) {
	products, err := g.GetPaginated(ctx, &product.FindParams{
		PositionID: opts.PositionID,
		Status:     string(opts.Status),
		SortBy:     opts.SortBy,
	})
	if err != nil {
		return nil, err
	}
	return products, nil
}

func (g *GormProductRepository) GetAll(ctx context.Context) ([]*product.Product, error) {
	products, err := g.GetPaginated(ctx, &product.FindParams{
		Limit: 100000,
	})
	if err != nil {
		return nil, err
	}
	return products, nil
}

func (g *GormProductRepository) GetByID(ctx context.Context, id uint) (*product.Product, error) {
	products, err := g.GetPaginated(ctx, &product.FindParams{
		ID: id,
	})
	if err != nil {
		return nil, err
	}
	if len(products) == 0 {
		return nil, ErrProductNotFound
	}
	return products[0], nil
}

func (g *GormProductRepository) GetByRfid(ctx context.Context, rfid string) (*product.Product, error) {
	products, err := g.GetPaginated(ctx, &product.FindParams{
		Rfids: []string{rfid},
	})
	if err != nil {
		return nil, err
	}
	if len(products) == 0 {
		return nil, ErrProductNotFound
	}
	return products[0], nil
}

func (g *GormProductRepository) GetByRfidMany(ctx context.Context, tags []string) ([]*product.Product, error) {
	products, err := g.GetPaginated(ctx, &product.FindParams{
		Rfids: tags,
	})
	if err != nil {
		return nil, err
	}
	return products, nil
}

func (g *GormProductRepository) Create(ctx context.Context, data *product.Product) error {
	tx, ok := composables.UseTx(ctx)
	if !ok {
		return composables.ErrNoTx
	}
	dbRow, err := toDBProduct(data)
	if err != nil {
		return err
	}
	return tx.Create(dbRow).Error
}

func (g *GormProductRepository) BulkCreate(ctx context.Context, data []*product.Product) error {
	tx, ok := composables.UseTx(ctx)
	if !ok {
		return composables.ErrNoTx
	}
	dbRows, err := mapping.MapDbModels(data, toDBProduct)
	if err != nil {
		return err
	}
	maxParams := 1000
	for i := 0; i < len(dbRows); i += maxParams {
		end := i + maxParams
		if end > len(dbRows) {
			end = len(dbRows)
		}
		if err := tx.Create(dbRows[i:end]).Error; err != nil {
			return err
		}
	}
	return nil
}

func (g *GormProductRepository) CreateOrUpdate(ctx context.Context, data *product.Product) error {
	tx, ok := composables.UseTx(ctx)
	if !ok {
		return composables.ErrNoTx
	}
	dbRow, err := toDBProduct(data)
	if err != nil {
		return err
	}
	return tx.Save(dbRow).Error
}

func (g *GormProductRepository) Update(ctx context.Context, data *product.Product) error {
	tx, ok := composables.UseTx(ctx)
	if !ok {
		return composables.ErrNoTx
	}
	dbRow, err := toDBProduct(data)
	if err != nil {
		return err
	}
	return tx.Save(dbRow).Error
}

func (g *GormProductRepository) UpdateStatus(ctx context.Context, uints []uint, status product.Status) error {
	tx, ok := composables.UseTx(ctx)
	if !ok {
		return composables.ErrNoTx
	}
	return tx.Model(&models.WarehouseProduct{}).Where("id IN (?)", uints).Update("status", status).Error
}

func (g *GormProductRepository) BulkDelete(ctx context.Context, IDs []uint) error {
	tx, ok := composables.UseTx(ctx)
	if !ok {
		return composables.ErrNoTx
	}
	return tx.Where("id IN (?)", IDs).Delete(&models.WarehouseProduct{}).Error
}

func (g *GormProductRepository) Delete(ctx context.Context, id uint) error {
	tx, ok := composables.UseTx(ctx)
	if !ok {
		return composables.ErrNoTx
	}
	return tx.Where("id = ?", id).Delete(&models.WarehouseProduct{}).Error //nolint:exhaustruct
}
