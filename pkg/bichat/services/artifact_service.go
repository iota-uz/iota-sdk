package services

import (
	"context"

	"github.com/google/uuid"
	corepersistence "github.com/iota-uz/iota-sdk/modules/core/infrastructure/persistence"
	"github.com/iota-uz/iota-sdk/pkg/bichat/domain"
	"github.com/iota-uz/iota-sdk/pkg/bichat/storage"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
)

// ArtifactService provides artifact read, update, and delete with optional file cleanup.
type ArtifactService interface {
	GetSessionArtifacts(ctx context.Context, sessionID uuid.UUID, opts domain.ListOptions) ([]domain.Artifact, error)
	GetArtifact(ctx context.Context, id uuid.UUID) (domain.Artifact, error)
	DeleteArtifact(ctx context.Context, id uuid.UUID) error
	UpdateArtifact(ctx context.Context, id uuid.UUID, name, description string) (domain.Artifact, error)
	UploadSessionArtifacts(ctx context.Context, sessionID uuid.UUID, uploads []ArtifactUpload) ([]domain.Artifact, error)
}

type ArtifactUpload struct {
	UploadID int64
}

type artifactService struct {
	repo    domain.ChatRepository
	storage storage.FileStorage
}

// NewArtifactService creates a new ArtifactService.
func NewArtifactService(repo domain.ChatRepository, fileStorage storage.FileStorage, _ AttachmentService) ArtifactService {
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

// DeleteArtifact removes the artifact and, for legacy non-upload artifacts, deletes local storage files (best effort).
func (s *artifactService) DeleteArtifact(ctx context.Context, id uuid.UUID) error {
	const op serrors.Op = "ArtifactService.DeleteArtifact"

	artifact, err := s.repo.GetArtifact(ctx, id)
	if err != nil {
		return serrors.E(op, err)
	}

	if artifact.UploadID() == nil && artifact.URL() != "" && s.storage != nil {
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

// UploadSessionArtifacts links existing core uploads as session attachment artifacts without creating chat turns.
func (s *artifactService) UploadSessionArtifacts(ctx context.Context, sessionID uuid.UUID, uploads []ArtifactUpload) ([]domain.Artifact, error) {
	const op serrors.Op = "ArtifactService.UploadSessionArtifacts"

	if len(uploads) == 0 {
		return nil, serrors.E(op, serrors.KindValidation, "no uploads provided")
	}

	session, err := s.repo.GetSession(ctx, sessionID)
	if err != nil {
		return nil, serrors.E(op, err)
	}

	uploadRepo := corepersistence.NewUploadRepository()

	var artifacts []domain.Artifact
	err = composables.InTx(ctx, func(txCtx context.Context) error {
		artifacts = make([]domain.Artifact, 0, len(uploads))
		for _, upload := range uploads {
			if upload.UploadID <= 0 {
				return serrors.E(op, serrors.KindValidation, "uploadId must be a positive integer")
			}

			found, err := uploadRepo.GetByIDs(txCtx, []uint{uint(upload.UploadID)})
			if err != nil {
				return err
			}
			if len(found) == 0 {
				return serrors.E(op, serrors.KindValidation, "upload not found")
			}
			entity := found[0]
			mimeType := ""
			if entity.Mimetype() != nil {
				mimeType = entity.Mimetype().String()
			}
			url := entity.URL().String()
			sizeBytes := int64(entity.Size().Bytes())
			uploadID := int64(entity.ID())

			artifact := domain.NewArtifact(
				domain.WithArtifactTenantID(session.TenantID()),
				domain.WithArtifactSessionID(sessionID),
				domain.WithArtifactType(domain.ArtifactTypeAttachment),
				domain.WithArtifactName(entity.Name()),
				domain.WithArtifactMimeType(mimeType),
				domain.WithArtifactURL(url),
				domain.WithArtifactSizeBytes(sizeBytes),
				domain.WithArtifactUploadID(uploadID),
			)
			if err := s.repo.SaveArtifact(txCtx, artifact); err != nil {
				return err
			}
			artifacts = append(artifacts, artifact)
		}
		return nil
	})
	if err != nil {
		return nil, serrors.E(op, err)
	}
	return artifacts, nil
}
