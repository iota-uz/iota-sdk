package persistence

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/modules/core/notifications"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
	"github.com/jackc/pgx/v5"
)

type NotificationNoticeRepository struct{}

func NewNotificationNoticeRepository() notifications.NoticeRepository {
	return &NotificationNoticeRepository{}
}

const noticeColumns = `id,tenant_id,event_key,event_version,snapshot_ref,event_id,mode,subject_entity,subject_key,producer_invocation_id,producer_event_id,producer_instance_id,producer_occurrence,event,created_at`

func scanNotice(row interface{ Scan(...any) error }) (notifications.Notice, error) {
	var n notifications.Notice
	var mode string
	var event []byte
	if err := row.Scan(&n.ID, &n.TenantID, &n.EventKey, &n.Version, &n.SnapshotRef, &n.EventID, &mode, &n.Subject.Entity, &n.Subject.Key, &n.Producer.InvocationID, &n.Producer.EventID, &n.Producer.InstanceID, &n.Producer.Occurrence, &event, &n.CreatedAt); err != nil {
		return n, err
	}
	n.Mode = notifications.NoticeMode(mode)
	if err := json.Unmarshal(event, &n.Event); err != nil {
		return n, err
	}
	return n, nil
}

// Ensure stores the notice once per tenant, event key and event ID. It runs in
// the caller's transaction, so a rolled back enqueue leaves no notice behind.
// A repeated enqueue returns the first stored notice unchanged.
func (r *NotificationNoticeRepository) Ensure(ctx context.Context, n notifications.Notice) (notifications.Notice, error) {
	const op serrors.Op = "NotificationNoticeRepository.Ensure"
	tenant, err := composables.UseTenantID(ctx)
	if err != nil {
		return n, serrors.Wrap(op, err)
	}
	if n.TenantID != tenant || n.EventKey == "" || n.EventID == "" || (n.Mode != notifications.NoticeRule && n.Mode != notifications.NoticeExplicit) {
		return n, serrors.New(serrors.Invalid, "Invalid notification notice").WithOp(op)
	}
	event, err := json.Marshal(n.Event)
	if err != nil {
		return n, serrors.Wrap(op, err)
	}
	db, err := composables.UseTx(ctx)
	if err != nil {
		return n, serrors.Wrap(op, err)
	}
	version := max(n.Version, 1)
	stored, err := scanNotice(db.QueryRow(ctx, `INSERT INTO core.notification_notices(id,tenant_id,event_key,event_version,snapshot_ref,event_id,mode,subject_entity,subject_key,producer_invocation_id,producer_event_id,producer_instance_id,producer_occurrence,event) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14) ON CONFLICT(tenant_id,event_key,event_id) DO UPDATE SET event_id=EXCLUDED.event_id RETURNING `+noticeColumns,
		uuid.New(), tenant, n.EventKey, version, n.SnapshotRef, n.EventID, string(n.Mode), n.Subject.Entity, n.Subject.Key, n.Producer.InvocationID, n.Producer.EventID, n.Producer.InstanceID, n.Producer.Occurrence, event))
	return stored, serrors.Wrap(op, err)
}

func (r *NotificationNoticeRepository) Get(ctx context.Context, id uuid.UUID) (notifications.Notice, error) {
	const op serrors.Op = "NotificationNoticeRepository.Get"
	tenant, err := composables.UseTenantID(ctx)
	if err != nil {
		return notifications.Notice{}, serrors.Wrap(op, err)
	}
	db, err := composables.UseTx(ctx)
	if err != nil {
		return notifications.Notice{}, serrors.Wrap(op, err)
	}
	n, err := scanNotice(db.QueryRow(ctx, `SELECT `+noticeColumns+` FROM core.notification_notices WHERE tenant_id=$1 AND id=$2`, tenant, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return n, serrors.New(serrors.NotFound, "").WithOp(op)
	}
	return n, serrors.Wrap(op, err)
}
