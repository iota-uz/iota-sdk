package testdb

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"strconv"
	"sync"
	"testing"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/pkg/config/stdconfig/dbconfig"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/require"
)

func testConfig(t *testing.T) dbconfig.Config {
	t.Helper()
	dsn := os.Getenv("TESTENV_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("set TESTENV_POSTGRES_DSN to an isolated PostgreSQL cluster")
	}
	cfg, err := pgx.ParseConfig(dsn)
	require.NoError(t, err)
	return dbconfig.Config{Host: cfg.Host, Port: strconv.Itoa(int(cfg.Port)), User: cfg.User, Password: cfg.Password}
}
func TestClonePreservesSchemaAndIsolatesDestinations(t *testing.T) {
	// Falsely green if both destinations use the same connection or only public schema is tested.
	db := testConfig(t)
	ctx := context.Background()
	role := "testenv_reader_" + Name(uuid.NewString())
	admin, err := sql.Open("postgres", AdminConnectionString(db))
	require.NoError(t, err)
	_, err = admin.ExecContext(ctx, fmt.Sprintf(`CREATE ROLE "%s"`, role))
	require.NoError(t, err)
	t.Cleanup(func() {
		_, err := admin.ExecContext(ctx, fmt.Sprintf(`DROP ROLE "%s"`, role))
		require.NoError(t, err)
		require.NoError(t, admin.Close())
	})
	base := "testenv_base_" + Name(uuid.NewString())
	require.NoError(t, Create(ctx, base, db))
	t.Cleanup(func() { require.NoError(t, Drop(ctx, base, db)) })
	seed, err := sql.Open("postgres", ConnectionString(base, db))
	require.NoError(t, err)
	_, err = seed.ExecContext(ctx, "CREATE SCHEMA scenario; CREATE TABLE scenario.marker(id int primary key, value text); INSERT INTO scenario.marker VALUES(1,'baseline')")
	require.NoError(t, err)
	_, err = seed.ExecContext(ctx, fmt.Sprintf(`GRANT USAGE ON SCHEMA scenario TO "%s"; GRANT SELECT ON scenario.marker TO "%s"`, role, role))
	require.NoError(t, err)
	require.NoError(t, seed.Close())
	name := "testenv_clone_" + Name(uuid.NewString())
	require.Error(t, Clone(ctx, name, base, db))
	require.NoError(t, Seal(ctx, base, db))
	const count = 4
	names := make([]string, count)
	errs := make([]error, count)
	var wg sync.WaitGroup
	for i := range count {
		names[i] = "testenv_clone_" + Name(uuid.NewString())
		wg.Add(1)
		go func(i int) { defer wg.Done(); errs[i] = Clone(ctx, names[i], base, db) }(i)
	}
	wg.Wait()
	for i := range count {
		require.NoError(t, errs[i])
		t.Cleanup(func() { require.NoError(t, Drop(ctx, names[i], db)) })
	}
	first, err := sql.Open("postgres", ConnectionString(names[0], db))
	require.NoError(t, err)
	defer func() { require.NoError(t, first.Close()) }()
	_, err = first.ExecContext(ctx, "UPDATE scenario.marker SET value='changed' WHERE id=1")
	require.NoError(t, err)
	require.Error(t, Clone(ctx, names[0], base, db))
	var retained string
	require.NoError(t, first.QueryRowContext(ctx, "SELECT value FROM scenario.marker WHERE id=1").Scan(&retained))
	require.Equal(t, "changed", retained)
	second, err := sql.Open("postgres", ConnectionString(names[1], db))
	require.NoError(t, err)
	defer func() { require.NoError(t, second.Close()) }()
	var value string
	require.NoError(t, second.QueryRowContext(ctx, "SELECT value FROM scenario.marker WHERE id=1").Scan(&value))
	require.Equal(t, "baseline", value)
	tx, err := second.BeginTx(ctx, nil)
	require.NoError(t, err)
	_, err = tx.ExecContext(ctx, fmt.Sprintf(`SET LOCAL ROLE "%s"`, role))
	require.NoError(t, err)
	require.NoError(t, tx.QueryRowContext(ctx, "SELECT value FROM scenario.marker WHERE id=1").Scan(&value))
	require.Equal(t, "baseline", value)
	require.NoError(t, tx.Rollback())
}
func TestOwnedNamesRejectSystemAndNormalization(t *testing.T) {
	// Falsely green if malformed names reach PostgreSQL before validation.
	for _, name := range []string{"", "postgres", "template0", "template1", "some/name", "UPPER"} {
		require.Error(t, Create(context.Background(), name, dbconfig.Config{}))
		require.Error(t, Drop(context.Background(), name, dbconfig.Config{}))
	}
}
