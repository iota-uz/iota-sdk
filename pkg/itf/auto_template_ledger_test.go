package itf

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/require"

	"github.com/iota-uz/iota-sdk/pkg/config"
	staticprov "github.com/iota-uz/iota-sdk/pkg/config/providers/static"
	"github.com/iota-uz/iota-sdk/pkg/dbctl/testdb"
)

const tenantsMigration = `-- +migrate Up
CREATE TABLE public.tenants (id uuid PRIMARY KEY, name text, domain text, created_at timestamptz, updated_at timestamptz);
-- +migrate Down
DROP TABLE public.tenants;
`

// Falsely green if the clone check looks for public.gorp_migrations only. A
// role whose search_path starts with a schema every new database already has
// (here created in template1) gets its migration history there, and the
// automatic clone must accept it, as a replay does.
//
// It changes template1 and creates a role, so it runs only against an
// isolated cluster named by TESTENV_POSTGRES_DSN (superuser).
func TestAutoTemplateAcceptsLedgerOutsidePublic(t *testing.T) {
	dsn := os.Getenv("TESTENV_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("set TESTENV_POSTGRES_DSN to an isolated PostgreSQL cluster")
	}
	ctx := context.Background()
	cfg, err := pgx.ParseConfig(dsn)
	require.NoError(t, err)
	execAs := func(user, password, database, sql string) {
		t.Helper()
		c := cfg.Copy()
		c.Database = database
		if user != "" {
			c.User, c.Password = user, password
		}
		conn, err := pgx.ConnectConfig(ctx, c)
		require.NoError(t, err)
		defer func() { _ = conn.Close(ctx) }()
		_, err = conn.Exec(ctx, sql)
		require.NoError(t, err, sql)
	}
	exec := func(database, sql string) { t.Helper(); execAs("", "", database, sql) }

	role := "itf_ledger_" + strings.ReplaceAll(uuid.NewString()[:8], "-", "")
	exec("postgres", fmt.Sprintf(`CREATE ROLE %s LOGIN CREATEDB PASSWORD 'pw'`, role))
	exec("postgres", fmt.Sprintf(`ALTER ROLE %s SET search_path = review_fixture, public`, role))
	exec("template1", fmt.Sprintf(`CREATE SCHEMA review_fixture AUTHORIZATION %s`, role))
	t.Cleanup(func() {
		c := cfg.Copy()
		c.Database = "postgres"
		conn, err := pgx.ConnectConfig(ctx, c)
		require.NoError(t, err)
		defer func() { _ = conn.Close(ctx) }()
		rows, err := conn.Query(ctx, `SELECT datname FROM pg_database WHERE datdba = (SELECT oid FROM pg_roles WHERE rolname = $1)`, role)
		require.NoError(t, err)
		owned, err := pgx.CollectRows(rows, pgx.RowTo[string])
		require.NoError(t, err)
		for _, name := range owned {
			_, err := conn.Exec(ctx, fmt.Sprintf(`DROP DATABASE %q WITH (FORCE)`, name))
			require.NoError(t, err)
		}
		exec("template1", `DROP SCHEMA review_fixture`)
		_, err = conn.Exec(ctx, fmt.Sprintf(`DROP ROLE %s`, role))
		require.NoError(t, err)
	})

	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "0001_tenants.sql"), []byte(tenantsMigration), 0o600))
	port := strconv.Itoa(int(cfg.Port))
	t.Setenv("DB_HOST", cfg.Host)
	t.Setenv("DB_PORT", port)
	t.Setenv("DB_USER", role)
	t.Setenv("DB_PASSWORD", "pw")
	t.Setenv("DB_NAME", "postgres")
	t.Setenv(AutoTemplateEnv, "1")
	t.Setenv(TemplateDBEnv, "")
	src, err := config.Build(staticprov.New(map[string]any{
		"db.host": cfg.Host, "db.port": port, "db.user": role, "db.password": "pw",
		"db.name": "postgres", "db.migrationsdir": dir,
	}))
	require.NoError(t, err)

	h := OpenHarness(t, HarnessConfig{
		Name:     "itf_ledger_outside_public",
		Source:   src,
		Database: DatabaseConfig{Provisioning: ProvisioningPerTestDatabase},
	})
	t.Run("clone", func(t *testing.T) {
		s := h.Scope(t)
		var schemas []string
		rows, err := s.Pool.Query(ctx, `SELECT n.nspname FROM pg_class c JOIN pg_namespace n ON n.oid = c.relnamespace
			WHERE c.relname = 'gorp_migrations' ORDER BY 1`)
		require.NoError(t, err)
		schemas, err = pgx.CollectRows(rows, pgx.RowTo[string])
		require.NoError(t, err)
		require.Equal(t, []string{"review_fixture"}, schemas, "the history lives outside public")
	})
	require.NoError(t, h.Close())

	// An incomplete history is still refused. The database belongs to the
	// role, so the cleanup above drops it.
	clone := testdb.Name("itf_ledger_check_" + uuid.NewString()[:8])
	execAs(role, "pw", "postgres", fmt.Sprintf(`CREATE DATABASE %s`, clone))
	execAs(role, "pw", clone, `CREATE TABLE review_fixture.gorp_migrations (id varchar(255) PRIMARY KEY, applied_at timestamptz)`)
	pool := NewPool(fmt.Sprintf("host=%s port=%s user=%s password=pw dbname=%s sslmode=disable", cfg.Host, port, role, clone))
	defer pool.Close()
	require.ErrorContains(t, verifyAutoClone(ctx, pool, []string{"0001_tenants.sql"}), "no complete migration history")
	execAs(role, "pw", clone, `INSERT INTO review_fixture.gorp_migrations VALUES ('0001_tenants.sql', now())`)
	require.NoError(t, verifyAutoClone(ctx, pool, []string{"0001_tenants.sql"}))
}
