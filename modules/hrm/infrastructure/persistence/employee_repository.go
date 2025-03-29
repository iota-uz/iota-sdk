package persistence

import (
	"context"
	"fmt"

	"github.com/iota-uz/iota-sdk/modules/hrm/domain/aggregates/employee"
	"github.com/iota-uz/iota-sdk/modules/hrm/infrastructure/persistence/models"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/repo"

	"github.com/go-faster/errors"
)

var (
	ErrEmployeeNotFound = errors.New("employee not found")
)

const (
	employeeFindQuery = `
		SELECT e.id,
		       e.tenant_id,
		       e.first_name,
		       e.last_name,
		       e.middle_name,
		       e.email,
		       e.phone,
		       e.salary,
		       e.salary_currency_id,
		       e.hourly_rate,
		       e.coefficient,
		       e.avatar_id,
		       e.created_at,
		       e.updated_at,
		       em.primary_language,
		       em.secondary_language,
		       em.tin,
		       em.notes,
		       em.birth_date,
		       em.hire_date,
		       em.resignation_date
		  FROM employees e
		       LEFT JOIN employee_meta em ON e.id = em.employee_id`
	employeeCountQuery  = `SELECT COUNT(*) as count FROM employees WHERE tenant_id = $1`
	employeeInsertQuery = `
		INSERT INTO employees (
			tenant_id, first_name, last_name, middle_name, email, phone, salary, salary_currency_id,
			hourly_rate, coefficient, avatar_id, created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13) RETURNING id`
	employeeMetaInsertQuery = `
		INSERT INTO employee_meta (
			employee_id,
			primary_language,
			secondary_language,
			tin,
			pin,
			notes,
			birth_date,
			hire_date,
			resignation_date
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`

	employeeUpdateQuery = `
		UPDATE employees
		   SET first_name = $1, last_name = $2, middle_name = $3, email = $4, phone = $5,
		       salary = $6, salary_currency_id = $7, hourly_rate = $8, coefficient = $9,
		       avatar_id = $10, updated_at = $11
		WHERE id = $12 AND tenant_id = $13`

	employeeUpdateMetaQuery = `
		UPDATE employee_meta
		   SET primary_language = $1, secondary_language = $2, tin = $3, notes = $4,
		       birth_date = $5, hire_date = $6, resignation_date = $7
		WHERE employee_id = $8`

	employeeDeleteQuery     = `DELETE FROM employees WHERE id = $1 AND tenant_id = $2`
	employeeMetaDeleteQuery = `DELETE FROM employee_meta WHERE employee_id = $1`
)

type GormEmployeeRepository struct{}

func NewEmployeeRepository() employee.Repository {
	return &GormEmployeeRepository{}
}

func (g *GormEmployeeRepository) GetPaginated(ctx context.Context, params *employee.FindParams) ([]employee.Employee, error) {
	tenant, err := composables.UseTenant(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get tenant from context: %w", err)
	}

	var args []interface{}
	where := []string{"e.tenant_id = $1"}
	args = append(args, tenant.ID)

	if params.Query != "" && params.Field != "" {
		where = append(where, fmt.Sprintf("e.%s::VARCHAR ILIKE $%d", params.Field, len(args)+1))
		args = append(args, "%"+params.Query+"%")
	}

	q := repo.Join(
		employeeFindQuery,
		repo.JoinWhere(where...),
		repo.FormatLimitOffset(params.Limit, params.Offset),
	)
	return g.queryEmployees(ctx, q, args...)
}

func (g *GormEmployeeRepository) Count(ctx context.Context) (int64, error) {
	tenant, err := composables.UseTenant(ctx)
	if err != nil {
		return 0, fmt.Errorf("failed to get tenant from context: %w", err)
	}

	tx, err := composables.UseTx(ctx)
	if err != nil {
		return 0, err
	}
	var count int64
	if err := tx.QueryRow(ctx, employeeCountQuery, tenant.ID).Scan(&count); err != nil {
		return 0, err
	}
	return count, nil
}

func (g *GormEmployeeRepository) GetAll(ctx context.Context) ([]employee.Employee, error) {
	tenant, err := composables.UseTenant(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get tenant from context: %w", err)
	}
	return g.queryEmployees(ctx, repo.Join(employeeFindQuery, "WHERE e.tenant_id = $1"), tenant.ID)
}

func (g *GormEmployeeRepository) GetByID(ctx context.Context, id uint) (employee.Employee, error) {
	tenant, err := composables.UseTenant(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get tenant from context: %w", err)
	}
	employees, err := g.queryEmployees(ctx, repo.Join(employeeFindQuery, "WHERE e.id = $1 AND e.tenant_id = $2"), id, tenant.ID)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get employee by id")
	}
	if len(employees) == 0 {
		return nil, ErrEmployeeNotFound
	}
	return employees[0], nil
}

