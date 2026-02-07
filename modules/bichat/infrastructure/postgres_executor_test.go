package infrastructure

import (
	"context"
	"net"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/modules"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/configuration"
	"github.com/iota-uz/iota-sdk/pkg/itf"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMain(m *testing.M) {
	// Change to project root so ITF can find .env files and config
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
		return
	}
	_ = conn.Close()
}

func TestPostgresQueryExecutor_ExecuteQuery_MissingTenantID(t *testing.T) {
	t.Parallel()

	requirePostgres(t)
	env := itf.Setup(t, itf.WithModules(modules.BuiltInModules...))
	executor := NewPostgresQueryExecutor(env.Pool)

	// Context without tenant ID
	ctx := context.Background()

	_, err := executor.ExecuteQuery(ctx, "SELECT 1", nil, 5*time.Second)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "tenant ID required")
}

func TestPostgresQueryExecutor_ExecuteQuery_RejectsNonAnalyticsSchema(t *testing.T) {
	t.Parallel()

	requirePostgres(t)
	env := itf.Setup(t, itf.WithModules(modules.BuiltInModules...))
	executor := NewPostgresQueryExecutor(env.Pool)

	_, err := executor.ExecuteQuery(env.Ctx, "SELECT 1 FROM public.some_table", nil, 5*time.Second)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "analytics schema")
}

func TestPostgresQueryExecutor_ExecuteQuery_TenantIsolationEnforced(t *testing.T) {
	t.Parallel()

	requirePostgres(t)
	env := itf.Setup(t, itf.WithModules(modules.BuiltInModules...))
	executor := NewPostgresQueryExecutor(env.Pool)

	// Get tenant ID from context
	tenantID, err := composables.UseTenantID(env.Ctx)
	require.NoError(t, err)

	// Create analytics view that uses session variable for tenant filtering
	_, err = env.Pool.Exec(env.Ctx, `
		CREATE SCHEMA IF NOT EXISTS analytics
	`)
	require.NoError(t, err)

	// Create base table in public (view in analytics references it)
	_, err = env.Pool.Exec(env.Ctx, `
		CREATE TABLE test_tenant_data (
			id SERIAL PRIMARY KEY,
			tenant_id UUID NOT NULL,
			name VARCHAR(100),
			value INT
		)
	`)
	require.NoError(t, err)
	defer func() {
		_, _ = env.Pool.Exec(env.Ctx, "DROP VIEW IF EXISTS analytics.analytics_test_tenant_data")
		_, _ = env.Pool.Exec(env.Ctx, "DROP TABLE IF EXISTS test_tenant_data")
	}()

	// Create view in analytics schema with automatic tenant filtering
	_, err = env.Pool.Exec(env.Ctx, `
		CREATE OR REPLACE VIEW analytics.analytics_test_tenant_data AS
		SELECT * FROM test_tenant_data
		WHERE tenant_id = current_setting('app.tenant_id', true)::UUID
	`)
	require.NoError(t, err)

	// Insert data for current tenant
	_, err = env.Pool.Exec(env.Ctx, `
		INSERT INTO test_tenant_data (tenant_id, name, value) VALUES
			($1, 'Alice', 100),
			($1, 'Bob', 200)
	`, tenantID)
	require.NoError(t, err)

	// Insert data for different tenant (should NOT be accessible through view)
	otherTenantID := uuid.New()
	_, err = env.Pool.Exec(env.Ctx, `
		INSERT INTO test_tenant_data (tenant_id, name, value) VALUES
			($1, 'Charlie', 300),
			($1, 'David', 400)
	`, otherTenantID)
	require.NoError(t, err)

	// Execute query on analytics view (session variable set by executor)
	result, err := executor.ExecuteQuery(env.Ctx, "SELECT name, value FROM analytics.analytics_test_tenant_data ORDER BY value", nil, 5*time.Second)
	require.NoError(t, err)

	// Verify ONLY current tenant's data is returned (automatic filtering by view)
	assert.Equal(t, 2, result.RowCount, "should only see current tenant's 2 rows")
	assert.Len(t, result.Rows, 2)
	assert.Equal(t, "Alice", result.ToMap(0)["name"])
	assert.Equal(t, "Bob", result.ToMap(1)["name"])

	// Verify other tenant's data is NOT visible
	for i := range result.Rows {
		rowMap := result.ToMap(i)
		assert.NotEqual(t, "Charlie", rowMap["name"])
		assert.NotEqual(t, "David", rowMap["name"])
	}
}

