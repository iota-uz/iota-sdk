// Package services provides this package.
package services

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/modules/logging/history"
	"github.com/iota-uz/iota-sdk/modules/projects/domain/aggregates/project"
	"github.com/iota-uz/iota-sdk/modules/projects/infrastructure/persistence"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/eventbus"
)

type ProjectService struct {
	repo          project.Repository
	publisher     eventbus.EventBus
	historyWriter history.Writer
}

func NewProjectService(repo project.Repository, publisher eventbus.EventBus, historyWriter history.Writer) *ProjectService {
	return &ProjectService{
		repo:          repo,
		publisher:     publisher,
		historyWriter: historyWriter,
	}
}

func (s *ProjectService) GetByID(ctx context.Context, id uuid.UUID) (project.Project, error) {
	return s.repo.GetByID(ctx, id)
}

func (s *ProjectService) GetAll(ctx context.Context) ([]project.Project, error) {
	return s.repo.GetAll(ctx)
}

func (s *ProjectService) GetPaginated(
	ctx context.Context,
	limit, offset int,
	sortBy []string,
) ([]project.Project, error) {
	return s.repo.GetPaginated(ctx, limit, offset, sortBy)
}

func (s *ProjectService) GetByCounterpartyID(ctx context.Context, counterpartyID uuid.UUID) ([]project.Project, error) {
	return s.repo.GetByCounterpartyID(ctx, counterpartyID)
}

func (s *ProjectService) Save(ctx context.Context, proj project.Project) (project.Project, error) {
	isNew := proj.ID() == uuid.Nil

	var savedProj project.Project
	err := composables.InTx(ctx, func(txCtx context.Context) error {
		var before project.Project
		if !isNew {
			existing, err := s.repo.GetByID(txCtx, proj.ID())
			if err != nil && !errors.Is(err, persistence.ErrProjectNotFound) {
				return err
			}
			before = existing
		}
		var err error
		savedProj, err = s.repo.Save(txCtx, proj)
		if err != nil {
			return err
		}
		m := history.Mutation{Type: ProjectCreated, Version: 1, Resource: ProjectResource, ID: savedProj.ID().String(), After: projectSnapshot(savedProj)}
		if before != nil {
			m.Type, m.Before = ProjectUpdated, projectSnapshot(before)
		}
		return history.RecordMutation(txCtx, s.historyWriter, m)
	})
	if err != nil {
		return nil, err
	}

	if isNew {
		createdEvent, err := project.NewCreatedEvent(ctx, savedProj)
		if err != nil {
			return nil, err
		}
		s.publisher.Publish(createdEvent)
	} else {
		updatedEvent, err := project.NewUpdatedEvent(ctx, savedProj)
		if err != nil {
			return nil, err
		}
		s.publisher.Publish(updatedEvent)
	}

	return savedProj, nil
}

func (s *ProjectService) Create(ctx context.Context, proj project.Project) error {
	_, err := s.Save(ctx, proj)
	return err
}

func (s *ProjectService) Update(ctx context.Context, proj project.Project) error {
	_, err := s.Save(ctx, proj)
	return err
}

func (s *ProjectService) Delete(ctx context.Context, id uuid.UUID) (project.Project, error) {
	var entity project.Project
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
			Type: ProjectDeleted, Version: 1, Resource: ProjectResource, ID: id.String(),
			Before: projectSnapshot(entity),
		})
	})
	if err != nil {
		return nil, err
	}
	deletedEvent, err := project.NewDeletedEvent(ctx, entity)
	if err != nil {
		return nil, err
	}
	s.publisher.Publish(deletedEvent)
	return entity, nil
}
