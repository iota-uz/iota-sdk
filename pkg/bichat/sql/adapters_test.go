package sql_test

import (
	"context"
	"net"
	"os"
	"testing"
	"time"

	"github.com/iota-uz/iota-sdk/modules"
	infrastructure "github.com/iota-uz/iota-sdk/modules/bichat/infrastructure"
	bichatsql "github.com/iota-uz/iota-sdk/pkg/bichat/sql"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/configuration"
	"github.com/iota-uz/iota-sdk/pkg/itf"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMain(m *testing.M) {
	if err := os.Chdir("../../../"); err != nil {
		panic(err)
	}
	os.Exit(m.Run())
}

func requirePostgres(t *testing.T) {
	t.Helper()
	conf := configuration.Use()
	addr := net.JoinHostPort(conf.Database.Host, conf.Database.Port)
	d := net.Dialer{Timeout: 500 * time.Millisecond}
	conn, err := d.DialContext(context.Background(), "tcp", addr)
	if err != nil {
		t.Skipf("postgres not available at %s: %v", addr, err)
	}
	_ = conn.Close()
}

func TestSchemaLister_ReturnsDescriptionsAndRowCounts(t *testing.T) {
	t.Parallel()
	requirePostgres(t)
	env := itf.Setup(t, itf.WithModules(modules.BuiltInModules...))

	tenantID, err := composables.UseTenantID(env.Ctx)
	require.NoError(t, err)

	_, err = env.Pool.Exec(env.Ctx, `CREATE SCHEMA IF NOT EXISTS analytics`)
	require.NoError(t, err)

	_, err = env.Pool.Exec(env.Ctx, `
		CREATE TABLE _test_lister_base (
			id SERIAL PRIMARY KEY,
			tenant_id UUID NOT NULL,
			name TEXT
		)
	`)
	require.NoError(t, err)
	defer func() {
		_, _ = env.Pool.Exec(env.Ctx, "DROP VIEW IF EXISTS analytics._test_lister_view")
		_, _ = env.Pool.Exec(env.Ctx, "DROP TABLE IF EXISTS _test_lister_base")
	}()

	_, err = env.Pool.Exec(env.Ctx, `
		CREATE OR REPLACE VIEW analytics._test_lister_view AS
		SELECT * FROM _test_lister_base
		WHERE tenant_id = current_setting('app.tenant_id', true)::UUID
	`)
	require.NoError(t, err)

	_, err = env.Pool.Exec(env.Ctx, `COMMENT ON VIEW analytics._test_lister_view IS 'Test view for regression testing'`)
	require.NoError(t, err)

	_, err = env.Pool.Exec(env.Ctx, `
		INSERT INTO _test_lister_base (tenant_id, name)
		VALUES ($1, 'a'), ($1, 'b'), ($1, 'c')
	`, tenantID)
	require.NoError(t, err)

	executor := infrastructure.NewPostgresQueryExecutor(env.Pool)
	lister := bichatsql.NewQueryExecutorSchemaLister(executor,
		bichatsql.WithCountCacheTTL(10*time.Minute),
		bichatsql.WithCacheKeyFunc(func(ctx context.Context) (string, error) {
			tid, err := composables.UseTenantID(ctx)
			if err != nil {
				return "", err
			}
			return tid.String(), nil
		}),
	)

	tables, err := lister.SchemaList(env.Ctx)
	require.NoError(t, err)

	// Find our test view in the results
	var found *bichatsql.TableInfo
	for i := range tables {
		if tables[i].Name == "_test_lister_view" {
			found = &tables[i]
			break
		}
	}
	require.NotNil(t, found, "expected _test_lister_view in schema list")

	assert.Equal(t, "analytics", found.Schema)
	assert.Equal(t, "Test view for regression testing", found.Description,
		"description should come from COMMENT ON VIEW (regression: relkind::text cast)")
	assert.Equal(t, int64(3), found.RowCount,
		"row count should reflect inserted rows (regression: relkind type assertion)")
}

func TestSchemaLister_CachesViewCounts(t *testing.T) {
	t.Parallel()
	requirePostgres(t)
	env := itf.Setup(t, itf.WithModules(modules.BuiltInModules...))

	tenantID, err := composables.UseTenantID(env.Ctx)
	require.NoError(t, err)

	_, err = env.Pool.Exec(env.Ctx, `CREATE SCHEMA IF NOT EXISTS analytics`)
	require.NoError(t, err)

	_, err = env.Pool.Exec(env.Ctx, `
		CREATE TABLE _test_cache_base (
			id SERIAL PRIMARY KEY,
			tenant_id UUID NOT NULL,
			val INT
		)
	`)
	require.NoError(t, err)
	defer func() {
		_, _ = env.Pool.Exec(env.Ctx, "DROP VIEW IF EXISTS analytics._test_cache_view")
		_, _ = env.Pool.Exec(env.Ctx, "DROP TABLE IF EXISTS _test_cache_base")
	}()

	_, err = env.Pool.Exec(env.Ctx, `
		CREATE OR REPLACE VIEW analytics._test_cache_view AS
		SELECT * FROM _test_cache_base
		WHERE tenant_id = current_setting('app.tenant_id', true)::UUID
	`)
	require.NoError(t, err)

	_, err = env.Pool.Exec(env.Ctx, `
		INSERT INTO _test_cache_base (tenant_id, val)
		SELECT $1, generate_series(1, 5)
	`, tenantID)
	require.NoError(t, err)

	executor := infrastructure.NewPostgresQueryExecutor(env.Pool)
	lister := bichatsql.NewQueryExecutorSchemaLister(executor,
		bichatsql.WithCountCacheTTL(1*time.Hour),
		bichatsql.WithCacheKeyFunc(func(ctx context.Context) (string, error) {
			tid, err := composables.UseTenantID(ctx)
			if err != nil {
				return "", err
			}
			return tid.String(), nil
		}),
	)

	// First call populates cache
	tables1, err := lister.SchemaList(env.Ctx)
	require.NoError(t, err)

	var count1 int64
	for _, t := range tables1 {
		if t.Name == "_test_cache_view" {
			count1 = t.RowCount
			break
		}
	}
	require.Equal(t, int64(5), count1)

	// Insert more rows (cache should still return old count)
	_, err = env.Pool.Exec(env.Ctx, `
		INSERT INTO _test_cache_base (tenant_id, val)
		SELECT $1, generate_series(6, 10)
	`, tenantID)
	require.NoError(t, err)

	// Second call should use cached counts
	tables2, err := lister.SchemaList(env.Ctx)
	require.NoError(t, err)

	var count2 int64
	for _, t := range tables2 {
		if t.Name == "_test_cache_view" {
			count2 = t.RowCount
			break
		}
	}
	assert.Equal(t, int64(5), count2, "second call should return cached count, not updated 10")
}

func TestSchemaLister_NoCacheWithoutKeyFunc(t *testing.T) {
	t.Parallel()
	requirePostgres(t)
	env := itf.Setup(t, itf.WithModules(modules.BuiltInModules...))

	tenantID, err := composables.UseTenantID(env.Ctx)
	require.NoError(t, err)

	_, err = env.Pool.Exec(env.Ctx, `CREATE SCHEMA IF NOT EXISTS analytics`)
	require.NoError(t, err)

	_, err = env.Pool.Exec(env.Ctx, `
		CREATE TABLE _test_nocache_base (
			id SERIAL PRIMARY KEY,
			tenant_id UUID NOT NULL,
			val INT
		)
	`)
	require.NoError(t, err)
	defer func() {
		_, _ = env.Pool.Exec(env.Ctx, "DROP VIEW IF EXISTS analytics._test_nocache_view")
		_, _ = env.Pool.Exec(env.Ctx, "DROP TABLE IF EXISTS _test_nocache_base")
	}()

	_, err = env.Pool.Exec(env.Ctx, `
		CREATE OR REPLACE VIEW analytics._test_nocache_view AS
		SELECT * FROM _test_nocache_base
		WHERE tenant_id = current_setting('app.tenant_id', true)::UUID
	`)
	require.NoError(t, err)

	_, err = env.Pool.Exec(env.Ctx, `
		INSERT INTO _test_nocache_base (tenant_id, val) VALUES ($1, 1), ($1, 2)
	`, tenantID)
	require.NoError(t, err)

	// No bichatsql.WithCacheKeyFunc — should still work, just not cache
	executor := infrastructure.NewPostgresQueryExecutor(env.Pool)
	lister := bichatsql.NewQueryExecutorSchemaLister(executor)

	tables, err := lister.SchemaList(env.Ctx)
	require.NoError(t, err)

	var found bool
	for _, tbl := range tables {
		if tbl.Name == "_test_nocache_view" {
			assert.Equal(t, int64(2), tbl.RowCount)
			found = true
			break
		}
	}
	assert.True(t, found, "expected _test_nocache_view in schema list")
}

func TestSchemaDescriber_ReturnsColumns(t *testing.T) {
	t.Parallel()
	requirePostgres(t)
	env := itf.Setup(t, itf.WithModules(modules.BuiltInModules...))

	_, err := env.Pool.Exec(env.Ctx, `CREATE SCHEMA IF NOT EXISTS analytics`)
	require.NoError(t, err)

	_, err = env.Pool.Exec(env.Ctx, `
		CREATE TABLE _test_desc_base (
			id SERIAL PRIMARY KEY,
			tenant_id UUID NOT NULL,
			name TEXT NOT NULL,
			amount NUMERIC(10,2) DEFAULT 0,
			created_at TIMESTAMPTZ DEFAULT now()
		)
	`)
	require.NoError(t, err)
	defer func() {
		_, _ = env.Pool.Exec(env.Ctx, "DROP VIEW IF EXISTS analytics._test_desc_view")
		_, _ = env.Pool.Exec(env.Ctx, "DROP TABLE IF EXISTS _test_desc_base")
	}()

	_, err = env.Pool.Exec(env.Ctx, `
		CREATE OR REPLACE VIEW analytics._test_desc_view AS
		SELECT id, name, amount, created_at FROM _test_desc_base
		WHERE tenant_id = current_setting('app.tenant_id', true)::UUID
	`)
	require.NoError(t, err)

	executor := infrastructure.NewPostgresQueryExecutor(env.Pool)
	describer := bichatsql.NewQueryExecutorSchemaDescriber(executor)

	// This uses parameterized query ($1) internally — catches the params regression
	schema, err := describer.SchemaDescribe(env.Ctx, "_test_desc_view")
	require.NoError(t, err)
	require.NotNil(t, schema)

	assert.Equal(t, "_test_desc_view", schema.Name)
	assert.Equal(t, "analytics", schema.Schema)
	require.Len(t, schema.Columns, 4)

	colMap := make(map[string]bichatsql.ColumnInfo)
	for _, c := range schema.Columns {
		colMap[c.Name] = c
	}

	assert.Equal(t, "integer", colMap["id"].Type)
	assert.Equal(t, "text", colMap["name"].Type)
	assert.Equal(t, "numeric", colMap["amount"].Type)
	assert.Contains(t, colMap["created_at"].Type, "timestamp")
}

func TestSchemaDescriber_NonExistentTable(t *testing.T) {
	t.Parallel()
	requirePostgres(t)
	env := itf.Setup(t, itf.WithModules(modules.BuiltInModules...))

	_, err := env.Pool.Exec(env.Ctx, `CREATE SCHEMA IF NOT EXISTS analytics`)
	require.NoError(t, err)

	executor := infrastructure.NewPostgresQueryExecutor(env.Pool)
	describer := bichatsql.NewQueryExecutorSchemaDescriber(executor)

	schema, err := describer.SchemaDescribe(env.Ctx, "nonexistent_view_xyz")
	require.NoError(t, err)
	require.NotNil(t, schema)
	assert.Empty(t, schema.Columns, "non-existent table should return empty columns")
}
