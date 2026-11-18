package persistence

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/notification"
	"github.com/iota-uz/iota-sdk/modules/core/notifications"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/repo"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
)

type NotificationDeliveryRepository struct{}

func NewNotificationDeliveryRepository() notifications.DeliveryRepository {
	return &NotificationDeliveryRepository{}
}

const deliveryColumns = `id,tenant_id,notice_id,event_key,event_id,content_version,recipient_kind,recipient_user_id,recipient_ref_entity,recipient_ref_key,recipient_key,channel,target_key,target_address,target_device_id,status,language,level,title,body,action_url,content,attempts,attempt_limit,error_code,provider_receipt,inbox_notification_id,created_at,available_at,delivered_at`

func scanDelivery(row interface{ Scan(...any) error }) (notifications.Delivery, error) {
	var d notifications.Delivery
	var userID *int
	var refEntity, refKey *string
	var kind, channel, status, level string
	var content []byte
	var inbox *uuid.UUID
	if err := row.Scan(&d.ID, &d.TenantID, &d.NoticeID, &d.EventKey, &d.EventID, &d.ContentVersion, &kind, &userID, &refEntity, &refKey, &d.RecipientKey, &channel, &d.TargetKey, &d.TargetAddress, &d.TargetDeviceID, &status, &d.Language, &level, &d.Title, &d.Body, &d.ActionURL, &content, &d.Attempts, &d.AttemptLimit, &d.ErrorCode, &d.ProviderReceipt, &inbox, &d.CreatedAt, &d.AvailableAt, &d.DeliveredAt); err != nil {
		return d, err
	}
	d.RecipientKind = notifications.RecipientKind(kind)
	if userID != nil {
		d.UserID = uint(*userID) //nolint:gosec // Constrained by the users foreign key.
	}
	if refEntity != nil && refKey != nil {
		d.Record = &notifications.ObjectRef{Entity: *refEntity, Key: *refKey}
	}
	if inbox != nil {
		d.InboxNotificationID = *inbox
	}
	d.Channel = notifications.Channel(channel)
	d.Status = notifications.DeliveryStatus(status)
	d.Level = notification.Level(level)
	if len(content) > 0 {
		if err := json.Unmarshal(content, &d.Content); err != nil {
			return d, err
		}
	}
	return d, nil
}

func scanDeliveries(rows interface {
	Next() bool
	Scan(...any) error
	Err() error
	Close()
}) ([]notifications.Delivery, error) {
	defer rows.Close()
	result := []notifications.Delivery{}
	for rows.Next() {
		d, err := scanDelivery(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, d)
	}
	return result, rows.Err()
}

