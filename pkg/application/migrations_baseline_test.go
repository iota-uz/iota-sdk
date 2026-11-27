package application_test

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"

	"github.com/iota-uz/iota-sdk/pkg/config/stdconfig/dbconfig"
	"github.com/iota-uz/iota-sdk/pkg/itf"
)

const (
	sdkMigrationsDir = "../../migrations"
	// squashedHistoryDir holds the migrations changes-0001-baseline.sql
	// replaces, kept only so this test can replay them.
	squashedHistoryDir = "../../migrations/squashed/history"
)

// The baseline must leave a fresh database exactly where replaying the
// squashed history leaves one. "old" replays the squashed files, "fresh" runs
// the baseline; both must describe the same schema and rows. Migrations added
// after the baseline are applied to both. The baseline's down must leave
// nothing behind.
func TestSquashedBaselineMatchesHistory(t *testing.T) {
	ctx := context.Background()
	db := itf.LoadDBConfigFromEnv()
	old := newScratchDatabase(t, db, "squash_old")
	fresh := newScratchDatabase(t, db, "squash_fresh")

	require.NoError(t, newManager(old.pool, historyThenLater(t)).Run(ctx))
	require.NoError(t, newManager(fresh.pool, sdkMigrationsDir).Run(ctx))

	assertSameSchema(t, db, old, fresh)

	freshManager := newManager(fresh.pool, sdkMigrationsDir)

	// A consumer's table that depends on an SDK table (here by inheritance,
	// which DROP ... CASCADE would remove with its rows) stops the reset; the
	// rollback runs in one transaction, so nothing is dropped.
	appliedBefore := appliedMigrationIDs(t, fresh.pool)
	_, err := fresh.pool.Exec(ctx, `CREATE TABLE public.consumer_department_archive (archived_at timestamptz) INHERITS (core.departments)`)
	require.NoError(t, err)
	_, err = fresh.pool.Exec(ctx, `INSERT INTO public.consumer_department_archive (tenant_id, code, name, archived_at)
		VALUES (gen_random_uuid(), 'kept', '{}', now())`)
	require.NoError(t, err)
	require.Error(t, freshManager.RollbackAll(ctx))
	var kept int
	require.NoError(t, fresh.pool.QueryRow(ctx, `SELECT count(*) FROM ONLY public.consumer_department_archive WHERE code = 'kept'`).Scan(&kept))
	require.Equal(t, 1, kept, "the consumer's row survives the refused rollback")
	require.True(t, relationExists(t, fresh.pool, "core.departments"))
	require.Equal(t, appliedBefore, appliedMigrationIDs(t, fresh.pool), "the baseline stays applied")
	_, err = fresh.pool.Exec(ctx, `DROP TABLE public.consumer_department_archive`)
	require.NoError(t, err)

	// A consumer's sequence OWNED BY a baseline column would go with its table
	// even under RESTRICT; the Down's guard refuses the rollback instead.
	_, err = fresh.pool.Exec(ctx, `CREATE SEQUENCE core.consumer_department_seq OWNED BY core.departments.id`)
	require.NoError(t, err)
	require.ErrorContains(t, freshManager.RollbackAll(ctx), "consumer_department_seq")
	require.True(t, relationExists(t, fresh.pool, "core.consumer_department_seq"))
	require.True(t, relationExists(t, fresh.pool, "core.departments"))
	require.Equal(t, appliedBefore, appliedMigrationIDs(t, fresh.pool))
	_, err = fresh.pool.Exec(ctx, `DROP SEQUENCE core.consumer_department_seq`)
	require.NoError(t, err)

	require.NoError(t, freshManager.RollbackAll(ctx))
	require.Empty(t, appliedMigrationIDs(t, fresh.pool))
	require.Empty(t, userObjects(t, fresh.pool), "the baseline's down drops everything it created")

	require.NoError(t, freshManager.Run(ctx))
	assertSameSchema(t, db, old, fresh)
}

// historyThenLater is a migrations directory holding the squashed history and
// every migration that came after the baseline: the full replay the baseline
// stands in for.
func historyThenLater(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	copySQL := func(from string, skip func(string) bool) {
		entries, err := os.ReadDir(from)
		require.NoError(t, err)
		for _, entry := range entries {
			if entry.IsDir() || filepath.Ext(entry.Name()) != ".sql" || skip(entry.Name()) {
				continue
			}
			body, err := os.ReadFile(filepath.Join(from, entry.Name()))
			require.NoError(t, err)
			require.NoError(t, os.WriteFile(filepath.Join(dir, entry.Name()), body, 0o600))
		}
	}
	copySQL(squashedHistoryDir, func(string) bool { return false })
	copySQL(sdkMigrationsDir, func(name string) bool { return name == "changes-0001-baseline.sql" })
	return dir
}

