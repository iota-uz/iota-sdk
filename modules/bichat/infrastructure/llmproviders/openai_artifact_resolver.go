package llmproviders

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/google/uuid"
	openai "github.com/openai/openai-go/v3"

	corepersistence "github.com/iota-uz/iota-sdk/modules/core/infrastructure/persistence"
	"github.com/iota-uz/iota-sdk/pkg/bichat/domain"
	"github.com/iota-uz/iota-sdk/pkg/bichat/logging"
)

const (
	codeInterpreterProviderOpenAI = "openai"
	maxOpenAIFileUploadBytes      = 512 << 20 // 512MB
)

type CodeInterpreterArtifactStorage interface {
	Get(ctx context.Context, path string) (io.ReadCloser, error)
}

type CodeInterpreterArtifactResolver interface {
	ResolveCodeInterpreterFileIDs(ctx context.Context, sessionID uuid.UUID, limit int) []string
}

type artifactProviderFileSyncRepository interface {
	GetArtifactProviderFile(ctx context.Context, artifactID uuid.UUID, provider string) (providerFileID, sourceURL string, sourceSizeBytes int64, err error)
	UpsertArtifactProviderFile(ctx context.Context, artifactID uuid.UUID, provider, providerFileID, sourceURL string, sourceSizeBytes int64) error
}

type OpenAICodeInterpreterArtifactResolver struct {
	client      *openai.Client
	chatRepo    domain.ChatRepository
	fileStorage CodeInterpreterArtifactStorage
	logger      logging.Logger
}

func NewOpenAICodeInterpreterArtifactResolver(
	client *openai.Client,
	chatRepo domain.ChatRepository,
	fileStorage CodeInterpreterArtifactStorage,
	logger logging.Logger,
) *OpenAICodeInterpreterArtifactResolver {
	if logger == nil {
		logger = logging.NewNoOpLogger()
	}
	return &OpenAICodeInterpreterArtifactResolver{
		client:      client,
		chatRepo:    chatRepo,
		fileStorage: fileStorage,
		logger:      logger,
	}
}

