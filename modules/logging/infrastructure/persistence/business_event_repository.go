package persistence

import (
	"context"
	"encoding/json"
	"errors"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/modules/logging/domain/aggregates/businessevent"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/constants"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
	"github.com/jackc/pgx/v5"
)

// Rows without event_type are legacy request-log records; they read as incomplete legacy events.
const businessEventColumns = `event_id, tenant_id, COALESCE(event_type, '` + businessevent.LegacyActionLogType + `'),
	COALESCE(schema_version, 1), COALESCE(category, '` + string(businessevent.CategoryLegacy) + `'),
	COALESCE(occurred_at, created_at), created_at, actor_snapshot, initiator_snapshot, COALESCE(channel, ''),
	COALESCE(operation_id, ''), COALESCE(request_id, ''), COALESCE(run_id, ''), COALESCE(client_id, ''),
	COALESCE(outcome, ''), subjects, changes,
	CASE WHEN event_type IS NULL THEN jsonb_build_object('source', 'action_logs', 'source_id', id::text,
		'user_id', user_id::text, 'incomplete', TRUE, 'payload', jsonb_build_object('method', method, 'path', path,
		'before', before::jsonb, 'after', after::jsonb, 'user_agent', user_agent, 'ip', ip)) END`

type PgBusinessEventRepository struct{}

func NewBusinessEventRepository() businessevent.Repository {
	return &PgBusinessEventRepository{}
}

func (r *PgBusinessEventRepository) Append(ctx context.Context, e businessevent.Event) error {
	const op serrors.Op = "PgBusinessEventRepository.Append"
	tx, ok := ctx.Value(constants.TxKey).(pgx.Tx)
	if !ok {
		return serrors.Wrap(op, composables.ErrNoTx)
	}
	if e.Actor == nil || e.Legacy != nil {
		return serrors.NewInvalid("only complete events with an actor can be appended").WithOp(op)
	}
	fingerprint, err := e.Fingerprint()
	if err != nil {
		return serrors.Wrap(op, err)
	}
	resources, refs := make([]string, 0, len(e.Subjects)), make([]string, 0, len(e.Subjects))
	for _, s := range e.Subjects {
		resources = append(resources, s.Resource)
		refs = append(refs, s.Resource+":"+s.ID)
	}
	subjects, err := json.Marshal(e.Subjects)
	if err != nil {
		return serrors.Wrap(op, err)
	}
	changes, err := json.Marshal(e.Changes)
	if err != nil {
		return serrors.Wrap(op, err)
	}
	actor, err := json.Marshal(e.Actor)
	if err != nil {
		return serrors.Wrap(op, err)
	}
	var initiator []byte
	if e.Initiator != nil {
		if initiator, err = json.Marshal(e.Initiator); err != nil {
			return serrors.Wrap(op, err)
		}
	}
	tag, err := tx.Exec(ctx, `INSERT INTO action_logs (
		event_id, tenant_id, event_type, schema_version, category, occurred_at, created_at,
		actor_kind, actor_id, actor_snapshot, initiator_snapshot, channel, operation_id,
		request_id, run_id, client_id, outcome, resources, subject_refs, subjects, changes, fingerprint
	) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, NULLIF($14, ''), NULLIF($15, ''), NULLIF($16, ''), $17, $18, $19, $20, $21, $22)
	ON CONFLICT (tenant_id, event_id) DO NOTHING`,
		e.ID, e.TenantID, e.Type, e.SchemaVersion, e.Category, e.OccurredAt, e.RecordedAt,
		e.Actor.Kind, e.Actor.Identity(), actor, initiator, e.Channel, e.OperationID,
		e.RequestID, e.RunID, e.ClientID, e.Outcome, uniqueStrings(resources), refs, subjects, changes, fingerprint)
	if err != nil {
		return serrors.Wrap(op, err)
	}
	if tag.RowsAffected() == 1 {
		return nil
	}
	var stored string
	if err := tx.QueryRow(ctx, `SELECT COALESCE(fingerprint, '') FROM action_logs WHERE tenant_id = $1 AND event_id = $2`, e.TenantID, e.ID).Scan(&stored); err != nil {
		return serrors.Wrap(op, err)
	}
	if stored != fingerprint {
		return serrors.Wrap(op, businessevent.ErrConflict)
	}
	return nil
}

