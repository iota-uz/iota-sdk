package services

import (
	"context"
	"io"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/pkg/bichat/domain"
)

// AttachmentService handles file uploads and validation for chat attachments.
// It validates allowed file types, enforces size/count limits, and saves files to storage.
type AttachmentService interface {
	// ValidateAndSave validates an attachment upload and saves it to storage.
	// Returns the saved attachment with storage URL.
	//
	// Validation rules:
	// - MIME type/extension must be in the supported attachment allowlist
	// - Max size: 20MB per file
	//
	// Parameters:
	// - filename: Original filename
	// - mimeType: MIME type (e.g., "image/jpeg")
	// - size: File size in bytes
	// - reader: File content reader
	// - tenantID: Tenant ID for storage isolation
	// - userID: User ID who uploaded the file
	ValidateAndSave(
		ctx context.Context,
		filename string,
		mimeType string,
		size int64,
		reader io.Reader,
		tenantID, userID uuid.UUID,
	) (domain.Attachment, error)

	// ValidateMultiple validates a batch of file uploads without saving.
	// Used for pre-flight validation before processing uploads.
	ValidateMultiple(files []FileUpload) error
}

// FileUpload represents a file being uploaded (for validation).
type FileUpload struct {
	Filename string
	MimeType string
	Size     int64
}
