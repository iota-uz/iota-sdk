package persistence

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/position"
	"github.com/iota-uz/iota-sdk/modules/core/infrastructure/persistence/models"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/utils/repo"
)

var (
	ErrPositionNotFound = errors.New("position not found")
)

type GormPositionRepository struct{}

func NewPositionRepository() position.Repository {
	return &GormPositionRepository{}
}

func (g *GormPositionRepository) GetPaginated(
	ctx context.Context, params *position.FindParams,
) ([]*position.Position, error) {
	pool, err := composables.UsePool(ctx)
	if err != nil {
		return nil, err
	}
	where, args := []string{"1 = 1"}, []interface{}{}
	if params.ID != 0 {
		where, args = append(where, fmt.Sprintf("id = $%d", len(args)+1)), append(args, params.ID)
	}

	rows, err := pool.Query(ctx, `
		SELECT id, name, description, created_at, updated_at FROM positions
		WHERE `+strings.Join(where, " AND ")+`
		`+repo.FormatLimitOffset(params.Limit, params.Offset)+`
	`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	positions := make([]*position.Position, 0)
	for rows.Next() {
		var p models.Position
		var description sql.NullString
		if err := rows.Scan(
			&p.ID,
			&p.Name,
			&description,
			&p.CreatedAt,
			&p.UpdatedAt,
		); err != nil {
			return nil, err
		}
		p.Description = description.String
		domainPosition, err := toDomainPosition(&p)
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

func (g *GormPositionRepository) Count(ctx context.Context) (int64, error) {
	pool, err := composables.UsePool(ctx)
	if err != nil {
		return 0, err
	}
	var count int64
	if err := pool.QueryRow(ctx, `
		SELECT COUNT(*) as count FROM positions
	`).Scan(&count); err != nil {
		return 0, err
	}
	return count, nil
}

func (g *GormPositionRepository) GetAll(ctx context.Context) ([]*position.Position, error) {
	return g.GetPaginated(ctx, &position.FindParams{
		Limit: 100000,
	})
}

func (g *GormPositionRepository) GetByID(ctx context.Context, id int64) (*position.Position, error) {
	positions, err := g.GetPaginated(ctx, &position.FindParams{
		ID: id,
	})
	if err != nil {
		return nil, err
	}

	if len(positions) == 0 {
		return nil, ErrPositionNotFound
	}
	return positions[0], nil
}

func (g *GormPositionRepository) Create(ctx context.Context, data *position.Position) error {
	tx, err := composables.UsePoolTx(ctx)
	if err != nil {
		return err
	}
	dbRow := toDBPosition(data)
	if err := tx.QueryRow(ctx, `
		INSERT INTO positions (name, description) VALUES ($1, $2)
	`, dbRow.Name, dbRow.Description).Scan(&data.ID); err != nil {
		return err
	}
	return nil
}

func (g *GormPositionRepository) Update(ctx context.Context, data *position.Position) error {
	tx, err := composables.UsePoolTx(ctx)
	if err != nil {
		return err
	}
	dbRow := toDBPosition(data)
	if _, err := tx.Exec(ctx, `
		UPDATE positions 
		SET name = $1, description = $2
		WHERE id = $3
	`, dbRow.Name, dbRow.Description, dbRow.ID); err != nil {
		return err
	}
	return nil
}

func (g *GormPositionRepository) Delete(ctx context.Context, id int64) error {
	tx, err := composables.UsePoolTx(ctx)
	if err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `
		DELETE FROM positions WHERE id = $1
	`, id); err != nil {
		return err
	}
	return nil
}
