package persistence

import (
	"context"
	_ "embed"
	"fmt"
	"github.com/go-faster/errors"
	"strings"

	"github.com/iota-uz/iota-sdk/modules/warehouse/domain/aggregates/position"
	"github.com/iota-uz/iota-sdk/modules/warehouse/infrastructure/persistence/mappers"
	"github.com/iota-uz/iota-sdk/modules/warehouse/infrastructure/persistence/models"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/utils/repo"
)

var (
	ErrPositionNotFound = errors.New("position not found")
)

var (
	//go:embed queries/position_queries.sql
	positionsQueries string
)

type GormPositionRepository struct {
	selectQuery       string
	selectIdQuery     string
	countQuery        string
	insertQuery       string
	insertImageQuery  string
	updateQuery       string
	deleteQuery       string
	deleteImagesQuery string
}

func NewPositionRepository() position.Repository {
	queries := repo.MustParseSQLQueries(positionsQueries)
	return &GormPositionRepository{
		selectQuery:       queries["select"],
		selectIdQuery:     queries["select_id_only"],
		countQuery:        queries["count"],
		insertQuery:       queries["insert"],
		insertImageQuery:  queries["insert_image"],
		updateQuery:       queries["update"],
		deleteQuery:       queries["delete"],
		deleteImagesQuery: queries["delete_images"],
	}
}

func (g *GormPositionRepository) queryPositions(ctx context.Context, query string, args ...interface{}) ([]*position.Position, error) {
	tx, err := composables.UsePoolTx(context.Background())
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
	positions := make([]*position.Position, 0)
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
			&u.ID,
			&u.Title,
			&u.ShortTitle,
			&u.CreatedAt,
			&u.UpdatedAt,
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
) ([]*position.Position, error) {
	where, args := []string{"1 = 1"}, []interface{}{}

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
			g.selectQuery,
			repo.JoinWhere(where...),
			repo.FormatLimitOffset(params.Limit, params.Offset),
		),
		args...,
	)
}

func (g *GormPositionRepository) Count(ctx context.Context) (int64, error) {
	pool, err := composables.UsePoolTx(ctx)
	if err != nil {
		return 0, err
	}
	var count int64
	if err := pool.QueryRow(ctx, g.countQuery).Scan(&count); err != nil {
		return 0, err
	}
	return count, nil
}

func (g *GormPositionRepository) GetAll(ctx context.Context) ([]*position.Position, error) {
	positions, err := g.queryPositions(ctx, g.selectQuery)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get all positions")
	}
	return positions, nil
}

func (g *GormPositionRepository) GetAllPositionIds(ctx context.Context) ([]uint, error) {
	pool, err := composables.UsePoolTx(ctx)
	if err != nil {
		return make([]uint, 0), err
	}
	rows, err := pool.Query(ctx, g.selectIdQuery)
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
	positions, err := g.queryPositions(ctx, repo.Join(g.selectQuery, "WHERE wp.id = $1"), id)
	if err != nil {
		return nil, err
	}
	if len(positions) == 0 {
		return nil, ErrPositionNotFound
	}
	return positions[0], nil
}

func (g *GormPositionRepository) GetByIDs(ctx context.Context, ids []uint) ([]*position.Position, error) {
	positions, err := g.queryPositions(ctx, repo.Join(g.selectQuery, "WHERE wp.id = ANY($1)"), ids)
	if err != nil {
		return nil, err
	}
	return positions, nil
}

func (g *GormPositionRepository) GetByBarcode(ctx context.Context, barcode string) (*position.Position, error) {
	positions, err := g.queryPositions(ctx, repo.Join(g.selectQuery, "WHERE wp.barcode = $1"), barcode)
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
	positionRow, junctionRows, _ := mappers.ToDBPosition(data)
	if err := tx.QueryRow(
		ctx,
		g.insertQuery,
		positionRow.Title,
		positionRow.Barcode,
		positionRow.UnitID,
	).Scan(&data.ID); err != nil {
		return err
	}
	for _, junctionRow := range junctionRows {
		if _, err := tx.Exec(ctx, g.insertImageQuery, data.ID, junctionRow.UploadID); err != nil {
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
	positionRow, uploadRows, _ := mappers.ToDBPosition(data)
	if _, err := tx.Exec(
		ctx,
		g.updateQuery,
		positionRow.Title,
		positionRow.Barcode,
		positionRow.UnitID,
		positionRow.ID,
	); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, g.deleteImagesQuery, data.ID); err != nil {
		return err
	}
	for _, uploadRow := range uploadRows {
		if _, err := tx.Exec(ctx, g.insertImageQuery, uploadRow.WarehousePositionID, uploadRow.UploadID); err != nil {
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
	if _, err := tx.Exec(ctx, g.deleteQuery, id); err != nil {
		return err
	}
	return nil
}
