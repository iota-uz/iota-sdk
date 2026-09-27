package repo

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// gateTx is a fake pgx.Tx whose Exec blocks while hold is set and counts
// in-flight calls, so a test can keep one call busy while starting another.
type gateTx struct {
	pgx.Tx

	hold     atomic.Bool
	inFlight atomic.Int32
}

func (t *gateTx) Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	t.inFlight.Add(1)
	defer t.inFlight.Add(-1)
	for t.hold.Load() {
		select {
		case <-time.After(time.Millisecond):
		case <-ctx.Done():
			return pgconn.CommandTag{}, ctx.Err()
		}
	}
	return pgconn.NewCommandTag("SELECT 1"), nil
}

func (t *gateTx) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	return fakeRow{}
}

// fakeRows is a scriptable pgx.Rows used to verify the guard's lifetime
// around lazy result handles.
type fakeRows struct {
	pgx.Rows
	remaining atomic.Int32
	closed    atomic.Bool
}

func (r *fakeRows) Next() bool {
	if r.closed.Load() {
		return false
	}
	return r.remaining.Add(-1) >= 0
}

func (r *fakeRows) Close() { r.closed.Store(true) }

type queryGateTx struct {
	gateTx
	rows fakeRows
}

func (t *queryGateTx) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	return &t.rows, nil
}

type fakeRow struct{}

func (fakeRow) Scan(dest ...any) error { return nil }

// waitFor polls until cond is true, failing the test after a timeout.
func waitFor(t *testing.T, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatal("timed out waiting for condition")
		}
		time.Sleep(time.Millisecond)
	}
}

func TestGuardedTx_SequentialExecSucceeds(t *testing.T) {
	t.Parallel()

	inner := &gateTx{}
	tx := NewGuardedTx(inner)

	for i := 0; i < 3; i++ {
		_, err := tx.Exec(context.Background(), "SELECT 1")
		require.NoError(t, err)
	}
}

func TestGuardedTx_ConcurrentExecFailsFast(t *testing.T) {
	t.Parallel()

	inner := &gateTx{}
	inner.hold.Store(true)
	tx := NewGuardedTx(inner)

	go func() {
		_, _ = tx.Exec(context.Background(), "SELECT 1 -- slow")
	}()
	waitFor(t, func() bool { return inner.inFlight.Load() == 1 })

	_, err := tx.Exec(context.Background(), "SELECT 2 -- concurrent")
	require.Error(t, err)
	require.ErrorIs(t, err, ErrTxInUse)
	assert.Contains(t, err.Error(), "single-connection")

	inner.hold.Store(false)
}

func TestGuardedTx_ConcurrentQueryFailsFast(t *testing.T) {
	t.Parallel()

	inner := &gateTx{}
	inner.hold.Store(true)
	tx := NewGuardedTx(inner)

	go func() {
		_, _ = tx.Exec(context.Background(), "SELECT 1")
	}()
	waitFor(t, func() bool { return inner.inFlight.Load() == 1 })

	_, err := tx.Query(context.Background(), "SELECT 2") //nolint:sqlclosecheck // rows are nil on the guarded error path
	require.ErrorIs(t, err, ErrTxInUse)

	inner.hold.Store(false)
}

func TestGuardedTx_ConcurrentCommitFailsFast(t *testing.T) {
	t.Parallel()

	inner := &gateTx{}
	inner.hold.Store(true)
	tx := NewGuardedTx(inner)

	go func() {
		_, _ = tx.Exec(context.Background(), "SELECT 1")
	}()
	waitFor(t, func() bool { return inner.inFlight.Load() == 1 })

	require.ErrorIs(t, tx.Commit(context.Background()), ErrTxInUse)

	inner.hold.Store(false)
}

func TestGuardedTx_ConcurrentRollbackFailsFast(t *testing.T) {
	t.Parallel()

	inner := &gateTx{}
	inner.hold.Store(true)
	tx := NewGuardedTx(inner)

	go func() {
		_, _ = tx.Exec(context.Background(), "SELECT 1")
	}()
	waitFor(t, func() bool { return inner.inFlight.Load() == 1 })

	require.ErrorIs(t, tx.Rollback(context.Background()), ErrTxInUse)

	inner.hold.Store(false)
}

