package services

import (
	"context"
	"github.com/iota-uz/iota-sdk/pkg/event"

	stage "github.com/iota-uz/iota-sdk/pkg/domain/entities/project_stages"
)

type ProjectStageService struct {
	repo      stage.Repository
	publisher event.Publisher
}

func NewProjectStageService(repo stage.Repository, publisher event.Publisher) *ProjectStageService {
	return &ProjectStageService{
		repo:      repo,
		publisher: publisher,
	}
}

func (s *ProjectStageService) Count(ctx context.Context) (uint, error) {
	return s.repo.Count(ctx)
}

func (s *ProjectStageService) GetAll(ctx context.Context) ([]*stage.ProjectStage, error) {
	return s.repo.GetAll(ctx)
}

func (s *ProjectStageService) GetByID(ctx context.Context, id uint) (*stage.ProjectStage, error) {
	return s.repo.GetByID(ctx, id)
}

func (s *ProjectStageService) GetPaginated(
	ctx context.Context,
	limit, offset int,
	sortBy []string,
) ([]*stage.ProjectStage, error) {
	return s.repo.GetPaginated(ctx, limit, offset, sortBy)
}

func (s *ProjectStageService) Create(ctx context.Context, data *stage.CreateDTO) error {
	createdEvent, err := stage.NewCreatedEvent(ctx, *data)
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

func (s *ProjectStageService) Update(ctx context.Context, id uint, data *stage.UpdateDTO) error {
	updatedEvent, err := stage.NewUpdatedEvent(ctx, *data)
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

func (s *ProjectStageService) Delete(ctx context.Context, id uint) (*stage.ProjectStage, error) {
	deletedEvent, err := stage.NewDeletedEvent(ctx)
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