func (r *PgBusinessEventRepository) Get(ctx context.Context, id uuid.UUID) (businessevent.Event, error) {
	const op serrors.Op = "PgBusinessEventRepository.Get"
	tenantID, err := composables.UseTenantID(ctx)
	if err != nil {
		return businessevent.Event{}, serrors.Wrap(op, err)
	}
	tx, err := composables.UseTx(ctx)
	if err != nil {
		return businessevent.Event{}, serrors.Wrap(op, err)
	}
	e, err := scanBusinessEvent(tx.QueryRow(ctx, `SELECT `+businessEventColumns+` FROM action_logs WHERE tenant_id = $1 AND event_id = $2`, tenantID, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return businessevent.Event{}, serrors.Wrap(op, businessevent.ErrNotFound)
	}
	if err != nil {
		return businessevent.Event{}, serrors.Wrap(op, err)
	}
	return e, nil
}

func (r *PgBusinessEventRepository) ListByOperation(ctx context.Context, operationID string) ([]businessevent.Event, error) {
	const op serrors.Op = "PgBusinessEventRepository.ListByOperation"
	tenantID, err := composables.UseTenantID(ctx)
	if err != nil {
		return nil, serrors.Wrap(op, err)
	}
	tx, err := composables.UseTx(ctx)
	if err != nil {
		return nil, serrors.Wrap(op, err)
	}
	rows, err := tx.Query(ctx, `SELECT `+businessEventColumns+` FROM action_logs WHERE tenant_id = $1 AND operation_id = $2 ORDER BY occurred_at, event_id`, tenantID, operationID)
	if err != nil {
		return nil, serrors.Wrap(op, err)
	}
	defer rows.Close()
	result := make([]businessevent.Event, 0)
	for rows.Next() {
		e, err := scanBusinessEvent(rows)
		if err != nil {
			return nil, serrors.Wrap(op, err)
		}
		result = append(result, e)
	}
	if err := rows.Err(); err != nil {
		return nil, serrors.Wrap(op, err)
	}
	return result, nil
}

func (r *PgBusinessEventRepository) Find(ctx context.Context, params businessevent.FindParams) ([]businessevent.Event, error) {
	const op serrors.Op = "PgBusinessEventRepository.Find"
	tenantID, err := composables.UseTenantID(ctx)
	if err != nil {
		return nil, serrors.Wrap(op, err)
	}
	tx, err := composables.UseTx(ctx)
	if err != nil {
		return nil, serrors.Wrap(op, err)
	}
	const position = "COALESCE(occurred_at, created_at)"
	args := []any{tenantID, params.Readable}
	where := []string{"tenant_id = $1", "(cardinality(resources) = 0 OR resources && $2)"}
	arg := func(v any) string {
		args = append(args, v)
		return "$" + strconv.Itoa(len(args))
	}
	if params.SubjectID != "" {
		where = append(where, "subject_refs @> ARRAY["+arg(params.Resource+":"+params.SubjectID)+"::text]")
	} else if params.Resource != "" {
		where = append(where, "resources @> ARRAY["+arg(params.Resource)+"::text]")
	}
	if params.ActorID != "" {
		where = append(where, "COALESCE(actor_id, user_id::text) = "+arg(params.ActorID))
	}
	if len(params.EventTypes) > 0 {
		where = append(where, "event_type = ANY("+arg(params.EventTypes)+")")
	}
	if len(params.Channels) > 0 || params.WithoutChannel {
		channels := make([]string, len(params.Channels))
		for i, c := range params.Channels {
			channels[i] = string(c)
		}
		clause := "channel = ANY(" + arg(channels) + ")"
		if params.WithoutChannel {
			clause = "(" + clause + " OR channel IS NULL)"
		}
		where = append(where, clause)
	}
	if len(params.Outcomes) > 0 {
		outcomes := make([]string, len(params.Outcomes))
		for i, o := range params.Outcomes {
			outcomes[i] = string(o)
		}
		where = append(where, "outcome = ANY("+arg(outcomes)+")")
	}
	if !params.From.IsZero() {
		where = append(where, position+" >= "+arg(params.From))
	}
	if !params.To.IsZero() {
		where = append(where, position+" < "+arg(params.To))
	}
	if params.After != nil {
		where = append(where, "("+position+", event_id) < ("+arg(params.After.Time)+", "+arg(params.After.ID)+")")
	}
	query := `SELECT ` + businessEventColumns + ` FROM action_logs WHERE ` + strings.Join(where, " AND ") +
		` ORDER BY ` + position + ` DESC, event_id DESC LIMIT ` + arg(params.Limit)
	rows, err := tx.Query(ctx, query, args...)
	if err != nil {
		return nil, serrors.Wrap(op, err)
	}
	defer rows.Close()
	result := make([]businessevent.Event, 0, params.Limit)
	for rows.Next() {
		e, err := scanBusinessEvent(rows)
		if err != nil {
			return nil, serrors.Wrap(op, err)
		}
		result = append(result, e)
	}
	if err := rows.Err(); err != nil {
		return nil, serrors.Wrap(op, err)
	}
	return result, nil
}

func scanBusinessEvent(row pgx.Row) (businessevent.Event, error) {
	var e businessevent.Event
	var occurredAt, recordedAt *time.Time
	var actor, initiator, legacy []byte
	var subjects, changes []byte
	var channel, outcome string
	if err := row.Scan(&e.ID, &e.TenantID, &e.Type, &e.SchemaVersion, &e.Category, &occurredAt, &recordedAt,
		&actor, &initiator, &channel, &e.OperationID, &e.RequestID, &e.RunID, &e.ClientID, &outcome,
		&subjects, &changes, &legacy); err != nil {
		return businessevent.Event{}, err
	}
	if occurredAt != nil {
		e.OccurredAt = *occurredAt
	}
	if recordedAt != nil {
		e.RecordedAt = *recordedAt
	}
	e.Channel = businessevent.Channel(channel)
	e.Outcome = businessevent.Outcome(outcome)
	for _, target := range []struct {
		data []byte
		dest any
	}{{actor, &e.Actor}, {initiator, &e.Initiator}, {subjects, &e.Subjects}, {changes, &e.Changes}, {legacy, &e.Legacy}} {
		if len(target.data) == 0 {
			continue
		}
		if err := json.Unmarshal(target.data, target.dest); err != nil {
			return businessevent.Event{}, err
		}
	}
	return e, nil
}

func uniqueStrings(values []string) []string {
	seen := make(map[string]bool, len(values))
	result := make([]string, 0, len(values))
	for _, v := range values {
		if !seen[v] {
			seen[v] = true
			result = append(result, v)
		}
	}
	return result
}
