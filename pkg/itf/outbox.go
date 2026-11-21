package itf

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/require"

	"github.com/iota-uz/iota-sdk/pkg/composables"
)

// OutboxRow is a row of the extension runtime's event outbox.
type OutboxRow struct {
	Event         string
	SubjectEntity string
	SubjectKey    string
	Initiator     *int64
	Payload       json.RawMessage
}

// OutboxRows returns the outbox rows of an event in a tenant, oldest first, as
// the transaction of ctx sees them.
func OutboxRows(tb testing.TB, ctx context.Context, tenantID uuid.UUID, event string) []OutboxRow {
	tb.Helper()
	tx, err := composables.UseTx(ctx)
	require.NoError(tb, err)
	rows, err := tx.Query(ctx, `
		SELECT event, subject_entity, subject_key, initiator_user_id, payload
		FROM extruntime.outbox_events WHERE tenant_id = $1 AND event = $2 ORDER BY seq`, tenantID, event)
	require.NoError(tb, err)
	defer rows.Close()
	var out []OutboxRow
	for rows.Next() {
		var r OutboxRow
		require.NoError(tb, rows.Scan(&r.Event, &r.SubjectEntity, &r.SubjectKey, &r.Initiator, &r.Payload))
		out = append(out, r)
	}
	require.NoError(tb, rows.Err())
	return out
}

// OutboxCounts is the number of outbox rows of each event.
func OutboxCounts(tb testing.TB, ctx context.Context, tenantID uuid.UUID, events ...string) map[string]int {
	tb.Helper()
	out := make(map[string]int, len(events))
	for _, id := range events {
		out[id] = len(OutboxRows(tb, ctx, tenantID, id))
	}
	return out
}

// OutboxCommitsAndRollsBack shows that the events a change publishes commit
// with it and roll back with it. ctx carries the transaction of the test.
// change runs twice: first in a savepoint that is rolled back, which must
// leave no outbox row, then in the test's transaction, which must leave
// exactly want[event] new rows of every event in events. It returns the new
// rows of the committed run by event. change gets a suffix to make the names
// it creates unique.
func OutboxCommitsAndRollsBack(tb testing.TB, ctx context.Context, tenantID uuid.UUID, events []string, want map[string]int, change func(ctx context.Context, suffix string) error) map[string][]OutboxRow {
	tb.Helper()
	before := OutboxCounts(tb, ctx, tenantID, events...)

	tx, err := composables.UseTx(ctx)
	require.NoError(tb, err)
	savepoint, err := tx.(pgx.Tx).Begin(ctx)
	require.NoError(tb, err)
	spCtx := composables.WithTx(ctx, savepoint)
	require.NoError(tb, change(spCtx, "rolled-back"))
	inside := OutboxCounts(tb, spCtx, tenantID, events...)
	for _, id := range events {
		require.Equal(tb, before[id]+want[id], inside[id], "%s: the event is in the transaction of the change", id)
	}
	require.NoError(tb, savepoint.Rollback(ctx))
	require.Equal(tb, before, OutboxCounts(tb, ctx, tenantID, events...), "rolling the change back removes its events")

	require.NoError(tb, change(ctx, "committed"))
	got := map[string][]OutboxRow{}
	for _, id := range events {
		rows := OutboxRows(tb, ctx, tenantID, id)
		require.Len(tb, rows, before[id]+want[id], "%s: the committed change publishes its events once", id)
		got[id] = rows[before[id]:]
	}
	return got
}
