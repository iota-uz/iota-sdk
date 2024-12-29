package services

import (
	"context"
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/project_stages"
	"github.com/iota-uz/iota-sdk/pkg/event"
)

type ProjectStageService struct {
	repo      project_stages.Repository
	publisher event.Publisher
}

func NewProjectStageService(repo project_stages.Repository, publisher event.Publisher) *ProjectStageService {
	return &ProjectStageService{
		repo:      repo,
		publisher: publisher,
	}
}

func (s *ProjectStageService) Count(ctx context.Context) (uint, error) {
	return s.repo.Count(ctx)
}

func (s *ProjectStageService) GetAll(ctx context.Context) ([]*project_stages.ProjectStage, error) {
	return s.repo.GetAll(ctx)
}

func (s *ProjectStageService) GetByID(ctx context.Context, id uint) (*project_stages.ProjectStage, error) {
	return s.repo.GetByID(ctx, id)
}

func (s *ProjectStageService) GetPaginated(
	ctx context.Context,
	limit, offset int,
	sortBy []string,
) ([]*project_stages.ProjectStage, error) {
	return s.repo.GetPaginated(ctx, limit, offset, sortBy)
}

func (s *ProjectStageService) Create(ctx context.Context, data *project_stages.CreateDTO) error {
	createdEvent, err := project_stages.NewCreatedEvent(ctx, *data)
	if err != nil {
		return err
	}
	entity := data.ToEntity()
	if err := s.repo.Create(ctx, entity); err != nil {
		return err
	}
	createdEvent.Result = *entity
	s.publisher.Publish(createdEvent)
	return nil
}

func (s *ProjectStageService) Update(ctx context.Context, id uint, data *project_stages.UpdateDTO) error {
	updatedEvent, err := project_stages.NewUpdatedEvent(ctx, *data)
	if err != nil {
		return err
	}
	entity := data.ToEntity(id)
	if err := s.repo.Update(ctx, entity); err != nil {
		return err
	}
	updatedEvent.Result = *entity
	s.publisher.Publish(updatedEvent)
	return nil
}

func (s *ProjectStageService) Delete(ctx context.Context, id uint) (*project_stages.ProjectStage, error) {
	deletedEvent, err := project_stages.NewDeletedEvent(ctx)
	if err != nil {
		return nil, err
	}
	entity, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if err := s.repo.Delete(ctx, id); err != nil {
		return nil, err
	}
	deletedEvent.Result = *entity
	s.publisher.Publish(deletedEvent)
	return entity, nil
}
