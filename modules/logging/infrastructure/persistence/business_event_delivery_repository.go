package persistence

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/modules/logging/domain/aggregates/businessevent"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/constants"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
	"github.com/jackc/pgx/v5"
)

const deliveryColumns = `d.tenant_id, d.event_id, d.consumer, COALESCE(e.operation_id, ''), d.attempts, d.last_error, d.created_at, d.available_at, d.delivered_at`

const deliveryFrom = ` FROM business_event_deliveries d JOIN business_events e ON e.tenant_id = d.tenant_id AND e.event_id = d.event_id`

type PgBusinessEventDeliveryRepository struct{}

func NewBusinessEventDeliveryRepository() businessevent.DeliveryRepository {
	return &PgBusinessEventDeliveryRepository{}
}

func (r *PgBusinessEventDeliveryRepository) Enqueue(ctx context.Context, eventID uuid.UUID, consumers []string) error {
	const op serrors.Op = "PgBusinessEventDeliveryRepository.Enqueue"
	tx, ok := ctx.Value(constants.TxKey).(pgx.Tx)
	if !ok {
		return serrors.Wrap(op, composables.ErrNoTx)
	}
	tenantID, err := composables.UseTenantID(ctx)
	if err != nil {
		return serrors.Wrap(op, err)
	}
	_, err = tx.Exec(ctx, `INSERT INTO business_event_deliveries (tenant_id, event_id, consumer)
		SELECT $1, $2, unnest($3::text[]) ON CONFLICT DO NOTHING`, tenantID, eventID, consumers)
	return serrors.Wrap(op, err)
}

func (r *PgBusinessEventDeliveryRepository) Claim(ctx context.Context) (*businessevent.Delivery, error) {
	const op serrors.Op = "PgBusinessEventDeliveryRepository.Claim"
	tx, ok := ctx.Value(constants.TxKey).(pgx.Tx)
	if !ok {
		return nil, serrors.Wrap(op, composables.ErrNoTx)
	}
	d, err := scanDelivery(tx.QueryRow(ctx, `SELECT `+deliveryColumns+deliveryFrom+`
		WHERE d.delivered_at IS NULL AND d.available_at <= now()
		ORDER BY d.available_at, d.created_at LIMIT 1 FOR UPDATE OF d SKIP LOCKED`))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil //nolint:nilnil // An empty outbox is a successful claim with no work.
	}
	if err != nil {
		return nil, serrors.Wrap(op, err)
	}
	return &d, nil
}

func (r *PgBusinessEventDeliveryRepository) Complete(ctx context.Context, d businessevent.Delivery) error {
	const op serrors.Op = "PgBusinessEventDeliveryRepository.Complete"
	return serrors.Wrap(op, r.update(ctx, d.TenantID, d.EventID, d.Consumer,
		`UPDATE business_event_deliveries SET delivered_at = now(), last_error = ''
		WHERE tenant_id = $1 AND event_id = $2 AND consumer = $3 AND delivered_at IS NULL`))
}

func (r *PgBusinessEventDeliveryRepository) Fail(ctx context.Context, d businessevent.Delivery, reason string) error {
	const op serrors.Op = "PgBusinessEventDeliveryRepository.Fail"
	return serrors.Wrap(op, r.update(ctx, d.TenantID, d.EventID, d.Consumer,
		`UPDATE business_event_deliveries SET attempts = attempts + 1, last_error = left($4, 1000),
		available_at = now() + make_interval(secs => LEAST(3600, POWER(2, LEAST(attempts + 1, 12)))::double precision)
		WHERE tenant_id = $1 AND event_id = $2 AND consumer = $3 AND delivered_at IS NULL`, reason))
}

func (r *PgBusinessEventDeliveryRepository) Retry(ctx context.Context, eventID uuid.UUID, consumer string) error {
	const op serrors.Op = "PgBusinessEventDeliveryRepository.Retry"
	tenantID, err := composables.UseTenantID(ctx)
	if err != nil {
		return serrors.Wrap(op, err)
	}
	return serrors.Wrap(op, r.update(ctx, tenantID, eventID, consumer,
		`UPDATE business_event_deliveries SET available_at = now()
		WHERE tenant_id = $1 AND event_id = $2 AND consumer = $3 AND delivered_at IS NULL`))
}

func (r *PgBusinessEventDeliveryRepository) ListFailed(ctx context.Context, limit int) ([]businessevent.Delivery, error) {
	const op serrors.Op = "PgBusinessEventDeliveryRepository.ListFailed"
	tenantID, err := composables.UseTenantID(ctx)
	if err != nil {
		return nil, serrors.Wrap(op, err)
	}
	tx, err := composables.UseTx(ctx)
	if err != nil {
		return nil, serrors.Wrap(op, err)
	}
	rows, err := tx.Query(ctx, `SELECT `+deliveryColumns+deliveryFrom+`
		WHERE d.tenant_id = $1 AND d.delivered_at IS NULL AND d.attempts > 0
		ORDER BY d.created_at, d.event_id, d.consumer LIMIT $2`, tenantID, limit)
	if err != nil {
		return nil, serrors.Wrap(op, err)
	}
	defer rows.Close()
	result := make([]businessevent.Delivery, 0)
	for rows.Next() {
		d, err := scanDelivery(rows)
		if err != nil {
			return nil, serrors.Wrap(op, err)
		}
		result = append(result, d)
	}
	return result, serrors.Wrap(op, rows.Err())
}

func (r *PgBusinessEventDeliveryRepository) Failures(ctx context.Context) (businessevent.FailureSummary, error) {
	const op serrors.Op = "PgBusinessEventDeliveryRepository.Failures"
	tx, err := composables.UseTx(ctx)
	if err != nil {
		return businessevent.FailureSummary{}, serrors.Wrap(op, err)
	}
	var summary businessevent.FailureSummary
	if err := tx.QueryRow(ctx, `SELECT COUNT(*) FROM business_event_deliveries WHERE delivered_at IS NULL AND attempts > 0`).Scan(&summary.Count); err != nil {
		return summary, serrors.Wrap(op, err)
	}
	if summary.Count == 0 {
		return summary, nil
	}
	oldest, err := scanDelivery(tx.QueryRow(ctx, `SELECT `+deliveryColumns+deliveryFrom+`
		WHERE d.delivered_at IS NULL AND d.attempts > 0 ORDER BY d.created_at LIMIT 1`))
	if errors.Is(err, pgx.ErrNoRows) {
		return summary, nil
	}
	if err != nil {
		return summary, serrors.Wrap(op, err)
	}
	summary.Oldest = &oldest
	return summary, nil
}

func (r *PgBusinessEventDeliveryRepository) update(ctx context.Context, tenantID, eventID uuid.UUID, consumer, query string, args ...any) error {
	tx, err := composables.UseTx(ctx)
	if err != nil {
		return err
	}
	tag, err := tx.Exec(ctx, query, append([]any{tenantID, eventID, consumer}, args...)...)
	if err != nil {
		return err
	}
	if tag.RowsAffected() != 1 {
		return businessevent.ErrDeliveryNotFound
	}
	return nil
}

func scanDelivery(row pgx.Row) (businessevent.Delivery, error) {
	var d businessevent.Delivery
	err := row.Scan(&d.TenantID, &d.EventID, &d.Consumer, &d.OperationID, &d.Attempts, &d.LastError, &d.CreatedAt, &d.AvailableAt, &d.DeliveredAt)
	return d, err
}
