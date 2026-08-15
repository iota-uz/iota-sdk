package itf

import (
	"context"
	"database/sql"
	"fmt"
	"sync"
	"testing"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/pkg/config/stdconfig/dbconfig"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// adminConn opens the maintenance connection the template helpers use, or skips
// the test when no cluster is reachable so a clean checkout still runs
// `go test ./...` without Postgres.
func adminConn(t *testing.T) (*sql.DB, dbconfig.Config) {
	t.Helper()

	db := LoadDBConfigFromEnv()
	conn, err := sql.Open("postgres", adminConnString(db))
	if err != nil {
		t.Skipf("no admin connection: %v", err)
	}
	if err := conn.PingContext(context.Background()); err != nil {
		_ = conn.Close()
		t.Skipf("no Postgres at %s:%s: %v", db.Host, db.Port, err)
	}
	t.Cleanup(func() {
		require.NoError(t, conn.Close())
	})
	return conn, db
}

// buildTemplate creates a database carrying one marker table and marks it as a
// template, mirroring what CI does with the real migration set.
func buildTemplate(t *testing.T, conn *sql.DB, db dbconfig.Config) string {
	t.Helper()

	name := sanitizeDBName("itf_tpl_" + uuid.New().String()[:8])
	CreateDB(name, db)
	t.Cleanup(func() {
		_, _ = conn.ExecContext(context.Background(),
			"UPDATE pg_database SET datistemplate = false WHERE datname = $1", name)
		require.NoError(t, DropDB(name, db))
	})

	seed, err := sql.Open("postgres", DBOpts(name, db))
	require.NoError(t, err)
	_, err = seed.ExecContext(context.Background(), "CREATE TABLE template_marker (id int primary key)")
	require.NoError(t, err)
	_, err = seed.ExecContext(context.Background(), "INSERT INTO template_marker (id) VALUES (42)")
	require.NoError(t, err)
	// Postgres refuses to clone a database that still has connections open.
	require.NoError(t, seed.Close())

	_, err = conn.ExecContext(context.Background(),
		"UPDATE pg_database SET datistemplate = true WHERE datname = $1", name)
	require.NoError(t, err)

	return name
}

func TestCreateDBFromTemplate_CopiesSchema(t *testing.T) {
	conn, db := adminConn(t)
	template := buildTemplate(t, conn, db)

	clone := sanitizeDBName("itf_clone_" + uuid.New().String()[:8])
	require.NoError(t, CreateDBFromTemplateE(clone, template, db))
	t.Cleanup(func() { require.NoError(t, DropDB(clone, db)) })

	cloneConn, err := sql.Open("postgres", DBOpts(clone, db))
	require.NoError(t, err)
	defer func() { require.NoError(t, cloneConn.Close()) }()

	var id int
	require.NoError(t, cloneConn.QueryRowContext(context.Background(),
		"SELECT id FROM template_marker").Scan(&id))
	require.Equal(t, 42, id)
}

// Concurrent clones of one template are the normal case under `go test -p 8`:
// Postgres locks the source out for the duration of each copy, so without the
// advisory lock and the busy retry they fail with "source database is being
// accessed by other users".
func TestCreateDBFromTemplate_ConcurrentClones(t *testing.T) {
	conn, db := adminConn(t)
	template := buildTemplate(t, conn, db)

	const clones = 6
	errs := make([]error, clones)
	names := make([]string, clones)

	var wg sync.WaitGroup
	for i := range clones {
		names[i] = sanitizeDBName(fmt.Sprintf("itf_par_%s_%d", uuid.New().String()[:8], i))
		wg.Add(1)
		go func() {
			defer wg.Done()
			errs[i] = CreateDBFromTemplateE(names[i], template, db)
		}()
	}
	wg.Wait()

	for i, err := range errs {
		require.NoErrorf(t, err, "clone %d", i)
		require.NoError(t, DropDB(names[i], db))
	}
}

func TestCreateDBFromTemplate_MissingTemplateIsLoud(t *testing.T) {
	_, db := adminConn(t)

	err := CreateDBFromTemplateE(
		sanitizeDBName("itf_missing_"+uuid.New().String()[:8]),
		"itf_template_that_does_not_exist",
		db,
	)
	require.Error(t, err)
	require.Contains(t, err.Error(), "does not exist")
	require.Contains(t, err.Error(), TemplateDBEnv)
}

func TestNormalizeHarnessConfig_TemplatePrecedence(t *testing.T) {
	for _, tt := range []struct {
		name  string
		env   string
		given MigrationConfig
		want  string
	}{
		{
			name:  "environment supplies the template when the config is silent",
			env:   "some_template",
			given: MigrationConfig{},
			want:  "some_template",
		},
		{
			name:  "an explicit template wins over the environment",
			env:   "some_template",
			given: MigrationConfig{TemplateDB: "explicit"},
			want:  "explicit",
		},
		{
			name:  "no environment and no config means no cloning",
			env:   "",
			given: MigrationConfig{},
			want:  "",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv(TemplateDBEnv, tt.env)

			cfg := normalizeHarnessConfig(t, HarnessConfig{Name: "x", Migration: tt.given})

			assert.Equal(t, tt.want, cfg.Migration.TemplateDB)
		})
	}
}

