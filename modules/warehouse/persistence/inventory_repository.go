package persistence

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"github.com/iota-agency/iota-sdk/modules/warehouse/domain/aggregates/position"
	"github.com/iota-agency/iota-sdk/modules/warehouse/domain/entities/inventory"
	"github.com/iota-agency/iota-sdk/modules/warehouse/persistence/models"
	"github.com/iota-agency/iota-sdk/pkg/composables"
	"github.com/iota-agency/iota-sdk/pkg/domain/aggregates/user"
	"github.com/iota-agency/iota-sdk/pkg/mapping"
	"github.com/iota-agency/iota-sdk/pkg/utils/repo"
	"strings"
)

var (
	ErrInventoryCheckNotFound = errors.New("inventory check not found")
)

type GormInventoryRepository struct {
	userRepo     user.Repository
	positionRepo position.Repository
}

func NewInventoryRepository(userRepo user.Repository, positionRepo position.Repository) inventory.Repository {
	return &GormInventoryRepository{
		userRepo:     userRepo,
		positionRepo: positionRepo,
	}
}

func (g *GormInventoryRepository) GetPaginated(
	ctx context.Context, params *inventory.FindParams,
) ([]*inventory.Check, error) {
	pool, err := composables.UsePool(ctx)
	if err != nil {
		return nil, err
	}
	where, args := []string{"1 = 1"}, []interface{}{}
	if params.ID != 0 {
		where, args = append(where, fmt.Sprintf("ic.id = $%d", len(args)+1)), append(args, params.ID)
	}

	if params.Status != "" {
		where, args = append(where, fmt.Sprintf("ic.status = $%d", len(args)+1)), append(args, params.Status)
	}

	if params.Type != "" {
		where, args = append(where, fmt.Sprintf("ic.type = $%d", len(args)+1)), append(args, params.Type)
	}

	if params.CreatedAt.To != "" && params.CreatedAt.From != "" {
		where, args = append(where, fmt.Sprintf("ic.created_at BETWEEN $%d and $%d", len(args)+1, len(args)+2)), append(args, params.CreatedAt.From, params.CreatedAt.To)
	}

	if params.Query != "" && params.Field != "" {
		where, args = append(where, fmt.Sprintf("ic.%s::VARCHAR ILIKE $%d", params.Field, len(args)+1)), append(args, "%"+params.Query+"%")
	}

	rows, err := pool.Query(ctx, `
		SELECT ic.id, status, type, name, ic.created_at, ic.finished_at, ic.created_by_id, ic.finished_by_id
		FROM inventory_checks ic
		WHERE `+strings.Join(where, " AND ")+`
		ORDER BY id DESC
		`+repo.FormatLimitOffset(params.Limit, params.Offset),
		args...,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	checks := make([]*inventory.Check, 0)
	for rows.Next() {
		var check models.InventoryCheck
		var finishedAt sql.NullTime
		var finishedByID sql.NullInt32
		if err := rows.Scan(
			&check.ID,
			&check.Status,
			&check.Type,
			&check.Name,
			&check.CreatedAt,
			&finishedAt,
			&check.CreatedByID,
			&finishedByID,
		); err != nil {
			return nil, err
		}

		if finishedAt.Valid {
			check.FinishedAt = &finishedAt.Time
		}

		if finishedByID.Valid {
			check.FinishedByID = mapping.Pointer(uint(finishedByID.Int32))
		}
		domainCheck, err := toDomainInventoryCheck(&check)
		if err != nil {
			return nil, err
		}
		if domainCheck.CreatedBy, err = g.userRepo.GetByID(ctx, domainCheck.CreatedByID); err != nil {
			return nil, err
		}
		if domainCheck.FinishedByID != 0 {
			if domainCheck.FinishedBy, err = g.userRepo.GetByID(ctx, domainCheck.FinishedByID); err != nil {
				return nil, err
			}
		}

		if params.AttachResults {
			if domainCheck.Results, err = g.getCheckResults(ctx, &findCheckResultsParams{
				checkID:        domainCheck.ID,
				attachPosition: true,
				withDifference: params.WithDifference,
			}); err != nil {
				return nil, err
			}
		}
		checks = append(checks, domainCheck)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return checks, nil
}

func (g *GormInventoryRepository) Count(ctx context.Context) (uint, error) {
	pool, err := composables.UsePool(ctx)
	if err != nil {
		return 0, err
	}
	var count uint
	if err := pool.QueryRow(ctx, `
		SELECT COUNT(*) as count FROM inventory_checks
	`).Scan(&count); err != nil {
		return 0, err
	}
	return count, nil
}

func (g *GormInventoryRepository) GetAll(ctx context.Context) ([]*inventory.Check, error) {
	checks, err := g.GetPaginated(ctx, &inventory.FindParams{
		Limit: 100000,
	})
	if err != nil {
		return nil, err
	}
	return checks, nil
}

func (g *GormInventoryRepository) GetByID(ctx context.Context, id uint) (*inventory.Check, error) {
	checks, err := g.GetPaginated(ctx, &inventory.FindParams{
		ID:            id,
		AttachResults: true,
	})
	if err != nil {
		return nil, err
	}
	if len(checks) == 0 {
		return nil, ErrInventoryCheckNotFound
	}
	return checks[0], nil
}

func (g *GormInventoryRepository) GetByIDWithDifference(ctx context.Context, id uint) (*inventory.Check, error) {
	checks, err := g.GetPaginated(ctx, &inventory.FindParams{
		ID:             id,
		WithDifference: true,
		AttachResults:  true,
	})
	if err != nil {
		return nil, err
	}

	if len(checks) == 0 {
		return nil, ErrInventoryCheckNotFound
	}
	return checks[0], nil
}

func (g *GormInventoryRepository) Create(ctx context.Context, data *inventory.Check) error {
	tx, err := composables.UsePoolTx(ctx)
	if err != nil {
		return err
	}
	dbRow, err := toDBInventoryCheck(data)
	if err != nil {
		return err
	}
	if err := tx.QueryRow(ctx, `
		INSERT INTO inventory_checks (status, name, type, created_by_id) 
		VALUES ($1, $2, $3, $4) RETURNING id
	`, dbRow.Status, dbRow.Name, dbRow.Type, dbRow.CreatedByID).Scan(&data.ID); err != nil {
		return err
	}

	if results := dbRow.Results; results != nil {
		for _, result := range results {
			if _, err := tx.Exec(ctx, `
				INSERT INTO inventory_check_results (inventory_check_id, position_id, expected_quantity, actual_quantity, difference) VALUES ($1, $2, $3, $4, $5)
			`, data.ID, result.PositionID, result.ExpectedQuantity, result.ActualQuantity, result.Difference); err != nil {
				return err
			}
		}
	}
	return nil
}

func (g *GormInventoryRepository) Update(ctx context.Context, data *inventory.Check) error {
	tx, err := composables.UsePoolTx(ctx)
	if err != nil {
		return err
	}
	dbRow, err := toDBInventoryCheck(data)
	if err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `
		UPDATE inventory_checks ic SET name = COALESCE(NULLIF($1, ''), ic.name)
		WHERE ic.id = $2
	`, dbRow.Name, dbRow.ID); err != nil {
		return err
	}
	return nil
}

func (g *GormInventoryRepository) Delete(ctx context.Context, id uint) error {
	tx, err := composables.UsePoolTx(ctx)
	if err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `DELETE FROM inventory_checks WHERE id = $1`, id); err != nil {
		return err
	}
	return nil
}