// insertDelivery stores one delivery, ignoring a repeat of the same notice,
// recipient, channel and target.
func insertDelivery(ctx context.Context, db repo.Tx, tenant uuid.UUID, d notifications.Delivery) error {
	if d.TenantID != tenant || !d.Channel.Valid() || !d.RecipientKind.Valid() || d.NoticeID == uuid.Nil || d.EventKey == "" || d.EventID == "" || d.RecipientKey == "" {
		return serrors.New(serrors.Invalid, "Invalid notification delivery")
	}
	var userID *int
	var refEntity, refKey *string
	switch d.RecipientKind {
	case notifications.RecipientUser:
		if d.UserID == 0 {
			return serrors.New(serrors.Invalid, "Invalid notification delivery")
		}
		id := int(d.UserID) //nolint:gosec // Recipient IDs come from the users table.
		userID = &id
	case notifications.RecipientRecord:
		if d.Record == nil {
			return serrors.New(serrors.Invalid, "Invalid notification delivery")
		}
		refEntity, refKey = &d.Record.Entity, &d.Record.Key
	case notifications.RecipientAddress:
		// An address recipient is identified by its recipient key alone.
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
	var inbox *uuid.UUID
	if d.InboxNotificationID != uuid.Nil {
		inbox = &d.InboxNotificationID
	}
	content, err := json.Marshal(d.Content)
	if err != nil {
		return err
	}
	language := d.Language
	if language == "" {
		language = "en"
	}
	level := d.Level
	if level == "" {
		level = notification.LevelInfo
	}
	_, err = db.Exec(ctx, `INSERT INTO core.notification_deliveries(id,tenant_id,notice_id,event_key,event_id,content_version,recipient_kind,recipient_user_id,recipient_ref_entity,recipient_ref_key,recipient_key,channel,target_key,target_address,target_device_id,status,language,level,title,body,action_url,content,error_code,inbox_notification_id,delivered_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20,$21,$22,$23,$24,$25) ON CONFLICT ON CONSTRAINT notification_deliveries_target_key DO NOTHING`,
		id, tenant, d.NoticeID, d.EventKey, d.EventID, max(d.ContentVersion, 1), string(d.RecipientKind), userID, refEntity, refKey, d.RecipientKey, string(d.Channel), d.TargetKey, d.TargetAddress, d.TargetDeviceID, string(status), language, string(level), d.Title, d.Body, d.ActionURL, content, d.ErrorCode, inbox, deliveredAt)
	return err
}

// Create stores deliveries idempotently by notice, recipient, channel and
// target, in the caller's transaction so intent commits or rolls back with the
// business data.
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
		if err := insertDelivery(ctx, db, tenant, d); err != nil {
			return serrors.Wrap(op, err)
		}
	}
	return nil
}

// ClaimUnresolved leases due recipient/channel rows that still need their
// targets, policy and content, and counts the attempt. Run it in its own short
// transaction and commit before calling resolvers or renderers.
func (r *NotificationDeliveryRepository) ClaimUnresolved(ctx context.Context, limit int, lease time.Duration) ([]notifications.Delivery, error) {
	const op serrors.Op = "NotificationDeliveryRepository.ClaimUnresolved"
	tenant, err := composables.UseTenantID(ctx)
	if err != nil {
		return nil, serrors.Wrap(op, err)
	}
	db, err := composables.UseTx(ctx)
	if err != nil {
		return nil, serrors.Wrap(op, err)
	}
	if _, err := db.Exec(ctx, `UPDATE core.notification_deliveries SET status='failed',error_code=$2,available_at=NOW() WHERE tenant_id=$1 AND status='unresolved' AND available_at<=NOW() AND attempts>=attempt_limit`, tenant, notifications.ErrorCodeResolutionLimit); err != nil {
		return nil, serrors.Wrap(op, err)
	}
	rows, err := db.Query(ctx, `UPDATE core.notification_deliveries SET attempts=attempts+1,available_at=NOW()+make_interval(secs=>$3::double precision) WHERE id IN (SELECT id FROM core.notification_deliveries WHERE tenant_id=$1 AND status='unresolved' AND available_at<=NOW() ORDER BY created_at,id LIMIT $2 FOR UPDATE SKIP LOCKED) RETURNING `+deliveryColumns, tenant, limit, lease.Seconds())
	if err != nil {
		return nil, serrors.Wrap(op, err)
	}
	defer rows.Close()
	result, err := scanDeliveries(rows)
	return result, serrors.Wrap(op, err)
}

// Expand replaces a claimed unresolved row by one delivery per target in one
// transaction. A claim that was superseded (the lease expired and another
// worker took the row, or it was retried) is dropped.
func (r *NotificationDeliveryRepository) Expand(ctx context.Context, claimed notifications.Delivery, targets []notifications.Delivery) error {
	const op serrors.Op = "NotificationDeliveryRepository.Expand"
	tenant, err := composables.UseTenantID(ctx)
	if err != nil {
		return serrors.Wrap(op, err)
	}
	return composables.InTx(ctx, func(txCtx context.Context) error {
		db, err := composables.UseTx(txCtx)
		if err != nil {
			return serrors.Wrap(op, err)
		}
		tag, err := db.Exec(txCtx, `DELETE FROM core.notification_deliveries WHERE tenant_id=$1 AND id=$2 AND status='unresolved' AND attempts=$3`, tenant, claimed.ID, claimed.Attempts)
		if err != nil {
			return serrors.Wrap(op, err)
		}
		if tag.RowsAffected() != 1 {
			return nil
		}
		for _, target := range targets {
			if err := insertDelivery(txCtx, db, tenant, target); err != nil {
				return serrors.Wrap(op, err)
			}
		}
		return nil
	})
}

