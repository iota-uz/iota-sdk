package types

import (
	"strings"
	"time"

	"github.com/google/uuid"
)

// Attachment represents a file attached to a message.
type Attachment struct {
	// ID is the unique identifier for this attachment
	ID uuid.UUID

	// MessageID is the ID of the message this attachment belongs to
	MessageID uuid.UUID

	// FileName is the original name of the file
	FileName string

	// MimeType is the MIME type of the file (e.g., "image/png", "application/pdf")
	MimeType string

	// SizeBytes is the size of the file in bytes
	SizeBytes int64

	// FilePath is the storage path or URL of the file
	FilePath string

	// Data contains the raw file data (optional, may be nil if stored externally)
	Data []byte

	// CreatedAt is when the attachment was created
	CreatedAt time.Time
}

// IsImage returns true if the attachment is an image file.
func (a *Attachment) IsImage() bool {
	return strings.HasPrefix(a.MimeType, "image/")
}

// IsDocument returns true if the attachment is a document file.
func (a *Attachment) IsDocument() bool {
	switch a.MimeType {
	case "application/pdf",
		"application/msword",
		"application/vnd.openxmlformats-officedocument.wordprocessingml.document",
		"application/vnd.ms-excel",
		"application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
		"application/vnd.ms-powerpoint",
		"application/vnd.openxmlformats-officedocument.presentationml.presentation",
		"text/plain",
		"text/csv":
		return true
	default:
		return false
	}
}
