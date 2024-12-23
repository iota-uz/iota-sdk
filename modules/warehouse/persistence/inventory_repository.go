package persistence

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	coremodels "github.com/iota-agency/iota-sdk/modules/core/infrastructure/persistence/models"
	"github.com/iota-agency/iota-sdk/modules/warehouse/domain/entities/inventory"
	"github.com/iota-agency/iota-sdk/modules/warehouse/persistence/models"
	"github.com/iota-agency/iota-sdk/pkg/composables"
	"github.com/iota-agency/iota-sdk/pkg/mapping"
)

type GormInventoryRepository struct{}

func NewInventoryRepository() inventory.Repository {
	return &GormInventoryRepository{}
}

func FormatLimitOffset(limit, offset int) string {
	if limit > 0 && offset > 0 {
		return fmt.Sprintf("LIMIT %d OFFSET %d", limit, offset)
	} else if limit > 0 {
		return fmt.Sprintf("LIMIT %d", limit)
	} else if offset > 0 {
		return fmt.Sprintf("OFFSET %d", offset)
	}
	return ""
}

func (g *GormInventoryRepository) GetPaginated(
	ctx context.Context, params *inventory.FindParams,
) ([]*inventory.Check, error) {
	pool, err := composables.UsePool(ctx)
	if err != nil {
		return nil, err
	}
	where, args := []string{"1 = 1"}, []interface{}{}

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
		SELECT ic.id, status, type, name, ic.created_at, ic.finished_at, ic.created_by_id, ic.finished_by_id, u.id as user_id, u.first_name, u.last_name, u.middle_name
		FROM inventory_checks ic
		INNER JOIN users u ON u.id = created_by_id 
		WHERE `+strings.Join(where, " AND ")+`
		ORDER BY id DESC
		`+FormatLimitOffset(params.Limit, params.Offset),
		args...,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	checks := make([]*models.InventoryCheck, 0)
	for rows.Next() {
		var check models.InventoryCheck
		var finishedAt sql.NullTime
		var finishedByID sql.NullInt32
		var middleName sql.NullString
		var createdBy coremodels.User
		if err := rows.Scan(
			&check.ID,
			&check.Status,
			&check.Type,
			&check.Name,
			&check.CreatedAt,
			&finishedAt,
			&check.CreatedByID,
			&finishedByID,
			&createdBy.ID,
			&createdBy.FirstName,
			&createdBy.LastName,
			&middleName,
		); err != nil {
			return nil, err
		}

		if middleName.Valid {
			createdBy.MiddleName = &middleName.String
		}

		if finishedAt.Valid {
			check.FinishedAt = &finishedAt.Time
		}

		if finishedByID.Valid {
			val := uint(finishedByID.Int32)
			check.FinishedByID = &val
		}

		check.CreatedBy = &createdBy

		checks = append(checks, &check)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return mapping.MapDbModels(checks, toDomainInventoryCheck)
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
	pool, err := composables.UsePool(ctx)
	rows, err := pool.Query(ctx, `
		SELECT ic.id, status, type, name, ic.created_at, ic.finished_at, ic.created_by_id, ic.finished_by_id, u.id as user_id, u.first_name, u.last_name, u.middle_name
		FROM inventory_checks ic
		INNER JOIN users u ON u.id = created_by_id 
		ORDER BY id DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	checks := make([]*models.InventoryCheck, 0)
	for rows.Next() {
		var check models.InventoryCheck
		var finishedAt sql.NullTime
		var finishedByID sql.NullInt32
		var middleName sql.NullString
		var createdBy coremodels.User
		if err := rows.Scan(
			&check.ID,
			&check.Status,
			&check.Type,
			&check.Name,
			&check.CreatedAt,
			&finishedAt,
			&check.CreatedByID,
			&finishedByID,
			&createdBy.ID,
			&createdBy.FirstName,
			&createdBy.LastName,
			&middleName,
		); err != nil {
			return nil, err
		}

		if middleName.Valid {
			createdBy.MiddleName = &middleName.String
		}

		if finishedAt.Valid {
			check.FinishedAt = &finishedAt.Time
		}

		if finishedByID.Valid {
			val := uint(finishedByID.Int32)
			check.FinishedByID = &val
		}

		check.CreatedBy = &createdBy

		checks = append(checks, &check)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return mapping.MapDbModels(checks, toDomainInventoryCheck)
}

func (g *GormInventoryRepository) GetByID(ctx context.Context, id uint) (*inventory.Check, error) {
	pool, err := composables.UsePool(ctx)
	if err != nil {
		return nil, err
	}
	rows, err := pool.Query(ctx, `
		SELECT ic.id, status, type, name, ic.created_at, ic.finished_at, ic.created_by_id, ic.finished_by_id, u.id as user_id, u.first_name, u.last_name, u.middle_name,
		icr.id, icr.position_id, icr.inventory_check_id, icr.expected_quantity, icr.actual_quantity, icr.difference, icr.created_at,
		wp.id, wp.title, wp.barcode,
		wu.title, wu.short_title
		FROM inventory_checks ic
		INNER JOIN users u ON u.id = created_by_id 
		INNER JOIN inventory_check_results icr ON icr.inventory_check_id = ic.id
		INNER JOIN warehouse_positions wp ON wp.id = icr.position_id
		INNER JOIN warehouse_units wu ON wu.id = wp.unit_id
		WHERE ic.id = $1
	`, id)
	if err != nil {
		fmt.Println("ERR: ", err)
		return nil, err
	}

	check := models.InventoryCheck{
		Results: make([]*models.InventoryCheckResult, 0),
	}

	for rows.Next() {
		var finishedAt sql.NullTime
		var finishedByID sql.NullInt32
		var middleName sql.NullString
		var createdBy coremodels.User
		result := models.InventoryCheckResult{
			Position: &models.WarehousePosition{
				Unit: &models.WarehouseUnit{},
			},
		}

		if err := rows.Scan(
			&check.ID,
			&check.Status,
			&check.Type,
			&check.Name,
			&check.CreatedAt,
			&finishedAt,
			&check.CreatedByID,
			&finishedByID,
			&createdBy.ID,
			&createdBy.FirstName,
			&createdBy.LastName,
			&middleName,
			&result.ID,
			&result.PositionID,
			&result.InventoryCheckID,
			&result.ExpectedQuantity,
			&result.ActualQuantity,
			&result.Difference,
			&result.CreatedAt,
			&result.Position.ID,
			&result.Position.Title,
			&result.Position.Barcode,
			&result.Position.Unit.Title,
			&result.Position.Unit.ShortTitle,
		); err != nil {
			fmt.Println("ERR: ", err)
			return nil, err
		}

		if middleName.Valid {
			createdBy.MiddleName = &middleName.String
		}

		if finishedAt.Valid {
			check.FinishedAt = &finishedAt.Time
		}

		if finishedByID.Valid {
			val := uint(finishedByID.Int32)
			check.FinishedByID = &val
		}
		check.CreatedBy = &createdBy
		check.Results = append(check.Results, &result)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return toDomainInventoryCheck(&check)
}

func (g *GormInventoryRepository) GetByIDWithDifference(ctx context.Context, id uint) (*inventory.Check, error) {
	pool, err := composables.UsePool(ctx)
	if err != nil {
		return nil, err
	}
	rows, err := pool.Query(ctx, `
		SELECT ic.id, status, type, name, ic.created_at, ic.finished_at, ic.created_by_id, ic.finished_by_id, u.id as user_id, u.first_name, u.last_name, u.middle_name,
		icr.id, icr.position_id, icr.inventory_check_id, icr.expected_quantity, icr.actual_quantity, icr.difference, icr.created_at,
		wp.id, wp.title, wp.barcode,
		wu.title, wu.short_title
		FROM inventory_checks ic
		INNER JOIN users u ON u.id = created_by_id 
		INNER JOIN inventory_check_results icr ON icr.inventory_check_id = ic.id AND icr.actual_quantity != icr.expected_quantity
		INNER JOIN warehouse_positions wp ON wp.id = icr.position_id
		INNER JOIN warehouse_units wu ON wu.id = wp.unit_id
		WHERE ic.id = $1
	`, id)
	if err != nil {
		return nil, err
	}

	check := models.InventoryCheck{
		Results: make([]*models.InventoryCheckResult, 0),
	}

	for rows.Next() {
		var finishedAt sql.NullTime
		var finishedByID sql.NullInt32
		var middleName sql.NullString
		var createdBy coremodels.User
		result := models.InventoryCheckResult{
			Position: &models.WarehousePosition{
				Unit: &models.WarehouseUnit{},
			},
		}

		if err := rows.Scan(
			&check.ID,
			&check.Status,
			&check.Type,
			&check.Name,
			&check.CreatedAt,
			&finishedAt,
			&check.CreatedByID,
			&finishedByID,
			&createdBy.ID,
			&createdBy.FirstName,
			&createdBy.LastName,
			&middleName,
			&result.ID,
			&result.PositionID,
			&result.InventoryCheckID,
			&result.ExpectedQuantity,
			&result.ActualQuantity,
			&result.Difference,
			&result.CreatedAt,
			&result.Position.ID,
			&result.Position.Title,
			&result.Position.Barcode,
			&result.Position.Unit.Title,
			&result.Position.Unit.ShortTitle,
		); err != nil {
			return nil, err
		}

		if middleName.Valid {
			createdBy.MiddleName = &middleName.String
		}

		if finishedAt.Valid {
			check.FinishedAt = &finishedAt.Time
		}

		if finishedByID.Valid {
			val := uint(finishedByID.Int32)
			check.FinishedByID = &val
		}
		check.CreatedBy = &createdBy
		check.Results = append(check.Results, &result)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return toDomainInventoryCheck(&check)
}

func (g *GormInventoryRepository) Create(ctx context.Context, data *inventory.Check) error {
	tx, ok := composables.UsePoolTx(ctx)
	if !ok {
		return composables.ErrNoTx
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
	return tx.Commit(ctx)
}

func (g *GormInventoryRepository) Update(ctx context.Context, data *inventory.Check) error {
	tx, ok := composables.UsePoolTx(ctx)
	if !ok {
		return composables.ErrNoTx
	}
	dbRow, err := toDBInventoryCheck(data)
	if err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `
		UPDATE inventory_checks ic SET name = COALESCE(NULLIF($1, ''), ic.name)
	`, dbRow.Name); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (g *GormInventoryRepository) Delete(ctx context.Context, id uint) error {
	tx, ok := composables.UsePoolTx(ctx)
	if !ok {
		return composables.ErrNoTx
	}
	if _, err := tx.Exec(ctx, `DELETE FROM inventory_checks WHERE id = $1`, id); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
