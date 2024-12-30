package persistence

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"github.com/iota-uz/iota-sdk/pkg/utils/repo"
	"strings"

	"github.com/iota-uz/iota-sdk/modules/warehouse/domain/aggregates/position"
	"github.com/iota-uz/iota-sdk/modules/warehouse/domain/entities/unit"
	"github.com/iota-uz/iota-sdk/modules/warehouse/persistence/models"
	"github.com/iota-uz/iota-sdk/pkg/composables"
)

var (
	ErrPositionNotFound = errors.New("position not found")
)

type GormPositionRepository struct {
	unitRepo unit.Repository
}

func NewPositionRepository(unitRepo unit.Repository) position.Repository {
	return &GormPositionRepository{
		unitRepo: unitRepo,
	}
}

func (g *GormPositionRepository) GetPaginated(
	ctx context.Context, params *position.FindParams,
) ([]*position.Position, error) {
	pool, err := composables.UsePool(ctx)
	if err != nil {
		return nil, err
	}
	where, args := []string{"1 = 1"}, []interface{}{}
	if len(params.IDs) > 0 {
		where, args = append(where, fmt.Sprintf("wp.id = ANY($%d)", len(args)+1)), append(args, params.IDs)
	}

	if params.Barcode != "" {
		where, args = append(where, fmt.Sprintf("wp.barcode = $%d", len(args)+1)), append(args, params.Barcode)
	}

	if params.CreatedAt.To != "" && params.CreatedAt.From != "" {
		where, args = append(where, fmt.Sprintf("wp.created_at BETWEEN $%d and $%d", len(args)+1, len(args)+2)), append(args, params.CreatedAt.From, params.CreatedAt.To)
	}

	if params.Query != "" && params.Field != "" {
		where, args = append(where, fmt.Sprintf("wp.%s::VARCHAR ILIKE $%d", params.Field, len(args)+1)), append(args, "%"+params.Query+"%")
	}

	if len(params.Fields) > 0 {
		args := []interface{}{}
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

	rows, err := pool.Query(ctx, `
		SELECT wp.id, wp.title, wp.barcode, wp.unit_id, wp.created_at, wp.updated_at
		FROM warehouse_positions wp
		WHERE `+strings.Join(where, " AND ")+`
		ORDER BY wp.id DESC
		`+repo.FormatLimitOffset(params.Limit, params.Offset),
		args...,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	positions := make([]*position.Position, 0)
	for rows.Next() {
		var position models.WarehousePosition
		var unitID sql.NullInt32
		if err := rows.Scan(
			&position.ID,
			&position.Title,
			&position.Barcode,
			&unitID,
			&position.CreatedAt,
			&position.UpdatedAt,
		); err != nil {
			return nil, err
		}

		domainPosition, err := toDomainPosition(&position)
		if err != nil {
			return nil, err
		}

		if unitID.Valid {
			domainPosition.UnitID = uint(unitID.Int32)
			u, err := g.unitRepo.GetByID(ctx, domainPosition.UnitID)
			if err != nil {
				return nil, err
			}
			domainPosition.Unit = *u
		}
		positions = append(positions, domainPosition)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return positions, nil
}

func (g *GormPositionRepository) Count(ctx context.Context) (int64, error) {
	pool, err := composables.UsePool(ctx)
	if err != nil {
		return 0, err
	}
	var count int64
	if err := pool.QueryRow(ctx, `
		SELECT COUNT(*) as count FROM warehouse_positions
	`).Scan(&count); err != nil {
		return 0, err
	}
	return count, nil
}

func (g *GormPositionRepository) GetAll(ctx context.Context) ([]*position.Position, error) {
	positions, err := g.GetPaginated(ctx, &position.FindParams{
		Limit: 100000,
	})
	if err != nil {
		return nil, err
	}
	return positions, nil
}

func (g *GormPositionRepository) GetAllPositionIds(ctx context.Context) ([]uint, error) {
	pool, err := composables.UsePool(ctx)
	if err != nil {
		return make([]uint, 0), err
	}
	rows, err := pool.Query(ctx, `SELECT id FROM warehouse_positions`)
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

func (g *GormPositionRepository) GetByID(ctx context.Context, id uint) (*position.Position, error) {
	positions, err := g.GetPaginated(ctx, &position.FindParams{
		IDs: []uint{id},
	})
	if err != nil {
		return nil, err
	}

	if len(positions) == 0 {
		return nil, ErrPositionNotFound
	}
	return positions[0], nil
}

func (g *GormPositionRepository) GetByIDs(ctx context.Context, ids []uint) ([]*position.Position, error) {
	positions, err := g.GetPaginated(ctx, &position.FindParams{
		IDs: ids,
	})
	if err != nil {
		return nil, err
	}
	return positions, nil
}

func (g *GormPositionRepository) GetByBarcode(ctx context.Context, barcode string) (*position.Position, error) {
	positions, err := g.GetPaginated(ctx, &position.FindParams{
		Barcode: barcode,
	})
	if err != nil {
		return nil, err
	}

	if len(positions) == 0 {
		return nil, ErrPositionNotFound
	}
	return positions[0], nil
}

func (g *GormPositionRepository) CreateOrUpdate(ctx context.Context, data *position.Position) error {
	p, err := g.GetByID(ctx, data.ID)
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

func (g *GormPositionRepository) Create(ctx context.Context, data *position.Position) error {
	tx, err := composables.UsePoolTx(ctx)
	if err != nil {
		return err
	}
	positionRow, junctionRows := toDBPosition(data)
	if err := tx.QueryRow(ctx, `
		INSERT INTO warehouse_positions (title, barcode, unit_id)
		VALUES ($1, $2, $3) RETURNING id
	`, positionRow.Title, positionRow.Barcode, positionRow.UnitID).Scan(&data.ID); err != nil {
		return err
	}
	for _, junctionRow := range junctionRows {
		if _, err := tx.Exec(ctx, `
			INSERT INTO warehouse_position_images (warehouse_position_id, upload_id)
			VALUES ($1, $2)
		`, data.ID, junctionRow.UploadID); err != nil {
			return err
		}
	}
	return nil
}

func (g *GormPositionRepository) Update(ctx context.Context, data *position.Position) error {
	tx, err := composables.UsePoolTx(ctx)
	if err != nil {
		return err
	}
	positionRow, uploadRows := toDBPosition(data)
	if _, err := tx.Exec(ctx, `
		UPDATE warehouse_positions wp 
		SET 
		title = COALESCE(NULLIF($1, ''), wp.title),
		barcode = COALESCE(NULLIF($2, ''), wp.barcode),
		unit_id = COALESCE(NULLIF($3, 0), wp.unit_id)
		WHERE id = $4
	`, positionRow.Title, positionRow.Barcode, positionRow.UnitID, positionRow.ID); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `
		DELETE from warehouse_position_images WHERE warehouse_position_id = $1 
	`, data.ID); err != nil {
		return err
	}
	for _, uploadRow := range uploadRows {
		if _, err := tx.Exec(ctx, `
			INSERT INTO warehouse_position_images (warehouse_position_id, upload_id)
			VALUES ($1, $2)
		`, uploadRow.WarehousePositionID, uploadRow.UploadID); err != nil {
			return err
		}
	}
	return nil
}

func (g *GormPositionRepository) Delete(ctx context.Context, id uint) error {
	tx, err := composables.UsePoolTx(ctx)
	if err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `DELETE FROM warehouse_positions WHERE id = $1`, id); err != nil {
		return err
	}
	return nil
}
