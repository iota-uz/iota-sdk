package domain

import (
	"time"

	"github.com/google/uuid"
)

// Attachment represents a file attached to a message (typically an image or document).
// This is a struct (not interface) following idiomatic Go patterns.
type Attachment struct {
	ID        uuid.UUID
	MessageID uuid.UUID
	FileName  string
	MimeType  string
	SizeBytes int64
	FilePath  string // Can store base64 data, file path, or URL depending on implementation
	CreatedAt time.Time
}

// NewAttachment creates a new attachment with the given parameters.
// Use functional options for optional fields.
func NewAttachment(opts ...AttachmentOption) *Attachment {
	a := &Attachment{
		ID:        uuid.New(),
		CreatedAt: time.Now(),
	}

	for _, opt := range opts {
		opt(a)
	}

	return a
}

// AttachmentOption is a functional option for creating attachments
type AttachmentOption func(*Attachment)

// WithAttachmentID sets the attachment ID
func WithAttachmentID(id uuid.UUID) AttachmentOption {
	return func(a *Attachment) {
		a.ID = id
	}
}

// WithAttachmentMessageID sets the message ID for the attachment
func WithAttachmentMessageID(messageID uuid.UUID) AttachmentOption {
	return func(a *Attachment) {
		a.MessageID = messageID
	}
}

// WithFileName sets the file name
func WithFileName(fileName string) AttachmentOption {
	return func(a *Attachment) {
		a.FileName = fileName
	}
}

// WithMimeType sets the MIME type
func WithMimeType(mimeType string) AttachmentOption {
	return func(a *Attachment) {
		a.MimeType = mimeType
	}
}

// WithSizeBytes sets the file size in bytes
func WithSizeBytes(sizeBytes int64) AttachmentOption {
	return func(a *Attachment) {
		a.SizeBytes = sizeBytes
	}
}

// WithFilePath sets the file path (or base64 data)
func WithFilePath(filePath string) AttachmentOption {
	return func(a *Attachment) {
		a.FilePath = filePath
	}
}

// IsImage returns true if the attachment is an image
func (a *Attachment) IsImage() bool {
	switch a.MimeType {
	case "image/png", "image/jpeg", "image/jpg", "image/gif", "image/webp":
		return true
	default:
		return false
	}
}

// IsDocument returns true if the attachment is a document
func (a *Attachment) IsDocument() bool {
	switch a.MimeType {
	case "application/pdf",
		"application/msword",
		"application/vnd.openxmlformats-officedocument.wordprocessingml.document",
		"application/vnd.ms-excel",
		"application/vnd.openxmlformats-officedocument.spreadsheetml.sheet":
		return true
	default:
		return false
	}
}
