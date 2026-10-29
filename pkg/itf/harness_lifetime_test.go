package itf

import (
	"context"
	"testing"

	"github.com/iota-uz/iota-sdk/modules"
	"github.com/stretchr/testify/require"
)

// Falsely green if the first scope commits: the second must see the same database without its writes.
func TestOpenHarnessRetainsDatabaseAndRollsBackScopes(t *testing.T) {
	h := OpenHarness(t, HarnessConfig{Name: "explicit_lifetime_test", Components: modules.Components()})
	t.Cleanup(func() { require.NoError(t, h.Close()) })
	var database string
	t.Run("first", func(t *testing.T) {
		s := h.Scope(t)
		require.NoError(t, s.Tx.QueryRow(s.Ctx, "SELECT current_database()").Scan(&database))
		_, err := s.Tx.Exec(s.Ctx, "CREATE TABLE lifetime_probe (id integer)")
		require.NoError(t, err)
	})
	t.Run("second", func(t *testing.T) {
		s := h.Scope(t)
		var next string
		require.NoError(t, s.Tx.QueryRow(s.Ctx, "SELECT current_database()").Scan(&next))
		require.Equal(t, database, next)
		var exists bool
		require.NoError(t, s.Pool.QueryRow(context.Background(), "SELECT to_regclass('public.lifetime_probe') IS NOT NULL").Scan(&exists))
		require.False(t, exists)
	})
}