func TestGuardedTx_ConcurrentQueryRowScanFailsFast(t *testing.T) {
	t.Parallel()

	inner := &gateTx{}
	inner.hold.Store(true)
	tx := NewGuardedTx(inner)

	go func() {
		_, _ = tx.Exec(context.Background(), "SELECT 1")
	}()
	waitFor(t, func() bool { return inner.inFlight.Load() == 1 })

	var n int
	err := tx.QueryRow(context.Background(), "SELECT 2").Scan(&n)
	require.ErrorIs(t, err, ErrTxInUse)

	inner.hold.Store(false)
}

func TestGuardedTx_GuardReleasedAfterScan(t *testing.T) {
	t.Parallel()

	inner := &gateTx{}
	tx := NewGuardedTx(inner)

	var n int
	require.NoError(t, tx.QueryRow(context.Background(), "SELECT 1").Scan(&n))

	// The lazy row released the guard on Scan: the next call goes through.
	_, err := tx.Exec(context.Background(), "SELECT 2")
	require.NoError(t, err)
}

func TestGuardedTx_ConcurrentUseAfterReleaseSucceeds(t *testing.T) {
	t.Parallel()

	inner := &gateTx{}
	inner.hold.Store(true)
	tx := NewGuardedTx(inner)

	done := make(chan struct{})
	go func() {
		defer close(done)
		_, _ = tx.Exec(context.Background(), "SELECT 1")
	}()
	waitFor(t, func() bool { return inner.inFlight.Load() == 1 })

	_, err := tx.Exec(context.Background(), "SELECT 2")
	require.ErrorIs(t, err, ErrTxInUse)

	inner.hold.Store(false)
	<-done

	_, err = tx.Exec(context.Background(), "SELECT 3")
	require.NoError(t, err)
}

func TestGuardedTx_KeepsGuardWhileRowsOpen(t *testing.T) {
	t.Parallel()

	inner := &queryGateTx{}
	inner.rows.remaining.Store(2)
	tx := NewGuardedTx(inner)

	rows, err := tx.Query(context.Background(), "SELECT 1")
	require.NoError(t, err)

	// Iteration not finished: the connection is still busy with the result
	// set, so another call must fail fast instead of racing it.
	require.True(t, rows.Next())
	_, err = tx.Exec(context.Background(), "SELECT 2")
	require.ErrorIs(t, err, ErrTxInUse)

	// Terminal Next releases the guard.
	require.True(t, rows.Next())
	require.False(t, rows.Next())
	_, err = tx.Exec(context.Background(), "SELECT 3")
	require.NoError(t, err)
}

func TestGuardedTx_ReleasesGuardOnRowsClose(t *testing.T) {
	t.Parallel()

	inner := &queryGateTx{}
	inner.rows.remaining.Store(5) // more rows than the test consumes
	tx := NewGuardedTx(inner)

	rows, err := tx.Query(context.Background(), "SELECT 1")
	require.NoError(t, err)

	require.True(t, rows.Next())
	rows.Close()

	_, err = tx.Exec(context.Background(), "SELECT 2")
	require.NoError(t, err, "Close must release the guard even with rows left")
}

func TestGuardedTx_BeginSharesGuardWithParent(t *testing.T) {
	t.Parallel()

	inner := &gateTx{}
	inner.hold.Store(true)
	tx := NewGuardedTx(inner)

	go func() {
		_, _ = tx.Exec(context.Background(), "SELECT 1")
	}()
	waitFor(t, func() bool { return inner.inFlight.Load() == 1 })

	// Begin while the parent is busy must fail fast…
	_, err := tx.Begin(context.Background())
	require.ErrorIs(t, err, ErrTxInUse)

	inner.hold.Store(false)
	waitFor(t, func() bool { return inner.inFlight.Load() == 0 })

	// …and the savepoint it returns shares the parent's guard: a busy
	// savepoint blocks the parent and vice versa.
	inner.hold.Store(true)
	go func() {
		_, _ = tx.Exec(context.Background(), "SELECT 1")
	}()
	waitFor(t, func() bool { return inner.inFlight.Load() == 1 })

	_, err = tx.Begin(context.Background())
	require.ErrorIs(t, err, ErrTxInUse)

	inner.hold.Store(false)
}

func TestGuardedTx_PrepareFailsFastWhenBusy(t *testing.T) {
	t.Parallel()

	inner := &gateTx{}
	inner.hold.Store(true)
	tx := NewGuardedTx(inner)

	go func() {
		_, _ = tx.Exec(context.Background(), "SELECT 1")
	}()
	waitFor(t, func() bool { return inner.inFlight.Load() == 1 })

	_, err := tx.Prepare(context.Background(), "p1", "SELECT 1")
	require.ErrorIs(t, err, ErrTxInUse)

	inner.hold.Store(false)
}
