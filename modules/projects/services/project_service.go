package services

import (
	"context"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/modules/projects/domain/aggregates/project"
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

	savedProj, err := s.repo.Save(ctx, proj)
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
	entity, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if err := s.repo.Delete(ctx, id); err != nil {
		return nil, err
	}
	deletedEvent, err := project.NewDeletedEvent(ctx, entity)
	if err != nil {
		return nil, err
	}
	s.publisher.Publish(deletedEvent)
	return entity, nil
}
