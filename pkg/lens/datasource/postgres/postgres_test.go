package postgres

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/iota-uz/iota-sdk/pkg/lens"
	"github.com/iota-uz/iota-sdk/pkg/lens/datasource"
)

func TestNewPostgreSQLDataSource(t *testing.T) {
	tests := []struct {
		name        string
		config      Config
		expectError bool
	}{
		{
			name: "valid configuration",
			config: Config{
				ConnectionString: "postgres://user:pass@localhost:5432/testdb",
				MaxConnections:   10,
				MinConnections:   2,
				QueryTimeout:     30 * time.Second,
			},
			expectError: false,
		},
		{
			name: "empty connection string",
			config: Config{
				MaxConnections: 10,
				QueryTimeout:   30 * time.Second,
			},
			expectError: true,
		},
		{
			name: "invalid connection string",
			config: Config{
				ConnectionString: "invalid-connection-string",
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ds, err := NewPostgreSQLDataSource(tt.config)

			if tt.expectError {
				require.Error(t, err)
				assert.Nil(t, ds)
			} else {
				// Note: This will fail in CI without a real PostgreSQL instance
				// In a real test environment, you'd want to use testcontainers
				// or a test database
				if err != nil {
					t.Skipf("Skipping test due to database connection: %v", err)
				}
				require.NoError(t, err)
				assert.NotNil(t, ds)
				assert.Equal(t, datasource.TypePostgreSQL, ds.GetMetadata().Type)

				// Clean up
				if ds != nil {
					if err := ds.Close(); err != nil {
						t.Logf("Failed to close data source: %v", err)
					}
				}
			}
		})
	}
}

func TestPostgreSQLDataSource_GetMetadata(t *testing.T) {
	config := Config{
		ConnectionString: "postgres://user:pass@localhost:5432/testdb",
		QueryTimeout:     30 * time.Second,
	}

	ds, err := NewPostgreSQLDataSource(config)
	if err != nil {
		t.Skipf("Skipping test due to database connection: %v", err)
	}
	defer func() {
		if err := ds.Close(); err != nil {
			t.Logf("Failed to close data source: %v", err)
		}
	}()

	metadata := ds.GetMetadata()
	assert.Equal(t, datasource.TypePostgreSQL, metadata.Type)
	assert.Equal(t, "PostgreSQL", metadata.Name)
	assert.Contains(t, metadata.Capabilities, datasource.CapabilityQuery)
	assert.Contains(t, metadata.Capabilities, datasource.CapabilityMetrics)
}

