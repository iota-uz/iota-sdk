package infrastructure

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/itf"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPostgresQueryExecutor_ValidateSQL(t *testing.T) {
	t.Parallel()

	// Note: We don't need actual DB connection for validation tests
	executor := &PostgresQueryExecutor{pool: nil}

	tests := []struct {
		name    string
		sql     string
		wantErr bool
	}{
		{
			name:    "valid SELECT",
			sql:     "SELECT * FROM users WHERE tenant_id = $1",
			wantErr: false,
		},
		{
			name:    "valid SELECT with JOIN",
			sql:     "SELECT u.name FROM users u JOIN orders o ON u.id = o.user_id WHERE u.tenant_id = $1",
			wantErr: false,
		},
		{
			name:    "valid CTE",
			sql:     "WITH filtered AS (SELECT * FROM users WHERE tenant_id = $1) SELECT * FROM filtered",
			wantErr: false,
		},
		{
			name:    "reject INSERT",
			sql:     "INSERT INTO users (name) VALUES ('test')",
			wantErr: true,
		},
		{
			name:    "reject UPDATE",
			sql:     "UPDATE users SET name = 'test' WHERE id = 1",
			wantErr: true,
		},
		{
			name:    "reject DELETE",
			sql:     "DELETE FROM users WHERE id = 1",
			wantErr: true,
		},
		{
			name:    "reject DROP",
			sql:     "DROP TABLE users",
			wantErr: true,
		},
		{
			name:    "reject CREATE",
			sql:     "CREATE TABLE test (id INT)",
			wantErr: true,
		},
		{
			name:    "reject ALTER",
			sql:     "ALTER TABLE users ADD COLUMN test VARCHAR",
			wantErr: true,
		},
		{
			name:    "reject TRUNCATE",
			sql:     "TRUNCATE TABLE users",
			wantErr: true,
		},
		{
			name:    "reject GRANT",
			sql:     "GRANT ALL ON users TO admin",
			wantErr: true,
		},
		{
			name:    "reject DROP after semicolon",
			sql:     "SELECT * FROM users; DROP TABLE sessions",
			wantErr: true,
		},
		{
			name:    "reject DROP between semicolons",
			sql:     "SELECT *; DROP; SELECT * FROM users",
			wantErr: true,
		},
		{
			name:    "reject TRUNCATE after semicolon",
			sql:     "SELECT id FROM users; TRUNCATE sessions",
			wantErr: true,
		},
		{
			name:    "reject administrative commands - VACUUM",
			sql:     "VACUUM users",
			wantErr: true,
		},
		{
			name:    "reject administrative commands - SET",
			sql:     "SET search_path = public",
			wantErr: true,
		},
		{
			name:    "reject transaction control - COMMIT",
			sql:     "COMMIT",
			wantErr: true,
		},
		{
			name:    "reject session control - LOCK",
			sql:     "LOCK TABLE users",
			wantErr: true,
		},
		{
			name:    "reject pub/sub - LISTEN",
			sql:     "LISTEN events",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := executor.validateSQL(tt.sql)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestPostgresQueryExecutor_ExecuteQuery_MissingTenantID(t *testing.T) {
	t.Parallel()

	env := itf.Setup(t)
	executor := NewPostgresQueryExecutor(env.Pool)

	// Context without tenant ID
	ctx := context.Background()

	_, err := executor.ExecuteQuery(ctx, "SELECT 1", nil, 5000)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "tenant ID required")
}

func TestPostgresQueryExecutor_ExecuteQuery_TenantIsolationEnforced(t *testing.T) {
	t.Parallel()

	env := itf.Setup(t)
	executor := NewPostgresQueryExecutor(env.Pool)

	// Create test table with tenant_id
	_, err := env.Pool.Exec(env.Ctx, `
		CREATE TEMP TABLE test_tenant_data (
			id SERIAL PRIMARY KEY,
			tenant_id UUID NOT NULL,
			name VARCHAR(100),
			value INT
		)
	`)
	require.NoError(t, err)

	// Get tenant ID from context
	tenantID, err := composables.UseTenantID(env.Ctx)
	require.NoError(t, err)

	// Insert data for current tenant
	_, err = env.Pool.Exec(env.Ctx, `
		INSERT INTO test_tenant_data (tenant_id, name, value) VALUES
			($1, 'Alice', 100),
			($1, 'Bob', 200)
	`, tenantID)
	require.NoError(t, err)

	// Insert data for different tenant (should NOT be accessible)
	otherTenantID := uuid.New()
	_, err = env.Pool.Exec(env.Ctx, `
		INSERT INTO test_tenant_data (tenant_id, name, value) VALUES
			($1, 'Charlie', 300),
			($1, 'David', 400)
	`, otherTenantID)
	require.NoError(t, err)

	// Test 1: Execute query WITH explicit tenant filter (correct usage)
	result, err := executor.ExecuteQuery(env.Ctx, "SELECT name, value FROM test_tenant_data WHERE tenant_id = $1 ORDER BY value", nil, 5000)
	require.NoError(t, err)

	// Verify ONLY current tenant's data is returned
	assert.Equal(t, 2, result.RowCount, "should only see current tenant's 2 rows")
	assert.Len(t, result.Rows, 2)
	assert.Equal(t, "Alice", result.Rows[0]["name"])
	assert.Equal(t, "Bob", result.Rows[1]["name"])

	// Verify other tenant's data is NOT visible
	for _, row := range result.Rows {
		assert.NotEqual(t, "Charlie", row["name"])
		assert.NotEqual(t, "David", row["name"])
	}

	// Test 2: Execute query WITHOUT tenant filter (should be rejected)
	_, err = executor.ExecuteQuery(env.Ctx, "SELECT name, value FROM test_tenant_data ORDER BY value", nil, 5000)
	assert.Error(t, err, "query without tenant_id filter should be rejected")
	assert.Contains(t, err.Error(), "must include WHERE tenant_id")
}

func TestPostgresQueryExecutor_ExecuteQuery_Success(t *testing.T) {
	t.Parallel()

	env := itf.Setup(t)
	executor := NewPostgresQueryExecutor(env.Pool)

	// Create test table
	_, err := env.Pool.Exec(env.Ctx, `
		CREATE TEMP TABLE test_data (
			id SERIAL PRIMARY KEY,
			name VARCHAR(100),
			value INT
		)
	`)
	require.NoError(t, err)

	// Insert test data
	_, err = env.Pool.Exec(env.Ctx, `
		INSERT INTO test_data (name, value) VALUES
			('Alice', 100),
			('Bob', 200),
			('Charlie', 300)
	`)
	require.NoError(t, err)

	// Execute query
	result, err := executor.ExecuteQuery(env.Ctx, "SELECT name, value FROM test_data ORDER BY value", nil, 5000)
	require.NoError(t, err)

	assert.Len(t, result.Columns, 2)
	assert.Contains(t, result.Columns, "name")
	assert.Contains(t, result.Columns, "value")
	assert.Equal(t, 3, result.RowCount)
	assert.Len(t, result.Rows, 3)
	assert.Equal(t, "Alice", result.Rows[0]["name"])
	assert.Greater(t, result.DurationMs, int64(0))
}

func TestPostgresQueryExecutor_ExecuteQuery_WithParameters(t *testing.T) {
	t.Parallel()

	env := itf.Setup(t)
	executor := NewPostgresQueryExecutor(env.Pool)

	// Create test table
	_, err := env.Pool.Exec(env.Ctx, `
		CREATE TEMP TABLE test_products (
			id SERIAL PRIMARY KEY,
			name VARCHAR(100),
			price DECIMAL(10,2)
		)
	`)
	require.NoError(t, err)

	// Insert test data
	_, err = env.Pool.Exec(env.Ctx, `
		INSERT INTO test_products (name, price) VALUES
			('Widget A', 10.50),
			('Widget B', 25.00),
			('Widget C', 5.99)
	`)
	require.NoError(t, err)

	// Execute query with parameters
	result, err := executor.ExecuteQuery(
		env.Ctx,
		"SELECT name, price FROM test_products WHERE price > $1 ORDER BY price",
		[]any{10.0},
		5000,
	)
	require.NoError(t, err)

	assert.Equal(t, 2, result.RowCount)
	assert.Equal(t, "Widget A", result.Rows[0]["name"])
	assert.Equal(t, "Widget B", result.Rows[1]["name"])
}

func TestPostgresQueryExecutor_ExecuteQuery_Timeout(t *testing.T) {
	t.Parallel()

	env := itf.Setup(t)
	executor := NewPostgresQueryExecutor(env.Pool)

	// Execute query with very short timeout (1ms) on a slow query
	ctx := composables.WithTenantID(context.Background(), uuid.New())
	_, err := executor.ExecuteQuery(ctx, "SELECT pg_sleep(1)", nil, 1)
	assert.Error(t, err)
	// Timeout errors vary by driver, just check that it failed
}

func TestPostgresQueryExecutor_ExecuteQuery_RowLimit(t *testing.T) {
	t.Parallel()

	env := itf.Setup(t)
	executor := NewPostgresQueryExecutor(env.Pool)

	// Create test table with many rows
	_, err := env.Pool.Exec(env.Ctx, `
		CREATE TEMP TABLE test_large (
			id SERIAL PRIMARY KEY,
			value INT
		)
	`)
	require.NoError(t, err)

	// Insert 1500 rows (exceeds 1000 row limit)
	_, err = env.Pool.Exec(env.Ctx, `
		INSERT INTO test_large (value)
		SELECT generate_series(1, 1500)
	`)
	require.NoError(t, err)

	// Execute query
	result, err := executor.ExecuteQuery(env.Ctx, "SELECT * FROM test_large", nil, 10000)
	require.NoError(t, err)

	// Should be limited to 1000 rows
	assert.Equal(t, 1000, result.RowCount)
	assert.True(t, result.IsLimited)
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

func TestPostgresQueryExecutor_ContainsTenantFilter(t *testing.T) {
	t.Parallel()

	executor := &PostgresQueryExecutor{}

	tests := []struct {
		name     string
		sql      string
		expected bool
	}{
		{
			name:     "contains tenant_id",
			sql:      "SELECT * FROM users WHERE tenant_id = $1",
			expected: true,
		},
		{
			name:     "contains TENANT_ID uppercase",
			sql:      "SELECT * FROM users WHERE TENANT_ID = $1",
			expected: true,
		},
		{
			name:     "no tenant filter",
			sql:      "SELECT * FROM users",
			expected: false,
		},
		{
			name:     "tenant_id in JOIN",
			sql:      "SELECT * FROM users u JOIN orders o ON u.id = o.user_id WHERE u.tenant_id = $1",
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := executor.containsTenantFilter(tt.sql)
			assert.Equal(t, tt.expected, result)
		})
	}
}
