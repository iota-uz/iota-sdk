// Package persistence provides this package.
package persistence

import (
	"context"
	"fmt"

	"github.com/go-faster/errors"
	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/userposition"
	"github.com/iota-uz/iota-sdk/modules/core/infrastructure/persistence/models"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/repo"
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
	tenantID, err := composables.UseTenantID(ctx)
	if err != nil {
		return nil, nil, errors.Wrap(err, "failed to get tenant from context")
	}

	where := []string{"p.tenant_id = $1"}
	args := []interface{}{tenantID.String()}

	for _, filter := range params.Filters {
		column, ok := r.fieldMap[filter.Column]
		if !ok {
			return nil, nil, errors.Wrap(fmt.Errorf("unknown filter field: %v", filter.Column), "invalid filter")
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
	where, args, err := r.buildFilters(ctx, params)
	if err != nil {
		return nil, err
	}

	query := repo.Join(
		userPositionFindQuery,
		repo.JoinWhere(where...),
		params.SortBy.ToSQL(r.fieldMap),
		repo.FormatLimitOffset(params.Limit, params.Offset),
	)

	positions, err := r.queryPositions(ctx, query, args...)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get paginated user positions")
	}
	return positions, nil
}

func (r *PgUserPositionRepository) Count(ctx context.Context, params *userposition.FindParams) (int64, error) {
	tx, err := composables.UseTx(ctx)
	if err != nil {
		return 0, errors.Wrap(err, "failed to get transaction")
	}

	where, args, err := r.buildFilters(ctx, params)
	if err != nil {
		return 0, err
	}

	query := repo.Join(
		userPositionCountQuery,
		repo.JoinWhere(where...),
	)

	var count int64
	if err := tx.QueryRow(ctx, query, args...).Scan(&count); err != nil {
		return 0, errors.Wrap(err, "failed to count user positions")
	}
	return count, nil
}

func (r *PgUserPositionRepository) GetByID(ctx context.Context, id uuid.UUID) (userposition.UserPosition, error) {
	tenantID, err := composables.UseTenantID(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get tenant from context")
	}

	q := repo.Join(userPositionFindQuery, "WHERE p.id = $1 AND p.tenant_id = $2")
	positions, err := r.queryPositions(ctx, q, id.String(), tenantID.String())
	if err != nil {
		return nil, errors.Wrap(err, fmt.Sprintf("failed to query user position with id: %s", id.String()))
	}
	if len(positions) == 0 {
		return nil, errors.Wrap(ErrUserPositionNotFound, fmt.Sprintf("id: %s", id.String()))
	}
	return positions[0], nil
}

func (r *PgUserPositionRepository) Exists(ctx context.Context, id uuid.UUID) (bool, error) {
	tx, err := composables.UseTx(ctx)
	if err != nil {
		return false, errors.Wrap(err, "failed to get transaction")
	}

	tenantID, err := composables.UseTenantID(ctx)
	if err != nil {
		return false, errors.Wrap(err, "failed to get tenant from context")
	}

	var exists bool
	if err := tx.QueryRow(ctx, userPositionExistsQuery, id.String(), tenantID.String()).Scan(&exists); err != nil {
		return false, errors.Wrap(err, "failed to check if user position exists")
	}
	return exists, nil
}

func (r *PgUserPositionRepository) Save(
	ctx context.Context,
	entity userposition.UserPosition,
) (userposition.UserPosition, error) {
	exists, err := r.Exists(ctx, entity.ID())
	if err != nil {
		return nil, errors.Wrap(err, "failed to check if user position exists")
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
	tx, err := composables.UseTx(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get transaction")
	}

	dbPosition, err := ToDBUserPosition(entity)
	if err != nil {
		return nil, errors.Wrap(err, "failed to map user position to db model")
	}
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
		return nil, errors.Wrap(err, "failed to insert user position")
	}

	id, err := uuid.Parse(dbPosition.ID)
	if err != nil {
		return nil, errors.Wrap(err, "failed to parse UUID")
	}
	return r.GetByID(ctx, id)
}

func (r *PgUserPositionRepository) update(
	ctx context.Context,
	entity userposition.UserPosition,
) (userposition.UserPosition, error) {
	tx, err := composables.UseTx(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get transaction")
	}

	dbPosition, err := ToDBUserPosition(entity)
	if err != nil {
		return nil, errors.Wrap(err, "failed to map user position to db model")
	}

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
		return nil, errors.Wrap(err, fmt.Sprintf("failed to update user position with ID: %s", dbPosition.ID))
	}

	id, err := uuid.Parse(dbPosition.ID)
	if err != nil {
		return nil, errors.Wrap(err, "failed to parse UUID")
	}
	return r.GetByID(ctx, id)
}

func (r *PgUserPositionRepository) Delete(ctx context.Context, id uuid.UUID) error {
	tx, err := composables.UseTx(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to get transaction")
	}

	tenantID, err := composables.UseTenantID(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to get tenant from context")
	}

	if _, err := tx.Exec(ctx, userPositionDeleteQuery, id.String(), tenantID.String()); err != nil {
		return errors.Wrap(err, fmt.Sprintf("failed to delete user position with ID: %s", id.String()))
	}
	return nil
}

func (r *PgUserPositionRepository) queryPositions(
	ctx context.Context,
	query string,
	args ...interface{},
) ([]userposition.UserPosition, error) {
	tx, err := composables.UseTx(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get transaction")
	}

	rows, err := tx.Query(ctx, query, args...)
	if err != nil {
		return nil, errors.Wrap(err, "failed to execute query")
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
			return nil, errors.Wrap(err, "failed to scan user position row")
		}
		dbPositions = append(dbPositions, &dbPosition)
	}

	if err := rows.Err(); err != nil {
		return nil, errors.Wrap(err, "row iteration error")
	}

	entities := make([]userposition.UserPosition, 0, len(dbPositions))
	for _, dbPosition := range dbPositions {
		domainPosition, err := ToDomainUserPosition(dbPosition)
		if err != nil {
			return nil, errors.Wrap(err, fmt.Sprintf("failed to convert user position ID: %s to domain entity", dbPosition.ID))
		}
		entities = append(entities, domainPosition)
	}

	return entities, nil
}
