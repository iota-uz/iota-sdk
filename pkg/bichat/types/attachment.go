package types

import (
	"strings"
	"time"

	"github.com/google/uuid"
)

// Attachment represents a file attached to a message.
type Attachment struct {
	ID        uuid.UUID `json:"id"`
	MessageID uuid.UUID `json:"message_id"`
	UploadID  *int64    `json:"upload_id,omitempty"`
	FileName  string    `json:"file_name"`
	MimeType  string    `json:"mime_type"`
	SizeBytes int64     `json:"size_bytes"`
	FilePath  string    `json:"file_path"`
	Data      []byte    `json:"data,omitempty"`
	CreatedAt time.Time `json:"created_at"`
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
