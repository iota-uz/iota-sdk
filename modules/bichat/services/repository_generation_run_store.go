package services

import (
	"context"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/pkg/bichat/domain"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
)

type repositoryGenerationRunStore struct {
	repo domain.GenerationRunRepository
}

func newRepositoryGenerationRunStore(repo domain.GenerationRunRepository) generationRunStore {
	if repo == nil {
		return nil
	}
	return &repositoryGenerationRunStore{repo: repo}
}

func (s *repositoryGenerationRunStore) CreateRun(ctx context.Context, run domain.GenerationRun) error {
	const op serrors.Op = "repositoryGenerationRunStore.CreateRun"
	if err := s.repo.CreateRun(ctx, run); err != nil {
		return serrors.E(op, err)
	}
	return nil
}

func (s *repositoryGenerationRunStore) GetActiveRunBySession(ctx context.Context, _ uuid.UUID, sessionID uuid.UUID) (domain.GenerationRun, error) {
	const op serrors.Op = "repositoryGenerationRunStore.GetActiveRunBySession"
	run, err := s.repo.GetActiveRunBySession(ctx, sessionID)
	if err != nil {
		return nil, serrors.E(op, err)
	}
	return run, nil
}

func (s *repositoryGenerationRunStore) GetRunByID(ctx context.Context, _ uuid.UUID, runID uuid.UUID) (domain.GenerationRun, error) {
	const op serrors.Op = "repositoryGenerationRunStore.GetRunByID"
	run, err := s.repo.GetRunByID(ctx, runID)
	if err != nil {
		return nil, serrors.E(op, err)
	}
	return run, nil
}

func (s *repositoryGenerationRunStore) UpdateRunSnapshot(ctx context.Context, _, _, runID uuid.UUID, partialContent string, partialMetadata map[string]any) error {
	const op serrors.Op = "repositoryGenerationRunStore.UpdateRunSnapshot"
	if err := s.repo.UpdateRunSnapshot(ctx, runID, partialContent, partialMetadata); err != nil {
		return serrors.E(op, err)
	}
	return nil
}

func (s *repositoryGenerationRunStore) CompleteRun(ctx context.Context, _, _, runID uuid.UUID) error {
	const op serrors.Op = "repositoryGenerationRunStore.CompleteRun"
	if err := s.repo.CompleteRun(ctx, runID); err != nil {
		return serrors.E(op, err)
	}
	return nil
}

func (s *repositoryGenerationRunStore) CancelRun(ctx context.Context, _, _, runID uuid.UUID) error {
	const op serrors.Op = "repositoryGenerationRunStore.CancelRun"
	if err := s.repo.CancelRun(ctx, runID); err != nil {
		return serrors.E(op, err)
	}
	return nil
}
