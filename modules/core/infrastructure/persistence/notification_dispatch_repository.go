package persistence

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/modules/core/notifications"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/constants"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
	"github.com/jackc/pgx/v5"
)

type NotificationDispatchRepository struct{}

func NewNotificationDispatchRepository() notifications.DispatchRepository {
	return &NotificationDispatchRepository{}
}

const dispatchColumns = `id,tenant_id,event,rule,recipient_ids,cursor,delivered,attempts,last_error,created_at,available_at,completed_at`

func scanDispatch(row interface{ Scan(...any) error }) (notifications.DispatchJob, error) {
	var job notifications.DispatchJob
	var event, rule, ids []byte
	if err := row.Scan(&job.ID, &job.TenantID, &event, &rule, &ids, &job.Cursor, &job.Delivered, &job.Attempts, &job.LastError, &job.CreatedAt, &job.AvailableAt, &job.CompletedAt); err != nil {
		return job, err
	}
	if err := json.Unmarshal(event, &job.Event); err != nil {
		return job, err
	}
	if err := json.Unmarshal(rule, &job.Rule); err != nil {
		return job, err
	}
	if err := json.Unmarshal(ids, &job.RecipientIDs); err != nil {
		return job, err
	}
	return job, nil
}
func (r *NotificationDispatchRepository) Enqueue(ctx context.Context, event notifications.Event, rule notifications.Rule, ids []uint) (notifications.DispatchJob, error) {
	const op serrors.Op = "NotificationDispatchRepository.Enqueue"
	tenant, err := composables.UseTenantID(ctx)
	if err != nil {
		return notifications.DispatchJob{}, serrors.Wrap(op, err)
	}
	if event.TenantID != tenant || event.ID == "" || event.Key == "" {
		return notifications.DispatchJob{}, serrors.New(serrors.Invalid, "Invalid notification event").WithOp(op)
	}
	eventJSON, err := json.Marshal(event)
	if err != nil {
		return notifications.DispatchJob{}, serrors.Wrap(op, err)
	}
	ruleJSON, err := json.Marshal(rule)
	if err != nil {
		return notifications.DispatchJob{}, serrors.Wrap(op, err)
	}
	if ids == nil {
		ids = []uint{}
	}
	idsJSON, err := json.Marshal(ids)
	if err != nil {
		return notifications.DispatchJob{}, serrors.Wrap(op, err)
	}
	db, err := composables.UseTx(ctx)
	if err != nil {
		return notifications.DispatchJob{}, serrors.Wrap(op, err)
	}
	identity := event.ID
	if event.DedupeKey != "" {
		identity = event.DedupeKey
	}
	job, err := scanDispatch(db.QueryRow(ctx, `INSERT INTO core.notification_dispatches(id,tenant_id,event_key,event_id,event,rule,recipient_ids,completed_at) VALUES($1,$2,$3,$4,$5,$6,$7,CASE WHEN jsonb_array_length($7::jsonb)=0 THEN NOW() ELSE NULL END) ON CONFLICT(tenant_id,event_key,event_id) DO UPDATE SET event_id=EXCLUDED.event_id RETURNING `+dispatchColumns, uuid.New(), tenant, event.Key, identity, eventJSON, ruleJSON, idsJSON))
	return job, serrors.Wrap(op, err)
}
func (r *NotificationDispatchRepository) Next(ctx context.Context) (*notifications.DispatchJob, error) {
	const op serrors.Op = "NotificationDispatchRepository.Next"
	if _, ok := ctx.Value(constants.TxKey).(pgx.Tx); !ok {
		return nil, serrors.New(serrors.FailedPrecondition, "Dispatch claims require a transaction").WithOp(op)
	}
	tenant, err := composables.UseTenantID(ctx)
	if err != nil {
		return nil, serrors.Wrap(op, err)
	}
	db, err := composables.UseTx(ctx)
	if err != nil {
		return nil, serrors.Wrap(op, err)
	}
	job, err := scanDispatch(db.QueryRow(ctx, `SELECT `+dispatchColumns+` FROM core.notification_dispatches WHERE tenant_id=$1 AND completed_at IS NULL AND available_at<=NOW() ORDER BY created_at,id LIMIT 1 FOR UPDATE SKIP LOCKED`, tenant))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil //nolint:nilnil // An empty queue is a successful claim with no work.
	}
	if err != nil {
		return nil, serrors.Wrap(op, err)
	}
	return &job, nil
}
func (r *NotificationDispatchRepository) Advance(ctx context.Context, id uuid.UUID, processed, delivered int) error {
	const op serrors.Op = "NotificationDispatchRepository.Advance"
	if processed < 0 || delivered < 0 || delivered > processed {
		return serrors.New(serrors.Invalid, "Invalid dispatch progress").WithOp(op)
	}
	tenant, err := composables.UseTenantID(ctx)
	if err != nil {
		return serrors.Wrap(op, err)
	}
	db, err := composables.UseTx(ctx)
	if err != nil {
		return serrors.Wrap(op, err)
	}
	tag, err := db.Exec(ctx, `UPDATE core.notification_dispatches SET cursor=cursor+$3,delivered=delivered+$4,attempts=0,last_error='',available_at=NOW(),completed_at=CASE WHEN cursor+$3>=jsonb_array_length(recipient_ids) THEN NOW() ELSE NULL END WHERE tenant_id=$1 AND id=$2 AND completed_at IS NULL AND cursor+$3<=jsonb_array_length(recipient_ids)`, tenant, id, processed, delivered)
	if err != nil {
		return serrors.Wrap(op, err)
	}
	if tag.RowsAffected() != 1 {
		return serrors.New(serrors.NotFound, "").WithOp(op).WithCause(notifications.ErrDispatchNotFound)
	}
	return nil
}
func (r *NotificationDispatchRepository) Fail(ctx context.Context, id uuid.UUID) error {
	const op serrors.Op = "NotificationDispatchRepository.Fail"
	tenant, err := composables.UseTenantID(ctx)
	if err != nil {
		return serrors.Wrap(op, err)
	}
	db, err := composables.UseTx(ctx)
	if err != nil {
		return serrors.Wrap(op, err)
	}
	_, err = db.Exec(ctx, `UPDATE core.notification_dispatches SET attempts=attempts+1,last_error='Delivery failed; retry scheduled',available_at=NOW()+make_interval(secs=>LEAST(300,POWER(2,LEAST(attempts+1,9)))::double precision) WHERE tenant_id=$1 AND id=$2 AND completed_at IS NULL`, tenant, id)
	return serrors.Wrap(op, err)
}
func (r *NotificationDispatchRepository) Retry(ctx context.Context, id uuid.UUID) error {
	const op serrors.Op = "NotificationDispatchRepository.Retry"
	tenant, err := composables.UseTenantID(ctx)
	if err != nil {
		return serrors.Wrap(op, err)
	}
	db, err := composables.UseTx(ctx)
	if err != nil {
		return serrors.Wrap(op, err)
	}
	tag, err := db.Exec(ctx, `UPDATE core.notification_dispatches SET available_at=NOW() WHERE tenant_id=$1 AND id=$2 AND completed_at IS NULL`, tenant, id)
	if err != nil {
		return serrors.Wrap(op, err)
	}
	if tag.RowsAffected() != 1 {
		return serrors.New(serrors.NotFound, "").WithOp(op).WithCause(notifications.ErrDispatchNotFound)
	}
	return nil
}
func (r *NotificationDispatchRepository) Stats(ctx context.Context) (notifications.DispatchStats, error) {
	const op serrors.Op = "NotificationDispatchRepository.Stats"
	var stats notifications.DispatchStats
	tenant, err := composables.UseTenantID(ctx)
	if err != nil {
		return stats, serrors.Wrap(op, err)
	}
	db, err := composables.UseTx(ctx)
	if err != nil {
		return stats, serrors.Wrap(op, err)
	}
	err = db.QueryRow(ctx, `SELECT COUNT(*) FILTER(WHERE completed_at IS NULL AND attempts=0),COUNT(*) FILTER(WHERE completed_at IS NULL AND attempts>0),COUNT(*) FILTER(WHERE completed_at IS NOT NULL) FROM core.notification_dispatches WHERE tenant_id=$1`, tenant).Scan(&stats.Pending, &stats.Retrying, &stats.Completed)
	return stats, serrors.Wrap(op, err)
}
func (r *NotificationDispatchRepository) List(ctx context.Context) ([]notifications.DispatchSummary, error) {
	const op serrors.Op = "NotificationDispatchRepository.List"
	tenant, err := composables.UseTenantID(ctx)
	if err != nil {
		return nil, serrors.Wrap(op, err)
	}
	db, err := composables.UseTx(ctx)
	if err != nil {
		return nil, serrors.Wrap(op, err)
	}
	rows, err := db.Query(ctx, `SELECT id,event_key,jsonb_array_length(recipient_ids),cursor,delivered,attempts,last_error,created_at,available_at,completed_at FROM core.notification_dispatches WHERE tenant_id=$1 ORDER BY completed_at NULLS FIRST,created_at DESC LIMIT 25`, tenant)
	if err != nil {
		return nil, serrors.Wrap(op, err)
	}
	defer rows.Close()
	jobs := []notifications.DispatchSummary{}
	for rows.Next() {
		var job notifications.DispatchSummary
		if err := rows.Scan(&job.ID, &job.EventKey, &job.Total, &job.Processed, &job.Delivered, &job.Attempts, &job.LastError, &job.CreatedAt, &job.AvailableAt, &job.CompletedAt); err != nil {
			return nil, serrors.Wrap(op, err)
		}
		jobs = append(jobs, job)
	}
	return jobs, serrors.Wrap(op, rows.Err())
}
