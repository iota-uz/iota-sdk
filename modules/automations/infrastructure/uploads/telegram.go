// Package uploads adapts the SDK upload service to what automation connectors need.
package uploads

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/iota-uz/iota-sdk/modules/automations/services"
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/upload"
	"github.com/iota-uz/iota-sdk/pkg/automation/connectors/telegram"
)

// CoreUploads is the part of the SDK's upload service (modules/core) the Telegram connector uses.
type CoreUploads interface {
	services.AttachmentUploadService
	GetByID(ctx context.Context, id uint) (upload.Upload, error)
	OpenReader(ctx context.Context, storedPath string) (io.ReadCloser, upload.ObjectInfo, error)
}

// legacySuffix is the suffix files of chats were stored under in the public root before they were private: Open
// gives such a file back under its own name.
const legacySuffix = ".tg.txt"

// maxTelegramFile is what a bot sends: 50 MB.
const maxTelegramFile = 50 << 20

type telegramUploads struct {
	svc   CoreUploads
	store services.PrivateAttachments
}

// NewTelegramUploads is the uploads storage of the Telegram connector (telegram.Config.Uploads) over the
// SDK's upload service. The files of incoming messages are saved as the tenant's uploads in a private namespace
// (services.TelegramAttachmentsNamespace below privateRoot, which is uploadsconfig.Config.PrivatePath()), like the
// attachments of email: no anonymous URL names them, and the event's link is the attachments route
// (/automations/attachments/<id>), which serves a file only to a user of its tenant who may read automations, as a
// sandboxed download. The files of a send are read from the tenant's uploads that route serves (its public uploads
// and the files connectors received), never from another tenant's or from another feature's private files.
//
// The upload repository reads its database scope from the context, which a webhook request or a call
// worker does not carry: the adapter binds the pool and the tenant itself on every call.
func NewTelegramUploads(svc CoreUploads, pool *pgxpool.Pool, privateRoot string) telegram.Uploads {
	return telegramUploads{svc: svc, store: services.NewPrivateAttachments(svc, pool, privateRoot, services.TelegramAttachmentsNamespace, "tgattachment")}
}

func (u telegramUploads) Save(ctx context.Context, tenantID uuid.UUID, f telegram.UploadFile) (telegram.Stored, error) {
	id, err := u.store.Save(ctx, tenantID, f.Name, f.Data)
	if err != nil {
		return telegram.Stored{}, err
	}
	return telegram.Stored{ID: id, URL: services.AttachmentPath(id)}, nil
}

func (u telegramUploads) Open(ctx context.Context, tenantID uuid.UUID, id string) (telegram.UploadFile, error) {
	n, err := strconv.ParseUint(id, 10, 64)
	if err != nil {
		return telegram.UploadFile{}, errors.New("not an upload id")
	}
	ctx = u.store.Scope(ctx, tenantID)
	found, err := u.svc.GetByID(ctx, uint(n))
	if err != nil {
		return telegram.UploadFile{}, err
	}
	// Another tenant's upload, and another feature's private file, are not there for a graph to send.
	if found.TenantID() != tenantID || !services.AttachmentServable(found.Path()) {
		return telegram.UploadFile{}, errors.New("upload not found")
	}
	body, _, err := u.svc.OpenReader(ctx, found.Path())
	if err != nil {
		return telegram.UploadFile{}, err
	}
	defer func() { _ = body.Close() }()
	data, err := io.ReadAll(io.LimitReader(body, maxTelegramFile+1))
	if err != nil {
		return telegram.UploadFile{}, err
	}
	if len(data) > maxTelegramFile {
		return telegram.UploadFile{}, fmt.Errorf("upload %s is larger than 50 MB", id)
	}
	mime := ""
	if m := found.Mimetype(); m != nil {
		mime = m.String()
	}
	return telegram.UploadFile{Name: strings.TrimSuffix(found.Name(), legacySuffix), Mime: mime, Data: data}, nil
}