type scratchDatabase struct {
	name string
	pool *pgxpool.Pool
}

func newScratchDatabase(t *testing.T, db dbconfig.Config, prefix string) scratchDatabase {
	t.Helper()
	name := prefix + "_" + strings.ReplaceAll(uuid.NewString()[:8], "-", "")
	require.NoError(t, itf.CreateDBE(name, db))
	pool := itf.NewPool(itf.DBOpts(name, db))
	t.Cleanup(func() {
		pool.Close()
		_ = itf.DropDB(name, db)
	})
	return scratchDatabase{name: name, pool: pool}
}

func assertSameSchema(t *testing.T, db dbconfig.Config, old, fresh scratchDatabase) {
	t.Helper()
	require.Equal(t, catalogSnapshot(t, old.pool), catalogSnapshot(t, fresh.pool))
	require.Equal(t, tableRows(t, old.pool), tableRows(t, fresh.pool), "rows the migrations insert")

	dump, ok := pgDumpFor(t, old.pool)
	if !ok {
		return
	}
	require.Equal(t, dump(t, db, old.name), dump(t, db, fresh.name), "pg_dump --schema-only")
}

// catalogSnapshot describes every user object through the server's own
// deparse functions, so it needs no client tools and runs wherever the tests
// do. pg_dump is compared as well when a compatible one is installed.
func catalogSnapshot(t *testing.T, pool *pgxpool.Pool) []string {
	t.Helper()
	const userSchema = `n.nspname NOT IN ('pg_catalog', 'information_schema', 'pg_toast') AND n.nspname NOT LIKE 'pg_temp%' AND n.nspname NOT LIKE 'pg_toast_temp%'`
	// An ACL equal to the owner's default is the same as none: REVOKE in the
	// history materialised the default on some objects, which pg_dump omits.
	queries := []string{
		`SELECT 'schema ' || n.nspname || ' ' || coalesce(obj_description(n.oid, 'pg_namespace'), '') || ' ' || coalesce(nullif(n.nspacl, acldefault('n', n.nspowner))::text, '')
		 FROM pg_namespace n WHERE ` + userSchema,
		// Columns in attnum order, without attnum itself: a history that drops
		// columns leaves gaps the baseline does not have.
		`SELECT 'column ' || n.nspname || '.' || c.relname || ' ' || string_agg(
			a.attname || ' ' || format_type(a.atttypid, a.atttypmod) || CASE WHEN a.attnotnull THEN ' not null' ELSE '' END
			|| coalesce(' default ' || pg_get_expr(d.adbin, d.adrelid), '') || CASE WHEN a.attidentity <> '' THEN ' identity ' || a.attidentity::text ELSE '' END
			|| CASE WHEN a.attgenerated <> '' THEN ' generated' ELSE '' END
			|| coalesce(' comment ' || col_description(c.oid, a.attnum), ''), ', ' ORDER BY a.attnum)
		 FROM pg_attribute a JOIN pg_class c ON c.oid = a.attrelid JOIN pg_namespace n ON n.oid = c.relnamespace
		 LEFT JOIN pg_attrdef d ON d.adrelid = a.attrelid AND d.adnum = a.attnum
		 WHERE a.attnum > 0 AND NOT a.attisdropped AND c.relkind IN ('r', 'p', 'v', 'm', 'f') AND ` + userSchema + `
		 AND NOT (n.nspname = 'public' AND c.relname = 'gorp_migrations')
		 GROUP BY n.nspname, c.relname`,
		`SELECT 'relation ' || n.nspname || '.' || c.relname || ' ' || c.relkind::text || ' rls=' || c.relrowsecurity::text || '/' || c.relforcerowsecurity::text
			|| ' acl=' || coalesce(nullif(c.relacl, acldefault(CASE WHEN c.relkind = 'S' THEN 's' ELSE 'r' END::"char", c.relowner))::text, '') || ' comment=' || coalesce(obj_description(c.oid, 'pg_class'), '')
		 FROM pg_class c JOIN pg_namespace n ON n.oid = c.relnamespace
		 WHERE c.relkind IN ('r', 'p', 'v', 'm', 'S', 'f') AND ` + userSchema,
		`SELECT 'constraint ' || n.nspname || '.' || c.relname || ' ' || con.conname || ' ' || pg_get_constraintdef(con.oid)
		 FROM pg_constraint con JOIN pg_class c ON c.oid = con.conrelid JOIN pg_namespace n ON n.oid = c.relnamespace
		 -- Postgres 18 records NOT NULL as named constraints, and a renamed column
		 -- keeps its old constraint name; nullability itself is in the columns.
		 WHERE con.contype <> 'n' AND ` + userSchema,
		`SELECT 'index ' || schemaname || ' ' || indexdef FROM pg_indexes WHERE schemaname NOT IN ('pg_catalog', 'information_schema')`,
		`SELECT 'view ' || n.nspname || '.' || c.relname || ' ' || pg_get_viewdef(c.oid)
		 FROM pg_class c JOIN pg_namespace n ON n.oid = c.relnamespace WHERE c.relkind IN ('v', 'm') AND ` + userSchema,
		`SELECT 'function ' || pg_get_functiondef(p.oid) || coalesce(' acl=' || nullif(p.proacl, acldefault('f', p.proowner))::text, '')
		 FROM pg_proc p JOIN pg_namespace n ON n.oid = p.pronamespace WHERE p.prokind IN ('f', 'p') AND ` + userSchema,
		`SELECT 'trigger ' || pg_get_triggerdef(tg.oid) FROM pg_trigger tg JOIN pg_class c ON c.oid = tg.tgrelid
		 JOIN pg_namespace n ON n.oid = c.relnamespace WHERE NOT tg.tgisinternal AND ` + userSchema,
		`SELECT 'policy ' || schemaname || '.' || tablename || ' ' || policyname || ' ' || permissive || ' ' || roles::text || ' ' || cmd
			|| ' ' || coalesce(qual, '') || ' ' || coalesce(with_check, '') FROM pg_policies`,
		`SELECT 'sequence ' || schemaname || '.' || sequencename || ' ' || data_type::text || ' ' || start_value || ' ' || increment_by || ' ' || max_value
		 FROM pg_sequences`,
		`SELECT 'type ' || n.nspname || '.' || t.typname || ' ' || t.typtype::text || ' ' || coalesce((SELECT string_agg(e.enumlabel, ',' ORDER BY e.enumsortorder) FROM pg_enum e WHERE e.enumtypid = t.oid), '')
		 FROM pg_type t JOIN pg_namespace n ON n.oid = t.typnamespace
		 WHERE t.typtype IN ('e', 'd', 'c') AND NOT EXISTS (SELECT 1 FROM pg_class c WHERE c.oid = t.typrelid AND c.relkind <> 'c') AND ` + userSchema,
		`SELECT 'default acl ' || coalesce(n.nspname, '') || ' ' || d.defaclobjtype::text || ' ' || d.defaclacl::text
		 FROM pg_default_acl d LEFT JOIN pg_namespace n ON n.oid = d.defaclnamespace`,
	}
	var out []string
	for _, q := range queries {
		rows, err := pool.Query(context.Background(), q+" ORDER BY 1")
		require.NoError(t, err, q)
		for rows.Next() {
			var line string
			require.NoError(t, rows.Scan(&line))
			out = append(out, line)
		}
		require.NoError(t, rows.Err())
		rows.Close()
	}
	return out
}

