// Package persistence provides this package.
package persistence

import (
	"context"
	"fmt"

	"github.com/go-faster/errors"
	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/department"
	"github.com/iota-uz/iota-sdk/modules/core/infrastructure/persistence/models"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/repo"
)

var (
	ErrDepartmentNotFound = errors.New("department not found")
)

const (
	departmentFindQuery = `
		SELECT
			d.id,
			d.tenant_id,
			d.parent_id,
			d.code,
			d.name,
			d."order",
			d.status,
			d.created_at,
			d.updated_at
		FROM core.departments d`

	departmentCountQuery  = `SELECT COUNT(d.id) FROM core.departments d`
	departmentExistsQuery = `SELECT EXISTS(SELECT 1 FROM core.departments WHERE id = $1 AND tenant_id = $2)`
	departmentDeleteQuery = `DELETE FROM core.departments WHERE id = $1 AND tenant_id = $2`
)

type PgDepartmentRepository struct {
	fieldMap map[department.Field]string
}

func NewDepartmentRepository() department.Repository {
	return &PgDepartmentRepository{
		fieldMap: map[department.Field]string{
			department.CreatedAtField: "d.created_at",
			department.UpdatedAtField: "d.updated_at",
			department.TenantIDField:  "d.tenant_id",
			department.CodeField:      "d.code",
			department.ParentIDField:  "d.parent_id",
			department.OrderField:     `d."order"`,
			department.StatusField:    "d.status",
		},
	}
}

func (r *PgDepartmentRepository) buildFilters(
	ctx context.Context,
	params *department.FindParams,
) ([]string, []interface{}, error) {
	tenantID, err := composables.UseTenantID(ctx)
	if err != nil {
		return nil, nil, errors.Wrap(err, "failed to get tenant from context")
	}

	where := []string{"d.tenant_id = $1"}
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
		where = append(where, fmt.Sprintf("(d.code ILIKE $%d OR d.name::text ILIKE $%d)", index, index))
		args = append(args, "%"+params.Search+"%")
	}

	return where, args, nil
}

func (r *PgDepartmentRepository) GetPaginated(
	ctx context.Context,
	params *department.FindParams,
) ([]department.Department, error) {
	where, args, err := r.buildFilters(ctx, params)
	if err != nil {
		return nil, err
	}

	query := repo.Join(
		departmentFindQuery,
		repo.JoinWhere(where...),
		params.SortBy.ToSQL(r.fieldMap),
		repo.FormatLimitOffset(params.Limit, params.Offset),
	)

	departments, err := r.queryDepartments(ctx, query, args...)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get paginated departments")
	}
	return departments, nil
}

func (r *PgDepartmentRepository) Count(ctx context.Context, params *department.FindParams) (int64, error) {
	tx, err := composables.UseTx(ctx)
	if err != nil {
		return 0, errors.Wrap(err, "failed to get transaction")
	}

	where, args, err := r.buildFilters(ctx, params)
	if err != nil {
		return 0, err
	}

	query := repo.Join(
		departmentCountQuery,
		repo.JoinWhere(where...),
	)

	var count int64
	if err := tx.QueryRow(ctx, query, args...).Scan(&count); err != nil {
		return 0, errors.Wrap(err, "failed to count departments")
	}
	return count, nil
}

func (r *PgDepartmentRepository) GetByID(ctx context.Context, id uuid.UUID) (department.Department, error) {
	tenantID, err := composables.UseTenantID(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get tenant from context")
	}

	q := repo.Join(departmentFindQuery, "WHERE d.id = $1 AND d.tenant_id = $2")
	departments, err := r.queryDepartments(ctx, q, id.String(), tenantID.String())
	if err != nil {
		return nil, errors.Wrap(err, fmt.Sprintf("failed to query department with id: %s", id.String()))
	}
	if len(departments) == 0 {
		return nil, errors.Wrap(ErrDepartmentNotFound, fmt.Sprintf("id: %s", id.String()))
	}
	return departments[0], nil
}

func (r *PgDepartmentRepository) Exists(ctx context.Context, id uuid.UUID) (bool, error) {
	tx, err := composables.UseTx(ctx)
	if err != nil {
		return false, errors.Wrap(err, "failed to get transaction")
	}

	tenantID, err := composables.UseTenantID(ctx)
	if err != nil {
		return false, errors.Wrap(err, "failed to get tenant from context")
	}

	var exists bool
	if err := tx.QueryRow(ctx, departmentExistsQuery, id.String(), tenantID.String()).Scan(&exists); err != nil {
		return false, errors.Wrap(err, "failed to check if department exists")
	}
	return exists, nil
}

func (r *PgDepartmentRepository) Save(ctx context.Context, entity department.Department) (department.Department, error) {
	exists, err := r.Exists(ctx, entity.ID())
	if err != nil {
		return nil, errors.Wrap(err, "failed to check if department exists")
	}

	if exists {
		return r.update(ctx, entity)
	}
	return r.create(ctx, entity)
}

