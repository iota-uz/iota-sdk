package domain

import (
	"time"

	"github.com/google/uuid"
)

// Attachment represents a file attached to a message (typically an image or document).
// Interface following the same design as other aggregates (e.g. Session, Artifact).
type Attachment interface {
	ID() uuid.UUID
	MessageID() uuid.UUID
	UploadID() *int64
	FileName() string
	MimeType() string
	SizeBytes() int64
	FilePath() string
	CreatedAt() time.Time

	IsImage() bool
	IsDocument() bool
}

type attachment struct {
	id        uuid.UUID
	messageID uuid.UUID
	uploadID  *int64
	fileName  string
	mimeType  string
	sizeBytes int64
	filePath  string
	createdAt time.Time
}

// AttachmentOption is a functional option for creating attachments
type AttachmentOption func(*attachment)

// NewAttachment creates a new attachment with the given parameters.
// Use functional options for optional fields.
func NewAttachment(opts ...AttachmentOption) Attachment {
	a := &attachment{
		id:        uuid.New(),
		createdAt: time.Now(),
	}

	for _, opt := range opts {
		opt(a)
	}

	return a
}

// WithAttachmentID sets the attachment ID
func WithAttachmentID(id uuid.UUID) AttachmentOption {
	return func(a *attachment) {
		a.id = id
	}
}

// WithAttachmentMessageID sets the message ID for the attachment
func WithAttachmentMessageID(messageID uuid.UUID) AttachmentOption {
	return func(a *attachment) {
		a.messageID = messageID
	}
}

// WithUploadID sets the upload ID linked to this attachment.
func WithUploadID(uploadID int64) AttachmentOption {
	return func(a *attachment) {
		a.uploadID = &uploadID
	}
}

// WithFileName sets the file name
func WithFileName(fileName string) AttachmentOption {
	return func(a *attachment) {
		a.fileName = fileName
	}
}

// WithMimeType sets the MIME type
func WithMimeType(mimeType string) AttachmentOption {
	return func(a *attachment) {
		a.mimeType = mimeType
	}
}

// WithSizeBytes sets the file size in bytes
func WithSizeBytes(sizeBytes int64) AttachmentOption {
	return func(a *attachment) {
		a.sizeBytes = sizeBytes
	}
}

// WithFilePath sets the file path (or base64 data)
func WithFilePath(filePath string) AttachmentOption {
	return func(a *attachment) {
		a.filePath = filePath
	}
}

// WithAttachmentCreatedAt sets the created timestamp
func WithAttachmentCreatedAt(t time.Time) AttachmentOption {
	return func(a *attachment) {
		a.createdAt = t
	}
}

// Getter methods implementing the Attachment interface

func (a *attachment) ID() uuid.UUID {
	return a.id
}

func (a *attachment) MessageID() uuid.UUID {
	return a.messageID
}

func (a *attachment) UploadID() *int64 {
	return a.uploadID
}

func (a *attachment) FileName() string {
	return a.fileName
}

func (a *attachment) MimeType() string {
	return a.mimeType
}

func (a *attachment) SizeBytes() int64 {
	return a.sizeBytes
}

func (a *attachment) FilePath() string {
	return a.filePath
}

func (a *attachment) CreatedAt() time.Time {
	return a.createdAt
}

// IsImage returns true if the attachment is an image
func (a *attachment) IsImage() bool {
	switch a.mimeType {
	case "image/png", "image/jpeg", "image/jpg", "image/gif", "image/webp":
		return true
	default:
		return false
	}
}

// IsDocument returns true if the attachment is a document
func (a *attachment) IsDocument() bool {
	switch a.mimeType {
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