func (g *GormEmployeeRepository) Create(ctx context.Context, data employee.Employee) (employee.Employee, error) {
	tenant, err := composables.UseTenant(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get tenant from context: %w", err)
	}

	dbEmployee, dbMeta := toDBEmployee(data)
	dbEmployee.TenantID = tenant.ID // Set tenant ID from context

	tx, err := composables.UseTx(ctx)
	if err != nil {
		return nil, err
	}
	row := tx.QueryRow(
		ctx,
		employeeInsertQuery,
		dbEmployee.TenantID,
		dbEmployee.FirstName,
		dbEmployee.LastName,
		dbEmployee.MiddleName,
		dbEmployee.Email,
		dbEmployee.Phone,
		dbEmployee.Salary,
		dbEmployee.SalaryCurrencyID,
		dbEmployee.HourlyRate,
		dbEmployee.Coefficient,
		dbEmployee.AvatarID,
		dbEmployee.CreatedAt,
		dbEmployee.UpdatedAt,
	)
	var id uint
	if err := row.Scan(&id); err != nil {
		return nil, errors.Wrap(err, "failed to create employee")
	}
	if _, err := tx.Exec(ctx, employeeMetaInsertQuery,
		id,
		dbMeta.PrimaryLanguage,
		dbMeta.SecondaryLanguage,
		dbMeta.Tin,
		dbMeta.Pin,
		dbMeta.Notes,
		dbMeta.BirthDate,
		dbMeta.HireDate,
		dbMeta.ResignationDate,
	); err != nil {
		return nil, errors.Wrap(err, "failed to create employee meta")
	}
	return g.GetByID(ctx, id)
}

func (g *GormEmployeeRepository) Update(ctx context.Context, data employee.Employee) error {
	tenant, err := composables.UseTenant(ctx)
	if err != nil {
		return fmt.Errorf("failed to get tenant from context: %w", err)
	}

	dbEmployee, dbMeta := toDBEmployee(data)
	if err := g.execQuery(
		ctx,
		employeeUpdateQuery,
		dbEmployee.FirstName,
		dbEmployee.LastName,
		dbEmployee.MiddleName,
		dbEmployee.Email,
		dbEmployee.Phone,
		dbEmployee.Salary,
		dbEmployee.SalaryCurrencyID,
		dbEmployee.HourlyRate,
		dbEmployee.Coefficient,
		dbEmployee.AvatarID,
		dbEmployee.UpdatedAt,
		dbEmployee.ID,
		tenant.ID,
	); err != nil {
		return err
	}
	return g.execQuery(
		ctx,
		employeeUpdateMetaQuery,
		dbMeta.PrimaryLanguage,
		dbMeta.SecondaryLanguage,
		dbMeta.Tin,
		dbMeta.Notes,
		dbMeta.BirthDate,
		dbMeta.HireDate,
		dbMeta.ResignationDate,
		dbEmployee.ID,
	)
}

func (g *GormEmployeeRepository) Delete(ctx context.Context, id uint) error {
	tenant, err := composables.UseTenant(ctx)
	if err != nil {
		return fmt.Errorf("failed to get tenant from context: %w", err)
	}

	if err := g.execQuery(ctx, employeeMetaDeleteQuery, id); err != nil {
		return err
	}
	return g.execQuery(ctx, employeeDeleteQuery, id, tenant.ID)
}

func (g *GormEmployeeRepository) queryEmployees(ctx context.Context, query string, args ...interface{}) ([]employee.Employee, error) {
	tx, err := composables.UseTx(ctx)
	if err != nil {
		return nil, err
	}
	rows, err := tx.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var employees []employee.Employee
	for rows.Next() {
		var employeeRow models.Employee
		var metaRow models.EmployeeMeta
		if err := rows.Scan(
			&employeeRow.ID,
			&employeeRow.TenantID,
			&employeeRow.FirstName,
			&employeeRow.LastName,
			&employeeRow.MiddleName,
			&employeeRow.Email,
			&employeeRow.Phone,
			&employeeRow.Salary,
			&employeeRow.SalaryCurrencyID,
			&employeeRow.HourlyRate,
			&employeeRow.Coefficient,
			&employeeRow.AvatarID,
			&employeeRow.CreatedAt,
			&employeeRow.UpdatedAt,
			&metaRow.PrimaryLanguage,
			&metaRow.SecondaryLanguage,
			&metaRow.Tin,
			&metaRow.Notes,
			&metaRow.BirthDate,
			&metaRow.HireDate,
			&metaRow.ResignationDate,
		); err != nil {
			return nil, err
		}
		employeeEntity, err := toDomainEmployee(&employeeRow, &metaRow)
		if err != nil {
			return nil, err
		}
		employees = append(employees, employeeEntity)
	}
	return employees, nil
}

func (g *GormEmployeeRepository) execQuery(ctx context.Context, query string, args ...interface{}) error {
	tx, err := composables.UseTx(ctx)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, query, args...)
	return err
}
