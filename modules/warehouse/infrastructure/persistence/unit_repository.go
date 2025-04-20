package persistence

import (
	"context"
	"errors"

	"github.com/iota-uz/iota-sdk/modules/warehouse/domain/entities/unit"
	"github.com/iota-uz/iota-sdk/modules/warehouse/infrastructure/persistence/mappers"
	"github.com/iota-uz/iota-sdk/modules/warehouse/infrastructure/persistence/models"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/repo"
)

var (
	ErrUnitNotFound = errors.New("unit not found")
)

const (
	selectUnitsQuery = `SELECT id, title, short_title, created_at, updated_at FROM warehouse_units`
	countUnitsQuery  = `SELECT COUNT(*) FROM warehouse_units`
	insertUnitQuery  = `INSERT INTO warehouse_units (title, short_title, created_at) VALUES ($1, $2, $3) RETURNING id`
	updateUnitQuery  = `UPDATE warehouse_units SET title = $1, short_title = $2, updated_at = $3 WHERE id = $4`
	deleteUnitQuery  = `DELETE FROM warehouse_units WHERE id = $1`
)

type GormUnitRepository struct{}

func NewUnitRepository() unit.Repository {
	return &GormUnitRepository{}
}

func (g *GormUnitRepository) GetPaginated(ctx context.Context, params *unit.FindParams) ([]*unit.Unit, error) {
	return g.queryUnits(
		ctx,
		repo.Join(
			selectUnitsQuery,
			repo.FormatLimitOffset(params.Limit, params.Offset),
		),
	)
}

func (g *GormUnitRepository) Count(ctx context.Context) (uint, error) {
	pool, err := composables.UseTx(ctx)
	if err != nil {
		return 0, err
	}
	var count uint
	if err := pool.QueryRow(ctx, countUnitsQuery).Scan(&count); err != nil {
		return 0, err
	}
	return count, nil
}

func (g *GormUnitRepository) GetAll(ctx context.Context) ([]*unit.Unit, error) {
	units, err := g.queryUnits(ctx, selectUnitsQuery)
	if err != nil {
		return nil, err
	}

	return units, nil
}

func (g *GormUnitRepository) GetByID(ctx context.Context, id uint) (*unit.Unit, error) {
	units, err := g.queryUnits(ctx, selectUnitsQuery+" WHERE id = $1", id)
	if err != nil {
		return nil, err
	}

	if len(units) == 0 {
		return nil, ErrUnitNotFound
	}

	return units[0], nil
}

func (g *GormUnitRepository) GetByTitleOrShortTitle(ctx context.Context, name string) (*unit.Unit, error) {
	units, err := g.queryUnits(ctx, selectUnitsQuery+" WHERE title = $1 OR short_title = $1", name)
	if err != nil {
		return nil, err
	}
	if len(units) == 0 {
		return nil, ErrUnitNotFound
	}

	return units[0], nil
}

func (g *GormUnitRepository) Create(ctx context.Context, data *unit.Unit) error {
	tx, err := composables.UseTx(ctx)
	if err != nil {
		return err
	}
	dbRow := mappers.ToDBUnit(data)
	if err := tx.QueryRow(
		ctx,
		insertUnitQuery,
		dbRow.Title,
		dbRow.ShortTitle,
		dbRow.CreatedAt,
	).Scan(&data.ID); err != nil {
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
	tx, err := composables.UseTx(ctx)
	if err != nil {
		return err
	}
	dbRow := mappers.ToDBUnit(data)
	if _, err := tx.Exec(
		ctx,
		updateUnitQuery,
		dbRow.Title,
		dbRow.ShortTitle,
		dbRow.UpdatedAt,
		dbRow.ID,
	); err != nil {
		return err
	}
	return nil
}

func (g *GormUnitRepository) Delete(ctx context.Context, id uint) error {
	tx, err := composables.UseTx(ctx)
	if err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, deleteUnitQuery, id); err != nil {
		return err
	}
	return nil
}

func (g *GormUnitRepository) queryUnits(ctx context.Context, query string, args ...interface{}) ([]*unit.Unit, error) {
	pool, err := composables.UseTx(ctx)
	if err != nil {
		return nil, err
	}
	rows, err := pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	units := make([]*unit.Unit, 0)
	for rows.Next() {
		var u models.WarehouseUnit
		if err := rows.Scan(
			&u.ID,
			&u.Title,
			&u.ShortTitle,
			&u.CreatedAt,
			&u.UpdatedAt,
		); err != nil {
			return nil, err
		}

		domainUnit := mappers.ToDomainUnit(&u)
		units = append(units, domainUnit)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return units, nil
}
