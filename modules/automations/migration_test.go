package automations_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/iota-uz/iota-sdk/pkg/itf"
)

// The module's migration rolls back and applies again on a fully migrated schema.
func TestMigrationRollsBackAndApplies(t *testing.T) {
	h := itf.NewHarness(t, itf.HarnessConfig{
		Name:      "automations_migration",
		Database:  itf.DatabaseConfig{Provisioning: itf.ProvisioningPerTestDatabase},
		Isolation: itf.IsolationConfig{Mode: itf.IsolationCommitted},
	})
	db := h.Scope(t).Pool
	ctx := context.Background()
	body, err := os.ReadFile(filepath.Join("migrations", "changes-1795268219.sql"))
	require.NoError(t, err)
	up, down, ok := strings.Cut(string(body), "-- +migrate Down")
	require.True(t, ok)
	up = strings.TrimPrefix(up, "-- +migrate Up")
	tables := func() []string {
		rows, err := db.Query(ctx, `SELECT table_name FROM information_schema.tables WHERE table_schema = 'automation'
			AND table_name IN ('oauth_states', 'connection_checks', 'connection_requests') ORDER BY table_name`)
		require.NoError(t, err)
		defer rows.Close()
		var out []string
		for rows.Next() {
			var n string
			require.NoError(t, rows.Scan(&n))
			out = append(out, n)
		}
		require.NoError(t, rows.Err())
		return out
	}
	want := []string{"connection_checks", "connection_requests", "oauth_states"}
	assert.Equal(t, want, tables())
	_, err = db.Exec(ctx, down)
	require.NoError(t, err)
	assert.Empty(t, tables())
	_, err = db.Exec(ctx, up)
	require.NoError(t, err)
	assert.Equal(t, want, tables())
}
