// Package persistence provides this package.
package persistence

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/department"
	"github.com/iota-uz/iota-sdk/modules/core/infrastructure/persistence/models"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/repo"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
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
	const op serrors.Op = "PgDepartmentRepository.buildFilters"
	tenantID, err := composables.UseTenantID(ctx)
	if err != nil {
		return nil, nil, serrors.E(op, err)
	}

	where := []string{"d.tenant_id = $1"}
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
		where = append(where, fmt.Sprintf("(d.code ILIKE $%d OR d.name::text ILIKE $%d)", index, index))
		args = append(args, "%"+params.Search+"%")
	}

	return where, args, nil
}

func (r *PgDepartmentRepository) GetPaginated(
	ctx context.Context,
	params *department.FindParams,
) ([]department.Department, error) {
	const op serrors.Op = "PgDepartmentRepository.GetPaginated"
	if params == nil {
		params = &department.FindParams{}
	}

	where, args, err := r.buildFilters(ctx, params)
	if err != nil {
		return nil, serrors.E(op, err)
	}

	query := repo.Join(
		departmentFindQuery,
		repo.JoinWhere(where...),
		params.SortBy.ToSQL(r.fieldMap),
		repo.FormatLimitOffset(params.Limit, params.Offset),
	)

	departments, err := r.queryDepartments(ctx, query, args...)
	if err != nil {
		return nil, serrors.E(op, err)
	}
	return departments, nil
}

func (r *PgDepartmentRepository) Count(ctx context.Context, params *department.FindParams) (int64, error) {
	const op serrors.Op = "PgDepartmentRepository.Count"
	if params == nil {
		params = &department.FindParams{}
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
		departmentCountQuery,
		repo.JoinWhere(where...),
	)

	var count int64
	if err := tx.QueryRow(ctx, query, args...).Scan(&count); err != nil {
		return 0, serrors.E(op, err)
	}
	return count, nil
}

func (r *PgDepartmentRepository) GetByID(ctx context.Context, id uuid.UUID) (department.Department, error) {
	const op serrors.Op = "PgDepartmentRepository.GetByID"
	tenantID, err := composables.UseTenantID(ctx)
	if err != nil {
		return nil, serrors.E(op, err)
	}

	q := repo.Join(departmentFindQuery, "WHERE d.id = $1 AND d.tenant_id = $2")
	departments, err := r.queryDepartments(ctx, q, id.String(), tenantID.String())
	if err != nil {
		return nil, serrors.E(op, err)
	}
	if len(departments) == 0 {
		return nil, serrors.E(op, serrors.NotFound, ErrDepartmentNotFound)
	}
	return departments[0], nil
}

func (r *PgDepartmentRepository) Exists(ctx context.Context, id uuid.UUID) (bool, error) {
	const op serrors.Op = "PgDepartmentRepository.Exists"
	tx, err := composables.UseTx(ctx)
	if err != nil {
		return false, serrors.E(op, err)
	}

	tenantID, err := composables.UseTenantID(ctx)
	if err != nil {
		return false, serrors.E(op, err)
	}

	var exists bool
	if err := tx.QueryRow(ctx, departmentExistsQuery, id.String(), tenantID.String()).Scan(&exists); err != nil {
		return false, serrors.E(op, err)
	}
	return exists, nil
}

func (r *PgDepartmentRepository) Save(ctx context.Context, entity department.Department) (department.Department, error) {
	const op serrors.Op = "PgDepartmentRepository.Save"
	exists, err := r.Exists(ctx, entity.ID())
	if err != nil {
		return nil, serrors.E(op, err)
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
	const op serrors.Op = "PgDepartmentRepository.create"
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

	dbDepartment, err := ToDBDepartment(entity)
	if err != nil {
		return nil, serrors.E(op, err)
	}
	dbDepartment.TenantID = tenantID.String()
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
		return nil, serrors.E(op, err)
	}

	id, err := uuid.Parse(dbDepartment.ID)
	if err != nil {
		return nil, serrors.E(op, err)
	}
	return r.GetByID(ctx, id)
}

func (r *PgDepartmentRepository) update(
	ctx context.Context,
	entity department.Department,
) (department.Department, error) {
	const op serrors.Op = "PgDepartmentRepository.update"
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

	dbDepartment, err := ToDBDepartment(entity)
	if err != nil {
		return nil, serrors.E(op, err)
	}
	dbDepartment.TenantID = tenantID.String()

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
		return nil, serrors.E(op, err)
	}

	id, err := uuid.Parse(dbDepartment.ID)
	if err != nil {
		return nil, serrors.E(op, err)
	}
	return r.GetByID(ctx, id)
}

func (r *PgDepartmentRepository) Delete(ctx context.Context, id uuid.UUID) error {
	const op serrors.Op = "PgDepartmentRepository.Delete"
	tx, err := composables.UseTx(ctx)
	if err != nil {
		return serrors.E(op, err)
	}

	tenantID, err := composables.UseTenantID(ctx)
	if err != nil {
		return serrors.E(op, err)
	}

	tag, err := tx.Exec(ctx, departmentDeleteQuery, id.String(), tenantID.String())
	if err != nil {
		return serrors.E(op, err)
	}
	if tag.RowsAffected() == 0 {
		return serrors.E(op, serrors.NotFound, ErrDepartmentNotFound)
	}
	return nil
}

func (r *PgDepartmentRepository) queryDepartments(
	ctx context.Context,
	query string,
	args ...interface{},
) ([]department.Department, error) {
	const op serrors.Op = "PgDepartmentRepository.queryDepartments"
	tx, err := composables.UseTx(ctx)
	if err != nil {
		return nil, serrors.E(op, err)
	}

	rows, err := tx.Query(ctx, query, args...)
	if err != nil {
		return nil, serrors.E(op, err)
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
			return nil, serrors.E(op, err)
		}
		dbDepartments = append(dbDepartments, &dbDepartment)
	}

	if err := rows.Err(); err != nil {
		return nil, serrors.E(op, err)
	}

	entities := make([]department.Department, 0, len(dbDepartments))
	for _, dbDepartment := range dbDepartments {
		domainDepartment, err := ToDomainDepartment(dbDepartment)
		if err != nil {
			return nil, serrors.E(op, err)
		}
		entities = append(entities, domainDepartment)
	}

	return entities, nil
}
