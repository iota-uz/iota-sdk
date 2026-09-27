// Package persistence provides this package.
package persistence

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/userposition"
	"github.com/iota-uz/iota-sdk/modules/core/infrastructure/persistence/models"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/repo"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
)

var (
	ErrUserPositionNotFound = errors.New("user position not found")
)

const (
	userPositionFindQuery = `
		SELECT
			p.id,
			p.tenant_id,
			p.user_id,
			p.department_id,
			p.title,
			p.is_manager,
			p.is_primary,
			p.status,
			p.created_at,
			p.updated_at
		FROM core.user_positions p`

	userPositionCountQuery  = `SELECT COUNT(p.id) FROM core.user_positions p`
	userPositionExistsQuery = `SELECT EXISTS(SELECT 1 FROM core.user_positions WHERE id = $1 AND tenant_id = $2)`
	userPositionDeleteQuery = `DELETE FROM core.user_positions WHERE id = $1 AND tenant_id = $2`
)

type PgUserPositionRepository struct {
	fieldMap map[userposition.Field]string
}

func NewUserPositionRepository() userposition.Repository {
	return &PgUserPositionRepository{
		fieldMap: map[userposition.Field]string{
			userposition.CreatedAtField:    "p.created_at",
			userposition.UpdatedAtField:    "p.updated_at",
			userposition.TenantIDField:     "p.tenant_id",
			userposition.UserIDField:       "p.user_id",
			userposition.DepartmentIDField: "p.department_id",
			userposition.IsManagerField:    "p.is_manager",
			userposition.IsPrimaryField:    "p.is_primary",
			userposition.StatusField:       "p.status",
		},
	}
}

func (r *PgUserPositionRepository) buildFilters(
	ctx context.Context,
	params *userposition.FindParams,
) ([]string, []interface{}, error) {
	const op serrors.Op = "PgUserPositionRepository.buildFilters"
	tenantID, err := composables.UseTenantID(ctx)
	if err != nil {
		return nil, nil, serrors.E(op, err)
	}

	where := []string{"p.tenant_id = $1"}
	args := []interface{}{tenantID.String()}

	for _, filter := range params.Filters {
		column, ok := r.fieldMap[filter.Column]
		if !ok {
			return nil, nil, serrors.E(op, fmt.Errorf("unknown filter field: %v", filter.Column))
		}
		where = append(where, filter.Filter.String(column, len(args)+1))
		args = append(args, filter.Filter.Value()...)
	}

	if params.Search != "" {
		index := len(args) + 1
		where = append(where, fmt.Sprintf("(p.title::text ILIKE $%d)", index))
		args = append(args, "%"+params.Search+"%")
	}

	return where, args, nil
}

func (r *PgUserPositionRepository) GetPaginated(
	ctx context.Context,
	params *userposition.FindParams,
) ([]userposition.UserPosition, error) {
	const op serrors.Op = "PgUserPositionRepository.GetPaginated"
	if params == nil {
		params = &userposition.FindParams{}
	}

	where, args, err := r.buildFilters(ctx, params)
	if err != nil {
		return nil, serrors.E(op, err)
	}

	query := repo.Join(
		userPositionFindQuery,
		repo.JoinWhere(where...),
		params.SortBy.ToSQL(r.fieldMap),
		repo.FormatLimitOffset(params.Limit, params.Offset),
	)

	positions, err := r.queryPositions(ctx, query, args...)
	if err != nil {
		return nil, serrors.E(op, err)
	}
	return positions, nil
}

func (r *PgUserPositionRepository) Count(ctx context.Context, params *userposition.FindParams) (int64, error) {
	const op serrors.Op = "PgUserPositionRepository.Count"
	if params == nil {
		params = &userposition.FindParams{}
	}

	tx, err := composables.UseTx(ctx)
	if err != nil {
		return 0, serrors.E(op, err)
	}

	where, args, err := r.buildFilters(ctx, params)
	if err != nil {
		return 0, serrors.E(op, err)
	}

	query := repo.Join(
		userPositionCountQuery,
		repo.JoinWhere(where...),
	)

	var count int64
	if err := tx.QueryRow(ctx, query, args...).Scan(&count); err != nil {
		return 0, serrors.E(op, err)
	}
	return count, nil
}

