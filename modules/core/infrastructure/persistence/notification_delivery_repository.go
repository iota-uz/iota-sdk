package persistence

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/notification"
	"github.com/iota-uz/iota-sdk/modules/core/notifications"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
)

type NotificationDeliveryRepository struct{}

func NewNotificationDeliveryRepository() notifications.DeliveryRepository {
	return &NotificationDeliveryRepository{}
}

const deliveryColumns = `id,tenant_id,event_key,event_id,content_version,user_id,channel,status,language,level,title,body,action_url,attempts,last_error,provider_receipt,created_at,available_at,delivered_at`

func scanDelivery(row interface{ Scan(...any) error }) (notifications.Delivery, error) {
	var d notifications.Delivery
	var userID int
	var channel, status, level string
	if err := row.Scan(&d.ID, &d.TenantID, &d.EventKey, &d.EventID, &d.ContentVersion, &userID, &channel, &status, &d.Language, &level, &d.Title, &d.Body, &d.ActionURL, &d.Attempts, &d.LastError, &d.ProviderReceipt, &d.CreatedAt, &d.AvailableAt, &d.DeliveredAt); err != nil {
		return d, err
	}
	d.UserID = uint(userID) //nolint:gosec // Constrained by the users foreign key.
	d.Channel = notifications.Channel(channel)
	d.Status = notifications.DeliveryStatus(status)
	d.Level = notification.Level(level)
	return d, nil
}

// Create stores deliveries idempotently by event, recipient and channel, in the
// caller's transaction so intent commits or rolls back with the business data.
func (r *NotificationDeliveryRepository) Create(ctx context.Context, deliveries []notifications.Delivery) error {
	const op serrors.Op = "NotificationDeliveryRepository.Create"
	if len(deliveries) == 0 {
		return nil
	}
	tenant, err := composables.UseTenantID(ctx)
	if err != nil {
		return serrors.Wrap(op, err)
	}
	db, err := composables.UseTx(ctx)
	if err != nil {
		return serrors.Wrap(op, err)
	}
	for _, d := range deliveries {
		if d.TenantID != tenant || !d.Channel.Valid() || d.UserID == 0 || d.EventKey == "" || d.EventID == "" {
			return serrors.New(serrors.Invalid, "Invalid notification delivery").WithOp(op)
		}
		status := d.Status
		if status == "" {
			status = notifications.DeliveryPending
		}
		id := d.ID
		if id == uuid.Nil {
			id = uuid.New()
		}
		var deliveredAt *time.Time
		if status == notifications.DeliverySent {
			now := time.Now().UTC()
			deliveredAt = &now
		}
		_, err := db.Exec(ctx, `INSERT INTO core.notification_deliveries(id,tenant_id,event_key,event_id,content_version,user_id,channel,status,language,level,title,body,action_url,delivered_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14) ON CONFLICT(tenant_id,event_key,event_id,user_id,channel) DO NOTHING`,
			id, tenant, d.EventKey, d.EventID, max(d.ContentVersion, 1), int(d.UserID), string(d.Channel), string(status), d.Language, string(d.Level), d.Title, d.Body, d.ActionURL, deliveredAt) //nolint:gosec // Recipient IDs come from the users table.
		if err != nil {
			return serrors.Wrap(op, err)
		}
	}
	return nil
}

// Claim leases due external deliveries and counts the attempt in one statement.
// Run it in its own short transaction and commit before calling providers.
func (r *NotificationDeliveryRepository) Claim(ctx context.Context, limit int, lease time.Duration) ([]notifications.Delivery, error) {
	const op serrors.Op = "NotificationDeliveryRepository.Claim"
	tenant, err := composables.UseTenantID(ctx)
	if err != nil {
		return nil, serrors.Wrap(op, err)
	}
	db, err := composables.UseTx(ctx)
	if err != nil {
		return nil, serrors.Wrap(op, err)
	}
	rows, err := db.Query(ctx, `UPDATE core.notification_deliveries SET attempts=attempts+1,available_at=NOW()+make_interval(secs=>$3::double precision) WHERE id IN (SELECT id FROM core.notification_deliveries WHERE tenant_id=$1 AND status='pending' AND channel<>'in_app' AND available_at<=NOW() ORDER BY created_at,id LIMIT $2 FOR UPDATE SKIP LOCKED) RETURNING `+deliveryColumns, tenant, limit, lease.Seconds())
	if err != nil {
		return nil, serrors.Wrap(op, err)
	}
	defer rows.Close()
	result := []notifications.Delivery{}
	for rows.Next() {
		d, err := scanDelivery(rows)
		if err != nil {
			return nil, serrors.Wrap(op, err)
		}
		result = append(result, d)
	}
	return result, serrors.Wrap(op, rows.Err())
}

