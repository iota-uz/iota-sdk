package services

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strconv"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/upload"
	"github.com/iota-uz/iota-sdk/pkg/automation/connectors/email"
	"github.com/iota-uz/iota-sdk/pkg/composables"
)

// EmailAttachmentsNamespace is the directory of the private upload namespace the attachments of incoming
// email are kept in; the public static handler never serves it.
const EmailAttachmentsNamespace = "automations-email"

// EmailAttachmentPath is where a tenant user opens an attachment: a route that checks the session's
// tenant and permission and serves the file as a download (presentation/attachments).
func EmailAttachmentPath(id string) string { return "/automations/attachments/" + id }

// EmailUploadService is the part of the SDK's upload service (modules/core) the email connector uses.
type EmailUploadService interface {
	CreatePrivate(ctx context.Context, data *upload.CreateDTO, privatePath string) (upload.Upload, error)
}

type emailUploads struct {
	svc  EmailUploadService
	pool *pgxpool.Pool
	root string
}

// NewEmailUploads is the uploads storage of the email connector (email.Config.Uploads) over the SDK's
// upload service: the attachments of incoming messages are saved as the tenant's uploads in a private
// namespace (privateRoot is uploadsconfig.Config.PrivatePath()), so no anonymous URL names them; the event
// carries the id and the URL of the tenant-authorized download route.
//
// The upload repository reads its database scope from the context, which a poll does not carry: the
// adapter binds the pool and the tenant itself on every call.
func NewEmailUploads(svc EmailUploadService, pool *pgxpool.Pool, privateRoot string) email.Uploads {
	return emailUploads{svc: svc, pool: pool, root: filepath.Join(privateRoot, EmailAttachmentsNamespace)}
}

func (u emailUploads) Save(ctx context.Context, tenantID uuid.UUID, f email.UploadFile) (email.Stored, error) {
	if !upload.IsS3Path(u.root) {
		// The file storage writes where it is told and makes no directory: the namespace's own is made here,
		// private to the application's user like the private root above it.
		if err := os.MkdirAll(u.root, 0o700); err != nil {
			return email.Stored{}, err
		}
	}
	ctx = composables.WithPool(composables.WithTenantID(ctx, tenantID), u.pool)
	created, err := u.svc.CreatePrivate(ctx, &upload.CreateDTO{File: bytes.NewReader(f.Data), Name: f.Name, Size: len(f.Data)}, u.root)
	if err != nil {
		return email.Stored{}, err
	}
	id := strconv.FormatUint(uint64(created.ID()), 10)
	return email.Stored{ID: id, URL: EmailAttachmentPath(id)}, nil
}
