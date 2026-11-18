package application_test

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"

	"github.com/iota-uz/iota-sdk/pkg/application"
	"github.com/iota-uz/iota-sdk/pkg/config/stdconfig/dbconfig"
	"github.com/iota-uz/iota-sdk/pkg/itf"
)

func writeMigrations(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	for name, body := range files {
		require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte(body), 0600))
	}
	return dir
}

func appliedMigrationIDs(t *testing.T, pool *pgxpool.Pool) []string {
	t.Helper()
	rows, err := pool.Query(context.Background(), `SELECT id FROM gorp_migrations ORDER BY id`)
	require.NoError(t, err)
	defer rows.Close()
	ids := []string{}
	for rows.Next() {
		var id string
		require.NoError(t, rows.Scan(&id))
		ids = append(ids, id)
	}
	require.NoError(t, rows.Err())
	return ids
}

func relationExists(t *testing.T, pool *pgxpool.Pool, name string) bool {
	t.Helper()
	var exists bool
	require.NoError(t, pool.QueryRow(context.Background(), `SELECT to_regclass($1) IS NOT NULL`, name).Scan(&exists))
	return exists
}

func newManager(pool *pgxpool.Pool, dir string, opts ...application.MigrationOption) application.MigrationManager {
	return application.NewMigrationManager(pool, dbconfig.Config{MigrationsDir: dir}, logrus.New(), opts...)
}

// Falsely green if Rollback still plans with no limit: all three tables would be dropped.
func TestMigrationRollbackRollsBackOneByDefault(t *testing.T) {
	pool := itf.NewDatabaseManager(t).Pool()
	ctx := context.Background()
	dir := writeMigrations(t, map[string]string{
		"0001_a.sql": "-- +migrate Up\nCREATE TABLE rollback_a (id int);\n-- +migrate Down\nDROP TABLE rollback_a;\n",
		"0002_b.sql": "-- +migrate Up\nCREATE TABLE rollback_b (id int);\n-- +migrate Down\nDROP TABLE rollback_b;\n",
		"0003_c.sql": "-- +migrate Up\nCREATE TABLE rollback_c (id int);\n-- +migrate Down\nDROP TABLE rollback_c;\n",
	})
	manager := newManager(pool, dir)

	require.NoError(t, manager.Run(ctx))
	require.Equal(t, []string{"0001_a.sql", "0002_b.sql", "0003_c.sql"}, appliedMigrationIDs(t, pool))

	require.NoError(t, manager.Rollback(ctx, 1))
	require.Equal(t, []string{"0001_a.sql", "0002_b.sql"}, appliedMigrationIDs(t, pool))
	require.True(t, relationExists(t, pool, "rollback_b"))
	require.False(t, relationExists(t, pool, "rollback_c"))

	require.Error(t, manager.Rollback(ctx, 0), "zero steps must not mean 'everything'")
	require.Len(t, appliedMigrationIDs(t, pool), 2)

	require.NoError(t, manager.RollbackAll(ctx))
	require.Empty(t, appliedMigrationIDs(t, pool))
	require.False(t, relationExists(t, pool, "rollback_a"))
}

// Falsely green if the notransaction migration is not the one that runs CONCURRENTLY.
func TestMigrationNoTransactionRunsOutsideTransaction(t *testing.T) {
	pool := itf.NewDatabaseManager(t).Pool()
	ctx := context.Background()
	dir := writeMigrations(t, map[string]string{
		"0001_table.sql": "-- +migrate Up\nCREATE TABLE concurrent_idx (id int);\n-- +migrate Down\nDROP TABLE concurrent_idx;\n",
		"0002_index.sql": "-- +migrate Up notransaction\nCREATE INDEX CONCURRENTLY concurrent_idx_id ON concurrent_idx (id);\n" +
			"-- +migrate Down notransaction\nDROP INDEX CONCURRENTLY concurrent_idx_id;\n",
	})
	manager := newManager(pool, dir)

	require.NoError(t, manager.Run(ctx))
	require.True(t, relationExists(t, pool, "concurrent_idx_id"))
	require.Equal(t, []string{"0001_table.sql", "0002_index.sql"}, appliedMigrationIDs(t, pool))

	require.NoError(t, manager.Rollback(ctx, 1))
	require.False(t, relationExists(t, pool, "concurrent_idx_id"))
	require.True(t, relationExists(t, pool, "concurrent_idx"))
	require.Equal(t, []string{"0001_table.sql"}, appliedMigrationIDs(t, pool))
}