func (r *PgUserPositionRepository) GetByID(ctx context.Context, id uuid.UUID) (userposition.UserPosition, error) {
	const op serrors.Op = "PgUserPositionRepository.GetByID"
	tenantID, err := composables.UseTenantID(ctx)
	if err != nil {
		return nil, serrors.E(op, err)
	}

	q := repo.Join(userPositionFindQuery, "WHERE p.id = $1 AND p.tenant_id = $2")
	positions, err := r.queryPositions(ctx, q, id.String(), tenantID.String())
	if err != nil {
		return nil, serrors.E(op, err)
	}
	if len(positions) == 0 {
		return nil, serrors.E(op, serrors.NotFound, ErrUserPositionNotFound)
	}
	return positions[0], nil
}

func (r *PgUserPositionRepository) Exists(ctx context.Context, id uuid.UUID) (bool, error) {
	const op serrors.Op = "PgUserPositionRepository.Exists"
	tx, err := composables.UseTx(ctx)
	if err != nil {
		return false, serrors.E(op, err)
	}

	tenantID, err := composables.UseTenantID(ctx)
	if err != nil {
		return false, serrors.E(op, err)
	}

	var exists bool
	if err := tx.QueryRow(ctx, userPositionExistsQuery, id.String(), tenantID.String()).Scan(&exists); err != nil {
		return false, serrors.E(op, err)
	}
	return exists, nil
}

func (r *PgUserPositionRepository) Save(
	ctx context.Context,
	entity userposition.UserPosition,
) (userposition.UserPosition, error) {
	const op serrors.Op = "PgUserPositionRepository.Save"
	exists, err := r.Exists(ctx, entity.ID())
	if err != nil {
		return nil, serrors.E(op, err)
	}

	if exists {
		return r.update(ctx, entity)
	}
	return r.create(ctx, entity)
}

func (r *PgUserPositionRepository) create(
	ctx context.Context,
	entity userposition.UserPosition,
) (userposition.UserPosition, error) {
	const op serrors.Op = "PgUserPositionRepository.create"
	tx, err := composables.UseTx(ctx)
	if err != nil {
		return nil, serrors.E(op, err)
	}

	// Tenant ownership comes from the request context, never the entity
	// payload, so a mismatched-entity tenant cannot insert into another tenant.
	tenantID, err := composables.UseTenantID(ctx)
	if err != nil {
		return nil, serrors.E(op, err)
	}

	dbPosition, err := ToDBUserPosition(entity)
	if err != nil {
		return nil, serrors.E(op, err)
	}
	dbPosition.TenantID = tenantID.String()
	if entity.ID() == uuid.Nil {
		dbPosition.ID = uuid.New().String()
	}

	fields := []string{
		"id",
		"tenant_id",
		"user_id",
		"department_id",
		"title",
		"is_manager",
		"is_primary",
		"status",
		"created_at",
		"updated_at",
	}

	values := []interface{}{
		dbPosition.ID,
		dbPosition.TenantID,
		dbPosition.UserID,
		dbPosition.DepartmentID,
		dbPosition.Title,
		dbPosition.IsManager,
		dbPosition.IsPrimary,
		dbPosition.Status,
		dbPosition.CreatedAt,
		dbPosition.UpdatedAt,
	}

	if _, err := tx.Exec(ctx, repo.Insert("core.user_positions", fields), values...); err != nil {
		return nil, serrors.E(op, err)
	}

	id, err := uuid.Parse(dbPosition.ID)
	if err != nil {
		return nil, serrors.E(op, err)
	}
	return r.GetByID(ctx, id)
}