// MigrationSkip only became reachable with cloning: before it, every harness
// started from an empty database and the readiness probe could not pass. These
// two cases are the contract - a clone of a migrated template satisfies the
// policy, a clone of one without migration bookkeeping is refused loudly rather
// than handing the suite a half-built schema.
func TestMigrationSkip_AcceptsAClonedTemplate(t *testing.T) {
	conn, db := adminConn(t)

	for _, tt := range []struct {
		name      string
		migrated  bool
		assertErr func(t *testing.T, err error)
	}{
		{
			name:     "a template carrying gorp_migrations is ready",
			migrated: true,
			assertErr: func(t *testing.T, err error) {
				t.Helper()
				assert.NoError(t, err)
			},
		},
		{
			name:     "a template without gorp_migrations is refused",
			migrated: false,
			assertErr: func(t *testing.T, err error) {
				t.Helper()
				require.Error(t, err)
				assert.Contains(t, err.Error(), "gorp_migrations")
			},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			template := buildTemplate(t, conn, db)
			if tt.migrated {
				markTemplateMigrated(t, conn, db, template)
			}

			clone := sanitizeDBName("itf_skip_" + uuid.New().String()[:8])
			require.NoError(t, CreateDBFromTemplateE(clone, template, db))
			t.Cleanup(func() { require.NoError(t, DropDB(clone, db)) })

			ctx := context.Background()
			pool, err := pgxpool.New(ctx, DBOpts(clone, db))
			require.NoError(t, err)
			t.Cleanup(pool.Close)

			err = runMigrationPolicy(ctx, pool, nil, MigrationConfig{
				Policy:     MigrationSkip,
				TemplateDB: template,
			})

			tt.assertErr(t, err)
		})
	}
}

// markTemplateMigrated gives a template the bookkeeping table the readiness
// probe looks for. It has to unmark and re-mark the template: Postgres refuses
// connections to a database while datistemplate is set.
func markTemplateMigrated(t *testing.T, conn *sql.DB, db dbconfig.Config, template string) {
	t.Helper()

	ctx := context.Background()
	_, err := conn.ExecContext(ctx,
		"UPDATE pg_database SET datistemplate = false WHERE datname = $1", template)
	require.NoError(t, err)

	seed, err := sql.Open("postgres", DBOpts(template, db))
	require.NoError(t, err)
	_, err = seed.ExecContext(ctx,
		"CREATE TABLE gorp_migrations (id text primary key, applied_at timestamptz)")
	require.NoError(t, err)
	require.NoError(t, seed.Close())

	_, err = conn.ExecContext(ctx,
		"UPDATE pg_database SET datistemplate = true WHERE datname = $1", template)
	require.NoError(t, err)
}

func TestBuildHarnessKey_SeparatesTemplates(t *testing.T) {
	t.Parallel()

	base := HarnessConfig{Name: "x"}
	withTemplate := HarnessConfig{Name: "x", Migration: MigrationConfig{TemplateDB: "tpl"}}

	require.NotEqual(t, buildHarnessKey(base), buildHarnessKey(withTemplate))
}
