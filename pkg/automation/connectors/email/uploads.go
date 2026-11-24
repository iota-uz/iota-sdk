package email

import (
	"context"

	"github.com/google/uuid"
)

// Limits of attachments of an incoming message, kept in the uploads storage of the SDK.
const (
	MaxAttachment        = 25 << 20
	MaxAttachmentsPerMsg = 50 << 20
)

// Codes of attachments[].error: why an attachment of an incoming message was not stored.
const (
	AttachmentTooLarge      = "too_large"       // more than 25 MB
	AttachmentMessageTooBig = "message_too_big" // the message's attachments are past 50 MB
	AttachmentStorage       = "storage_failed"
	AttachmentNoStorage     = "no_storage"
)

// Uploads is the uploads storage of the SDK, as the connector needs it: the attachments of incoming
// messages are stored in it. The application provides the adapter over its upload service; every call is
// for one tenant.
type Uploads interface {
	// Save stores a file; the id is what a graph passes on, the URL is where a person opens it.
	Save(ctx context.Context, tenantID uuid.UUID, f UploadFile) (Stored, error)
}

// UploadFile is a file of the uploads storage.
type UploadFile struct {
	Name string
	Mime string
	Data []byte
}

// Stored is a saved file.
type Stored struct {
	ID  string
	URL string
}
