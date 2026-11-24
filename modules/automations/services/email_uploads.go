package services

import (
	"bytes"
	"context"
	"crypto/md5" //nolint:gosec // the upload table keys content by md5
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"strconv"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
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
	GetByHash(ctx context.Context, hash string) (upload.Upload, error)
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
	// A directory of the tenant in the private namespace.
	dir := filepath.Join(u.root, tenantID.String())
	if !upload.IsS3Path(dir) {
		// The file storage writes where it is told and makes no directory: the tenant's own is made here,
		// private to the application's user like the private root above it.
		if err := os.MkdirAll(dir, 0o700); err != nil {
			return email.Stored{}, err
		}
	}
	ctx = composables.WithPool(composables.WithTenantID(ctx, tenantID), u.pool)
	// The slug of an upload is global, and the core derives it from the content: the same image uploaded in
	// public would own it, and a private upload under it is refused. The attachments have a slug of their
	// own, of the tenant and the content, so no public upload can stand in their way (and one attachment
	// received twice is one upload).
	sum := sha256.Sum256(append([]byte(tenantID.String()+"\x00"), f.Data...))
	slug := "emailattachment" + hex.EncodeToString(sum[:20])
	created, err := u.svc.CreatePrivate(ctx, &upload.CreateDTO{File: bytes.NewReader(f.Data), Name: f.Name, Size: len(f.Data), Slug: slug}, dir)
	if err != nil {
		// A tenant has one upload per content: the same file the tenant uploaded itself (in public) is
		// already theirs, readable at its own address, and the attachment is that upload. The download route
		// serves it to the tenant's users like any attachment.
		var pg *pgconn.PgError
		if errors.As(err, &pg) && pg.Code == "23505" /* unique_violation */ {
			md := md5.Sum(f.Data)
			if existing, gerr := u.svc.GetByHash(ctx, hex.EncodeToString(md[:])); gerr == nil && existing.TenantID() == tenantID {
				id := strconv.FormatUint(uint64(existing.ID()), 10)
				return email.Stored{ID: id, URL: EmailAttachmentPath(id)}, nil
			}
		}
		return email.Stored{}, err
	}
	id := strconv.FormatUint(uint64(created.ID()), 10)
	return email.Stored{ID: id, URL: EmailAttachmentPath(id)}, nil
}
