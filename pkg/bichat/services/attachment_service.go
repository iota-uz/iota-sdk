package services

import (
	"context"
	"io"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/pkg/bichat/domain"
)

// AttachmentService handles file uploads and validation for vision support.
// It validates image types, enforces size limits, and saves files to storage.
type AttachmentService interface {
	// ValidateAndSave validates an image upload and saves it to storage.
	// Returns the saved attachment with storage URL.
	//
	// Validation rules:
	// - MIME type: image/jpeg, image/png, image/gif, image/webp
	// - Max size: 20MB per image
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
	) (*domain.Attachment, error)

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
