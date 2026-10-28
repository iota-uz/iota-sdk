// Package services provides this package.
package services

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/modules/logging/history"
	projectstage "github.com/iota-uz/iota-sdk/modules/projects/domain/aggregates/project_stage"
	"github.com/iota-uz/iota-sdk/modules/projects/infrastructure/persistence"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/eventbus"
)

type ProjectStageService struct {
	repo          projectstage.Repository
	publisher     eventbus.EventBus
	historyWriter history.Writer
}

func NewProjectStageService(repo projectstage.Repository, publisher eventbus.EventBus, historyWriter history.Writer) *ProjectStageService {
	return &ProjectStageService{
		repo:          repo,
		publisher:     publisher,
		historyWriter: historyWriter,
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

	var savedStage projectstage.ProjectStage
	err := composables.InTx(ctx, func(txCtx context.Context) error {
		// Auto-generate stage number if not set and it's a new stage
		if isNew && stage.StageNumber() == 0 {
			nextNumber, err := s.repo.GetNextStageNumber(txCtx, stage.ProjectID())
			if err != nil {
				return err
			}
			stage = stage.UpdateStageNumber(nextNumber)
		}
		var before projectstage.ProjectStage
		if !isNew {
			existing, err := s.repo.GetByID(txCtx, stage.ID())
			if err != nil && !errors.Is(err, persistence.ErrProjectStageNotFound) {
				return err
			}
			before = existing
		}
		var err error
		savedStage, err = s.repo.Save(txCtx, stage)
		if err != nil {
			return err
		}
		m := history.Mutation{Type: ProjectStageCreated, Version: 1, Resource: ProjectStageResource, ID: savedStage.ID().String(), After: projectStageSnapshot(savedStage)}
		if before != nil {
			m.Type, m.Before = ProjectStageUpdated, projectStageSnapshot(before)
		}
		return history.RecordMutation(txCtx, s.historyWriter, m)
	})
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
	var entity projectstage.ProjectStage
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
			Type: ProjectStageDeleted, Version: 1, Resource: ProjectStageResource, ID: id.String(),
			Before: projectStageSnapshot(entity),
		})
	})
	if err != nil {
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