// Settle ends or reschedules a claimed unresolved row that produced no targets.
func (r *NotificationDeliveryRepository) Settle(ctx context.Context, claimed notifications.Delivery, status notifications.DeliveryStatus, errorCode string, retryIn time.Duration) error {
	const op serrors.Op = "NotificationDeliveryRepository.Settle"
	tenant, err := composables.UseTenantID(ctx)
	if err != nil {
		return serrors.Wrap(op, err)
	}
	db, err := composables.UseTx(ctx)
	if err != nil {
		return serrors.Wrap(op, err)
	}
	_, err = db.Exec(ctx, `UPDATE core.notification_deliveries SET status=$4,error_code=$5,available_at=NOW()+make_interval(secs=>$6::double precision) WHERE tenant_id=$1 AND id=$2 AND status='unresolved' AND attempts=$3`, tenant, claimed.ID, claimed.Attempts, string(status), errorCode, retryIn.Seconds())
	return serrors.Wrap(op, err)
}

// Claim leases due external deliveries, counts the attempt and opens its
// attempt record in one statement. A previous attempt that never recorded an
// outcome (crash, expired lease) becomes ambiguous: the provider may have
// accepted it. Run it in its own short transaction and commit before calling
// providers.
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
	if _, err := db.Exec(ctx, `WITH exhausted AS (UPDATE core.notification_deliveries SET status='failed',error_code=$2,available_at=NOW() WHERE tenant_id=$1 AND status='pending' AND channel<>'in_app' AND available_at<=NOW() AND attempts>=attempt_limit RETURNING id) UPDATE core.notification_delivery_attempts SET outcome='ambiguous',finished_at=NOW(),error_code=$3 WHERE tenant_id=$1 AND delivery_id IN (SELECT id FROM exhausted) AND outcome IS NULL`, tenant, notifications.ErrorCodeAttemptLimit, notifications.ErrorCodeLeaseExpired); err != nil {
		return nil, serrors.Wrap(op, err)
	}
	rows, err := db.Query(ctx, `WITH claimed AS (
UPDATE core.notification_deliveries SET attempts=attempts+1,available_at=NOW()+make_interval(secs=>$3::double precision) WHERE id IN (SELECT id FROM core.notification_deliveries WHERE tenant_id=$1 AND status='pending' AND channel<>'in_app' AND available_at<=NOW() ORDER BY created_at,id LIMIT $2 FOR UPDATE SKIP LOCKED) RETURNING `+deliveryColumns+`),
stale AS (
UPDATE core.notification_delivery_attempts SET outcome='ambiguous',finished_at=NOW(),error_code=$4 WHERE tenant_id=$1 AND delivery_id IN (SELECT id FROM claimed) AND outcome IS NULL RETURNING delivery_id),
opened AS (
INSERT INTO core.notification_delivery_attempts(tenant_id,delivery_id,attempt) SELECT tenant_id,id,attempts FROM claimed RETURNING delivery_id)
SELECT `+deliveryColumns+` FROM claimed`, tenant, limit, lease.Seconds(), notifications.ErrorCodeLeaseExpired)
	if err != nil {
		return nil, serrors.Wrap(op, err)
	}
	defer rows.Close()
	result, err := scanDeliveries(rows)
	return result, serrors.Wrap(op, err)
}

