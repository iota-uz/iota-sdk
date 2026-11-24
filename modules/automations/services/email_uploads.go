package services

import (
	"bytes"
	"context"
	"strconv"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/upload"
	"github.com/iota-uz/iota-sdk/pkg/automation/connectors/email"
	"github.com/iota-uz/iota-sdk/pkg/composables"
)

// EmailUploadService is the part of the SDK's upload service (modules/core) the email connector uses.
type EmailUploadService interface {
	Create(ctx context.Context, data *upload.CreateDTO) (upload.Upload, error)
}

type emailUploads struct {
	svc  EmailUploadService
	pool *pgxpool.Pool
}

// NewEmailUploads is the uploads storage of the email connector (email.Config.Uploads) over the SDK's
// upload service: the attachments of incoming messages are saved as the tenant's uploads, and the event
// carries their id and URL.
//
// The upload repository reads its database scope from the context, which a poll does not carry: the
// adapter binds the pool and the tenant itself on every call.
func NewEmailUploads(svc EmailUploadService, pool *pgxpool.Pool) email.Uploads {
	return emailUploads{svc: svc, pool: pool}
}

func (u emailUploads) Save(ctx context.Context, tenantID uuid.UUID, f email.UploadFile) (email.Stored, error) {
	ctx = composables.WithPool(composables.WithTenantID(ctx, tenantID), u.pool)
	created, err := u.svc.Create(ctx, &upload.CreateDTO{File: bytes.NewReader(f.Data), Name: f.Name, Size: len(f.Data)})
	if err != nil {
		return email.Stored{}, err
	}
	return email.Stored{ID: strconv.FormatUint(uint64(created.ID()), 10), URL: created.URL().String()}, nil
}
