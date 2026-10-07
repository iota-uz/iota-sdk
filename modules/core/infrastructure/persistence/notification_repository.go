package persistence

import (
	"context"
	"database/sql"
	"errors"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/notification"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
	"github.com/jackc/pgx/v5"
)

type PgNotificationRepository struct{}

func NewNotificationRepository() notification.Repository { return &PgNotificationRepository{} }

const notificationColumns = `id,tenant_id,user_id,event_key,title,body,action_url,COALESCE(dedupe_key,''),created_at,read_at`

func scanNotification(row interface{ Scan(...any) error }) (notification.Notification, error) {
	var id, tenantID uuid.UUID
	var userID uint
	var eventKey, title, body, actionURL, dedupeKey string
	var createdAt sql.NullTime
	var readAt sql.NullTime
	if err := row.Scan(&id, &tenantID, &userID, &eventKey, &title, &body, &actionURL, &dedupeKey, &createdAt, &readAt); err != nil {
		return nil, err
	}
	opts := []notification.Option{notification.WithID(id), notification.WithTenantID(tenantID), notification.WithEventKey(eventKey), notification.WithActionURL(actionURL), notification.WithDedupeKey(dedupeKey), notification.WithCreatedAt(createdAt.Time)}
	if readAt.Valid {
		opts = append(opts, notification.WithReadAt(&readAt.Time))
	}
	return notification.New(userID, title, body, opts...)
}
func (r *PgNotificationRepository) Create(ctx context.Context, n notification.Notification) (notification.Notification, error) {
	const op serrors.Op = "PgNotificationRepository.Create"
	if err := notification.Validate(n); err != nil {
		return nil, serrors.New(serrors.Invalid, "").WithOp(op).WithCause(err)
	}
	tenant, err := composables.UseTenantID(ctx)
	if err != nil {
		return nil, serrors.Wrap(op, err)
	}
	if n.TenantID() != uuid.Nil && n.TenantID() != tenant {
		return nil, serrors.New(serrors.Invalid, "notification tenant differs from context").WithOp(op)
	}
	tx, err := composables.UseTx(ctx)
	if err != nil {
		return nil, serrors.Wrap(op, err)
	}
	saved, err := scanNotification(tx.QueryRow(ctx, `INSERT INTO core.notifications (id,tenant_id,user_id,event_key,title,body,action_url,dedupe_key,created_at) VALUES ($1,$2,$3,$4,$5,$6,$7,NULLIF($8,''),$9) ON CONFLICT (tenant_id,user_id,dedupe_key) DO UPDATE SET dedupe_key=EXCLUDED.dedupe_key RETURNING `+notificationColumns, n.ID(), tenant, n.UserID(), n.EventKey(), n.Title(), n.Body(), n.ActionURL(), n.DedupeKey(), n.CreatedAt()))
	if err != nil {
		return nil, serrors.Wrap(op, err)
	}
	return saved, nil
}
func (r *PgNotificationRepository) List(ctx context.Context, userID uint, p notification.FindParams) ([]notification.Notification, error) {
	const op serrors.Op = "PgNotificationRepository.List"
	tenant, err := composables.UseTenantID(ctx)
	if err != nil {
		return nil, serrors.Wrap(op, err)
	}
	tx, err := composables.UseTx(ctx)
	if err != nil {
		return nil, serrors.Wrap(op, err)
	}
	p = p.Bounded()
	rows, err := tx.Query(ctx, `SELECT `+notificationColumns+` FROM core.notifications WHERE tenant_id=$1 AND user_id=$2 AND ($3=FALSE OR read_at IS NULL) ORDER BY created_at DESC,id DESC LIMIT $4 OFFSET $5`, tenant, userID, p.UnreadOnly, p.Limit, p.Offset)
	if err != nil {
		return nil, serrors.Wrap(op, err)
	}
	defer rows.Close()
	result := make([]notification.Notification, 0)
	for rows.Next() {
		n, err := scanNotification(rows)
		if err != nil {
			return nil, serrors.Wrap(op, err)
		}
		result = append(result, n)
	}
	if err := rows.Err(); err != nil {
		return nil, serrors.Wrap(op, err)
	}
	return result, nil
}
func (r *PgNotificationRepository) UnreadCount(ctx context.Context, userID uint) (int64, error) {
	const op serrors.Op = "PgNotificationRepository.UnreadCount"
	tenant, err := composables.UseTenantID(ctx)
	if err != nil {
		return 0, serrors.Wrap(op, err)
	}
	tx, err := composables.UseTx(ctx)
	if err != nil {
		return 0, serrors.Wrap(op, err)
	}
	var count int64
	if err := tx.QueryRow(ctx, `SELECT COUNT(*) FROM core.notifications WHERE tenant_id=$1 AND user_id=$2 AND read_at IS NULL`, tenant, userID).Scan(&count); err != nil {
		return 0, serrors.Wrap(op, err)
	}
	return count, nil
}
func (r *PgNotificationRepository) MarkRead(ctx context.Context, userID uint, id uuid.UUID) error {
	const op serrors.Op = "PgNotificationRepository.MarkRead"
	tenant, err := composables.UseTenantID(ctx)
	if err != nil {
		return serrors.Wrap(op, err)
	}
	tx, err := composables.UseTx(ctx)
	if err != nil {
		return serrors.Wrap(op, err)
	}
	var found uuid.UUID
	err = tx.QueryRow(ctx, `UPDATE core.notifications SET read_at=COALESCE(read_at,CURRENT_TIMESTAMP) WHERE tenant_id=$1 AND user_id=$2 AND id=$3 RETURNING id`, tenant, userID, id).Scan(&found)
	if errors.Is(err, pgx.ErrNoRows) {
		return serrors.New(serrors.NotFound, "").WithOp(op).WithCause(notification.ErrNotFound)
	}
	if err != nil {
		return serrors.Wrap(op, err)
	}
	return nil
}
func (r *PgNotificationRepository) MarkAllRead(ctx context.Context, userID uint) error {
	const op serrors.Op = "PgNotificationRepository.MarkAllRead"
	tenant, err := composables.UseTenantID(ctx)
	if err != nil {
		return serrors.Wrap(op, err)
	}
	tx, err := composables.UseTx(ctx)
	if err != nil {
		return serrors.Wrap(op, err)
	}
	_, err = tx.Exec(ctx, `UPDATE core.notifications SET read_at=CURRENT_TIMESTAMP WHERE tenant_id=$1 AND user_id=$2 AND read_at IS NULL`, tenant, userID)
	if err != nil {
		return serrors.Wrap(op, err)
	}
	return nil
}
