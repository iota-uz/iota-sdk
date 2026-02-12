package services

import (
	"context"
	"fmt"
	"io"
	"mime"
	"path/filepath"
	"strings"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/pkg/bichat/domain"
	bichatservices "github.com/iota-uz/iota-sdk/pkg/bichat/services"
	"github.com/iota-uz/iota-sdk/pkg/bichat/storage"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
)

var supportedAttachmentTypes = map[string]bool{
	"image/jpeg":         true,
	"image/jpg":          true,
	"image/png":          true,
	"image/gif":          true,
	"image/webp":         true,
	"application/pdf":    true,
	"application/msword": true,
	"application/vnd.openxmlformats-officedocument.wordprocessingml.document": true,
	"application/vnd.ms-excel": true,
	"application/vnd.openxmlformats-officedocument.spreadsheetml.sheet": true,
	"text/csv":                  true,
	"text/tab-separated-values": true,
	"text/plain":                true,
	"text/markdown":             true,
	"application/json":          true,
	"application/xml":           true,
	"text/xml":                  true,
	"application/yaml":          true,
	"text/yaml":                 true,
	"application/x-yaml":        true,
	"text/x-yaml":               true,
	"text/log":                  true,
}

const (
	maxAttachmentSize  = 20 * 1024 * 1024 // 20MB
	maxAttachmentCount = 10
)

var extensionToMIME = map[string]string{
	".png":  "image/png",
	".jpg":  "image/jpeg",
	".jpeg": "image/jpeg",
	".gif":  "image/gif",
	".webp": "image/webp",
	".pdf":  "application/pdf",
	".doc":  "application/msword",
	".docx": "application/vnd.openxmlformats-officedocument.wordprocessingml.document",
	".xls":  "application/vnd.ms-excel",
	".xlsx": "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
	".csv":  "text/csv",
	".tsv":  "text/tab-separated-values",
	".txt":  "text/plain",
	".md":   "text/markdown",
	".json": "application/json",
	".xml":  "application/xml",
	".yaml": "application/yaml",
	".yml":  "application/yaml",
	".log":  "text/plain",
}

type attachmentService struct {
	storage storage.FileStorage
}

// NewAttachmentService creates a new attachment service
func NewAttachmentService(storage storage.FileStorage) bichatservices.AttachmentService {
	return &attachmentService{
		storage: storage,
	}
}

// ValidateAndSave validates an attachment upload and saves it to storage.
func (s *attachmentService) ValidateAndSave(
	ctx context.Context,
	filename string,
	mimeType string,
	size int64,
	reader io.Reader,
	tenantID, userID uuid.UUID,
) (domain.Attachment, error) {
	const op serrors.Op = "AttachmentService.ValidateAndSave"

	canonicalMime, err := normalizeAttachmentMimeType(filename, mimeType)
	if err != nil {
		return nil, serrors.E(op, err)
	}

	if size > maxAttachmentSize {
		return nil, serrors.E(
			op,
			serrors.KindValidation,
			fmt.Sprintf("attachment too large: %d bytes (max: %d bytes / 20MB)", size, maxAttachmentSize),
		)
	}

	// Save to storage
	// Note: Tenant isolation should be handled by storage path structure
	metadata := storage.FileMetadata{
		ContentType: canonicalMime,
		Size:        size,
	}

	url, err := s.storage.Save(ctx, filename, reader, metadata)
	if err != nil {
		return nil, serrors.E(op, serrors.Internal, "failed to save attachment", err)
	}

	attachment := domain.NewAttachment(
		domain.WithFileName(filename),
		domain.WithMimeType(canonicalMime),
		domain.WithSizeBytes(size),
		domain.WithFilePath(url),
	)

	return attachment, nil
}

// ValidateMultiple validates a batch of uploads without saving.
func (s *attachmentService) ValidateMultiple(files []bichatservices.FileUpload) error {
	const op serrors.Op = "AttachmentService.ValidateMultiple"

	if len(files) > maxAttachmentCount {
		return serrors.E(
			op,
			serrors.KindValidation,
			fmt.Sprintf("too many attachments: %d (max: %d)", len(files), maxAttachmentCount),
		)
	}

	for i, file := range files {
		if _, err := normalizeAttachmentMimeType(file.Filename, file.MimeType); err != nil {
			return serrors.E(op, serrors.KindValidation, fmt.Sprintf("attachment %d: %v", i+1, err))
		}

		if file.Size > maxAttachmentSize {
			return serrors.E(
				op,
				serrors.KindValidation,
				fmt.Sprintf("attachment %d too large: %d bytes (max: 20MB)", i+1, file.Size),
			)
		}
	}

	return nil
}

// DeleteFiles removes the given storage paths (best effort).
func (s *attachmentService) DeleteFiles(ctx context.Context, paths []string) {
	for _, path := range paths {
		if path == "" {
			continue
		}
		_ = s.storage.Delete(ctx, path)
	}
}

func normalizeAttachmentMimeType(filename string, mimeType string) (string, error) {
	normalized := strings.ToLower(strings.TrimSpace(strings.Split(mimeType, ";")[0]))
	ext := strings.ToLower(filepath.Ext(strings.TrimSpace(filename)))

	if normalized != "" && supportedAttachmentTypes[normalized] {
		return normalized, nil
	}

	if ext == "" {
		return "", fmt.Errorf("unsupported attachment type: %s", mimeType)
	}

	if inferred, ok := extensionToMIME[ext]; ok {
		return inferred, nil
	}

	inferred := strings.ToLower(strings.TrimSpace(strings.Split(mime.TypeByExtension(ext), ";")[0]))
	if inferred != "" && supportedAttachmentTypes[inferred] {
		return inferred, nil
	}

	return "", fmt.Errorf("unsupported attachment extension: %s", ext)
}
