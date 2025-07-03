package persistence

import (
	"context"
	_ "embed"
	"fmt"
	"strings"

	"github.com/go-faster/errors"

	"github.com/iota-uz/iota-sdk/modules/warehouse/domain/aggregates/position"
	"github.com/iota-uz/iota-sdk/modules/warehouse/infrastructure/persistence/mappers"
	"github.com/iota-uz/iota-sdk/modules/warehouse/infrastructure/persistence/models"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/repo"
)

var (
	ErrPositionNotFound = errors.New("position not found")
)

const (
	selectPositionQuery = `
	SELECT
		wp.id,
		wp.title,
		wp.barcode,
		wp.unit_id,
		wp.created_at,
		wp.updated_at,
		wp.tenant_id,
		wu.id,
		wu.title,
		wu.short_title,
		wu.created_at,
		wu.updated_at,
		wu.tenant_id
	FROM warehouse_positions wp JOIN warehouse_units wu ON wp.unit_id = wu.id`
	selectPositionIdQuery     = `SELECT id FROM warehouse_positions`
	countPositionQuery        = `SELECT COUNT(*) FROM warehouse_positions`
	insertPositionQuery       = `INSERT INTO warehouse_positions (title, barcode, unit_id, created_at, tenant_id) VALUES ($1, $2, $3, $4, $5) RETURNING id`
	insertPositionImageQuery  = `INSERT INTO warehouse_position_images (warehouse_position_id, upload_id) VALUES`
	updatePositionQuery       = `UPDATE warehouse_positions SET title = $1, barcode = $2, unit_id = $3 WHERE id = $4 AND tenant_id = $5`
	deletePositionQuery       = `DELETE FROM warehouse_positions WHERE id = $1 AND tenant_id = $2`
	deletePositionImagesQuery = `DELETE FROM warehouse_position_images WHERE warehouse_position_id = $1`
)

type GormPositionRepository struct {
}

func NewPositionRepository() position.Repository {
	return &GormPositionRepository{}
}

func (g *GormPositionRepository) queryPositions(ctx context.Context, query string, args ...interface{}) ([]position.Position, error) {
	tx, err := composables.UseTx(ctx)
	if err != nil {
		return nil, err
	}
	rows, err := tx.Query(ctx,
		query,
		args...,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	positions := make([]position.Position, 0)
	for rows.Next() {
		var p models.WarehousePosition
		var u models.WarehouseUnit
		if err := rows.Scan(
			&p.ID,
			&p.Title,
			&p.Barcode,
			&p.UnitID,
			&p.CreatedAt,
			&p.UpdatedAt,
			&p.TenantID,
			&u.ID,
			&u.Title,
			&u.ShortTitle,
			&u.CreatedAt,
			&u.UpdatedAt,
			&u.TenantID,
		); err != nil {
			return nil, err
		}
		domainPosition, err := mappers.ToDomainPosition(&p, &u)
		if err != nil {
			return nil, err
		}
		positions = append(positions, domainPosition)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return positions, nil
}

func (g *GormPositionRepository) GetPaginated(
	ctx context.Context, params *position.FindParams,
) ([]position.Position, error) {
	tenantID, err := composables.UseTenantID(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get tenant from context")
	}

	where, args := []string{"wp.tenant_id = $1"}, []interface{}{tenantID}

	if params.CreatedAt.To != "" && params.CreatedAt.From != "" {
		where, args = append(where, fmt.Sprintf("wp.created_at BETWEEN $%d and $%d", len(args)+1, len(args)+2)), append(args, params.CreatedAt.From, params.CreatedAt.To)
	}

	if params.Query != "" && params.Field != "" {
		where, args = append(where, fmt.Sprintf("wp.%s::VARCHAR ILIKE $%d", params.Field, len(args)+1)), append(args, "%"+params.Query+"%")
	}

	if len(params.Fields) > 0 {
		queries := []string{}
		for _, field := range params.Fields {
			if field == "" {
				continue
			}
			queries, args = append(queries, fmt.Sprintf("%s::varchar ILIKE ?", field)), append(args, "%"+params.Query+"%")
		}
		if len(queries) > 0 {
			where = append(where, strings.Join(queries, " OR "))
		}
	}
	return g.queryPositions(
		ctx,
		repo.Join(
			selectPositionQuery,
			repo.JoinWhere(where...),
			repo.FormatLimitOffset(params.Limit, params.Offset),
		),
		args...,
	)
}

func (g *GormPositionRepository) Count(ctx context.Context) (int64, error) {
	tx, err := composables.UseTx(ctx)
	if err != nil {
		return 0, err
	}

	tenantID, err := composables.UseTenantID(ctx)
	if err != nil {
		return 0, errors.Wrap(err, "failed to get tenant from context")
	}

	var count int64
	if err := tx.QueryRow(ctx, countPositionQuery+" WHERE tenant_id = $1", tenantID).Scan(&count); err != nil {
		return 0, err
	}
	return count, nil
}

func (g *GormPositionRepository) GetAll(ctx context.Context) ([]position.Position, error) {
	tenantID, err := composables.UseTenantID(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get tenant from context")
	}

	positions, err := g.queryPositions(ctx, selectPositionQuery+" WHERE wp.tenant_id = $1", tenantID)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get all positions")
	}
	return positions, nil
}

func (g *GormPositionRepository) GetAllPositionIds(ctx context.Context) ([]uint, error) {
	pool, err := composables.UseTx(ctx)
	if err != nil {
		return make([]uint, 0), err
	}

	tenantID, err := composables.UseTenantID(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get tenant from context")
	}

	rows, err := pool.Query(ctx, selectPositionIdQuery+" WHERE tenant_id = $1", tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]uint, 0)
	for rows.Next() {
		var id uint
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out = append(out, id)
	}
	return out, nil
}

