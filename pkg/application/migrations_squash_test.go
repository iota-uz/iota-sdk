package application_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/iota-uz/iota-sdk/pkg/application"
	"github.com/iota-uz/iota-sdk/pkg/itf"
)

var oldHistory = map[string]string{
	"0001_a.sql": "-- +migrate Up\nCREATE TABLE squash_a (id int);\n-- +migrate Down\nDROP TABLE squash_a;\n",
	"0002_b.sql": "-- +migrate Up\nCREATE TABLE squash_b (id int);\n-- +migrate Down\nDROP TABLE squash_b;\n",
}

// The baseline fails if it runs on a database that has the old history: the
// tables already exist. Adoption must record it without running it.
const squashBaseline = "-- Baseline of 0001 and 0002.\n" +
	"-- +squashes 0001_a.sql\n" +
	"-- +squashes 0002_b.sql\n" +
	"-- +migrate Up\nCREATE TABLE squash_a (id int);\nCREATE TABLE squash_b (id int);\n" +
	"-- +migrate Down\nDROP TABLE squash_b;\nDROP TABLE squash_a;\n"

func squashedDir(t *testing.T) string {
	t.Helper()
	return writeMigrations(t, map[string]string{
		"0000_baseline.sql": squashBaseline,
		"0003_c.sql":        "-- +migrate Up\nCREATE TABLE squash_c (id int);\n-- +migrate Down\nDROP TABLE squash_c;\n",
	})
}

func TestSquashedBaselineRunsOnFreshDatabase(t *testing.T) {
	pool := itf.NewDatabaseManager(t).Pool()
	ctx := context.Background()
	manager := newManager(pool, squashedDir(t))

	require.NoError(t, manager.Run(ctx))
	require.Equal(t, []string{"0000_baseline.sql", "0003_c.sql"}, appliedMigrationIDs(t, pool))
	require.True(t, relationExists(t, pool, "squash_a"))
	require.True(t, relationExists(t, pool, "squash_c"))

	// The baseline's down drops what it created; nothing is left to roll back.
	require.NoError(t, manager.RollbackAll(ctx))
	require.Empty(t, appliedMigrationIDs(t, pool))
	require.False(t, relationExists(t, pool, "squash_a"))
}

func TestSquashedBaselineIsAdoptedByOldHistory(t *testing.T) {
	pool := itf.NewDatabaseManager(t).Pool()
	ctx := context.Background()
	require.NoError(t, newManager(pool, writeMigrations(t, oldHistory)).Run(ctx))
	require.Equal(t, []string{"0001_a.sql", "0002_b.sql"}, appliedMigrationIDs(t, pool))

	manager := newManager(pool, squashedDir(t))
	statuses, err := manager.Status(ctx)
	require.NoError(t, err)
	require.Len(t, statuses, 2)
	require.True(t, statuses[0].Applied, "an unadopted old history already has the baseline's schema")
	require.False(t, statuses[1].Applied)

	require.NoError(t, manager.Run(ctx))
	require.Equal(t, []string{"0000_baseline.sql", "0003_c.sql"}, appliedMigrationIDs(t, pool))
	require.True(t, relationExists(t, pool, "squash_c"))

	// A second run is a no-op.
	require.NoError(t, manager.Run(ctx))
	require.Equal(t, []string{"0000_baseline.sql", "0003_c.sql"}, appliedMigrationIDs(t, pool))
}

func TestSquashedBaselineRejectsPartialHistory(t *testing.T) {
	pool := itf.NewDatabaseManager(t).Pool()
	ctx := context.Background()
	require.NoError(t, newManager(pool, writeMigrations(t, map[string]string{"0001_a.sql": oldHistory["0001_a.sql"]})).Run(ctx))

	err := newManager(pool, squashedDir(t)).Run(ctx)
	var partial *application.PartialSquashedHistoryError
	require.True(t, errors.As(err, &partial), "got %v", err)
	require.Equal(t, "0000_baseline.sql", partial.Baseline)
	require.Equal(t, 1, partial.Applied)
	require.Equal(t, []string{"0002_b.sql"}, partial.Missing)
	require.Contains(t, err.Error(), "migrate it up with the last release")
	require.Equal(t, []string{"0001_a.sql"}, appliedMigrationIDs(t, pool), "a refused run changes nothing")
	require.False(t, relationExists(t, pool, "squash_c"))
}

func TestSquashedBaselineClearsLeftoverIDs(t *testing.T) {
	pool := itf.NewDatabaseManager(t).Pool()
	ctx := context.Background()
	manager := newManager(pool, squashedDir(t))
	require.NoError(t, manager.Run(ctx))
	_, err := pool.Exec(ctx, `INSERT INTO gorp_migrations (id, applied_at) VALUES ('0001_a.sql', now())`)
	require.NoError(t, err)

	require.NoError(t, manager.Run(ctx))
	require.Equal(t, []string{"0000_baseline.sql", "0003_c.sql"}, appliedMigrationIDs(t, pool))
}

func TestSquashedBaselineRejectsReplacedFileStillPresent(t *testing.T) {
	pool := itf.NewDatabaseManager(t).Pool()
	dir := writeMigrations(t, map[string]string{
		"0000_baseline.sql": squashBaseline,
		"0001_a.sql":        oldHistory["0001_a.sql"],
	})
	err := newManager(pool, dir).Run(context.Background())
	require.ErrorContains(t, err, "still in the migrations directory")
}
