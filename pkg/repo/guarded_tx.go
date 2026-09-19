package repo

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// ErrTxInUse is returned by a guarded transaction when a second database call
// starts while another one is still in flight. A pgx transaction is bound to a
// single connection and is not safe for concurrent use: without the guard the
// failure surfaces later as an opaque driver error ("conn busy") or as silently
// interleaved protocol traffic.
var ErrTxInUse = errors.New("transaction is already executing a database call")

// GuardedTx wraps a pgx.Tx so that overlapping calls fail fast with
// ErrTxInUse instead of racing. The guard covers the data-access methods
// (Exec, Query, QueryRow, SendBatch, CopyFrom) and the terminal methods
// (Commit, Rollback). Sequential use is unaffected.
//
// Concurrent use of one transaction is always a defect in the caller — most
// commonly an errgroup or worker pool sharing an ambient request-scoped
// transaction — so tests built on the itf harness wrap their scope transaction
// automatically. Use NewGuardedTx to get the same fail-fast behaviour in other
// contexts, e.g. around request-scoped transactions in application code.
//
// Savepoints created through Begin are separate pgx.Tx values and are not
// tracked by the parent's guard.
type GuardedTx struct {
	pgx.Tx

	mu    sync.Mutex
	inUse bool
}

// NewGuardedTx wraps tx so overlapping calls fail fast with ErrTxInUse.
// The returned value satisfies pgx.Tx; type assertions and interface
// comparisons against the original transaction keep working through the
// embedded value.
func NewGuardedTx(tx pgx.Tx) pgx.Tx {
	return &GuardedTx{Tx: tx}
}

func (t *GuardedTx) enter() error {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.inUse {
		return fmt.Errorf(
			"%w; pgx transactions are single-connection: do not share one transaction across goroutines",
			ErrTxInUse,
		)
	}
	t.inUse = true
	return nil
}

func (t *GuardedTx) leave() {
	t.mu.Lock()
	t.inUse = false
	t.mu.Unlock()
}

func (t *GuardedTx) Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	if err := t.enter(); err != nil {
		return pgconn.CommandTag{}, err
	}
	defer t.leave()
	return t.Tx.Exec(ctx, sql, args...)
}

func (t *GuardedTx) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	if err := t.enter(); err != nil {
		return nil, err
	}
	defer t.leave()
	//nolint:sqlclosecheck // rows are returned to the caller, which owns closing them
	return t.Tx.Query(ctx, sql, args...)
}

func (t *GuardedTx) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	if err := t.enter(); err != nil {
		return errRow{err: err}
	}
	// leave() cannot run in a defer here: pgx.Row is lazy — the query executes
	// when Scan is called — so the guard must hold until then. The row wrapper
	// releases it on Scan.
	return &guardedRow{Row: t.Tx.QueryRow(ctx, sql, args...), leave: t.leave}
}

func (t *GuardedTx) SendBatch(ctx context.Context, b *pgx.Batch) pgx.BatchResults {
	if err := t.enter(); err != nil {
		return errBatchResults{err: err}
	}
	defer t.leave()
	return t.Tx.SendBatch(ctx, b)
}

func (t *GuardedTx) CopyFrom(ctx context.Context, tableName pgx.Identifier, columnNames []string, rowSrc pgx.CopyFromSource) (int64, error) {
	if err := t.enter(); err != nil {
		return 0, err
	}
	defer t.leave()
	return t.Tx.CopyFrom(ctx, tableName, columnNames, rowSrc)
}

func (t *GuardedTx) Commit(ctx context.Context) error {
	if err := t.enter(); err != nil {
		return err
	}
	defer t.leave()
	return t.Tx.Commit(ctx)
}

func (t *GuardedTx) Rollback(ctx context.Context) error {
	if err := t.enter(); err != nil {
		return err
	}
	defer t.leave()
	return t.Tx.Rollback(ctx)
}

// guardedRow releases the transaction guard when the lazy row is consumed.
type guardedRow struct {
	pgx.Row
	once  sync.Once
	leave func()
}

func (r *guardedRow) Scan(dest ...any) error {
	defer r.once.Do(r.leave)
	return r.Row.Scan(dest...)
}

type errRow struct {
	err error
}

func (r errRow) Scan(dest ...any) error {
	return r.err
}

type errBatchResults struct {
	err error
}

func (b errBatchResults) Exec() (pgconn.CommandTag, error) { return pgconn.CommandTag{}, b.err }
func (b errBatchResults) Query() (pgx.Rows, error)         { return nil, b.err }
func (b errBatchResults) QueryRow() pgx.Row                { return errRow{err: b.err} } //nolint:staticcheck // S1016 misreads the field/type name match as a conversion
func (b errBatchResults) Close() error                     { return b.err }