func (r *PgDepartmentRepository) create(
	ctx context.Context,
	entity department.Department,
) (department.Department, error) {
	tx, err := composables.UseTx(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get transaction")
	}

	dbDepartment, err := ToDBDepartment(entity)
	if err != nil {
		return nil, errors.Wrap(err, "failed to map department to db model")
	}
	if entity.ID() == uuid.Nil {
		dbDepartment.ID = uuid.New().String()
	}

	fields := []string{
		"id",
		"tenant_id",
		"parent_id",
		"code",
		"name",
		`"order"`,
		"status",
		"created_at",
		"updated_at",
	}

	values := []interface{}{
		dbDepartment.ID,
		dbDepartment.TenantID,
		dbDepartment.ParentID,
		dbDepartment.Code,
		dbDepartment.Name,
		dbDepartment.Order,
		dbDepartment.Status,
		dbDepartment.CreatedAt,
		dbDepartment.UpdatedAt,
	}

	if _, err := tx.Exec(ctx, repo.Insert("core.departments", fields), values...); err != nil {
		return nil, errors.Wrap(err, "failed to insert department")
	}

	id, err := uuid.Parse(dbDepartment.ID)
	if err != nil {
		return nil, errors.Wrap(err, "failed to parse UUID")
	}
	return r.GetByID(ctx, id)
}

func (r *PgDepartmentRepository) update(
	ctx context.Context,
	entity department.Department,
) (department.Department, error) {
	tx, err := composables.UseTx(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get transaction")
	}

	dbDepartment, err := ToDBDepartment(entity)
	if err != nil {
		return nil, errors.Wrap(err, "failed to map department to db model")
	}

	fields := []string{
		"parent_id",
		"code",
		"name",
		`"order"`,
		"status",
		"updated_at",
	}

	values := []interface{}{
		dbDepartment.ParentID,
		dbDepartment.Code,
		dbDepartment.Name,
		dbDepartment.Order,
		dbDepartment.Status,
		dbDepartment.UpdatedAt,
		dbDepartment.ID,
		dbDepartment.TenantID,
	}

	query := repo.Update(
		"core.departments",
		fields,
		fmt.Sprintf("id = $%d", len(values)-1),
		fmt.Sprintf("tenant_id = $%d", len(values)),
	)
	if _, err := tx.Exec(ctx, query, values...); err != nil {
		return nil, errors.Wrap(err, fmt.Sprintf("failed to update department with ID: %s", dbDepartment.ID))
	}

	id, err := uuid.Parse(dbDepartment.ID)
	if err != nil {
		return nil, errors.Wrap(err, "failed to parse UUID")
	}
	return r.GetByID(ctx, id)
}

func (r *PgDepartmentRepository) Delete(ctx context.Context, id uuid.UUID) error {
	tx, err := composables.UseTx(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to get transaction")
	}

	tenantID, err := composables.UseTenantID(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to get tenant from context")
	}

	if _, err := tx.Exec(ctx, departmentDeleteQuery, id.String(), tenantID.String()); err != nil {
		return errors.Wrap(err, fmt.Sprintf("failed to delete department with ID: %s", id.String()))
	}
	return nil
}

func (r *PgDepartmentRepository) queryDepartments(
	ctx context.Context,
	query string,
	args ...interface{},
) ([]department.Department, error) {
	tx, err := composables.UseTx(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get transaction")
	}

	rows, err := tx.Query(ctx, query, args...)
	if err != nil {
		return nil, errors.Wrap(err, "failed to execute query")
	}
	defer rows.Close()

	var dbDepartments []*models.Department
	for rows.Next() {
		var dbDepartment models.Department
		if err := rows.Scan(
			&dbDepartment.ID,
			&dbDepartment.TenantID,
			&dbDepartment.ParentID,
			&dbDepartment.Code,
			&dbDepartment.Name,
			&dbDepartment.Order,
			&dbDepartment.Status,
			&dbDepartment.CreatedAt,
			&dbDepartment.UpdatedAt,
		); err != nil {
			return nil, errors.Wrap(err, "failed to scan department row")
		}
		dbDepartments = append(dbDepartments, &dbDepartment)
	}

	if err := rows.Err(); err != nil {
		return nil, errors.Wrap(err, "row iteration error")
	}

	entities := make([]department.Department, 0, len(dbDepartments))
	for _, dbDepartment := range dbDepartments {
		domainDepartment, err := ToDomainDepartment(dbDepartment)
		if err != nil {
			return nil, errors.Wrap(err, fmt.Sprintf("failed to convert department ID: %s to domain entity", dbDepartment.ID))
		}
		entities = append(entities, domainDepartment)
	}

	return entities, nil
}
