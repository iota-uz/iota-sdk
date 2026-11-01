package bootstrap

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/iota-uz/iota-sdk/pkg/application"
	"github.com/iota-uz/iota-sdk/pkg/config"
	"github.com/iota-uz/iota-sdk/pkg/config/providers/static"
	"github.com/iota-uz/iota-sdk/pkg/itf"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
)

func TestMain(m *testing.M) {
	if err := os.Chdir("../.."); err != nil {
		panic(err)
	}
	os.Exit(m.Run())
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
