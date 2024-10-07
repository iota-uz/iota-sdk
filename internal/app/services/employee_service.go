package services

import (
	"context"

	"github.com/iota-agency/iota-erp/internal/domain/entities/employee"
)

type EmployeeService struct {
	repo employee.Repository
	app  *Application
}

func NewEmployeeService(repo employee.Repository, app *Application) *EmployeeService {
	return &EmployeeService{
		repo: repo,
		app:  app,
	}
}

func (s *EmployeeService) Count(ctx context.Context) (int64, error) {
	return s.repo.Count(ctx)
}

func (s *EmployeeService) GetAll(ctx context.Context) ([]*employee.Employee, error) {
	return s.repo.GetAll(ctx)
}

func (s *EmployeeService) GetByID(ctx context.Context, id int64) (*employee.Employee, error) {
	return s.repo.GetByID(ctx, id)
}

func (s *EmployeeService) GetMeta(ctx context.Context, id int64) (*employee.Meta, error) {
	return s.repo.GetEmployeeMeta(ctx, id)
}

func (s *EmployeeService) GetPaginated(
	ctx context.Context,
	limit, offset int,
	sortBy []string,
) ([]*employee.Employee, error) {
	return s.repo.GetPaginated(ctx, limit, offset, sortBy)
}

func (s *EmployeeService) Create(ctx context.Context, data *employee.Employee) error {
	if err := s.repo.Create(ctx, data); err != nil {
		return err
	}
	s.app.EventPublisher.Publish("employee.created", data)
	return nil
}

func (s *EmployeeService) Update(ctx context.Context, data *employee.Employee) error {
	if err := s.repo.Update(ctx, data); err != nil {
		return err
	}
	s.app.EventPublisher.Publish("employee.updated", data)
	return nil
}

func (s *EmployeeService) Delete(ctx context.Context, id int64) error {
	if err := s.repo.Delete(ctx, id); err != nil {
		return err
	}
	s.app.EventPublisher.Publish("employee.deleted", id)
	return nil
}
