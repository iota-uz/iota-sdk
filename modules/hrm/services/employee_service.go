// Package services provides this package.
package services

import (
	"context"
	"strconv"

	"github.com/iota-uz/iota-sdk/modules/hrm/domain/aggregates/employee"
	"github.com/iota-uz/iota-sdk/modules/logging/history"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/eventbus"
)

type EmployeeService struct {
	repo          employee.Repository
	publisher     eventbus.EventBus
	historyWriter history.Writer
}

func NewEmployeeService(repo employee.Repository, publisher eventbus.EventBus, historyWriter history.Writer) *EmployeeService {
	return &EmployeeService{
		repo:          repo,
		publisher:     publisher,
		historyWriter: historyWriter,
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
	var createdEntity employee.Employee
	err = composables.InTx(ctx, func(txCtx context.Context) error {
		createdEntity, err = s.repo.Create(txCtx, entity)
		if err != nil {
			return err
		}
		return history.RecordMutation(txCtx, s.historyWriter, history.Mutation{
			Type: EmployeeCreated, Version: 1, Resource: EmployeeResource, ID: employeeID(createdEntity.ID()),
			After: employeeSnapshot(createdEntity),
		})
	})
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
	err = composables.InTx(ctx, func(txCtx context.Context) error {
		before, err := s.repo.GetByID(txCtx, id)
		if err != nil {
			return err
		}
		if err := s.repo.Update(txCtx, entity); err != nil {
			return err
		}
		after, err := s.repo.GetByID(txCtx, id)
		if err != nil {
			return err
		}
		return history.RecordMutation(txCtx, s.historyWriter, history.Mutation{
			Type: EmployeeUpdated, Version: 1, Resource: EmployeeResource, ID: employeeID(id),
			Before: employeeSnapshot(before), After: employeeSnapshot(after),
		})
	})
	if err != nil {
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
	var entity employee.Employee
	err := composables.InTx(ctx, func(txCtx context.Context) error {
		var err error
		entity, err = s.repo.GetByID(txCtx, id)
		if err != nil {
			return err
		}
		if err := s.repo.Delete(txCtx, id); err != nil {
			return err
		}
		return history.RecordMutation(txCtx, s.historyWriter, history.Mutation{
			Type: EmployeeDeleted, Version: 1, Resource: EmployeeResource, ID: employeeID(id),
			Before: employeeSnapshot(entity),
		})
	})
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

func employeeID(id uint) string {
	return strconv.FormatUint(uint64(id), 10)
}