// tableRows returns the rows migrations insert (the default tenant,
// permissions and the like), table by table, as sorted JSON. Timestamp columns
// are left out: they hold the time each database was migrated.
func tableRows(t *testing.T, pool *pgxpool.Pool) map[string][]string {
	t.Helper()
	ctx := context.Background()
	rows, err := pool.Query(ctx, `
		SELECT quote_ident(n.nspname) || '.' || quote_ident(c.relname),
			coalesce((SELECT array_agg(a.attname::text) FROM pg_attribute a
				WHERE a.attrelid = c.oid AND a.attnum > 0 AND NOT a.attisdropped
				AND a.atttypid IN ('timestamp'::regtype, 'timestamptz'::regtype, 'date'::regtype)), '{}')
		FROM pg_class c JOIN pg_namespace n ON n.oid = c.relnamespace
		WHERE c.relkind IN ('r', 'p') AND n.nspname NOT IN ('pg_catalog', 'information_schema', 'pg_toast')
		AND NOT (n.nspname = 'public' AND c.relname = 'gorp_migrations')`)
	require.NoError(t, err)
	type table struct {
		name     string
		volatile []string
	}
	var tables []table
	for rows.Next() {
		var tb table
		require.NoError(t, rows.Scan(&tb.name, &tb.volatile))
		tables = append(tables, tb)
	}
	require.NoError(t, rows.Err())
	rows.Close()

	out := map[string][]string{}
	for _, tb := range tables {
		var values []string
		require.NoError(t, pool.QueryRow(ctx,
			`SELECT coalesce(array_agg(v ORDER BY v), '{}') FROM (SELECT (to_jsonb(r) - $1::text[])::text AS v FROM `+tb.name+` r) s`,
			tb.volatile).Scan(&values))
		if len(values) > 0 {
			out[tb.name] = values
		}
	}
	return out
}