func TestPostgreSQLDataSource_ValidateQuery(t *testing.T) {
	config := Config{
		ConnectionString: "postgres://user:pass@localhost:5432/testdb",
		QueryTimeout:     30 * time.Second,
	}

	ds, err := NewPostgreSQLDataSource(config)
	if err != nil {
		t.Skipf("Skipping test due to database connection: %v", err)
	}
	defer func() {
		if err := ds.Close(); err != nil {
			t.Logf("Failed to close data source: %v", err)
		}
	}()

	tests := []struct {
		name        string
		query       datasource.Query
		expectError bool
	}{
		{
			name: "valid SELECT query",
			query: datasource.Query{
				Raw: "SELECT * FROM users WHERE id = 1",
			},
			expectError: false,
		},
		{
			name: "empty query",
			query: datasource.Query{
				Raw: "",
			},
			expectError: true,
		},
		{
			name: "dangerous DROP query",
			query: datasource.Query{
				Raw: "DROP TABLE users",
			},
			expectError: true,
		},
		{
			name: "dangerous DELETE query",
			query: datasource.Query{
				Raw: "DELETE FROM users",
			},
			expectError: true,
		},
		{
			name: "dangerous INSERT query",
			query: datasource.Query{
				Raw: "INSERT INTO users (name) VALUES ('test')",
			},
			expectError: true,
		},
		{
			name: "case insensitive SELECT",
			query: datasource.Query{
				Raw: "select * from users",
			},
			expectError: false,
		},
		{
			name: "query with whitespace",
			query: datasource.Query{
				Raw: "  SELECT count(*) FROM orders  ",
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ds.ValidateQuery(tt.query)

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestPostgreSQLDataSource_InterpolateVariables(t *testing.T) {
	config := Config{
		ConnectionString: "postgres://user:pass@localhost:5432/testdb",
		QueryTimeout:     30 * time.Second,
	}

	ds, err := NewPostgreSQLDataSource(config)
	if err != nil {
		t.Skipf("Skipping test due to database connection: %v", err)
	}
	defer func() {
		if err := ds.Close(); err != nil {
			t.Logf("Failed to close data source: %v", err)
		}
	}()

	tests := []struct {
		name      string
		query     string
		variables map[string]interface{}
		expected  string
	}{
		{
			name:  "string variable",
			query: "SELECT * FROM users WHERE name = $username",
			variables: map[string]interface{}{
				"username": "john",
			},
			expected: "SELECT * FROM users WHERE name = 'john'",
		},
		{
			name:  "integer variable",
			query: "SELECT * FROM users WHERE id = $id",
			variables: map[string]interface{}{
				"id": 42,
			},
			expected: "SELECT * FROM users WHERE id = 42",
		},
		{
			name:  "boolean variable",
			query: "SELECT * FROM users WHERE active = $active",
			variables: map[string]interface{}{
				"active": true,
			},
			expected: "SELECT * FROM users WHERE active = true",
		},
		{
			name:  "time variable",
			query: "SELECT * FROM events WHERE created_at > $start_time",
			variables: map[string]interface{}{
				"start_time": time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC),
			},
			expected: "SELECT * FROM events WHERE created_at > '2023-01-01T00:00:00Z'",
		},
		{
			name:  "time range variable",
			query: "SELECT * FROM events WHERE created_at > $timeRange",
			variables: map[string]interface{}{
				"timeRange": lens.TimeRange{
					Start: time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC),
					End:   time.Date(2023, 1, 2, 0, 0, 0, 0, time.UTC),
				},
			},
			expected: "SELECT * FROM events WHERE created_at > '2023-01-01T00:00:00Z'",
		},
		{
			name:  "multiple variables",
			query: "SELECT * FROM users WHERE name = $name AND age > $age",
			variables: map[string]interface{}{
				"name": "alice",
				"age":  25,
			},
			expected: "SELECT * FROM users WHERE name = 'alice' AND age > 25",
		},
		{
			name:  "string with quotes",
			query: "SELECT * FROM users WHERE name = $name",
			variables: map[string]interface{}{
				"name": "o'connor",
			},
			expected: "SELECT * FROM users WHERE name = 'o''connor'",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ds.interpolateVariables(tt.query, tt.variables)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestPostgreSQLDataSource_PgTypeToDataType(t *testing.T) {
	config := Config{
		ConnectionString: "postgres://user:pass@localhost:5432/testdb",
		QueryTimeout:     30 * time.Second,
	}

	ds, err := NewPostgreSQLDataSource(config)
	if err != nil {
		t.Skipf("Skipping test due to database connection: %v", err)
	}
	defer func() {
		if err := ds.Close(); err != nil {
			t.Logf("Failed to close data source: %v", err)
		}
	}()

	tests := []struct {
		name     string
		pgType   uint32
		expected datasource.DataType
	}{
		{"text", 25, datasource.DataTypeString},           // TextOID
		{"varchar", 1043, datasource.DataTypeString},      // VarcharOID
		{"int4", 23, datasource.DataTypeNumber},           // Int4OID
		{"int8", 20, datasource.DataTypeNumber},           // Int8OID
		{"float8", 701, datasource.DataTypeNumber},        // Float8OID
		{"bool", 16, datasource.DataTypeBoolean},          // BoolOID
		{"timestamp", 1114, datasource.DataTypeTimestamp}, // TimestampOID
		{"unknown", 9999, datasource.DataTypeString},      // Default case
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ds.pgTypeToDataType(tt.pgType)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestFactory_Create(t *testing.T) {
	factory := NewFactory()

	tests := []struct {
		name        string
		config      datasource.DataSourceConfig
		expectError bool
	}{
		{
			name: "valid PostgreSQL config",
			config: datasource.DataSourceConfig{
				Type:    datasource.TypePostgreSQL,
				Name:    "Test DB",
				URL:     "postgres://user:pass@localhost:5432/testdb",
				Timeout: 30 * time.Second,
			},
			expectError: false,
		},
		{
			name: "invalid type",
			config: datasource.DataSourceConfig{
				Type: datasource.TypeMongoDB,
				Name: "Test DB",
				URL:  "postgres://user:pass@localhost:5432/testdb",
			},
			expectError: true,
		},
		{
			name: "empty URL",
			config: datasource.DataSourceConfig{
				Type: datasource.TypePostgreSQL,
				Name: "Test DB",
				URL:  "",
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ds, err := factory.Create(tt.config)

			if tt.expectError {
				require.Error(t, err)
				assert.Nil(t, ds)
			} else {
				// Note: This will fail without a real database
				if err != nil {
					t.Skipf("Skipping test due to database connection: %v", err)
				}
				require.NoError(t, err)
				assert.NotNil(t, ds)

				// Clean up
				if ds != nil {
					if err := ds.Close(); err != nil {
						t.Logf("Failed to close data source: %v", err)
					}
				}
			}
		})
	}
}

func TestFactory_SupportedTypes(t *testing.T) {
	factory := NewFactory()
	types := factory.SupportedTypes()

	assert.Len(t, types, 1)
	assert.Contains(t, types, datasource.TypePostgreSQL)
}

func TestFactory_ValidateConfig(t *testing.T) {
	factory := NewFactory()

	tests := []struct {
		name        string
		config      datasource.DataSourceConfig
		expectError bool
	}{
		{
			name: "valid config",
			config: datasource.DataSourceConfig{
				Type: datasource.TypePostgreSQL,
				URL:  "postgres://user:pass@localhost:5432/testdb",
			},
			expectError: false,
		},
		{
			name: "valid config with postgresql scheme",
			config: datasource.DataSourceConfig{
				Type: datasource.TypePostgreSQL,
				URL:  "postgresql://user:pass@localhost:5432/testdb",
			},
			expectError: false,
		},
		{
			name: "invalid type",
			config: datasource.DataSourceConfig{
				Type: datasource.TypeMongoDB,
				URL:  "postgres://user:pass@localhost:5432/testdb",
			},
			expectError: true,
		},
		{
			name: "empty URL",
			config: datasource.DataSourceConfig{
				Type: datasource.TypePostgreSQL,
				URL:  "",
			},
			expectError: true,
		},
		{
			name: "invalid connection string format",
			config: datasource.DataSourceConfig{
				Type: datasource.TypePostgreSQL,
				URL:  "mysql://user:pass@localhost:3306/testdb",
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := factory.ValidateConfig(tt.config)

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
