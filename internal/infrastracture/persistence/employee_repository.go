package persistence

import (
	"context"
	"github.com/iota-agency/iota-erp/internal/domain/employee"
	"github.com/iota-agency/iota-erp/sdk/composables"
	"github.com/iota-agency/iota-erp/sdk/graphql/helpers"
	"github.com/iota-agency/iota-erp/sdk/service"
)

type GormEmployeeRepository struct {
}

func NewEmployeeRepository() employee.Repository {
	return &GormEmployeeRepository{}
}

func (g *GormEmployeeRepository) GetPaginated(ctx context.Context, limit, offset int, sortBy []string) ([]*employee.Employee, error) {
	tx, ok := composables.UseTx(ctx)
	if !ok {
		return nil, service.ErrNoTx
	}
	var uploads []*employee.Employee
	q := tx.Limit(limit).Offset(offset)
	q, err := helpers.ApplySort(q, sortBy, &employee.Employee{})
	if err != nil {
		return nil, err
	}
	if err := q.Find(&uploads).Error; err != nil {
		return nil, err
	}
	return uploads, nil
}

func (g *GormEmployeeRepository) Count(ctx context.Context) (int64, error) {
	tx, ok := composables.UseTx(ctx)
	if !ok {
		return 0, service.ErrNoTx
	}
	var count int64
	if err := tx.Model(&employee.Employee{}).Count(&count).Error; err != nil {
		return 0, err
	}
	return count, nil
}

func (g *GormEmployeeRepository) GetAll(ctx context.Context) ([]*employee.Employee, error) {
	tx, ok := composables.UseTx(ctx)
	if !ok {
		return nil, service.ErrNoTx
	}
	var entities []*employee.Employee
	if err := tx.Find(&entities).Error; err != nil {
		return nil, err
	}
	return entities, nil
}

func (g *GormEmployeeRepository) GetByID(ctx context.Context, id int64) (*employee.Employee, error) {
	tx, ok := composables.UseTx(ctx)
	if !ok {
		return nil, service.ErrNoTx
	}
	var entity employee.Employee
	if err := tx.First(&entity, id).Error; err != nil {
		return nil, err
	}
	return &entity, nil
}

func (g *GormEmployeeRepository) GetEmployeeMeta(ctx context.Context, id int64) (*employee.Meta, error) {
	tx, ok := composables.UseTx(ctx)
	if !ok {
		return nil, service.ErrNoTx
	}
	var entity employee.Meta
	if err := tx.First(&entity, id).Error; err != nil {
		return nil, err
	}
	return &entity, nil
}

func (g *GormEmployeeRepository) Create(ctx context.Context, data *employee.Employee) error {
	tx, ok := composables.UseTx(ctx)
	if !ok {
		return service.ErrNoTx
	}
	if err := tx.Create(data).Error; err != nil {
		return err
	}
	return nil
}

func (g *GormEmployeeRepository) Update(ctx context.Context, data *employee.Employee) error {
	tx, ok := composables.UseTx(ctx)
	if !ok {
		return service.ErrNoTx
	}
	if err := tx.Save(data).Error; err != nil {
		return err
	}
	return nil
}

func (g *GormEmployeeRepository) Delete(ctx context.Context, id int64) error {
	tx, ok := composables.UseTx(ctx)
	if !ok {
		return service.ErrNoTx
	}
	if err := tx.Delete(&employee.Employee{}, id).Error; err != nil {
		return err
	}
	return nil
}
