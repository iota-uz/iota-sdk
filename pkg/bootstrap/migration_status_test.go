package bootstrap

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/iota-uz/iota-sdk/pkg/application"
	"github.com/iota-uz/iota-sdk/pkg/config"
	"github.com/iota-uz/iota-sdk/pkg/config/providers/static"
	"github.com/iota-uz/iota-sdk/pkg/config/stdconfig/dbconfig"
	"github.com/iota-uz/iota-sdk/pkg/itf"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
)

func TestMain(m *testing.M) {
	if err := os.Chdir("../.."); err != nil {
		panic(err)
	}
	os.Exit(m.Run())
}

// Falsely green if the status requests use a pool with a different application name.
func TestMigrationStatusReleasesConnections(t *testing.T) {
	env := itf.Setup(t)
	ctx := context.Background()
	cfg := env.Pool.Config().Copy()
	cfg.MinConns = 0
	cfg.ConnConfig.RuntimeParams["application_name"] = "sdk-migration-status-test"
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	require.NoError(t, err)
	t.Cleanup(pool.Close)
	manager := application.NewMigrationManager(pool, dbconfig.Config{MigrationsDir: "migrations"}, logrus.New())
	for i := 0; i < 3; i++ {
		_, err = manager.Status(ctx)
		require.NoError(t, err)
	}
	var connections int
	require.NoError(t, env.Pool.QueryRow(ctx, `SELECT count(*) FROM pg_stat_activity WHERE application_name = $1`, "sdk-migration-status-test").Scan(&connections))
	require.Zero(t, connections)
}

// Falsely green if the migration tracking table still exists before the status call.
func TestMigrationStatusDoesNotCreateTable(t *testing.T) {
	env := itf.Setup(t)
	ctx := context.Background()
	_, err := env.Pool.Exec(ctx, `DROP TABLE public.gorp_migrations`)
	require.NoError(t, err)
	manager := application.NewMigrationManager(env.Pool, dbconfig.Config{MigrationsDir: "migrations"}, logrus.New())
	_, err = manager.Status(ctx)
	require.Error(t, err)
	var exists bool
	require.NoError(t, env.Pool.QueryRow(ctx, `SELECT to_regclass('public.gorp_migrations') IS NOT NULL`).Scan(&exists))
	require.False(t, exists)
}

// Falsely green if the fixture is not an applied migration in the test database.
func TestIotaSourceReportsAppliedMigration(t *testing.T) {
	env := itf.Setup(t)
	ctx := context.Background()
	var id string
	require.NoError(t, env.Pool.QueryRow(ctx, `SELECT id FROM gorp_migrations LIMIT 1`).Scan(&id))
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, id), []byte("-- +migrate Up\nSELECT 1;\n-- +migrate Down\nSELECT 1;\n"), 0600))
	src, err := config.Build(static.New(map[string]any{"db.migrationsdir": dir}))
	require.NoError(t, err)
	opts := &options{}
	IotaSource(src)(opts)
	app, err := opts.appFactory(ctx, &Runtime{Pool: env.Pool, Logger: logrus.New(), Bundle: application.LoadBundle()})
	require.NoError(t, err)
	t.Cleanup(func() { _ = app.Websocket().Close() })
	statuses, err := app.Migrations().Status(ctx)
	require.NoError(t, err)
	require.Len(t, statuses, 1)
	require.Equal(t, id, statuses[0].ID)
	require.True(t, statuses[0].Applied)
}