func TestPostgresQueryExecutor_ExecuteQuery_Success(t *testing.T) {
	t.Parallel()

	requirePostgres(t)
	env := itf.Setup(t, itf.WithModules(modules.BuiltInModules...))
	executor := NewPostgresQueryExecutor(env.Pool)

	// Get tenant ID from context
	tenantID, err := composables.UseTenantID(env.Ctx)
	require.NoError(t, err)

	_, err = env.Pool.Exec(env.Ctx, `CREATE SCHEMA IF NOT EXISTS analytics`)
	require.NoError(t, err)

	_, err = env.Pool.Exec(env.Ctx, `
		CREATE TABLE test_data (
			id SERIAL PRIMARY KEY,
			tenant_id UUID NOT NULL,
			name VARCHAR(100),
			value INT
		)
	`)
	require.NoError(t, err)
	defer func() {
		_, _ = env.Pool.Exec(env.Ctx, "DROP VIEW IF EXISTS analytics.analytics_test_data")
		_, _ = env.Pool.Exec(env.Ctx, "DROP TABLE IF EXISTS test_data")
	}()

	_, err = env.Pool.Exec(env.Ctx, `
		CREATE OR REPLACE VIEW analytics.analytics_test_data AS
		SELECT * FROM test_data
		WHERE tenant_id = current_setting('app.tenant_id', true)::UUID
	`)
	require.NoError(t, err)

	// Insert test data with tenant_id
	_, err = env.Pool.Exec(env.Ctx, `
		INSERT INTO test_data (tenant_id, name, value) VALUES
			($1, 'Alice', 100),
			($1, 'Bob', 200),
			($1, 'Charlie', 300)
	`, tenantID)
	require.NoError(t, err)

	// Execute query on analytics view (session variable handles tenant filtering)
	result, err := executor.ExecuteQuery(env.Ctx, "SELECT name, value FROM analytics.analytics_test_data ORDER BY value", nil, 5*time.Second)
	require.NoError(t, err)

	assert.Len(t, result.Columns, 2)
	assert.Contains(t, result.Columns, "name")
	assert.Contains(t, result.Columns, "value")
	assert.Equal(t, 3, result.RowCount)
	assert.Len(t, result.Rows, 3)
	assert.Equal(t, "Alice", result.ToMap(0)["name"])
	assert.NotZero(t, result.Duration, "query should have non-zero duration")
}

func TestPostgresQueryExecutor_ExecuteQuery_WithParameters(t *testing.T) {
	t.Parallel()

	requirePostgres(t)
	env := itf.Setup(t, itf.WithModules(modules.BuiltInModules...))
	executor := NewPostgresQueryExecutor(env.Pool)

	// Get tenant ID from context
	tenantID, err := composables.UseTenantID(env.Ctx)
	require.NoError(t, err)

	_, err = env.Pool.Exec(env.Ctx, `CREATE SCHEMA IF NOT EXISTS analytics`)
	require.NoError(t, err)

	_, err = env.Pool.Exec(env.Ctx, `
		CREATE TABLE test_products (
			id SERIAL PRIMARY KEY,
			tenant_id UUID NOT NULL,
			name VARCHAR(100),
			price DECIMAL(10,2)
		)
	`)
	require.NoError(t, err)
	defer func() {
		_, _ = env.Pool.Exec(env.Ctx, "DROP VIEW IF EXISTS analytics.analytics_test_products")
		_, _ = env.Pool.Exec(env.Ctx, "DROP TABLE IF EXISTS test_products")
	}()

	_, err = env.Pool.Exec(env.Ctx, `
		CREATE OR REPLACE VIEW analytics.analytics_test_products AS
		SELECT * FROM test_products
		WHERE tenant_id = current_setting('app.tenant_id', true)::UUID
	`)
	require.NoError(t, err)

	// Insert test data with tenant_id
	_, err = env.Pool.Exec(env.Ctx, `
		INSERT INTO test_products (tenant_id, name, price) VALUES
			($1, 'Widget A', 10.50),
			($1, 'Widget B', 25.00),
			($1, 'Widget C', 5.99)
	`, tenantID)
	require.NoError(t, err)

	// Execute query with user parameter (price is $1, tenant_id is automatic via session variable)
	result, err := executor.ExecuteQuery(
		env.Ctx,
		"SELECT name, price FROM analytics.analytics_test_products WHERE price > $1 ORDER BY price",
		[]any{10.0},
		5*time.Second,
	)
	require.NoError(t, err)

	assert.Equal(t, 2, result.RowCount)
	assert.Equal(t, "Widget A", result.ToMap(0)["name"])
	assert.Equal(t, "Widget B", result.ToMap(1)["name"])
}

