package services

import (
	"bytes"
	"context"
	"fmt"

	"github.com/google/uuid"
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
	Filename  string
	MimeType  string
	SizeBytes int64
	Data      []byte
}

type artifactService struct {
	repo              domain.ChatRepository
	storage           storage.FileStorage
	attachmentService AttachmentService
}

// NewArtifactService creates a new ArtifactService.
func NewArtifactService(repo domain.ChatRepository, fileStorage storage.FileStorage, attachmentService AttachmentService) ArtifactService {
	return &artifactService{
		repo:              repo,
		storage:           fileStorage,
		attachmentService: attachmentService,
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

// UploadSessionArtifacts stores files and saves them as attachment artifacts without creating chat turns.
func (s *artifactService) UploadSessionArtifacts(ctx context.Context, sessionID uuid.UUID, uploads []ArtifactUpload) ([]domain.Artifact, error) {
	const op serrors.Op = "ArtifactService.UploadSessionArtifacts"

	if s.attachmentService == nil {
		return nil, serrors.E(op, serrors.Internal, "attachment service is not configured")
	}
	if len(uploads) == 0 {
		return nil, serrors.E(op, serrors.KindValidation, "no uploads provided")
	}

	session, err := s.repo.GetSession(ctx, sessionID)
	if err != nil {
		return nil, serrors.E(op, err)
	}

	u, err := composables.UseUser(ctx)
	if err != nil || u == nil {
		return nil, serrors.E(op, serrors.PermissionDenied, "upload requires an authenticated user", err)
	}
	uploaderID := uuid.NewSHA1(uuid.NameSpaceOID, []byte(fmt.Sprintf("user:%d", u.ID())))

	files := make([]FileUpload, 0, len(uploads))
	for _, upload := range uploads {
		files = append(files, FileUpload{
			Filename: upload.Filename,
			MimeType: upload.MimeType,
			Size:     upload.SizeBytes,
		})
	}
	if err := s.attachmentService.ValidateMultiple(files); err != nil {
		return nil, serrors.E(op, err)
	}

	var artifacts []domain.Artifact
	var savedPaths []string
	err = composables.InTx(ctx, func(txCtx context.Context) error {
		artifacts = make([]domain.Artifact, 0, len(uploads))
		savedPaths = make([]string, 0, len(uploads))
		for _, upload := range uploads {
			attachment, err := s.attachmentService.ValidateAndSave(
				txCtx,
				upload.Filename,
				upload.MimeType,
				upload.SizeBytes,
				bytes.NewReader(upload.Data),
				session.TenantID(),
				uploaderID,
			)
			if err != nil {
				return err
			}
			savedPaths = append(savedPaths, attachment.FilePath())

			artifact := domain.NewArtifact(
				domain.WithArtifactTenantID(session.TenantID()),
				domain.WithArtifactSessionID(sessionID),
				domain.WithArtifactType(domain.ArtifactTypeAttachment),
				domain.WithArtifactName(attachment.FileName()),
				domain.WithArtifactMimeType(attachment.MimeType()),
				domain.WithArtifactURL(attachment.FilePath()),
				domain.WithArtifactSizeBytes(attachment.SizeBytes()),
			)
			if err := s.repo.SaveArtifact(txCtx, artifact); err != nil {
				return err
			}
			artifacts = append(artifacts, artifact)
		}
		return nil
	})
	if err != nil {
		s.attachmentService.DeleteFiles(ctx, savedPaths)
		return nil, serrors.E(op, err)
	}
	return artifacts, nil
}
