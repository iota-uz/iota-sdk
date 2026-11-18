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
}

// Falsely green if the two runners do not overlap: the migration sleeps so that
// without the advisory lock both would plan it as pending and the second CREATE
// TABLE would fail.
func TestMigrationConcurrentRunAppliesOnce(t *testing.T) {
	dm := itf.NewDatabaseManager(t)
	ctx := context.Background()
	dir := writeMigrations(t, map[string]string{
		"0001_slow.sql": "-- +migrate Up\nSELECT pg_sleep(0.5);\nCREATE TABLE concurrent_run (id int);\n-- +migrate Down\nDROP TABLE concurrent_run;\n",
	})

	const runners = 2
	errs := make([]error, runners)
	var wg sync.WaitGroup
	for i := range runners {
		// Separate pools stand in for separate replicas.
		pool, err := pgxpool.NewWithConfig(ctx, dm.Pool().Config().Copy())
		require.NoError(t, err)
		t.Cleanup(pool.Close)
		manager := newManager(pool, dir)
		wg.Add(1)
		go func() {
			defer wg.Done()
			errs[i] = manager.Run(ctx)
		}()
	}
	wg.Wait()

	for _, err := range errs {
		require.NoError(t, err)
	}
	require.Equal(t, []string{"0001_slow.sql"}, appliedMigrationIDs(t, dm.Pool()))
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