func userObjects(t *testing.T, pool *pgxpool.Pool) []string {
	t.Helper()
	rows, err := pool.Query(context.Background(), `
		SELECT n.nspname || '.' || c.relname FROM pg_class c JOIN pg_namespace n ON n.oid = c.relnamespace
		WHERE n.nspname NOT IN ('pg_catalog', 'information_schema', 'pg_toast')
		AND c.relname NOT IN ('gorp_migrations', 'gorp_migrations_pkey')
		UNION ALL
		SELECT n.nspname || '.' || p.proname || '()' FROM pg_proc p JOIN pg_namespace n ON n.oid = p.pronamespace
		WHERE n.nspname NOT IN ('pg_catalog', 'information_schema')
		UNION ALL
		SELECT 'schema ' || nspname FROM pg_namespace
		WHERE nspname NOT IN ('pg_catalog', 'information_schema', 'pg_toast', 'public') AND nspname NOT LIKE 'pg_temp%' AND nspname NOT LIKE 'pg_toast_temp%'
		ORDER BY 1`)
	require.NoError(t, err)
	defer rows.Close()
	var out []string
	for rows.Next() {
		var name string
		require.NoError(t, rows.Scan(&name))
		out = append(out, name)
	}
	require.NoError(t, rows.Err())
	return out
}

type schemaDumper func(t *testing.T, db dbconfig.Config, name string) string

var (
	pgDumpVersion = regexp.MustCompile(`\(PostgreSQL\) (\d+)`)
	notNullName   = regexp.MustCompile(`CONSTRAINT \S+ NOT NULL`)
)

// pgDumpFor returns a schema-only pg_dump runner when an installed pg_dump can
// read this server (its major version is not older), so the test compares the
// two databases exactly the way the migration guide asks people to.
func pgDumpFor(t *testing.T, pool *pgxpool.Pool) (schemaDumper, bool) {
	t.Helper()
	bin, err := exec.LookPath("pg_dump")
	if err != nil {
		t.Log("pg_dump not installed; compared the catalogs only")
		return nil, false
	}
	out, err := exec.Command(bin, "--version").Output()
	require.NoError(t, err)
	m := pgDumpVersion.FindSubmatch(out)
	require.NotNil(t, m, string(out))
	clientMajor, _ := strconv.Atoi(string(m[1]))
	var serverNum int
	require.NoError(t, pool.QueryRow(context.Background(), `SELECT current_setting('server_version_num')::int`).Scan(&serverNum))
	if clientMajor < serverNum/10000 {
		t.Logf("pg_dump %d cannot dump server %d; compared the catalogs only", clientMajor, serverNum/10000)
		return nil, false
	}
	return func(t *testing.T, db dbconfig.Config, name string) string {
		t.Helper()
		cmd := exec.Command(bin, "--schema-only", "--no-owner", "--exclude-table=public.gorp_migrations",
			"-h", db.Host, "-p", db.Port, "-U", db.User, "-d", name)
		cmd.Env = append(os.Environ(), "PGPASSWORD="+db.Password)
		var stderr bytes.Buffer
		cmd.Stderr = &stderr
		dump, err := cmd.Output()
		require.NoError(t, err, stderr.String())
		var kept []string
		for _, line := range strings.Split(string(dump), "\n") {
			// \restrict carries a random key per run. pg_attver attribute
			// options are stamped by some server builds per column write and
			// differ between any two databases.
			if strings.HasPrefix(line, `\restrict`) || strings.HasPrefix(line, `\unrestrict`) || strings.Contains(line, "SET (pg_attver=") {
				continue
			}
			// Postgres 18 names NOT NULL constraints, and a renamed column keeps
			// the old name; the name carries no meaning, so it is dropped.
			kept = append(kept, notNullName.ReplaceAllString(line, "NOT NULL"))
		}
		return strings.Join(kept, "\n")
	}, true
}

func TestSquashedHistoryIsComplete(t *testing.T) {
	baseline, err := os.ReadFile(filepath.Join(sdkMigrationsDir, "changes-0001-baseline.sql"))
	require.NoError(t, err)
	entries, err := os.ReadDir(squashedHistoryDir)
	require.NoError(t, err)
	var squashed int
	for _, entry := range entries {
		if filepath.Ext(entry.Name()) != ".sql" {
			continue
		}
		squashed++
		require.Contains(t, string(baseline), fmt.Sprintf("-- +squashes %s\n", entry.Name()),
			"every kept file is one the baseline replaces")
	}
	require.Equal(t, squashed, strings.Count(string(baseline), "-- +squashes "))
}
