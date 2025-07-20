package services

import (
	"context"

	"github.com/google/uuid"
	projectstage "github.com/iota-uz/iota-sdk/modules/projects/domain/aggregates/project_stage"
	"github.com/iota-uz/iota-sdk/pkg/eventbus"
)

type ProjectStageService struct {
	repo      projectstage.Repository
	publisher eventbus.EventBus
}

func NewProjectStageService(repo projectstage.Repository, publisher eventbus.EventBus) *ProjectStageService {
	return &ProjectStageService{
		repo:      repo,
		publisher: publisher,
	}
}

func (s *ProjectStageService) GetByID(ctx context.Context, id uuid.UUID) (projectstage.ProjectStage, error) {
	return s.repo.GetByID(ctx, id)
}

func (s *ProjectStageService) GetAll(ctx context.Context) ([]projectstage.ProjectStage, error) {
	return s.repo.GetAll(ctx)
}

func (s *ProjectStageService) GetPaginated(
	ctx context.Context,
	limit, offset int,
	sortBy []string,
) ([]projectstage.ProjectStage, error) {
	return s.repo.GetPaginated(ctx, limit, offset, sortBy)
}

func (s *ProjectStageService) Count(ctx context.Context) (int64, error) {
	return s.repo.Count(ctx)
}

func (s *ProjectStageService) GetByProjectID(ctx context.Context, projectID uuid.UUID) ([]projectstage.ProjectStage, error) {
	return s.repo.GetByProjectID(ctx, projectID)
}

func (s *ProjectStageService) Save(ctx context.Context, stage projectstage.ProjectStage) (projectstage.ProjectStage, error) {
	isNew := stage.ID() == uuid.Nil

	// Auto-generate stage number if not set and it's a new stage
	if isNew && stage.StageNumber() == 0 {
		nextNumber, err := s.repo.GetNextStageNumber(ctx, stage.ProjectID())
		if err != nil {
			return nil, err
		}
		stage = stage.UpdateStageNumber(nextNumber)
	}

	savedStage, err := s.repo.Save(ctx, stage)
	if err != nil {
		return nil, err
	}

	if isNew {
		createdEvent, err := projectstage.NewCreatedEvent(ctx, savedStage)
		if err != nil {
			return nil, err
		}
		s.publisher.Publish(createdEvent)
	} else {
		updatedEvent, err := projectstage.NewUpdatedEvent(ctx, savedStage)
		if err != nil {
			return nil, err
		}
		s.publisher.Publish(updatedEvent)
	}

	return savedStage, nil
}

func (s *ProjectStageService) Create(ctx context.Context, stage projectstage.ProjectStage) error {
	_, err := s.Save(ctx, stage)
	return err
}

func (s *ProjectStageService) Update(ctx context.Context, stage projectstage.ProjectStage) error {
	_, err := s.Save(ctx, stage)
	return err
}

func (s *ProjectStageService) Delete(ctx context.Context, id uuid.UUID) (projectstage.ProjectStage, error) {
	entity, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if err := s.repo.Delete(ctx, id); err != nil {
		return nil, err
	}
	deletedEvent, err := projectstage.NewDeletedEvent(ctx, entity)
	if err != nil {
		return nil, err
	}
	s.publisher.Publish(deletedEvent)
	return entity, nil
}

func (s *ProjectStageService) UpdatePaidAmounts(ctx context.Context, stageID uuid.UUID) error {
	return s.repo.UpdatePaidAmounts(ctx, stageID)
}
