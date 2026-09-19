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

// txGuard is the shared concurrency state of one connection's transaction
// tree: a transaction and every savepoint begun from it run on the same
// connection, so they must contend on one flag.
type txGuard struct {
	mu    sync.Mutex
	inUse bool
}

func (g *txGuard) enter() error {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.inUse {
		return fmt.Errorf(
			"%w; pgx transactions are single-connection: do not share one transaction across goroutines",
			ErrTxInUse,
		)
	}
	g.inUse = true
	return nil
}

func (g *txGuard) leave() {
	g.mu.Lock()
	g.inUse = false
	g.mu.Unlock()
}

// GuardedTx wraps a pgx.Tx so that overlapping calls fail fast with
// ErrTxInUse instead of racing. The guard covers every method that drives the
// connection — Exec, Query, QueryRow, SendBatch, CopyFrom, Begin (savepoints),
// Prepare, Commit and Rollback. Lazy result handles keep the guard for their
// lifetime: pgx.Rows releases it on Close or when iteration finishes,
// pgx.BatchResults on Close, a QueryRow row on Scan. Sequential use is
// unaffected.
//
// Concurrent use of one transaction is always a defect in the caller — most
// commonly an errgroup or worker pool sharing an ambient request-scoped
// transaction — so tests built on the itf harness wrap their scope transaction
// automatically. Use NewGuardedTx to get the same fail-fast behaviour in other
// contexts, e.g. around request-scoped transactions in application code.
//
// Savepoint transactions returned by Begin share the parent's guard state,
// because they share the parent's connection.
type GuardedTx struct {
	pgx.Tx
	g *txGuard
}

// NewGuardedTx wraps tx so overlapping calls fail fast with ErrTxInUse.
// The returned value satisfies pgx.Tx; type assertions and interface
// comparisons against the original transaction keep working through the
// embedded value.
func NewGuardedTx(tx pgx.Tx) pgx.Tx {
	return &GuardedTx{Tx: tx, g: &txGuard{}}
}

func (t *GuardedTx) Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	if err := t.g.enter(); err != nil {
		return pgconn.CommandTag{}, err
	}
	defer t.g.leave()
	return t.Tx.Exec(ctx, sql, args...)
}

func (t *GuardedTx) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	if err := t.g.enter(); err != nil {
		return nil, err
	}
	rows, err := t.Tx.Query(ctx, sql, args...)
	if err != nil {
		t.g.leave()
		return nil, err
	}
	// pgx.Rows holds the connection until iteration completes: the guard
	// transfers to the wrapper and releases on Close or terminal Next.
	return &guardedRows{Rows: rows, g: t.g}, nil
}

func (t *GuardedTx) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	if err := t.g.enter(); err != nil {
		return errRow{err: err}
	}
	// leave() cannot run in a defer here: pgx.Row is lazy — the query executes
	// when Scan is called — so the guard must hold until then. The row wrapper
	// releases it on Scan.
	return &guardedRow{Row: t.Tx.QueryRow(ctx, sql, args...), leave: t.g.leave}
}

func (t *GuardedTx) SendBatch(ctx context.Context, b *pgx.Batch) pgx.BatchResults {
	if err := t.g.enter(); err != nil {
		return errBatchResults{err: err}
	}
	// BatchResults drives the same connection until Close: the guard
	// transfers to the wrapper and releases there.
	return &guardedBatchResults{BatchResults: t.Tx.SendBatch(ctx, b), g: t.g}
}

func (t *GuardedTx) CopyFrom(ctx context.Context, tableName pgx.Identifier, columnNames []string, rowSrc pgx.CopyFromSource) (int64, error) {
	if err := t.g.enter(); err != nil {
		return 0, err
	}
	defer t.g.leave()
	return t.Tx.CopyFrom(ctx, tableName, columnNames, rowSrc)
}

// Begin enters the guard for the savepoint round-trip and wraps the returned
// transaction with the SAME guard: a savepoint executes on the parent's
// connection, so parent and child must contend on one flag.
func (t *GuardedTx) Begin(ctx context.Context) (pgx.Tx, error) {
	if err := t.g.enter(); err != nil {
		return nil, err
	}
	defer t.g.leave()
	nested, err := t.Tx.Begin(ctx)
	if err != nil {
		return nil, err
	}
	return &GuardedTx{Tx: nested, g: t.g}, nil
}

func (t *GuardedTx) Prepare(ctx context.Context, name, sql string) (*pgconn.StatementDescription, error) {
	if err := t.g.enter(); err != nil {
		return nil, err
	}
	defer t.g.leave()
	return t.Tx.Prepare(ctx, name, sql)
}

func (t *GuardedTx) Commit(ctx context.Context) error {
	if err := t.g.enter(); err != nil {
		return err
	}
	defer t.g.leave()
	return t.Tx.Commit(ctx)
}

func (t *GuardedTx) Rollback(ctx context.Context) error {
	if err := t.g.enter(); err != nil {
		return err
	}
	defer t.g.leave()
	return t.Tx.Rollback(ctx)
}

// guardedRows keeps the transaction guard while pgx.Rows holds the
// connection. The guard is released exactly once, on Close or on the first
// terminal Next — whichever runs first.
type guardedRows struct {
	pgx.Rows
	release sync.Once
	g       *txGuard
}

func (r *guardedRows) Next() bool {
	hasNext := r.Rows.Next()
	if !hasNext {
		r.release.Do(r.g.leave)
	}
	return hasNext
}

func (r *guardedRows) Close() {
	r.release.Do(r.g.leave)
	r.Rows.Close()
}

// guardedBatchResults keeps the transaction guard until the batch is closed.
type guardedBatchResults struct {
	pgx.BatchResults
	release sync.Once
	g       *txGuard
}

func (b *guardedBatchResults) Close() error {
	defer b.release.Do(b.g.leave)
	return b.BatchResults.Close()
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

func (r errRow) Scan(dest ...any) error { return r.err }

type errBatchResults struct {
	err error
}

func (b errBatchResults) Exec() (pgconn.CommandTag, error) { return pgconn.CommandTag{}, b.err }
func (b errBatchResults) Query() (pgx.Rows, error)         { return nil, b.err }
func (b errBatchResults) QueryRow() pgx.Row                { return errRow{err: b.err} } //nolint:staticcheck // S1016 misreads the field/type name match as a conversion
func (b errBatchResults) Close() error                     { return b.err }
