package services

import (
	"context"

	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/project"
	"github.com/iota-uz/iota-sdk/pkg/eventbus"
)

type ProjectService struct {
	repo      project.Repository
	publisher eventbus.EventBus
}

func NewProjectService(repo project.Repository, publisher eventbus.EventBus) *ProjectService {
	return &ProjectService{
		repo:      repo,
		publisher: publisher,
	}
}

func (s *ProjectService) GetByID(ctx context.Context, id uint) (*project.Project, error) {
	return s.repo.GetByID(ctx, id)
}

func (s *ProjectService) GetAll(ctx context.Context) ([]*project.Project, error) {
	return s.repo.GetAll(ctx)
}

func (s *ProjectService) GetPaginated(
	ctx context.Context,
	limit, offset int,
	sortBy []string,
) ([]*project.Project, error) {
	return s.repo.GetPaginated(ctx, limit, offset, sortBy)
}

func (s *ProjectService) Create(ctx context.Context, data *project.CreateDTO) error {
	entity := data.ToEntity()
	if err := s.repo.Create(ctx, entity); err != nil {
		return err
	}
	createdEvent, err := project.NewCreatedEvent(ctx, *data, *entity)
	if err != nil {
		return err
	}
	s.publisher.Publish(createdEvent)
	return nil
}

func (s *ProjectService) Update(ctx context.Context, id uint, data *project.UpdateDTO) error {
	entity := data.ToEntity(id)
	if err := s.repo.Update(ctx, entity); err != nil {
		return err
	}
	updatedEvent, err := project.NewUpdatedEvent(ctx, *data, *entity)
	if err != nil {
		return err
	}
	s.publisher.Publish(updatedEvent)
	return nil
}

func (s *ProjectService) Delete(ctx context.Context, id uint) (*project.Project, error) {
	entity, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if err := s.repo.Delete(ctx, id); err != nil {
		return nil, err
	}
	deletedEvent, err := project.NewDeletedEvent(ctx, *entity)
	if err != nil {
		return nil, err
	}
	s.publisher.Publish(deletedEvent)
	return entity, nil
}
