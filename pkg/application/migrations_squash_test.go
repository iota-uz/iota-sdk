package application_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/iota-uz/iota-sdk/pkg/application"
	"github.com/iota-uz/iota-sdk/pkg/itf"
)

var oldHistory = map[string]string{
	"0001_a.sql": "-- +migrate Up\nCREATE TABLE squash_a (id int);\n-- +migrate Down\nDROP TABLE squash_a;\n",
	"0002_b.sql": "-- +migrate Up\nCREATE TABLE squash_b (id int);\n-- +migrate Down\nDROP TABLE squash_b;\n",
}

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

// Falsely green if a database built from the squashed history is upgraded
// (or half-upgraded) instead of being refused with a "recreate" error.
func TestSquashedBaselineRefusesOldHistory(t *testing.T) {
	for name, history := range map[string]map[string]string{
		"complete": oldHistory,
		"partial":  {"0001_a.sql": oldHistory["0001_a.sql"]},
	} {
		t.Run(name, func(t *testing.T) {
			pool := itf.NewDatabaseManager(t).Pool()
			ctx := context.Background()
			require.NoError(t, newManager(pool, writeMigrations(t, history)).Run(ctx))
			before := appliedMigrationIDs(t, pool)

			manager := newManager(pool, squashedDir(t))
			for _, op := range []func() error{
				func() error { return manager.Run(ctx) },
				func() error { return manager.Rollback(ctx, 1) },
				func() error { return manager.RollbackAll(ctx) },
			} {
				err := op()
				var squashed *application.SquashedHistoryError
				require.ErrorAs(t, err, &squashed)
				require.Equal(t, "0000_baseline.sql", squashed.Baseline)
				require.Contains(t, err.Error(), "recreate the database")
			}
			require.Equal(t, before, appliedMigrationIDs(t, pool), "a refused run changes nothing")
			require.True(t, relationExists(t, pool, "squash_a"))
			require.False(t, relationExists(t, pool, "squash_c"))
		})
	}
}

// Falsely green if a bounded rollback runs the baseline's down: "roll back the
// last migration" would drop the whole schema.
func TestSquashedBaselineBoundedRollbackStopsAtBaseline(t *testing.T) {
	pool := itf.NewDatabaseManager(t).Pool()
	ctx := context.Background()
	manager := newManager(pool, squashedDir(t))
	require.NoError(t, manager.Run(ctx))

	require.ErrorContains(t, manager.Rollback(ctx, 2), "squashed baseline 0000_baseline.sql")
	require.Equal(t, []string{"0000_baseline.sql", "0003_c.sql"}, appliedMigrationIDs(t, pool), "a refused rollback changes nothing")

	require.NoError(t, manager.Rollback(ctx, 1), "migrations after the baseline still roll back one by one")
	require.Equal(t, []string{"0000_baseline.sql"}, appliedMigrationIDs(t, pool))
	require.False(t, relationExists(t, pool, "squash_c"))
	require.ErrorContains(t, manager.Rollback(ctx, 1), "RollbackAll")
	require.True(t, relationExists(t, pool, "squash_a"))

	require.NoError(t, manager.RollbackAll(ctx), "an explicit reset may run the baseline's down")
	require.Empty(t, appliedMigrationIDs(t, pool))
	require.False(t, relationExists(t, pool, "squash_a"))
}

func TestSquashDirectiveTakesTheWholeFileName(t *testing.T) {
	pool := itf.NewDatabaseManager(t).Pool()
	ctx := context.Background()
	require.NoError(t, newManager(pool, writeMigrations(t, map[string]string{
		"0001 old setup.sql": oldHistory["0001_a.sql"],
	})).Run(ctx))

	dir := writeMigrations(t, map[string]string{
		"0000_baseline.sql": "-- +squashes 0001 old setup.sql\n-- +migrate Up\nCREATE TABLE squash_a (id int);\n-- +migrate Down\nDROP TABLE squash_a;\n",
	})
	var squashed *application.SquashedHistoryError
	require.ErrorAs(t, newManager(pool, dir).Run(ctx), &squashed)
	require.Equal(t, []string{"0001 old setup.sql"}, squashed.Found)
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
