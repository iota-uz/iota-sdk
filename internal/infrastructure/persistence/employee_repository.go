package persistence

import (
	"context"
	"github.com/iota-agency/iota-erp/internal/infrastructure/persistence/models"

	"github.com/iota-agency/iota-erp/internal/domain/entities/employee"
	"github.com/iota-agency/iota-erp/sdk/composables"
	"github.com/iota-agency/iota-erp/sdk/graphql/helpers"
	"github.com/iota-agency/iota-erp/sdk/service"
)

type GormEmployeeRepository struct{}

func NewEmployeeRepository() employee.Repository {
	return &GormEmployeeRepository{}
}

func (g *GormEmployeeRepository) GetPaginated(
	ctx context.Context,
	limit, offset int,
	sortBy []string,
) ([]*employee.Employee, error) {
	tx, ok := composables.UseTx(ctx)
	if !ok {
		return nil, service.ErrNoTx
	}
	var rows []*models.Employee
	q := tx.Limit(limit).Offset(offset)
	q, err := helpers.ApplySort(q, sortBy, &models.Employee{}) //nolint:exhaustruct
	if err != nil {
		return nil, err
	}
	if err := q.Find(&rows).Error; err != nil {
		return nil, err
	}
	entities := make([]*employee.Employee, len(rows))
	for i, r := range rows {
		entities[i] = toDomainEmployee(r)
	}
	return entities, nil
}

func (g *GormEmployeeRepository) Count(ctx context.Context) (int64, error) {
	tx, ok := composables.UseTx(ctx)
	if !ok {
		return 0, service.ErrNoTx
	}
	var count int64
	if err := tx.Model(&models.Employee{}).Count(&count).Error; err != nil { //nolint:exhaustruct
		return 0, err
	}
	return count, nil
}

func (g *GormEmployeeRepository) GetAll(ctx context.Context) ([]*employee.Employee, error) {
	tx, ok := composables.UseTx(ctx)
	if !ok {
		return nil, service.ErrNoTx
	}
	var rows []*models.Employee
	if err := tx.Find(&rows).Error; err != nil {
		return nil, err
	}
	entities := make([]*employee.Employee, len(rows))
	for i, r := range rows {
		entities[i] = toDomainEmployee(r)
	}
	return entities, nil
}

func (g *GormEmployeeRepository) GetByID(ctx context.Context, id uint) (*employee.Employee, error) {
	tx, ok := composables.UseTx(ctx)
	if !ok {
		return nil, service.ErrNoTx
	}
	var entity models.Employee
	if err := tx.First(&entity, id).Error; err != nil {
		return nil, err
	}
	return toDomainEmployee(&entity), nil
}

func (g *GormEmployeeRepository) Create(ctx context.Context, data *employee.Employee) error {
	tx, ok := composables.UseTx(ctx)
	if !ok {
		return service.ErrNoTx
	}
	employeeEntity, metaEntity := toDBEmployee(data)
	if err := tx.Create(employeeEntity).Error; err != nil {
		return err
	}
	metaEntity.EmployeeID = employeeEntity.ID
	if err := tx.Create(metaEntity).Error; err != nil {
		return err
	}
	return nil
}

func (g *GormEmployeeRepository) Update(ctx context.Context, data *employee.Employee) error {
	tx, ok := composables.UseTx(ctx)
	if !ok {
		return service.ErrNoTx
	}
	employeeEntity, metaEntity := toDBEmployee(data)
	if err := tx.Save(employeeEntity).Error; err != nil {
		return err
	}
	if err := tx.Save(metaEntity).Error; err != nil {
		return err
	}
	return nil
}

func (g *GormEmployeeRepository) Delete(ctx context.Context, id uint) error {
	tx, ok := composables.UseTx(ctx)
	if !ok {
		return service.ErrNoTx
	}
	if err := tx.Delete(&models.Employee{}, id).Error; err != nil { //nolint:exhaustruct
		return err
	}
	return nil
}
