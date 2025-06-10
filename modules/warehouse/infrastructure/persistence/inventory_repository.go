package persistence

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/iota-uz/iota-sdk/pkg/repo"

	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/user"
	"github.com/iota-uz/iota-sdk/modules/warehouse/domain/aggregates/position"
	"github.com/iota-uz/iota-sdk/modules/warehouse/domain/entities/inventory"
	"github.com/iota-uz/iota-sdk/modules/warehouse/infrastructure/persistence/mappers"
	"github.com/iota-uz/iota-sdk/modules/warehouse/infrastructure/persistence/models"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/mapping"
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
	pool, err := composables.UseTx(ctx)
	if err != nil {
		return nil, err
	}

	tenant, err := composables.UseTenantID(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get tenant from context: %w", err)
	}

	where, args := []string{"ic.tenant_id = $1"}, []interface{}{tenant.ID}
	if params.ID != 0 {
		where, args = append(where, fmt.Sprintf("ic.id = $%d", len(args)+1)), append(args, params.ID)
	}

	if params.Status != "" {
		where, args = append(where, fmt.Sprintf("ic.status = $%d", len(args)+1)), append(args, params.Status)
	}

	if params.CreatedAt.To != "" && params.CreatedAt.From != "" {
		where, args = append(where, fmt.Sprintf("ic.created_at BETWEEN $%d and $%d", len(args)+1, len(args)+2)), append(args, params.CreatedAt.From, params.CreatedAt.To)
	}

	if params.Query != "" && params.Field != "" {
		where, args = append(where, fmt.Sprintf("ic.%s::VARCHAR ILIKE $%d", params.Field, len(args)+1)), append(args, "%"+params.Query+"%")
	}

	rows, err := pool.Query(ctx, `
		SELECT ic.id, ic.tenant_id, status, name, ic.created_at, ic.finished_at, ic.created_by_id, ic.finished_by_id
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
			&check.TenantID,
			&check.Status,
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
		domainCheck, err := mappers.ToDomainInventoryCheck(&check)
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

func (g *GormInventoryRepository) Positions(ctx context.Context) ([]*inventory.Position, error) {
	tx, err := composables.UseTx(ctx)
	if err != nil {
		return nil, err
	}

	tenant, err := composables.UseTenantID(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get tenant from context: %w", err)
	}

	var entities []*models.InventoryPosition
	sql := `
	SELECT warehouse_positions.id, warehouse_positions.title, COUNT(warehouse_products.id) quantity, array_agg(warehouse_products.rfid) rfid_tags
	FROM warehouse_positions
	JOIN warehouse_products ON warehouse_positions.id = warehouse_products.position_id
	WHERE warehouse_positions.tenant_id = $1
	GROUP BY warehouse_positions.id;
	`
	rows, err := tx.Query(ctx, sql, tenant.ID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	entities = make([]*models.InventoryPosition, 0)
	for rows.Next() {
		var entity models.InventoryPosition
		if err := rows.Scan(
			&entity.ID,
			&entity.Title,
			&entity.Quantity,
			&entity.RfidTags,
		); err != nil {
			return nil, err
		}
		entities = append(entities, &entity)
	}
	return mapping.MapDBModels(entities, mappers.ToDomainInventoryPosition)
}

func (g *GormInventoryRepository) Count(ctx context.Context) (uint, error) {
	pool, err := composables.UseTx(ctx)
	if err != nil {
		return 0, err
	}

	tenant, err := composables.UseTenantID(ctx)
	if err != nil {
		return 0, fmt.Errorf("failed to get tenant from context: %w", err)
	}

	var count uint
	if err := pool.QueryRow(ctx, `
		SELECT COUNT(*) as count FROM inventory_checks WHERE tenant_id = $1
	`, tenant.ID).Scan(&count); err != nil {
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
	tx, err := composables.UseTx(ctx)
	if err != nil {
		return err
	}

	tenantID, err := composables.UseTenantID(ctx)
	if err != nil {
		return fmt.Errorf("failed to get tenant from context: %w", err)
	}

	// Set tenant ID in domain entity
	data.TenantID = tenantID

	dbRow, err := mappers.ToDBInventoryCheck(data)
	if err != nil {
		return err
	}
	if err := tx.QueryRow(ctx, `
		INSERT INTO inventory_checks (tenant_id, status, name, created_by_id)
		VALUES ($1, $2, $3, $4) RETURNING id
	`, dbRow.TenantID, dbRow.Status, dbRow.Name, dbRow.CreatedByID).Scan(&data.ID); err != nil {
		return err
	}

	if results := dbRow.Results; results != nil {
		for _, result := range results {
			if _, err := tx.Exec(ctx, `
				INSERT INTO inventory_check_results (tenant_id, inventory_check_id, position_id, expected_quantity, actual_quantity, difference) VALUES ($1, $2, $3, $4, $5, $6)
			`, tenantID, data.ID, result.PositionID, result.ExpectedQuantity, result.ActualQuantity, result.Difference); err != nil {
				return err
			}
		}
	}
	return nil
}

func (g *GormInventoryRepository) Update(ctx context.Context, data *inventory.Check) error {
	tx, err := composables.UseTx(ctx)
	if err != nil {
		return err
	}

	tenantID, err := composables.UseTenantID(ctx)
	if err != nil {
		return fmt.Errorf("failed to get tenant from context: %w", err)
	}

	// Set tenant ID in domain entity
	data.TenantID = tenantID

	dbRow, err := mappers.ToDBInventoryCheck(data)
	if err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `
		UPDATE inventory_checks ic SET name = COALESCE(NULLIF($1, ''), ic.name)
		WHERE ic.id = $2 AND ic.tenant_id = $3
	`, dbRow.Name, dbRow.ID, dbRow.TenantID); err != nil {
		return err
	}
	return nil
}

func (g *GormInventoryRepository) Delete(ctx context.Context, id uint) error {
	tx, err := composables.UseTx(ctx)
	if err != nil {
		return err
	}

	tenant, err := composables.UseTenantID(ctx)
	if err != nil {
		return fmt.Errorf("failed to get tenant from context: %w", err)
	}

	if _, err := tx.Exec(ctx, `DELETE FROM inventory_checks WHERE id = $1 AND tenant_id = $2`, id, tenant.ID); err != nil {
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
	pool, err := composables.UseTx(ctx)
	if err != nil {
		return nil, err
	}

	tenant, err := composables.UseTenantID(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get tenant from context: %w", err)
	}

	where, args := []string{"icr.tenant_id = $1"}, []interface{}{tenant.ID}
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
		SELECT id, tenant_id, inventory_check_id, position_id, expected_quantity, actual_quantity, difference, created_at
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
			&result.TenantID,
			&result.InventoryCheckID,
			&result.PositionID,
			&result.ExpectedQuantity,
			&result.ActualQuantity,
			&result.Difference,
			&result.CreatedAt,
		); err != nil {
			return nil, err
		}

		domainResult, err := mappers.ToDomainInventoryCheckResult(&result)
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
