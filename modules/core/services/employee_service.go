package services

import (
	"context"
	employee2 "github.com/iota-uz/iota-sdk/modules/core/domain/entities/employee"
	"github.com/iota-uz/iota-sdk/pkg/event"
)

type EmployeeService struct {
	repo      employee2.Repository
	publisher event.Publisher
}

func NewEmployeeService(repo employee2.Repository, publisher event.Publisher) *EmployeeService {
	return &EmployeeService{
		repo:      repo,
		publisher: publisher,
	}
}

func (s *EmployeeService) Count(ctx context.Context) (int64, error) {
	return s.repo.Count(ctx)
}

func (s *EmployeeService) GetAll(ctx context.Context) ([]*employee2.Employee, error) {
	return s.repo.GetAll(ctx)
}

func (s *EmployeeService) GetByID(ctx context.Context, id uint) (*employee2.Employee, error) {
	return s.repo.GetByID(ctx, id)
}

func (s *EmployeeService) GetPaginated(
	ctx context.Context,
	limit, offset int,
	sortBy []string,
) ([]*employee2.Employee, error) {
	return s.repo.GetPaginated(ctx, limit, offset, sortBy)
}

func (s *EmployeeService) Create(ctx context.Context, data *employee2.CreateDTO) error {
	entity := data.ToEntity()
	if err := s.repo.Create(ctx, entity); err != nil {
		return err
	}
	ev, err := employee2.NewCreatedEvent(ctx, *data, *entity)
	if err != nil {
		return err
	}
	s.publisher.Publish(ev)
	return nil
}

func (s *EmployeeService) Update(ctx context.Context, id uint, data *employee2.UpdateDTO) error {
	entity := data.ToEntity(id)
	if err := s.repo.Update(ctx, entity); err != nil {
		return err
	}
	ev, err := employee2.NewUpdatedEvent(ctx, *data, *entity)
	if err != nil {
		return err
	}
	s.publisher.Publish(ev)
	return nil
}

func (s *EmployeeService) Delete(ctx context.Context, id uint) (*employee2.Employee, error) {
	if err := s.repo.Delete(ctx, id); err != nil {
		return nil, err
	}
	entity, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	ev, err := employee2.NewDeletedEvent(ctx, *entity)
	if err != nil {
		return nil, err
	}
	s.publisher.Publish(ev)
	return entity, nil
}
