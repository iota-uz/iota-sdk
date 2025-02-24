package services

import (
	"context"

	"github.com/iota-uz/iota-sdk/modules/hrm/domain/aggregates/employee"
	"github.com/iota-uz/iota-sdk/pkg/eventbus"
)

type EmployeeService struct {
	repo      employee.Repository
	publisher eventbus.EventBus
}

func NewEmployeeService(repo employee.Repository, publisher eventbus.EventBus) *EmployeeService {
	return &EmployeeService{
		repo:      repo,
		publisher: publisher,
	}
}

func (s *EmployeeService) Count(ctx context.Context) (int64, error) {
	return s.repo.Count(ctx)
}

func (s *EmployeeService) GetAll(ctx context.Context) ([]employee.Employee, error) {
	return s.repo.GetAll(ctx)
}

func (s *EmployeeService) GetByID(ctx context.Context, id uint) (employee.Employee, error) {
	return s.repo.GetByID(ctx, id)
}

func (s *EmployeeService) GetPaginated(ctx context.Context, params *employee.FindParams) ([]employee.Employee, error) {
	return s.repo.GetPaginated(ctx, params)
}

func (s *EmployeeService) Create(ctx context.Context, data *employee.CreateDTO) error {
	entity, err := data.ToEntity()
	if err != nil {
		return err
	}
	createdEntity, err := s.repo.Create(ctx, entity)
	if err != nil {
		return err
	}
	ev, err := employee.NewCreatedEvent(ctx, *data, createdEntity)
	if err != nil {
		return err
	}
	s.publisher.Publish(ev)
	return nil
}

func (s *EmployeeService) Update(ctx context.Context, id uint, data *employee.UpdateDTO) error {
	entity, err := data.ToEntity(id)
	if err != nil {
		return err
	}
	if err := s.repo.Update(ctx, entity); err != nil {
		return err
	}
	ev, err := employee.NewUpdatedEvent(ctx, *data, entity)
	if err != nil {
		return err
	}
	s.publisher.Publish(ev)
	return nil
}

func (s *EmployeeService) Delete(ctx context.Context, id uint) (employee.Employee, error) {
	if err := s.repo.Delete(ctx, id); err != nil {
		return nil, err
	}
	entity, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	ev, err := employee.NewDeletedEvent(ctx, entity)
	if err != nil {
		return nil, err
	}
	s.publisher.Publish(ev)
	return entity, nil
}