func (r *PgUserPositionRepository) update(
	ctx context.Context,
	entity userposition.UserPosition,
) (userposition.UserPosition, error) {
	const op serrors.Op = "PgUserPositionRepository.update"
	tx, err := composables.UseTx(ctx)
	if err != nil {
		return nil, serrors.E(op, err)
	}

	// Tenant ownership comes from the request context, never the entity
	// payload, so the update can only ever target the caller's own tenant row.
	tenantID, err := composables.UseTenantID(ctx)
	if err != nil {
		return nil, serrors.E(op, err)
	}

	dbPosition, err := ToDBUserPosition(entity)
	if err != nil {
		return nil, serrors.E(op, err)
	}
	dbPosition.TenantID = tenantID.String()

	fields := []string{
		"department_id",
		"title",
		"is_manager",
		"is_primary",
		"status",
		"updated_at",
	}

	values := []interface{}{
		dbPosition.DepartmentID,
		dbPosition.Title,
		dbPosition.IsManager,
		dbPosition.IsPrimary,
		dbPosition.Status,
		dbPosition.UpdatedAt,
		dbPosition.ID,
		dbPosition.TenantID,
	}

	query := repo.Update(
		"core.user_positions",
		fields,
		fmt.Sprintf("id = $%d", len(values)-1),
		fmt.Sprintf("tenant_id = $%d", len(values)),
	)
	if _, err := tx.Exec(ctx, query, values...); err != nil {
		return nil, serrors.E(op, err)
	}

	id, err := uuid.Parse(dbPosition.ID)
	if err != nil {
		return nil, serrors.E(op, err)
	}
	return r.GetByID(ctx, id)
}

func (r *PgUserPositionRepository) Delete(ctx context.Context, id uuid.UUID) error {
	const op serrors.Op = "PgUserPositionRepository.Delete"
	tx, err := composables.UseTx(ctx)
	if err != nil {
		return serrors.E(op, err)
	}

	tenantID, err := composables.UseTenantID(ctx)
	if err != nil {
		return serrors.E(op, err)
	}

	tag, err := tx.Exec(ctx, userPositionDeleteQuery, id.String(), tenantID.String())
	if err != nil {
		return serrors.E(op, err)
	}
	if tag.RowsAffected() == 0 {
		return serrors.E(op, serrors.NotFound, ErrUserPositionNotFound)
	}
	return nil
}

func (r *PgUserPositionRepository) queryPositions(
	ctx context.Context,
	query string,
	args ...interface{},
) ([]userposition.UserPosition, error) {
	const op serrors.Op = "PgUserPositionRepository.queryPositions"
	tx, err := composables.UseTx(ctx)
	if err != nil {
		return nil, serrors.E(op, err)
	}

	rows, err := tx.Query(ctx, query, args...)
	if err != nil {
		return nil, serrors.E(op, err)
	}
	defer rows.Close()

	var dbPositions []*models.UserPosition
	for rows.Next() {
		var dbPosition models.UserPosition
		if err := rows.Scan(
			&dbPosition.ID,
			&dbPosition.TenantID,
			&dbPosition.UserID,
			&dbPosition.DepartmentID,
			&dbPosition.Title,
			&dbPosition.IsManager,
			&dbPosition.IsPrimary,
			&dbPosition.Status,
			&dbPosition.CreatedAt,
			&dbPosition.UpdatedAt,
		); err != nil {
			return nil, serrors.E(op, err)
		}
		dbPositions = append(dbPositions, &dbPosition)
	}

	if err := rows.Err(); err != nil {
		return nil, serrors.E(op, err)
	}

	entities := make([]userposition.UserPosition, 0, len(dbPositions))
	for _, dbPosition := range dbPositions {
		domainPosition, err := ToDomainUserPosition(dbPosition)
		if err != nil {
			return nil, serrors.E(op, err)
		}
		entities = append(entities, domainPosition)
	}

	return entities, nil
}
