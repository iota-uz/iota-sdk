package services

import (
	"context"
	"github.com/iota-agency/iota-erp/internal/domain/entities/employee"
	"github.com/iota-agency/iota-erp/pkg/event"
)

type EmployeeService struct {
	repo      employee.Repository
	publisher event.Publisher
}

func NewEmployeeService(repo employee.Repository, publisher event.Publisher) *EmployeeService {
	return &EmployeeService{
		repo:      repo,
		publisher: publisher,
	}
}

func (s *EmployeeService) Count(ctx context.Context) (int64, error) {
	return s.repo.Count(ctx)
}

func (s *EmployeeService) GetAll(ctx context.Context) ([]*employee.Employee, error) {
	return s.repo.GetAll(ctx)
}

func (s *EmployeeService) GetByID(ctx context.Context, id uint) (*employee.Employee, error) {
	return s.repo.GetByID(ctx, id)
}

func (s *EmployeeService) GetPaginated(
	ctx context.Context,
	limit, offset int,
	sortBy []string,
) ([]*employee.Employee, error) {
	return s.repo.GetPaginated(ctx, limit, offset, sortBy)
}

func (s *EmployeeService) Create(ctx context.Context, data *employee.CreateDTO) error {
	entity := data.ToEntity()
	if err := s.repo.Create(ctx, entity); err != nil {
		return err
	}
	ev, err := employee.NewCreatedEvent(ctx, *data, *entity)
	if err != nil {
		return err
	}
	s.publisher.Publish(ev)
	return nil
}

func (s *EmployeeService) Update(ctx context.Context, id uint, data *employee.UpdateDTO) error {
	entity := data.ToEntity(id)
	if err := s.repo.Update(ctx, entity); err != nil {
		return err
	}
	ev, err := employee.NewUpdatedEvent(ctx, *data, *entity)
	if err != nil {
		return err
	}
	s.publisher.Publish(ev)
	return nil
}

func (s *EmployeeService) Delete(ctx context.Context, id uint) (*employee.Employee, error) {
	if err := s.repo.Delete(ctx, id); err != nil {
		return nil, err
	}
	entity, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	ev, err := employee.NewDeletedEvent(ctx, *entity)
	if err != nil {
		return nil, err
	}
	s.publisher.Publish(ev)
	return entity, nil
}