func TestPostgresQueryExecutor_ExecuteQuery_Timeout(t *testing.T) {
	t.Parallel()

	requirePostgres(t)
	env := itf.Setup(t, itf.WithModules(modules.BuiltInModules...))
	executor := NewPostgresQueryExecutor(env.Pool)

	// Get tenant ID from context
	tenantID, err := composables.UseTenantID(env.Ctx)
	require.NoError(t, err)

	_, err = env.Pool.Exec(env.Ctx, `CREATE SCHEMA IF NOT EXISTS analytics`)
	require.NoError(t, err)

	_, err = env.Pool.Exec(env.Ctx, `
		CREATE TABLE test_timeout (
			id SERIAL PRIMARY KEY,
			tenant_id UUID NOT NULL
		)
	`)
	require.NoError(t, err)
	defer func() {
		_, _ = env.Pool.Exec(env.Ctx, "DROP VIEW IF EXISTS analytics.analytics_test_timeout")
		_, _ = env.Pool.Exec(env.Ctx, "DROP TABLE IF EXISTS test_timeout")
	}()

	_, err = env.Pool.Exec(env.Ctx, `
		CREATE OR REPLACE VIEW analytics.analytics_test_timeout AS
		SELECT * FROM test_timeout
		WHERE tenant_id = current_setting('app.tenant_id', true)::UUID
	`)
	require.NoError(t, err)

	// Insert one row
	_, err = env.Pool.Exec(env.Ctx, `INSERT INTO test_timeout (tenant_id) VALUES ($1)`, tenantID)
	require.NoError(t, err)

	// Execute query with very short timeout (1ms) on a slow query
	_, err = executor.ExecuteQuery(env.Ctx, "SELECT pg_sleep(1) FROM analytics.analytics_test_timeout", nil, 1*time.Millisecond)
	assert.Error(t, err)
	// Timeout errors vary by driver, just check that it failed
}

func TestPostgresQueryExecutor_ExecuteQuery_RowLimit(t *testing.T) {
	t.Parallel()

	requirePostgres(t)
	env := itf.Setup(t, itf.WithModules(modules.BuiltInModules...))
	executor := NewPostgresQueryExecutor(env.Pool)

	// Get tenant ID from context
	tenantID, err := composables.UseTenantID(env.Ctx)
	require.NoError(t, err)

	_, err = env.Pool.Exec(env.Ctx, `CREATE SCHEMA IF NOT EXISTS analytics`)
	require.NoError(t, err)

	_, err = env.Pool.Exec(env.Ctx, `
		CREATE TABLE test_large (
			id SERIAL PRIMARY KEY,
			tenant_id UUID NOT NULL,
			value INT
		)
	`)
	require.NoError(t, err)
	defer func() {
		_, _ = env.Pool.Exec(env.Ctx, "DROP VIEW IF EXISTS analytics.analytics_test_large")
		_, _ = env.Pool.Exec(env.Ctx, "DROP TABLE IF EXISTS test_large")
	}()

	_, err = env.Pool.Exec(env.Ctx, `
		CREATE OR REPLACE VIEW analytics.analytics_test_large AS
		SELECT * FROM test_large
		WHERE tenant_id = current_setting('app.tenant_id', true)::UUID
	`)
	require.NoError(t, err)

	// Insert 1500 rows (exceeds 1000 row limit) with tenant_id
	_, err = env.Pool.Exec(env.Ctx, `
		INSERT INTO test_large (tenant_id, value)
		SELECT $1, generate_series(1, 1500)
	`, tenantID)
	require.NoError(t, err)

	// Execute query on analytics view (session variable handles tenant filtering)
	result, err := executor.ExecuteQuery(env.Ctx, "SELECT * FROM analytics.analytics_test_large", nil, 10*time.Second)
	require.NoError(t, err)

	// Should be limited to 1000 rows
	assert.Equal(t, 1000, result.RowCount)
	assert.True(t, result.Truncated)
}

func TestPostgresQueryExecutor_FormatValue(t *testing.T) {
	t.Parallel()

	executor := &PostgresQueryExecutor{}

	tests := []struct {
		name     string
		input    interface{}
		expected interface{}
	}{
		{
			name:     "nil value",
			input:    nil,
			expected: nil,
		},
		{
			name:     "byte array",
			input:    []byte("test"),
			expected: "test",
		},
		{
			name:     "string value",
			input:    "hello",
			expected: "hello",
		},
		{
			name:     "int value",
			input:    int64(42),
			expected: int64(42),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := executor.formatValue(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestPostgresQueryExecutor_IsSystemCatalogQuery(t *testing.T) {
	t.Parallel()

	executor := &PostgresQueryExecutor{}

	tests := []struct {
		name     string
		sql      string
		expected bool
	}{
		{
			name:     "pg_catalog qualified",
			sql:      "SELECT * FROM pg_catalog.pg_class",
			expected: true,
		},
		{
			name:     "information_schema qualified",
			sql:      "SELECT * FROM information_schema.columns",
			expected: true,
		},
		{
			name:     "pg_class without schema",
			sql:      "SELECT * FROM pg_class WHERE relname = 'test'",
			expected: true,
		},
		{
			name:     "pg_namespace in JOIN",
			sql:      "SELECT * FROM pg_class c JOIN pg_namespace n ON c.relnamespace = n.oid",
			expected: true,
		},
		{
			name:     "regular analytics query",
			sql:      "SELECT * FROM analytics.users",
			expected: false,
		},
		{
			name:     "regular table name containing pg",
			sql:      "SELECT * FROM my_pg_table",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := executor.isSystemCatalogQuery(tt.sql)
			assert.Equal(t, tt.expected, result)
		})
	}
}
