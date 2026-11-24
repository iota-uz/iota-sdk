// Package uploads adapts the SDK upload service to what automation connectors need.
package uploads

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"strconv"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/upload"
	"github.com/iota-uz/iota-sdk/pkg/automation/connectors/telegram"
	"github.com/iota-uz/iota-sdk/pkg/composables"
)

// CoreUploads is the part of the SDK's upload service (modules/core) the Telegram connector uses.
type CoreUploads interface {
	Create(ctx context.Context, data *upload.CreateDTO) (upload.Upload, error)
	GetByID(ctx context.Context, id uint) (upload.Upload, error)
	OpenReader(ctx context.Context, storedPath string) (io.ReadCloser, upload.ObjectInfo, error)
}

// maxTelegramFile is what a bot sends: 50 MB.
const maxTelegramFile = 50 << 20

type telegramUploads struct {
	svc  CoreUploads
	pool *pgxpool.Pool
}

// NewTelegramUploads is the uploads storage of the Telegram connector (telegram.Config.Uploads) over the
// SDK's upload service: files of incoming messages are saved as the tenant's uploads, and the files of a
// send are read from them, never from another tenant's.
//
// The upload repository reads its database scope from the context, which a webhook request or a call
// worker does not carry: the adapter binds the pool and the tenant itself on every call.
func NewTelegramUploads(svc CoreUploads, pool *pgxpool.Pool) telegram.Uploads {
	return telegramUploads{svc: svc, pool: pool}
}

// scope is a context the SDK's upload service can run in: the tenant and the database pool.
func (u telegramUploads) scope(ctx context.Context, tenantID uuid.UUID) context.Context {
	return composables.WithPool(composables.WithTenantID(ctx, tenantID), u.pool)
}

func (u telegramUploads) Save(ctx context.Context, tenantID uuid.UUID, f telegram.UploadFile) (telegram.Stored, error) {
	ctx = u.scope(ctx, tenantID)
	created, err := u.svc.Create(ctx, &upload.CreateDTO{File: bytes.NewReader(f.Data), Name: f.Name, Size: len(f.Data)})
	if err != nil {
		return telegram.Stored{}, err
	}
	return telegram.Stored{ID: strconv.FormatUint(uint64(created.ID()), 10), URL: created.URL().String()}, nil
}

func (u telegramUploads) Open(ctx context.Context, tenantID uuid.UUID, id string) (telegram.UploadFile, error) {
	n, err := strconv.ParseUint(id, 10, 64)
	if err != nil {
		return telegram.UploadFile{}, errors.New("not an upload id")
	}
	ctx = u.scope(ctx, tenantID)
	found, err := u.svc.GetByID(ctx, uint(n))
	if err != nil {
		return telegram.UploadFile{}, err
	}
	if found.TenantID() != tenantID {
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
	return telegram.UploadFile{Name: found.Name(), Mime: mime, Data: data}, nil
}