// Record stores an attempt outcome. A pending status with retryIn schedules the
// next attempt; terminal statuses end the delivery.
func (r *NotificationDeliveryRepository) Record(ctx context.Context, id uuid.UUID, status notifications.DeliveryStatus, receipt, lastError string, retryIn time.Duration) error {
	const op serrors.Op = "NotificationDeliveryRepository.Record"
	tenant, err := composables.UseTenantID(ctx)
	if err != nil {
		return serrors.Wrap(op, err)
	}
	db, err := composables.UseTx(ctx)
	if err != nil {
		return serrors.Wrap(op, err)
	}
	tag, err := db.Exec(ctx, `UPDATE core.notification_deliveries SET status=$3,provider_receipt=$4,last_error=$5,available_at=NOW()+make_interval(secs=>$6::double precision),delivered_at=CASE WHEN $3='sent' THEN NOW() ELSE NULL END WHERE tenant_id=$1 AND id=$2`, tenant, id, string(status), receipt, lastError, retryIn.Seconds())
	if err != nil {
		return serrors.Wrap(op, err)
	}
	if tag.RowsAffected() != 1 {
		return serrors.New(serrors.NotFound, "").WithOp(op).WithCause(notifications.ErrDeliveryNotFound)
	}
	return nil
}

// Retry reopens one failed or exhausted delivery. Sent and sibling deliveries
// are untouched, so a retry never repeats another channel.
func (r *NotificationDeliveryRepository) Retry(ctx context.Context, id uuid.UUID) error {
	const op serrors.Op = "NotificationDeliveryRepository.Retry"
	tenant, err := composables.UseTenantID(ctx)
	if err != nil {
		return serrors.Wrap(op, err)
	}
	db, err := composables.UseTx(ctx)
	if err != nil {
		return serrors.Wrap(op, err)
	}
	tag, err := db.Exec(ctx, `UPDATE core.notification_deliveries SET status='pending',attempts=0,last_error='',available_at=NOW() WHERE tenant_id=$1 AND id=$2 AND channel<>'in_app' AND status IN ('failed','no_address')`, tenant, id)
	if err != nil {
		return serrors.Wrap(op, err)
	}
	if tag.RowsAffected() != 1 {
		return serrors.New(serrors.NotFound, "").WithOp(op).WithCause(notifications.ErrDeliveryNotFound)
	}
	return nil
}

func (r *NotificationDeliveryRepository) ForEvent(ctx context.Context, eventKey, eventID string) ([]notifications.Delivery, error) {
	const op serrors.Op = "NotificationDeliveryRepository.ForEvent"
	tenant, err := composables.UseTenantID(ctx)
	if err != nil {
		return nil, serrors.Wrap(op, err)
	}
	db, err := composables.UseTx(ctx)
	if err != nil {
		return nil, serrors.Wrap(op, err)
	}
	rows, err := db.Query(ctx, `SELECT `+deliveryColumns+` FROM core.notification_deliveries WHERE tenant_id=$1 AND event_key=$2 AND event_id=$3 ORDER BY user_id,channel`, tenant, eventKey, eventID)
	if err != nil {
		return nil, serrors.Wrap(op, err)
	}
	defer rows.Close()
	result := []notifications.Delivery{}
	for rows.Next() {
		d, err := scanDelivery(rows)
		if err != nil {
			return nil, serrors.Wrap(op, err)
		}
		result = append(result, d)
	}
	return result, serrors.Wrap(op, rows.Err())
}

func (r *NotificationDeliveryRepository) Stats(ctx context.Context) ([]notifications.DeliveryStats, error) {
	const op serrors.Op = "NotificationDeliveryRepository.Stats"
	tenant, err := composables.UseTenantID(ctx)
	if err != nil {
		return nil, serrors.Wrap(op, err)
	}
	db, err := composables.UseTx(ctx)
	if err != nil {
		return nil, serrors.Wrap(op, err)
	}
	rows, err := db.Query(ctx, `SELECT channel,status,COUNT(*) FROM core.notification_deliveries WHERE tenant_id=$1 GROUP BY channel,status ORDER BY channel,status`, tenant)
	if err != nil {
		return nil, serrors.Wrap(op, err)
	}
	defer rows.Close()
	result := []notifications.DeliveryStats{}
	for rows.Next() {
		var s notifications.DeliveryStats
		var channel, status string
		if err := rows.Scan(&channel, &status, &s.Count); err != nil {
			return nil, serrors.Wrap(op, err)
		}
		s.Channel, s.Status = notifications.Channel(channel), notifications.DeliveryStatus(status)
		result = append(result, s)
	}
	return result, serrors.Wrap(op, rows.Err())
}
