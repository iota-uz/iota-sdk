package services

import (
	"context"
	"fmt"
	"io"
	"mime"
	"path/filepath"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/pkg/bichat/domain"
	bichatservices "github.com/iota-uz/iota-sdk/pkg/bichat/services"
	"github.com/iota-uz/iota-sdk/pkg/bichat/storage"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
)

// Supported image MIME types
var supportedImageTypes = map[string]bool{
	"image/jpeg": true,
	"image/png":  true,
	"image/gif":  true,
	"image/webp": true,
}

const (
	maxImageSize  = 20 * 1024 * 1024 // 20MB
	maxImageCount = 10
)

type attachmentService struct {
	storage storage.FileStorage
}

// NewAttachmentService creates a new attachment service
func NewAttachmentService(storage storage.FileStorage) bichatservices.AttachmentService {
	return &attachmentService{
		storage: storage,
	}
}

// ValidateAndSave validates an image upload and saves it to storage
func (s *attachmentService) ValidateAndSave(
	ctx context.Context,
	filename string,
	mimeType string,
	size int64,
	reader io.Reader,
	tenantID, userID uuid.UUID,
) (domain.Attachment, error) {
	const op serrors.Op = "AttachmentService.ValidateAndSave"

	// Validate MIME type
	if mimeType == "" {
		// Try to detect from filename extension
		ext := filepath.Ext(filename)
		mimeType = mime.TypeByExtension(ext)
	}

	if !supportedImageTypes[mimeType] {
		return nil, serrors.E(
			op,
			serrors.KindValidation,
			fmt.Sprintf("unsupported image type: %s (supported: jpeg, png, gif, webp)", mimeType),
		)
	}

	// Validate size
	if size > maxImageSize {
		return nil, serrors.E(
			op,
			serrors.KindValidation,
			fmt.Sprintf("image too large: %d bytes (max: %d bytes / 20MB)", size, maxImageSize),
		)
	}

	// Save to storage
	// Note: Tenant isolation should be handled by storage path structure
	metadata := storage.FileMetadata{
		ContentType: mimeType,
		Size:        size,
	}

	url, err := s.storage.Save(ctx, filename, reader, metadata)
	if err != nil {
		return nil, serrors.E(op, serrors.Internal, "failed to save image", err)
	}

	// Create attachment entity
	attachment := domain.NewAttachment(
		domain.WithFileName(filename),
		domain.WithMimeType(mimeType),
		domain.WithSizeBytes(size),
		domain.WithFilePath(url), // FilePath stores the URL
	)

	return attachment, nil
}

// ValidateMultiple validates a batch of file uploads
func (s *attachmentService) ValidateMultiple(files []bichatservices.FileUpload) error {
	const op serrors.Op = "AttachmentService.ValidateMultiple"

	if len(files) > maxImageCount {
		return serrors.E(
			op,
			serrors.KindValidation,
			fmt.Sprintf("too many images: %d (max: %d)", len(files), maxImageCount),
		)
	}

	for i, file := range files {
		// Validate MIME type
		if !supportedImageTypes[file.MimeType] {
			return serrors.E(
				op,
				serrors.KindValidation,
				fmt.Sprintf("image %d has unsupported type: %s", i+1, file.MimeType),
			)
		}

		// Validate size
		if file.Size > maxImageSize {
			return serrors.E(
				op,
				serrors.KindValidation,
				fmt.Sprintf("image %d too large: %d bytes (max: 20MB)", i+1, file.Size),
			)
		}
	}

	return nil
}