type findCheckResultsParams struct {
	id             uint
	checkID        uint
	attachPosition bool
	withDifference bool
}

func (g *GormInventoryRepository) getCheckResults(
	ctx context.Context, params *findCheckResultsParams,
) ([]*inventory.CheckResult, error) {
	pool, err := composables.UsePool(ctx)
	if err != nil {
		return nil, err
	}
	where, args := []string{"1 = 1"}, []interface{}{}
	if params.id != 0 {
		where, args = append(where, fmt.Sprintf("ic.id = $%d", len(args)+1)), append(args, params.id)
	}

	if params.checkID != 0 {
		where, args = append(where, fmt.Sprintf("icr.inventory_check_id = $%d", len(args)+1)), append(args, params.checkID)
	}

	if params.withDifference {
		where = append(where, "icr.expected_quantity != icr.actual_quantity")
	}

	rows, err := pool.Query(ctx, `
		SELECT id, inventory_check_id, position_id, expected_quantity, actual_quantity, difference, created_at
		FROM inventory_check_results icr
		WHERE `+strings.Join(where, " AND ")+`
		ORDER BY id DESC`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	results := make([]*inventory.CheckResult, 0)
	for rows.Next() {
		var result models.InventoryCheckResult
		if err := rows.Scan(
			&result.ID,
			&result.InventoryCheckID,
			&result.PositionID,
			&result.ExpectedQuantity,
			&result.ActualQuantity,
			&result.Difference,
			&result.CreatedAt,
		); err != nil {
			return nil, err
		}

		domainResult, err := toDomainInventoryCheckResult(&result)
		if err != nil {
			return nil, err
		}
		if params.attachPosition {
			if domainResult.Position, err = g.positionRepo.GetByID(ctx, result.PositionID); err != nil {
				return nil, err
			}
		}
		results = append(results, domainResult)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return results, nil
}