func (r *OpenAICodeInterpreterArtifactResolver) ResolveCodeInterpreterFileIDs(
	ctx context.Context,
	sessionID uuid.UUID,
	limit int,
) []string {
	if r == nil || r.chatRepo == nil || r.client == nil {
		return nil
	}

	artifacts, err := r.chatRepo.GetSessionArtifacts(ctx, sessionID, domain.ListOptions{
		Limit:  limit,
		Offset: 0,
		Types:  []domain.ArtifactType{domain.ArtifactTypeAttachment},
	})
	if err != nil {
		r.logger.Warn(ctx, "failed to list session artifacts for code_interpreter files", map[string]any{
			"session_id": sessionID.String(),
			"error":      err.Error(),
		})
		return nil
	}

	fileIDs := make([]string, 0, len(artifacts))
	seenSources := make(map[string]struct{}, len(artifacts))
	var syncRepo artifactProviderFileSyncRepository
	if repo, ok := r.chatRepo.(artifactProviderFileSyncRepository); ok {
		syncRepo = repo
	}
	uploadRepo := corepersistence.NewUploadRepository()

	for _, artifact := range artifacts {
		sourceURL := strings.TrimSpace(artifact.URL())
		sourceSizeBytes := artifact.SizeBytes()
		filename := strings.TrimSpace(artifact.Name())
		contentType := strings.TrimSpace(artifact.MimeType())

		var sourceKey string
		var data []byte
		if artifact.UploadID() != nil && *artifact.UploadID() > 0 {
			uploadID := uint(*artifact.UploadID())
			sourceKey = fmt.Sprintf("upload:%d", uploadID)
			if _, exists := seenSources[sourceKey]; exists {
				continue
			}

			uploads, err := uploadRepo.GetByIDs(ctx, []uint{uploadID})
			if err != nil {
				r.logger.Warn(ctx, "failed to resolve upload-backed artifact for code_interpreter", map[string]any{
					"session_id":  sessionID.String(),
					"artifact_id": artifact.ID().String(),
					"upload_id":   uploadID,
					"error":       err.Error(),
				})
				continue
			}
			if len(uploads) == 0 {
				r.logger.Warn(ctx, "upload-backed artifact points to missing upload", map[string]any{
					"session_id":  sessionID.String(),
					"artifact_id": artifact.ID().String(),
					"upload_id":   uploadID,
				})
				continue
			}

			entity := uploads[0]
			if sourceURL == "" {
				sourceURL = entity.URL().String()
			}
			if sourceSizeBytes <= 0 {
				sourceSizeBytes = int64(entity.Size().Bytes())
			}
			if filename == "" {
				filename = strings.TrimSpace(entity.Name())
			}
			if contentType == "" && entity.Mimetype() != nil {
				contentType = strings.TrimSpace(entity.Mimetype().String())
			}

			data, err = os.ReadFile(entity.Path())
			if err != nil {
				r.logger.Warn(ctx, "failed to read upload-backed artifact content for code_interpreter upload", map[string]any{
					"session_id":  sessionID.String(),
					"artifact_id": artifact.ID().String(),
					"upload_id":   uploadID,
					"path":        entity.Path(),
					"error":       err.Error(),
				})
				continue
			}
		} else {
			fileURL := strings.TrimSpace(artifact.URL())
			if fileURL == "" || r.fileStorage == nil {
				continue
			}
			sourceKey = "url:" + fileURL
			if _, exists := seenSources[sourceKey]; exists {
				continue
			}

			rc, err := r.fileStorage.Get(ctx, fileURL)
			if err != nil {
				r.logger.Warn(ctx, "failed to open artifact content for code_interpreter upload", map[string]any{
					"session_id":  sessionID.String(),
					"artifact_id": artifact.ID().String(),
					"url":         fileURL,
					"error":       err.Error(),
				})
				continue
			}

			readData, readErr := io.ReadAll(io.LimitReader(rc, maxOpenAIFileUploadBytes+1))
			_ = rc.Close()
			if readErr != nil {
				r.logger.Warn(ctx, "failed to read artifact content for code_interpreter upload", map[string]any{
					"session_id":  sessionID.String(),
					"artifact_id": artifact.ID().String(),
					"url":         fileURL,
					"error":       readErr.Error(),
				})
				continue
			}
			data = readData
		}

		if sourceKey == "" {
			continue
		}
		seenSources[sourceKey] = struct{}{}

		if syncRepo != nil {
			providerFileID, mappedSourceURL, mappedSourceSizeBytes, mapErr := syncRepo.GetArtifactProviderFile(
				ctx,
				artifact.ID(),
				codeInterpreterProviderOpenAI,
			)
			if mapErr == nil {
				if strings.TrimSpace(providerFileID) != "" &&
					mappedSourceURL == sourceURL &&
					mappedSourceSizeBytes == sourceSizeBytes {
					fileIDs = append(fileIDs, providerFileID)
					if len(fileIDs) >= limit {
						break
					}
					continue
				}
			}
		}
		if int64(len(data)) > maxOpenAIFileUploadBytes {
			r.logger.Warn(ctx, "artifact exceeds OpenAI file upload size limit", map[string]any{
				"session_id":        sessionID.String(),
				"artifact_id":       artifact.ID().String(),
				"url":               sourceURL,
				"size_bytes":        int64(len(data)),
				"max_allowed_bytes": maxOpenAIFileUploadBytes,
			})
			continue
		}
		if len(data) == 0 {
			continue
		}

		if filename == "" {
			filename = "artifact.bin"
		}

		uploaded, uploadErr := r.client.Files.New(ctx, openai.FileNewParams{
			File:    openai.File(bytes.NewReader(data), filename, contentType),
			Purpose: openai.FilePurposeAssistants,
		})
		if uploadErr != nil {
			r.logger.Warn(ctx, "failed to upload artifact to OpenAI for code_interpreter", map[string]any{
				"session_id":  sessionID.String(),
				"artifact_id": artifact.ID().String(),
				"filename":    filename,
				"mime_type":   contentType,
				"error":       uploadErr.Error(),
			})
			continue
		}
		if strings.TrimSpace(uploaded.ID) == "" {
			continue
		}

		if syncRepo != nil {
			if err := syncRepo.UpsertArtifactProviderFile(
				ctx,
				artifact.ID(),
				codeInterpreterProviderOpenAI,
				uploaded.ID,
				sourceURL,
				sourceSizeBytes,
			); err != nil {
				r.logger.Warn(ctx, "failed to persist artifact/provider file mapping", map[string]any{
					"session_id":  sessionID.String(),
					"artifact_id": artifact.ID().String(),
					"provider":    codeInterpreterProviderOpenAI,
					"file_id":     uploaded.ID,
					"error":       err.Error(),
				})
			}
		}

		fileIDs = append(fileIDs, uploaded.ID)
		if len(fileIDs) >= limit {
			break
		}
	}

	return fileIDs
}
