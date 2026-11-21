package itf

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/require"

	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/constants"
)

// OutboxRow is a row of the extension runtime's event outbox.
type OutboxRow struct {
	Event         string
	SubjectEntity string
	SubjectKey    string
	Initiator     *int64
	Payload       json.RawMessage
}

// OutboxRows returns the committed outbox rows of an event in a tenant, oldest
// first, read over a connection of the pool: another transaction than the
// test's.
func (te *TestEnvironment) OutboxRows(tb testing.TB, tenantID uuid.UUID, event string) []OutboxRow {
	tb.Helper()
	rows, err := te.Pool.Query(context.Background(), `
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

// OutboxCounts is the number of committed outbox rows of each event.
func (te *TestEnvironment) OutboxCounts(tb testing.TB, tenantID uuid.UUID, events ...string) map[string]int {
	tb.Helper()
	out := make(map[string]int, len(events))
	for _, id := range events {
		out[id] = len(te.OutboxRows(tb, tenantID, id))
	}
	return out
}

// Domain is a query that reads, as one text value, the state a change writes
// (a column, a count); the helper compares it before and after.
type Domain struct {
	SQL  string
	Args []any
}

func (te *TestEnvironment) domain(tb testing.TB, d Domain) string {
	tb.Helper()
	if d.SQL == "" {
		return ""
	}
	var v string
	require.NoError(tb, te.Pool.QueryRow(context.Background(), d.SQL, d.Args...).Scan(&v))
	return v
}

var errRollBack = errors.New("itf: roll the change back")

// OutboxCommitsAndRollsBack shows that the events a change publishes commit
// with it and roll back with it, in real transactions on separate connections.
//
// The seed the change needs must be committed first (CommitTx, from the
// test that owns the environment: the fresh scope transaction it begins is
// rolled back with the testing.TB it was called with). Then change runs twice in
// transactions of its own (composables.InTx on the pool): first one that is
// rolled back after the change, then one that commits. After each, the outbox
// and the domain state are read over another connection: nothing of the change
// is visible after the rollback (both the events and the state equal what they
// were before), and after the commit exactly want[event] new rows of every
// event in events are there and the state differs from before. It returns the
// new rows of the committed run by event. ctx carries the actor, tenant and
// history execution the change needs; change gets a suffix to make the names it
// creates unique and must report only its own error.
func (te *TestEnvironment) OutboxCommitsAndRollsBack(tb testing.TB, ctx context.Context, tenantID uuid.UUID, events []string, want map[string]int, domain Domain, change func(ctx context.Context, suffix string) error) map[string][]OutboxRow {
	tb.Helper()
	detached := context.WithValue(ctx, constants.TxKey, nil)
	detached = context.WithValue(detached, constants.AfterCommitKey, nil)

	before := te.OutboxCounts(tb, tenantID, events...)
	stateBefore := te.domain(tb, domain)

	var inside map[string]int
	err := composables.InTx(detached, func(txCtx context.Context) error {
		if err := change(txCtx, "rolled-back"); err != nil {
			return err
		}
		inside = countIn(tb, txCtx, tenantID, events)
		return errRollBack
	})
	require.ErrorIs(tb, err, errRollBack, "the change reports no error of its own")
	for _, id := range events {
		require.Equal(tb, before[id]+want[id], inside[id], "%s: the event is in the transaction of the change", id)
	}
	require.Equal(tb, before, te.OutboxCounts(tb, tenantID, events...), "rolling the change back removes its events")
	require.Equal(tb, stateBefore, te.domain(tb, domain), "rolling the change back removes its state")

	require.NoError(tb, composables.InTx(detached, func(txCtx context.Context) error { return change(txCtx, "committed") }))
	got := map[string][]OutboxRow{}
	for _, id := range events {
		rows := te.OutboxRows(tb, tenantID, id)
		require.Len(tb, rows, before[id]+want[id], "%s: the committed change publishes its events once", id)
		got[id] = rows[before[id]:]
	}
	if domain.SQL != "" {
		require.NotEqual(tb, stateBefore, te.domain(tb, domain), "the committed change wrote its state")
	}
	return got
}

func countIn(tb testing.TB, ctx context.Context, tenantID uuid.UUID, events []string) map[string]int {
	tb.Helper()
	tx, err := composables.UseTx(ctx)
	require.NoError(tb, err)
	out := make(map[string]int, len(events))
	for _, id := range events {
		var n int
		require.NoError(tb, tx.QueryRow(ctx, `SELECT count(*) FROM extruntime.outbox_events WHERE tenant_id = $1 AND event = $2`, tenantID, id).Scan(&n))
		out[id] = n
	}
	return out
}

// OutboxRowsIn is OutboxRows as the transaction of ctx sees them, for changes a
// test makes in its own (rolled back) scope.
func OutboxRowsIn(tb testing.TB, ctx context.Context, tenantID uuid.UUID, event string) []OutboxRow {
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

// scopeTx carries the rollback scope transaction of a test. The scope never
// commits, so the owner it declares runs after-commit callbacks (the eventbus
// events of the services under test) at once: a test observes them as it would
// after the commit of a real request.
func scopeTx(ctx context.Context, tx pgx.Tx) context.Context {
	return composables.WithAfterCommit(composables.WithTx(ctx, tx), tx, func(fn func()) { fn() })
}
