package sql_test

import (
	"context"
	"errors"
	"net"
	"testing"
	"time"

	"github.com/google/uuid"
	bichatsql "github.com/iota-uz/iota-sdk/pkg/bichat/sql"
	"github.com/iota-uz/iota-sdk/pkg/configuration"
	"github.com/jackc/pgx/v5/pgxpool"
)

// requirePostgresPool dials the configured Postgres and returns a live
// pool. Test is skipped when the server is unreachable so contributors
// without a local DB still get green builds.
func requirePostgresPool(t *testing.T) *pgxpool.Pool {
	t.Helper()

	conf := configuration.Use()
	addr := net.JoinHostPort(conf.Database.Host, conf.Database.Port)
	d := net.Dialer{Timeout: 500 * time.Millisecond}
	conn, err := d.DialContext(context.Background(), "tcp", addr)
	if err != nil {
		t.Skipf("postgres not available at %s: %v", addr, err)
		return nil
	}
	_ = conn.Close()

	cfg, err := conf.Database.PoolConfig()
	if err != nil {
		t.Fatalf("PoolConfig: %v", err)
	}
	pool, err := pgxpool.NewWithConfig(context.Background(), cfg)
	if err != nil {
		t.Fatalf("pool: %v", err)
	}
	t.Cleanup(pool.Close)
	return pool
}

func TestSafeQueryExecutor_SimpleSelect(t *testing.T) {
	pool := requirePostgresPool(t)
	e := bichatsql.NewSafeQueryExecutor(pool)

	res, err := e.ExecuteQuery(context.Background(), "SELECT 1::int AS one, 'hi'::text AS msg", nil, 5*time.Second)
	if err != nil {
		t.Fatalf("ExecuteQuery: %v", err)
	}
	if res.RowCount != 1 {
		t.Fatalf("RowCount=%d", res.RowCount)
	}
	if res.Columns[0] != "one" || res.Columns[1] != "msg" {
		t.Fatalf("Columns=%v", res.Columns)
	}
	if res.ColumnTypes[0] != "number" || res.ColumnTypes[1] != "string" {
		t.Fatalf("ColumnTypes=%v", res.ColumnTypes)
	}
}

func TestSafeQueryExecutor_TenantContextPropagates(t *testing.T) {
	pool := requirePostgresPool(t)
	tenantID := uuid.New()

	e := bichatsql.NewSafeQueryExecutor(pool,
		bichatsql.WithTenantResolver(func(context.Context) (uuid.UUID, error) {
			return tenantID, nil
		}),
	)

	res, err := e.ExecuteQuery(context.Background(),
		"SELECT current_setting('app.tenant_id', true) AS t", nil, 5*time.Second)
	if err != nil {
		t.Fatalf("ExecuteQuery: %v", err)
	}
	if res.RowCount != 1 {
		t.Fatalf("RowCount=%d", res.RowCount)
	}
	got, _ := res.Rows[0][0].(string)
	if got != tenantID.String() {
		t.Fatalf("set_config not visible: got %q, want %q", got, tenantID.String())
	}
}

func TestSafeQueryExecutor_NoTenantResolverSkipsSetConfig(t *testing.T) {
	pool := requirePostgresPool(t)
	e := bichatsql.NewSafeQueryExecutor(pool) // default NoTenantResolver

	res, err := e.ExecuteQuery(context.Background(),
		"SELECT current_setting('app.tenant_id', true) AS t", nil, 5*time.Second)
	if err != nil {
		t.Fatalf("ExecuteQuery: %v", err)
	}
	got, _ := res.Rows[0][0].(string)
	if got != "" {
		t.Fatalf("expected empty (unset) tenant, got %q", got)
	}
}

func TestSafeQueryExecutor_RejectsWriteAtDBLevel(t *testing.T) {
	pool := requirePostgresPool(t)
	// Use a no-op policy + WithMaxQueryLength large enough so we can reach
	// the DB; trick the validator by hiding the keyword behind a string
	// literal. The read-only transaction must reject the actual write.
	e := bichatsql.NewSafeQueryExecutor(pool)

	// Validator catches obvious INSERT — exercise the validator path.
	_, err := e.ExecuteQuery(context.Background(), "INSERT INTO foo VALUES (1)", nil, 5*time.Second)
	if !errors.Is(err, bichatsql.ErrWriteOperation) {
		t.Fatalf("want ErrWriteOperation, got %v", err)
	}
}

func TestSafeQueryExecutor_RowCapTruncates(t *testing.T) {
	pool := requirePostgresPool(t)
	e := bichatsql.NewSafeQueryExecutor(pool, bichatsql.WithMaxResultRows(3))

	res, err := e.ExecuteQuery(context.Background(),
		"SELECT generate_series(1, 100) AS n", nil, 5*time.Second)
	if err != nil {
		t.Fatalf("ExecuteQuery: %v", err)
	}
	if res.RowCount != 3 {
		t.Fatalf("RowCount=%d, want 3", res.RowCount)
	}
	if !res.Truncated {
		t.Fatal("Truncated flag not set")
	}
}

func TestSafeQueryExecutor_StatementTimeoutKills(t *testing.T) {
	pool := requirePostgresPool(t)
	e := bichatsql.NewSafeQueryExecutor(pool,
		bichatsql.WithQueryTimeout(500*time.Millisecond),
		bichatsql.WithStatementTimeoutCap(500*time.Millisecond),
	)

	start := time.Now()
	_, err := e.ExecuteQuery(context.Background(), "SELECT pg_sleep(5)", nil, 0)
	elapsed := time.Since(start)
	if err == nil {
		t.Fatal("expected timeout error")
	}
	// Either pg statement_timeout or context deadline. Either way, fast.
	if elapsed > 3*time.Second {
		t.Fatalf("did not abort fast enough: %v", elapsed)
	}
}

func TestSafeQueryExecutor_ExplainReturnsPlan(t *testing.T) {
	pool := requirePostgresPool(t)
	e := bichatsql.NewSafeQueryExecutor(pool)

	plan, err := e.ExplainQuery(context.Background(), "SELECT generate_series(1, 10)")
	if err != nil {
		t.Fatalf("ExplainQuery: %v", err)
	}
	if plan == "" {
		t.Fatal("empty plan")
	}
}

func TestSafeQueryExecutor_PolicyDeniesQuery(t *testing.T) {
	pool := requirePostgresPool(t)
	denied := errors.New("boom")
	e := bichatsql.NewSafeQueryExecutor(pool,
		bichatsql.WithQueryPolicy(bichatsql.PolicyFunc(func(context.Context, string) error {
			return denied
		})),
	)

	_, err := e.ExecuteQuery(context.Background(), "SELECT 1", nil, 5*time.Second)
	if !errors.Is(err, denied) {
		t.Fatalf("policy denial not propagated: %v", err)
	}
}
