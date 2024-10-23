package graph

// This file will be automatically regenerated based on the schema, any resolver implementations
// will be copied through when generating and any unknown code will be moved to the end.
// Code generated by github.com/99designs/gqlgen version v0.17.47

import (
	"context"
	"fmt"

	"github.com/iota-agency/iota-erp/internal/domain/entities/employee"
	model "github.com/iota-agency/iota-erp/internal/interfaces/graph/gqlmodels"
	"github.com/iota-agency/iota-erp/sdk/mapper"
)

// Meta is the resolver for the meta field.
func (r *employeeResolver) Meta(ctx context.Context, obj *model.Employee) (*model.EmployeeMeta, error) {
	return &model.EmployeeMeta{}, nil
}

// CreateEmployee is the resolver for the createEmployee field.
func (r *mutationResolver) CreateEmployee(ctx context.Context, input model.CreateEmployee) (*model.Employee, error) {
	entity := &employee.Employee{}
	if err := mapper.LenientMapping(&input, entity); err != nil {
		return nil, err
	}
	if err := r.app.EmployeeService.Create(ctx, entity); err != nil {
		return nil, err
	}
	return entity.ToGraph(), nil
}

// UpdateEmployee is the resolver for the updateEmployee field.
func (r *mutationResolver) UpdateEmployee(ctx context.Context, id int64, input model.UpdateEmployee) (*model.Employee, error) {
	entity, err := r.app.EmployeeService.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if err := mapper.LenientMapping(&input, entity); err != nil {
		return nil, err
	}
	if err := r.app.EmployeeService.Update(ctx, entity); err != nil {
		return nil, err
	}
	return entity.ToGraph(), nil
}

// DeleteEmployee is the resolver for the deleteEmployee field.
func (r *mutationResolver) DeleteEmployee(ctx context.Context, id int64) (*model.Employee, error) {
	entity, err := r.app.EmployeeService.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if err := r.app.EmployeeService.Delete(ctx, id); err != nil {
		return nil, err
	}
	return entity.ToGraph(), nil
}

// Data is the resolver for the data field.
func (r *paginatedEmployeesResolver) Data(ctx context.Context, obj *model.PaginatedEmployees) ([]*model.Employee, error) {
	entities, err := r.app.EmployeeService.GetPaginated(ctx, len(obj.Data), 0, nil)
	if err != nil {
		return nil, err
	}
	result := make([]*model.Employee, len(entities))
	for i, entity := range entities {
		result[i] = entity.ToGraph()
	}
	return result, nil
}

// Total is the resolver for the total field.
func (r *paginatedEmployeesResolver) Total(ctx context.Context, obj *model.PaginatedEmployees) (int64, error) {
	return r.app.EmployeeService.Count(ctx)
}

// Employee is the resolver for the employee field.
func (r *queryResolver) Employee(ctx context.Context, id int64) (*model.Employee, error) {
	entity, err := r.app.EmployeeService.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	return entity.ToGraph(), nil
}

// Employees is the resolver for the employees field.
func (r *queryResolver) Employees(ctx context.Context, offset int, limit int, sortBy []string) (*model.PaginatedEmployees, error) {
	return &model.PaginatedEmployees{}, nil
}

// EmployeeCreated is the resolver for the employeeCreated field.
func (r *subscriptionResolver) EmployeeCreated(ctx context.Context) (<-chan *model.Employee, error) {
	panic(fmt.Errorf("not implemented: EmployeeCreated - employeeCreated"))
}

// EmployeeUpdated is the resolver for the employeeUpdated field.
func (r *subscriptionResolver) EmployeeUpdated(ctx context.Context) (<-chan *model.Employee, error) {
	panic(fmt.Errorf("not implemented: EmployeeUpdated - employeeUpdated"))
}

// EmployeeDeleted is the resolver for the employeeDeleted field.
func (r *subscriptionResolver) EmployeeDeleted(ctx context.Context) (<-chan *model.Employee, error) {
	panic(fmt.Errorf("not implemented: EmployeeDeleted - employeeDeleted"))
}

// Employee returns EmployeeResolver implementation.
func (r *Resolver) Employee() EmployeeResolver { return &employeeResolver{r} }

// PaginatedEmployees returns PaginatedEmployeesResolver implementation.
func (r *Resolver) PaginatedEmployees() PaginatedEmployeesResolver {
	return &paginatedEmployeesResolver{r}
}

type employeeResolver struct{ *Resolver }
type paginatedEmployeesResolver struct{ *Resolver }
