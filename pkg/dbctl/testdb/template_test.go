package testdb

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"testing/fstest"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestMigrationsFingerprintTracksTopLevelSQL(t *testing.T) {
	base := fstest.MapFS{
		"changes-1.sql":          {Data: []byte("-- +migrate Up\nSELECT 1;\n")},
		"README.md":              {Data: []byte("ignored")},
		"squashed/changes-0.sql": {Data: []byte("ignored too")},
	}
	fp, err := migrationsFingerprintFS(base)
	require.NoError(t, err)

	same := fstest.MapFS{"changes-1.sql": base["changes-1.sql"], "README.md": {Data: []byte("other")}}
	fpSame, err := migrationsFingerprintFS(same)
	require.NoError(t, err)
	require.Equal(t, fp, fpSame, "only top-level *.sql files count")

	edited := fstest.MapFS{"changes-1.sql": {Data: []byte("-- +migrate Up\nSELECT 2;\n")}}
	fpEdited, err := migrationsFingerprintFS(edited)
	require.NoError(t, err)
	require.NotEqual(t, fp, fpEdited)

	_, err = migrationsFingerprintFS(fstest.MapFS{"README.md": {Data: []byte("x")}})
	require.Error(t, err)
}

func templateTestFingerprint() string {
	return "test" + strings.ReplaceAll(uuid.NewString(), "-", "")
}

func dropTestTemplate(t *testing.T, conn *sql.DB, name string) {
	t.Helper()
	t.Cleanup(func() {
		require.NoError(t, dropTemplate(context.Background(), conn, name))
	})
}

func markerBuilder(t *testing.T, builds *atomic.Int32) TemplateBuilder {
	t.Helper()
	db := testConfig(t)
	return func(ctx context.Context, name string) error {
		builds.Add(1)
		conn, err := sql.Open("postgres", DBOpts(name, db))
		if err != nil {
			return err
		}
		defer func() { _ = conn.Close() }()
		_, err = conn.ExecContext(ctx, "CREATE TABLE template_marker (id int); INSERT INTO template_marker VALUES (7)")
		return err
	}
}

// Falsely green if concurrent callers each build their own copy, or if the
// template is left connectable (clones would then race autovacuum).
func TestEnsureMigratedTemplateBuildsOnceAndSeals(t *testing.T) {
	db := testConfig(t)
	ctx := context.Background()
	admin, err := sql.Open("postgres", AdminConnectionString(db))
	require.NoError(t, err)
	t.Cleanup(func() { _ = admin.Close() })
	fp := templateTestFingerprint()
	dropTestTemplate(t, admin, MigratedTemplateName(fp))

	var builds atomic.Int32
	build := markerBuilder(t, &builds)
	names := make([]string, 4)
	errs := make([]error, 4)
	var wg sync.WaitGroup
	for i := range names {
		wg.Add(1)
		go func() {
			defer wg.Done()
			names[i], errs[i] = EnsureMigratedTemplate(ctx, db, fp, build)
		}()
	}
	wg.Wait()
	for i := range names {
		require.NoError(t, errs[i])
		require.Equal(t, MigratedTemplateName(fp), names[i])
	}
	require.Equal(t, int32(1), builds.Load())

	ready, exists, err := templateState(ctx, admin, names[0])
	require.NoError(t, err)
	require.True(t, exists)
	require.True(t, ready)

	clone := Name("tpl_clone_" + uuid.NewString()[:8])
	require.NoError(t, createDatabase(ctx, clone, names[0], db, false, true))
	t.Cleanup(func() { _ = DropDB(clone, db) })
	conn, err := sql.Open("postgres", DBOpts(clone, db))
	require.NoError(t, err)
	defer func() { _ = conn.Close() }()
	var marker int
	require.NoError(t, conn.QueryRowContext(ctx, "SELECT id FROM template_marker").Scan(&marker))
	require.Equal(t, 7, marker)
}

// Falsely green if a database left behind by a killed builder is reused.
func TestEnsureMigratedTemplateRebuildsUnfinishedTemplate(t *testing.T) {
	db := testConfig(t)
	ctx := context.Background()
	admin, err := sql.Open("postgres", AdminConnectionString(db))
	require.NoError(t, err)
	t.Cleanup(func() { _ = admin.Close() })
	fp := templateTestFingerprint()
	name := MigratedTemplateName(fp)
	dropTestTemplate(t, admin, name)
	_, err = admin.ExecContext(ctx, fmt.Sprintf(`CREATE DATABASE "%s"`, name))
	require.NoError(t, err)

	var builds atomic.Int32
	got, err := EnsureMigratedTemplate(ctx, db, fp, markerBuilder(t, &builds))
	require.NoError(t, err)
	require.Equal(t, name, got)
	require.Equal(t, int32(1), builds.Load())
}

// Falsely green if a failed build is cached for the process or leaves the
// half-built database behind.
func TestEnsureMigratedTemplateRetriesAfterFailure(t *testing.T) {
	db := testConfig(t)
	ctx := context.Background()
	admin, err := sql.Open("postgres", AdminConnectionString(db))
	require.NoError(t, err)
	t.Cleanup(func() { _ = admin.Close() })
	fp := templateTestFingerprint()
	name := MigratedTemplateName(fp)
	dropTestTemplate(t, admin, name)

	boom := errors.New("boom")
	_, err = EnsureMigratedTemplate(ctx, db, fp, func(context.Context, string) error { return boom })
	require.ErrorIs(t, err, boom)
	_, exists, err := templateState(ctx, admin, name)
	require.NoError(t, err)
	require.False(t, exists, "a failed build is dropped")

	var builds atomic.Int32
	_, err = EnsureMigratedTemplate(ctx, db, fp, markerBuilder(t, &builds))
	require.NoError(t, err)
	require.Equal(t, int32(1), builds.Load())
}
