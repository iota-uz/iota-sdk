package itf

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/iota-uz/iota-sdk/pkg/config"
	staticprov "github.com/iota-uz/iota-sdk/pkg/config/providers/static"
)

// searchPathMigration is a valid migration that moves the database's
// search_path in front of public, as a consumer module might.
const searchPathMigration = `-- +migrate Up
CREATE SCHEMA review_settings;
CREATE TABLE review_settings.thresholds (id integer);
-- +migrate StatementBegin
DO $$
BEGIN
    EXECUTE format('ALTER DATABASE %I SET search_path = review_settings, public', current_database());
END
$$;
-- +migrate StatementEnd

-- +migrate Down
DROP SCHEMA review_settings CASCADE;
`

// Falsely green if a clone of the auto template is migrated again: it carries
// the template's search_path, so the migrator's unqualified ledger lookup lands
// in review_settings, finds no history and replays every migration
// ("schema already exists"). The post-clone policy must only verify the clone.
func TestAutoTemplateCloneIsNotMigratedAgain(t *testing.T) {
	db := LoadDBConfigFromEnv()
	dir := t.TempDir()
	entries, err := os.ReadDir("migrations")
	require.NoError(t, err)
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".sql" {
			continue
		}
		body, err := os.ReadFile(filepath.Join("migrations", entry.Name()))
		require.NoError(t, err)
		require.NoError(t, os.WriteFile(filepath.Join(dir, entry.Name()), body, 0o600))
	}
	// A unique file keeps this template apart from every other test's.
	name := "changes-9999999999-search-path-" + uuid.NewString()[:8] + ".sql"
	require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte(searchPathMigration), 0o600))

	src, err := config.Build(staticprov.New(map[string]any{
		"db.host": db.Host, "db.port": db.Port, "db.user": db.User, "db.password": db.Password,
		"db.name": db.Name, "db.migrationsdir": dir,
	}))
	require.NoError(t, err)

	for _, run := range []string{"builds", "reuses"} { // the second harness clones the template the first one built
		h := OpenHarness(t, HarnessConfig{
			Name:     "itf_auto_tpl_search_path",
			Source:   src,
			Database: DatabaseConfig{Provisioning: ProvisioningPerTestDatabase},
		})
		// The scope's transaction holds a pooled connection until its test
		// ends, so it runs in a subtest that finishes before Close.
		t.Run(run, func(t *testing.T) { assertClonedOnce(t, h.Scope(t)) })
		require.NoError(t, h.Close())
	}
}

func assertClonedOnce(t *testing.T, s *Scope) {
	t.Helper()
	{
		var path string
		require.NoError(t, s.Pool.QueryRow(context.Background(), "SHOW search_path").Scan(&path))
		require.Equal(t, "review_settings, public", path, "the clone keeps the template's settings")
		var ledgers []string
		rows, err := s.Pool.Query(context.Background(),
			`SELECT table_schema FROM information_schema.tables WHERE table_name = 'gorp_migrations' ORDER BY 1`)
		require.NoError(t, err)
		for rows.Next() {
			var schema string
			require.NoError(t, rows.Scan(&schema))
			ledgers = append(ledgers, schema)
		}
		require.NoError(t, rows.Err())
		rows.Close()
		require.Equal(t, []string{"public"}, ledgers, "no second ledger in the search_path's first schema")
	}
}
