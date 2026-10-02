package testenv

import (
	"context"
	"database/sql"
	"os"
	"strconv"
	"testing"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/pkg/config/stdconfig/dbconfig"
	"github.com/iota-uz/iota-sdk/pkg/dbctl/testdb"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/require"
)

func TestScenarioHTTPCommitsAtomicGraphAndCompensates(t *testing.T) {
	// Falsely green if only returned JSON is checked instead of durable PostgreSQL rows.
	dsn := os.Getenv("TESTENV_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("set TESTENV_POSTGRES_DSN to an isolated PostgreSQL cluster")
	}
	cfg, err := pgx.ParseConfig(dsn)
	require.NoError(t, err)
	dbcfg := dbconfig.Config{Host: cfg.Host, Port: strconv.Itoa(int(cfg.Port)), User: cfg.User, Password: cfg.Password}
	name := "testenv_registry_" + testdb.Name(uuid.NewString())
	ctx := context.Background()
	require.NoError(t, testdb.Create(ctx, name, dbcfg))
	t.Cleanup(func() { require.NoError(t, testdb.Drop(ctx, name, dbcfg)) })
	db, err := sql.Open("postgres", testdb.ConnectionString(name, dbcfg))
	require.NoError(t, err)
	defer db.Close()
	_, err = db.ExecContext(ctx, `CREATE TABLE scenario_invoice(scope text PRIMARY KEY, amount numeric CHECK(amount<100))`)
	require.NoError(t, err)
	r := NewRegistry([]string{"postgres"}, true)
	require.NoError(t, r.Register(testDefinition(), func(ctx context.Context, input Input) (Result, error) {
		tx, err := db.BeginTx(ctx, nil)
		if err != nil {
			return Result{}, err
		}
		defer tx.Rollback()
		if _, err = tx.ExecContext(ctx, "INSERT INTO scenario_invoice VALUES($1,$2)", input.ScopeID, input.Params["amount"]); err != nil {
			return Result{}, err
		}
		if err = tx.Commit(); err != nil {
			return Result{}, err
		}
		return Result{Data: map[string]any{"id": input.ScopeID}}, nil
	}, func(ctx context.Context, scope string) error {
		_, err := db.ExecContext(ctx, "DELETE FROM scenario_invoice WHERE scope=$1", scope)
		return err
	}))
	for _, id := range []string{"good", "bad"} {
		require.NoError(t, r.AllowScope(id))
	}
	handler, err := NewHandler(r, testToken)
	require.NoError(t, err)
	for range 2 {
		require.Equal(t, 200, request(t, handler, "POST", "/__test__/scenarios/prepare", testInput("good"), testToken).Code)
	}
	invalid := testInput("bad")
	invalid.Params["amount"] = float64(100)
	require.Equal(t, 500, request(t, handler, "POST", "/__test__/scenarios/prepare", invalid, testToken).Code)
	var count int
	require.NoError(t, db.QueryRowContext(ctx, "SELECT count(*) FROM scenario_invoice").Scan(&count))
	require.Equal(t, 1, count)
	require.Equal(t, 200, request(t, handler, "DELETE", "/__test__/scopes/good", nil, testToken).Code)
	require.NoError(t, db.QueryRowContext(ctx, "SELECT count(*) FROM scenario_invoice").Scan(&count))
	require.Zero(t, count)
}