func (g *GormPositionRepository) GetByID(ctx context.Context, id uint) (position.Position, error) {
	tenantID, err := composables.UseTenantID(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get tenant from context")
	}

	positions, err := g.queryPositions(ctx, repo.Join(selectPositionQuery, "WHERE wp.id = $1 AND wp.tenant_id = $2"), id, tenantID)
	if err != nil {
		return nil, err
	}
	if len(positions) == 0 {
		return nil, ErrPositionNotFound
	}
	return positions[0], nil
}

func (g *GormPositionRepository) GetByIDs(ctx context.Context, ids []uint) ([]position.Position, error) {
	tenantID, err := composables.UseTenantID(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get tenant from context")
	}

	positions, err := g.queryPositions(ctx, repo.Join(selectPositionQuery, "WHERE wp.id = ANY($1) AND wp.tenant_id = $2"), ids, tenantID)
	if err != nil {
		return nil, err
	}
	return positions, nil
}

func (g *GormPositionRepository) GetByBarcode(ctx context.Context, barcode string) (position.Position, error) {
	tenantID, err := composables.UseTenantID(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get tenant from context")
	}

	positions, err := g.queryPositions(ctx, repo.Join(selectPositionQuery, "WHERE wp.barcode = $1 AND wp.tenant_id = $2"), barcode, tenantID)
	if err != nil {
		return nil, err
	}
	if len(positions) == 0 {
		return nil, ErrPositionNotFound
	}
	return positions[0], nil
}

func (g *GormPositionRepository) CreateOrUpdate(ctx context.Context, data position.Position) error {
	p, err := g.GetByID(ctx, data.ID())
	if err != nil && !errors.Is(err, ErrPositionNotFound) {
		return err
	}
	if p != nil {
		if err := g.Update(ctx, data); err != nil {
			return err
		}
	} else {
		if err := g.Create(ctx, data); err != nil {
			return err
		}
	}
	return nil
}

func (g *GormPositionRepository) Create(ctx context.Context, data position.Position) error {
	tx, err := composables.UseTx(ctx)
	if err != nil {
		return err
	}

	tenantID, err := composables.UseTenantID(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to get tenant from context")
	}

	positionRow, junctionRows := mappers.ToDBPosition(data)
	positionRow.TenantID = tenantID.String()
	// Note: Position is now immutable, so we can't set TenantID directly
	// This should be handled by returning a new Position with TenantID set

	if err := tx.QueryRow(
		ctx,
		insertPositionQuery,
		positionRow.Title,
		positionRow.Barcode,
		positionRow.UnitID,
		positionRow.CreatedAt,
		positionRow.TenantID,
	).Scan(&positionRow.ID); err != nil {
		return err
	}
	if len(junctionRows) == 0 {
		return nil
	}
	values := make([][]interface{}, 0, len(junctionRows)*2)
	for _, junctionRow := range junctionRows {
		values = append(values, []interface{}{positionRow.ID, junctionRow.UploadID})
	}
	q, args := repo.BatchInsertQueryN(insertPositionImageQuery, values)
	if _, err := tx.Exec(ctx, q, args...); err != nil {
		return err
	}
	return nil
}

func (g *GormPositionRepository) Update(ctx context.Context, data position.Position) error {
	tx, err := composables.UseTx(ctx)
	if err != nil {
		return err
	}

	tenantID, err := composables.UseTenantID(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to get tenant from context")
	}

	positionRow, junctionRows := mappers.ToDBPosition(data)
	positionRow.TenantID = tenantID.String()
	// Note: Position is now immutable, TenantID should already be set

	if _, err := tx.Exec(
		ctx,
		updatePositionQuery,
		positionRow.Title,
		positionRow.Barcode,
		positionRow.UnitID,
		positionRow.ID,
		positionRow.TenantID,
	); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, deletePositionImagesQuery, positionRow.ID); err != nil {
		return err
	}
	if len(junctionRows) == 0 {
		return nil
	}
	values := make([][]interface{}, 0, len(junctionRows)*2)
	for _, junctionRow := range junctionRows {
		values = append(values, []interface{}{positionRow.ID, junctionRow.UploadID})
	}
	q, args := repo.BatchInsertQueryN(insertPositionImageQuery, values)
	if _, err := tx.Exec(ctx, q, args...); err != nil {
		return err
	}
	return nil
}

func (g *GormPositionRepository) Delete(ctx context.Context, id uint) error {
	tx, err := composables.UseTx(ctx)
	if err != nil {
		return err
	}

	tenantID, err := composables.UseTenantID(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to get tenant from context")
	}

	if _, err := tx.Exec(ctx, deletePositionQuery, id, tenantID); err != nil {
		return err
	}
	return nil
}
