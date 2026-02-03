package services

import (
	"context"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/pkg/bichat/domain"
	"github.com/iota-uz/iota-sdk/pkg/bichat/storage"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
)

// ArtifactService provides artifact read, update, and delete with optional file cleanup.
type ArtifactService interface {
	GetSessionArtifacts(ctx context.Context, sessionID uuid.UUID, opts domain.ListOptions) ([]domain.Artifact, error)
	GetArtifact(ctx context.Context, id uuid.UUID) (domain.Artifact, error)
	DeleteArtifact(ctx context.Context, id uuid.UUID) error
	UpdateArtifact(ctx context.Context, id uuid.UUID, name, description string) (domain.Artifact, error)
}

type artifactService struct {
	repo    domain.ChatRepository
	storage storage.FileStorage
}

// NewArtifactService creates a new ArtifactService.
func NewArtifactService(repo domain.ChatRepository, fileStorage storage.FileStorage) ArtifactService {
	return &artifactService{
		repo:    repo,
		storage: fileStorage,
	}
}

// GetSessionArtifacts returns artifacts for a session with pagination and optional type filter.
func (s *artifactService) GetSessionArtifacts(ctx context.Context, sessionID uuid.UUID, opts domain.ListOptions) ([]domain.Artifact, error) {
	const op serrors.Op = "ArtifactService.GetSessionArtifacts"
	list, err := s.repo.GetSessionArtifacts(ctx, sessionID, opts)
	if err != nil {
		return nil, serrors.E(op, err)
	}
	return list, nil
}

// GetArtifact returns an artifact by ID.
func (s *artifactService) GetArtifact(ctx context.Context, id uuid.UUID) (domain.Artifact, error) {
	const op serrors.Op = "ArtifactService.GetArtifact"
	a, err := s.repo.GetArtifact(ctx, id)
	if err != nil {
		return nil, serrors.E(op, err)
	}
	return a, nil
}

// DeleteArtifact removes the artifact and, if it has a file URL, deletes the file from storage (best effort).
func (s *artifactService) DeleteArtifact(ctx context.Context, id uuid.UUID) error {
	const op serrors.Op = "ArtifactService.DeleteArtifact"

	artifact, err := s.repo.GetArtifact(ctx, id)
	if err != nil {
		return serrors.E(op, err)
	}

	if artifact.URL() != "" && s.storage != nil {
		_ = s.storage.Delete(ctx, artifact.URL())
	}

	if err := s.repo.DeleteArtifact(ctx, id); err != nil {
		return serrors.E(op, err)
	}
	return nil
}

// UpdateArtifact updates an artifact's name and description and returns the updated artifact.
func (s *artifactService) UpdateArtifact(ctx context.Context, id uuid.UUID, name, description string) (domain.Artifact, error) {
	const op serrors.Op = "ArtifactService.UpdateArtifact"

	if err := s.repo.UpdateArtifact(ctx, id, name, description); err != nil {
		return nil, serrors.E(op, err)
	}
	artifact, err := s.repo.GetArtifact(ctx, id)
	if err != nil {
		return nil, serrors.E(op, err)
	}
	return artifact, nil
}