func countWaitingLocks(t *testing.T, pool *pgxpool.Pool, where string) int {
	t.Helper()
	var n int
	require.NoError(t, pool.QueryRow(context.Background(),
		`SELECT count(*) FROM pg_locks WHERE NOT granted AND database = (SELECT oid FROM pg_database WHERE datname = current_database()) AND `+where,
	).Scan(&n))
	return n
}

// Falsely green if the second runner is not proven to start while the first
// still holds the plan: the first runner is parked on a table lock the test
// holds, and the second must then be seen waiting on the advisory lock, not on
// that table. Without the advisory lock both would plan the migration and the
// second CREATE TABLE would fail.
func TestMigrationConcurrentRunAppliesOnce(t *testing.T) {
	dm := itf.NewDatabaseManager(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	_, err := dm.Pool().Exec(ctx, `CREATE TABLE concurrent_gate (id int)`)
	require.NoError(t, err)
	dir := writeMigrations(t, map[string]string{
		"0001_gated.sql": "-- +migrate Up\nLOCK TABLE concurrent_gate IN ACCESS SHARE MODE;\nCREATE TABLE concurrent_run (id int);\n-- +migrate Down\nDROP TABLE concurrent_run;\n",
	})

	var wg sync.WaitGroup
	t.Cleanup(wg.Wait)
	gate, err := dm.Pool().Begin(ctx)
	require.NoError(t, err)
	t.Cleanup(func() { _ = gate.Rollback(context.Background()) })
	_, err = gate.Exec(ctx, `LOCK TABLE concurrent_gate IN ACCESS EXCLUSIVE MODE`)
	require.NoError(t, err)

	errs := make([]error, 2)
	start := func(i int) {
		// Separate pools stand in for separate replicas.
		pool, err := pgxpool.NewWithConfig(ctx, dm.Pool().Config().Copy())
		require.NoError(t, err)
		t.Cleanup(pool.Close)
		manager := newManager(pool, dir, application.WithMigrationLockTimeout(0))
		wg.Add(1)
		go func() {
			defer wg.Done()
			errs[i] = manager.Run(ctx)
		}()
	}

	start(0)
	require.Eventually(t, func() bool {
		return countWaitingLocks(t, dm.Pool(), `relation = 'concurrent_gate'::regclass`) == 1
	}, 10*time.Second, 20*time.Millisecond, "first runner must block inside its migration")
	start(1)
	require.Eventually(t, func() bool {
		return countWaitingLocks(t, dm.Pool(), `locktype = 'advisory'`) == 1
	}, 10*time.Second, 20*time.Millisecond, "second runner must wait for the advisory lock")
	require.Equal(t, 1, countWaitingLocks(t, dm.Pool(), `relation = 'concurrent_gate'::regclass`), "second runner must not reach the migration")

	require.NoError(t, gate.Commit(ctx))
	wg.Wait()
	require.NoError(t, errs[0])
	require.NoError(t, errs[1])
	require.Equal(t, []string{"0001_gated.sql"}, appliedMigrationIDs(t, dm.Pool()))
}

// Falsely green if the session statement_timeout is not shorter than the time
// the advisory lock is held by someone else.
func TestMigrationAdvisoryWaitIgnoresStatementTimeout(t *testing.T) {
	dm := itf.NewDatabaseManager(t)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	holder, err := dm.Pool().Acquire(ctx)
	require.NoError(t, err)
	defer holder.Release()
	_, err = holder.Exec(ctx, `SELECT pg_advisory_lock($1)`, application.DefaultMigrationAdvisoryLockKey)
	require.NoError(t, err)

	cfg := dm.Pool().Config().Copy()
	cfg.ConnConfig.RuntimeParams["statement_timeout"] = "100"
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	require.NoError(t, err)
	t.Cleanup(pool.Close)
	dir := writeMigrations(t, map[string]string{
		"0001_a.sql": "-- +migrate Up\nCREATE TABLE statement_timeout_a (id int);\n-- +migrate Down\nDROP TABLE statement_timeout_a;\n",
	})

	done := make(chan error, 1)
	go func() { done <- newManager(pool, dir).Run(ctx) }()
	require.Eventually(t, func() bool {
		return countWaitingLocks(t, dm.Pool(), `locktype = 'advisory'`) == 1
	}, 10*time.Second, 20*time.Millisecond)
	time.Sleep(300 * time.Millisecond) // longer than statement_timeout
	_, err = holder.Exec(ctx, `SELECT pg_advisory_unlock($1)`, application.DefaultMigrationAdvisoryLockKey)
	require.NoError(t, err)
	require.NoError(t, <-done)
	require.Equal(t, []string{"0001_a.sql"}, appliedMigrationIDs(t, dm.Pool()))
}

// Falsely green if 0002 is not genuinely pending when the rollback runs.
func TestMigrationRollbackIgnoresPendingOlderMigrations(t *testing.T) {
	pool := itf.NewDatabaseManager(t).Pool()
	ctx := context.Background()
	dir := writeMigrations(t, map[string]string{
		"0001_a.sql": "-- +migrate Up\nCREATE TABLE gap_a (id int);\n-- +migrate Down\nDROP TABLE gap_a;\n",
		"0003_c.sql": "-- +migrate Up\nCREATE TABLE gap_c (id int);\n-- +migrate Down\nDROP TABLE gap_c;\n",
	})
	manager := newManager(pool, dir)
	require.NoError(t, manager.Run(ctx))
	// A merge brings in an older migration that was never applied here.
	require.NoError(t, os.WriteFile(filepath.Join(dir, "0002_b.sql"), []byte("-- +migrate Up\nCREATE TABLE gap_b (id int);\n-- +migrate Down\nDROP TABLE gap_b;\n"), 0600))

	require.NoError(t, manager.Rollback(ctx, 1))
	require.Equal(t, []string{"0001_a.sql"}, appliedMigrationIDs(t, pool))
	require.False(t, relationExists(t, pool, "gap_b"), "rollback must not run pending Up SQL")
	require.False(t, relationExists(t, pool, "gap_c"))

	require.NoError(t, manager.RollbackAll(ctx))
	require.Empty(t, appliedMigrationIDs(t, pool))
	require.False(t, relationExists(t, pool, "gap_b"))
}

// Falsely green if fewer migrations are applied than requested only by accident.
func TestMigrationRollbackRejectsMoreStepsThanApplied(t *testing.T) {
	pool := itf.NewDatabaseManager(t).Pool()
	ctx := context.Background()
	dir := writeMigrations(t, map[string]string{
		"0001_a.sql": "-- +migrate Up\nCREATE TABLE steps_a (id int);\n-- +migrate Down\nDROP TABLE steps_a;\n",
	})
	manager := newManager(pool, dir)
	require.NoError(t, manager.Run(ctx))

	require.Error(t, manager.Rollback(ctx, 2))
	require.Equal(t, []string{"0001_a.sql"}, appliedMigrationIDs(t, pool), "nothing may change when the request cannot be met")
	require.True(t, relationExists(t, pool, "steps_a"))
}

// Falsely green if the migration does not actually wait on the held table lock.
func TestMigrationLockTimeoutAppliesToStatements(t *testing.T) {
	pool := itf.NewDatabaseManager(t).Pool()
	ctx := context.Background()
	_, err := pool.Exec(ctx, `CREATE TABLE lock_timeout_target (id int)`)
	require.NoError(t, err)

	tx, err := pool.Begin(ctx)
	require.NoError(t, err)
	t.Cleanup(func() { _ = tx.Rollback(ctx) })
	_, err = tx.Exec(ctx, `LOCK TABLE lock_timeout_target IN ACCESS EXCLUSIVE MODE`)
	require.NoError(t, err)

	dir := writeMigrations(t, map[string]string{
		"0001_alter.sql": "-- +migrate Up\nALTER TABLE lock_timeout_target ADD COLUMN name text;\n-- +migrate Down\nALTER TABLE lock_timeout_target DROP COLUMN name;\n",
	})
	manager := newManager(pool, dir, application.WithMigrationLockTimeout(200*time.Millisecond))

	runCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	err = manager.Run(runCtx)
	require.Error(t, err)
	var pgErr *pgconn.PgError
	require.ErrorAs(t, err, &pgErr)
	require.Equal(t, "55P03", pgErr.Code, "expected lock_not_available")
	require.Empty(t, appliedMigrationIDs(t, pool))
}
