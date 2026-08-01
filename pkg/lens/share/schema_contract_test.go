package share

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPostgresStoreMigrationContainsQueryContract(t *testing.T) {
	t.Parallel()
	ddl, err := os.ReadFile("../../../migrations/changes-1785498000.sql")
	require.NoError(t, err)
	schema := string(ddl)
	for _, fragment := range []string{
		"CREATE TABLE lens_saved_views",
		"CREATE TABLE lens_export_schedules",
		"UNIQUE (tenant_id, dashboard_id, id)",
		"FOREIGN KEY (tenant_id, owner_user_id)",
		"FOREIGN KEY (tenant_id, dashboard_id, view_id)",
		"scope IN ('personal', 'team')",
		"last_error TEXT NOT NULL DEFAULT ''",
		"consecutive_failures INTEGER NOT NULL DEFAULT 0",
		"CREATE INDEX lens_export_schedules_due_idx",
	} {
		require.Contains(t, schema, fragment)
	}
}
