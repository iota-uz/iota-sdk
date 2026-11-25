package services

import (
	"bytes"
	"context"
	"crypto/md5" //nolint:gosec // the upload table keys content by md5
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/upload"
	"github.com/iota-uz/iota-sdk/pkg/automation/connectors/email"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/config/stdconfig/uploadsconfig"
)

// Directories of the private upload namespaces the files connectors receive are kept in (the attachments of
// incoming email, the files of messages to a Telegram bot); the public static handler never serves them.
const (
	EmailAttachmentsNamespace    = "automations-email"
	TelegramAttachmentsNamespace = "automations-telegram"
)

// attachmentNamespaces are the namespaces the attachments route serves.
var attachmentNamespaces = []string{EmailAttachmentsNamespace, TelegramAttachmentsNamespace}

// AttachmentPath is where a tenant user opens a received file: a route that checks the session's tenant and
// permission and serves the file as a download (presentation/attachments).
func AttachmentPath(id string) string { return "/automations/attachments/" + id }

// EmailAttachmentPath is AttachmentPath.
func EmailAttachmentPath(id string) string { return AttachmentPath(id) }

// AttachmentServable reports whether the download route serves a stored path: a private namespace of received
// files (email, Telegram), or a file that is not in a private namespace at all (a file the tenant uploaded itself
// in public: it is readable at its own address, and no more is shown here). Another feature's private files are
// never served.
func AttachmentServable(stored string) bool {
	p := path.Clean("/"+filepath.ToSlash(upload.LayoutPath(stored))) + "/"
	private := "/" + uploadsconfig.PrivateDirectory + "/"
	// The private directory is the first one on the path (the uploads' own root is above it): what follows
	// it must be one of the namespaces, so a private directory nested in another feature's namespace (or a
	// namespace of the same name below it) is not taken for it.
	_, rest, inPrivate := strings.Cut(p, private)
	if !inPrivate {
		return true
	}
	for _, ns := range attachmentNamespaces {
		if strings.HasPrefix(rest, ns+"/") {
			return true
		}
	}
	return false
}

// EmailAttachmentServable is AttachmentServable.
func EmailAttachmentServable(stored string) bool { return AttachmentServable(stored) }

// AttachmentUploadService is the part of the SDK's upload service (modules/core) the private attachments use.
type AttachmentUploadService interface {
	CreatePrivate(ctx context.Context, data *upload.CreateDTO, privatePath string) (upload.Upload, error)
	GetByHash(ctx context.Context, hash string) (upload.Upload, error)
}

// EmailUploadService is AttachmentUploadService.
type EmailUploadService = AttachmentUploadService

// PrivateAttachments keeps the files a connector receives as the tenant's uploads in a private namespace, so no
// anonymous URL names them; AttachmentPath is where a person opens one.
//
// The upload repository reads its database scope from the context, which a poll or a webhook does not carry:
// it binds the pool and the tenant itself on every call.
type PrivateAttachments struct {
	svc  AttachmentUploadService
	pool *pgxpool.Pool
	root string
	slug string
}

// NewPrivateAttachments is the store of one namespace (EmailAttachmentsNamespace, TelegramAttachmentsNamespace)
// below privateRoot (uploadsconfig.Config.PrivatePath()); slugPrefix names the feature in the uploads' slugs.
func NewPrivateAttachments(svc AttachmentUploadService, pool *pgxpool.Pool, privateRoot, namespace, slugPrefix string) PrivateAttachments {
	return PrivateAttachments{svc: svc, pool: pool, root: filepath.Join(privateRoot, namespace), slug: slugPrefix}
}

// Scope is a context the SDK's upload service runs in for the tenant.
func (u PrivateAttachments) Scope(ctx context.Context, tenantID uuid.UUID) context.Context {
	return composables.WithPool(composables.WithTenantID(ctx, tenantID), u.pool)
}

// Save stores a received file and returns its upload id.
func (u PrivateAttachments) Save(ctx context.Context, tenantID uuid.UUID, name string, data []byte) (string, error) {
	// A directory of the tenant in the private namespace.
	dir := filepath.Join(u.root, tenantID.String())
	if !upload.IsS3Path(dir) {
		// The file storage writes where it is told and makes no directory: the tenant's own is made here,
		// private to the application's user like the private root above it.
		if err := os.MkdirAll(dir, 0o700); err != nil {
			return "", err
		}
	}
	ctx = u.Scope(ctx, tenantID)
	// The slug of an upload is global, and the core derives it from the content: the same image uploaded in
	// public would own it, and a private upload under it is refused. The attachments have a slug of their
	// own, of the tenant and the content, so no public upload can stand in their way (and one attachment
	// received twice is one upload).
	sum := sha256.Sum256(append([]byte(tenantID.String()+"\x00"), data...))
	slug := u.slug + hex.EncodeToString(sum[:20])
	created, err := u.svc.CreatePrivate(ctx, &upload.CreateDTO{File: bytes.NewReader(data), Name: name, Size: len(data), Slug: slug}, dir)
	if err != nil {
		// A tenant has one upload per content: the same file the tenant uploaded itself (in public), or received
		// through another connector, is already theirs, and the attachment is that upload. The download route
		// serves it to the tenant's users like any attachment. The same content in another feature's private
		// namespace is not the route's to serve: the attachment is then not stored (a URL that always gives
		// 404 is not given), and the event says so.
		var pg *pgconn.PgError
		if errors.As(err, &pg) && pg.Code == "23505" /* unique_violation */ {
			md := md5.Sum(data)
			if existing, gerr := u.svc.GetByHash(ctx, hex.EncodeToString(md[:])); gerr == nil && existing.TenantID() == tenantID && AttachmentServable(existing.Path()) {
				return strconv.FormatUint(uint64(existing.ID()), 10), nil
			}
		}
		return "", err
	}
	return strconv.FormatUint(uint64(created.ID()), 10), nil
}

type emailUploads struct{ store PrivateAttachments }

// NewEmailUploads is the uploads storage of the email connector (email.Config.Uploads) over the SDK's
// upload service: the attachments of incoming messages are saved as the tenant's uploads in a private
// namespace (privateRoot is uploadsconfig.Config.PrivatePath()), so no anonymous URL names them; the event
// carries the id and the URL of the tenant-authorized download route.
func NewEmailUploads(svc EmailUploadService, pool *pgxpool.Pool, privateRoot string) email.Uploads {
	return emailUploads{store: NewPrivateAttachments(svc, pool, privateRoot, EmailAttachmentsNamespace, "emailattachment")}
}

func (u emailUploads) Save(ctx context.Context, tenantID uuid.UUID, f email.UploadFile) (email.Stored, error) {
	id, err := u.store.Save(ctx, tenantID, f.Name, f.Data)
	if err != nil {
		return email.Stored{}, err
	}
	return email.Stored{ID: id, URL: AttachmentPath(id)}, nil
}
