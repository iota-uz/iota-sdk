package persistence

import (
	"context"
	"errors"
	"fmt"
	"github.com/iota-uz/iota-sdk/modules/warehouse/infrastructure/persistence/mappers"
	"github.com/iota-uz/iota-sdk/modules/warehouse/infrastructure/persistence/models"
	"github.com/iota-uz/iota-sdk/pkg/utils/repo"
	"strings"

	"github.com/iota-uz/iota-sdk/modules/warehouse/domain/entities/unit"
	"github.com/iota-uz/iota-sdk/pkg/composables"
)

var (
	ErrUnitNotFound = errors.New("unit not found")
)

type GormUnitRepository struct{}

func NewUnitRepository() unit.Repository {
	return &GormUnitRepository{}
}

func (g *GormUnitRepository) GetPaginated(
	ctx context.Context, params *unit.FindParams,
) ([]*unit.Unit, error) {
	pool, err := composables.UsePool(ctx)
	if err != nil {
		return nil, err
	}
	where, args := []string{"1 = 1"}, []interface{}{}
	if params.ID != 0 {
		where, args = append(where, fmt.Sprintf("id = $%d", len(args)+1)), append(args, params.ID)
	}

	if params.Title != "" {
		where, args = append(where, fmt.Sprintf("title = $%d OR short_title = $%d", len(args)+1, len(args)+2)), append(args, params.Title, params.Title)
	}

	rows, err := pool.Query(ctx, `
		SELECT id, title, short_title, created_at, updated_at FROM warehouse_units
		WHERE `+strings.Join(where, " AND ")+`
		`+repo.FormatLimitOffset(params.Limit, params.Offset)+`
	`, args...)

	if err != nil {
		return nil, err
	}
	defer rows.Close()

	units := make([]*unit.Unit, 0)
	for rows.Next() {
		var unit models.WarehouseUnit
		if err := rows.Scan(
			&unit.ID,
			&unit.Title,
			&unit.ShortTitle,
			&unit.CreatedAt,
			&unit.UpdatedAt,
		); err != nil {
			return nil, err
		}

		domainUnit := mappers.ToDomainUnit(&unit)
		units = append(units, domainUnit)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return units, nil
}

func (g *GormUnitRepository) Count(ctx context.Context) (uint, error) {
	pool, err := composables.UsePool(ctx)
	if err != nil {
		return 0, err
	}
	var count uint
	if err := pool.QueryRow(ctx, `
		SELECT COUNT(*) as count FROM warehouse_units
	`).Scan(&count); err != nil {
		return 0, err
	}
	return count, nil
}

func (g *GormUnitRepository) GetAll(ctx context.Context) ([]*unit.Unit, error) {
	units, err := g.GetPaginated(ctx, &unit.FindParams{
		Limit: 100000,
	})
	if err != nil {
		return nil, err
	}

	return units, nil
}

func (g *GormUnitRepository) GetByID(ctx context.Context, id uint) (*unit.Unit, error) {
	units, err := g.GetPaginated(ctx, &unit.FindParams{
		ID: id,
	})
	if err != nil {
		return nil, err
	}

	if len(units) == 0 {
		return nil, ErrUnitNotFound
	}

	return units[0], nil
}

func (g *GormUnitRepository) GetByTitleOrShortTitle(ctx context.Context, name string) (*unit.Unit, error) {
	units, err := g.GetPaginated(ctx, &unit.FindParams{
		Title: name,
	})
	if err != nil {
		return nil, err
	}
	if len(units) == 0 {
		return nil, ErrUnitNotFound
	}

	return units[0], nil
}

func (g *GormUnitRepository) Create(ctx context.Context, data *unit.Unit) error {
	tx, err := composables.UsePoolTx(ctx)
	if err != nil {
		return err
	}
	dbRow := mappers.ToDBUnit(data)
	if err := tx.QueryRow(ctx, `
		INSERT INTO warehouse_units (title, short_title)
		VALUES ($1, $2) RETURNING id
	`, dbRow.Title, dbRow.ShortTitle).Scan(&data.ID); err != nil {
		return err
	}
	return nil
}

func (g *GormUnitRepository) CreateOrUpdate(ctx context.Context, data *unit.Unit) error {
	u, err := g.GetByID(ctx, data.ID)
	if err != nil && !errors.Is(err, ErrUnitNotFound) {
		return err
	}
	if u != nil {
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

func (g *GormUnitRepository) Update(ctx context.Context, data *unit.Unit) error {
	tx, err := composables.UsePoolTx(ctx)
	if err != nil {
		return err
	}
	dbRow := mappers.ToDBUnit(data)
	if _, err := tx.Exec(ctx, `
		UPDATE warehouse_units wu 
		SET 
		title = COALESCE(NULLIF($1, ''), wu.title),
		short_title = COALESCE(NULLIF($2, ''), wu.short_title)
		WHERE wu.id = $3
	`, dbRow.Title, dbRow.ShortTitle, dbRow.ID); err != nil {
		return err
	}
	return nil
}

func (g *GormUnitRepository) Delete(ctx context.Context, id uint) error {
	tx, err := composables.UsePoolTx(ctx)
	if err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `DELETE FROM warehouse_units where id = $1`, id); err != nil {
		return err
	}
	return nil
}