// Record stores an attempt outcome on the delivery and its attempt record. A
// pending status with RetryIn schedules the next attempt; other statuses end the
// delivery. Zero rows means the claim was superseded; the newer claim owns the
// outcome and the superseded attempt stays ambiguous.
func (r *NotificationDeliveryRepository) Record(ctx context.Context, id uuid.UUID, attempt int, result notifications.AttemptResult) error {
	const op serrors.Op = "NotificationDeliveryRepository.Record"
	tenant, err := composables.UseTenantID(ctx)
	if err != nil {
		return serrors.Wrap(op, err)
	}
	db, err := composables.UseTx(ctx)
	if err != nil {
		return serrors.Wrap(op, err)
	}
	_, err = db.Exec(ctx, `WITH updated AS (UPDATE core.notification_deliveries SET status=$3,provider_receipt=CASE WHEN $4='sent' THEN $5 ELSE provider_receipt END,error_code=$6,available_at=NOW()+make_interval(secs=>$7::double precision),delivered_at=CASE WHEN $3='sent' THEN NOW() ELSE NULL END WHERE tenant_id=$1 AND id=$2 AND attempts=$8 AND status='pending' RETURNING id) UPDATE core.notification_delivery_attempts SET outcome=$4,finished_at=NOW(),provider_receipt=$5,error_code=$6 WHERE tenant_id=$1 AND delivery_id IN (SELECT id FROM updated) AND attempt=$8 AND outcome IS NULL`,
		tenant, id, string(result.Status), string(result.Outcome), result.Receipt, result.ErrorCode, result.RetryIn.Seconds(), attempt)
	return serrors.Wrap(op, err)
}

// Retry reopens one failed, ambiguous or unreachable delivery with a fresh
// attempt budget. A row that never got targets goes back to resolution. Sent and
// sibling deliveries are untouched, so a retry never repeats another target.
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
	tag, err := db.Exec(ctx, `UPDATE core.notification_deliveries SET status=CASE WHEN error_code LIKE 'resolution\_%' OR (target_address='' AND target_device_id='') THEN 'unresolved' ELSE 'pending' END,attempt_limit=attempts+$3,error_code='',available_at=NOW() WHERE tenant_id=$1 AND id=$2 AND channel<>'in_app' AND status IN ('failed','no_address','ambiguous') AND NOT (recipient_kind='address' AND status='no_address')`, tenant, id, notifications.MaxDeliveryAttempts)
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
	rows, err := db.Query(ctx, `SELECT `+deliveryColumns+` FROM core.notification_deliveries WHERE tenant_id=$1 AND event_key=$2 AND event_id=$3 ORDER BY recipient_key,channel,target_key`, tenant, eventKey, eventID)
	if err != nil {
		return nil, serrors.Wrap(op, err)
	}
	defer rows.Close()
	result, err := scanDeliveries(rows)
	return result, serrors.Wrap(op, err)
}

func (r *NotificationDeliveryRepository) Attempts(ctx context.Context, deliveryID uuid.UUID) ([]notifications.Attempt, error) {
	const op serrors.Op = "NotificationDeliveryRepository.Attempts"
	tenant, err := composables.UseTenantID(ctx)
	if err != nil {
		return nil, serrors.Wrap(op, err)
	}
	db, err := composables.UseTx(ctx)
	if err != nil {
		return nil, serrors.Wrap(op, err)
	}
	rows, err := db.Query(ctx, `SELECT delivery_id,attempt,started_at,finished_at,COALESCE(outcome,''),provider_receipt,error_code FROM core.notification_delivery_attempts WHERE tenant_id=$1 AND delivery_id=$2 ORDER BY attempt`, tenant, deliveryID)
	if err != nil {
		return nil, serrors.Wrap(op, err)
	}
	defer rows.Close()
	result := []notifications.Attempt{}
	for rows.Next() {
		var a notifications.Attempt
		var outcome string
		if err := rows.Scan(&a.DeliveryID, &a.Number, &a.StartedAt, &a.FinishedAt, &outcome, &a.ProviderReceipt, &a.ErrorCode); err != nil {
			return nil, serrors.Wrap(op, err)
		}
		a.Outcome = notifications.AttemptOutcome(outcome)
		result = append(result, a)
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
