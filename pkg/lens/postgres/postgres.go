// Package postgres provides a PostgreSQL implementation of lens.DataSource.
package postgres

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/iota-uz/iota-sdk/pkg/lens"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Config holds connection settings for the PostgreSQL data source.
type Config struct {
	ConnectionString string
	MaxConnections   int32
	MinConnections   int32
	QueryTimeout     time.Duration
}

// DataSource implements lens.DataSource for PostgreSQL using pgxpool.
type DataSource struct {
	pool    *pgxpool.Pool
	timeout time.Duration
}

// New creates a new PostgreSQL data source from config.
func New(cfg Config) (*DataSource, error) {
	if cfg.ConnectionString == "" {
		return nil, fmt.Errorf("lens/postgres: connection string is required")
	}

	poolCfg, err := pgxpool.ParseConfig(cfg.ConnectionString)
	if err != nil {
		return nil, fmt.Errorf("lens/postgres: %w", err)
	}
	if cfg.MaxConnections > 0 {
		poolCfg.MaxConns = cfg.MaxConnections
	}
	if cfg.MinConnections > 0 {
		poolCfg.MinConns = cfg.MinConnections
	}

	pool, err := pgxpool.NewWithConfig(context.Background(), poolCfg)
	if err != nil {
		return nil, fmt.Errorf("lens/postgres: %w", err)
	}

	timeout := cfg.QueryTimeout
	if timeout == 0 {
		timeout = 30 * time.Second
	}

	return &DataSource{pool: pool, timeout: timeout}, nil
}

// NewFromPool wraps an existing pgxpool.Pool as a lens.DataSource.
func NewFromPool(pool *pgxpool.Pool) *DataSource {
	return &DataSource{pool: pool, timeout: 30 * time.Second}
}

// Execute runs the given SQL query and returns the result as a QueryResult.
// Only SELECT statements are allowed.
func (ds *DataSource) Execute(ctx context.Context, query string) (*lens.QueryResult, error) {
	if err := validateQuery(query); err != nil {
		return nil, err
	}

	qCtx, cancel := context.WithTimeout(ctx, ds.timeout)
	defer cancel()

	rows, err := ds.pool.Query(qCtx, query)
	if err != nil {
		return nil, fmt.Errorf("lens/postgres: query failed: %w", err)
	}
	defer rows.Close()

	// Build column metadata.
	descs := rows.FieldDescriptions()
	columns := make([]lens.QueryColumn, len(descs))
	for i, d := range descs {
		columns[i] = lens.QueryColumn{
			Name: d.Name,
			Type: pgTypeToString(d.DataTypeOID),
		}
	}

	// Read rows into maps.
	var result []map[string]any
	for rows.Next() {
		values, err := rows.Values()
		if err != nil {
			return nil, fmt.Errorf("lens/postgres: row scan: %w", err)
		}

		row := make(map[string]any, len(columns))
		for i, v := range values {
			row[columns[i].Name] = v
		}
		result = append(result, row)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("lens/postgres: %w", err)
	}

	return &lens.QueryResult{Columns: columns, Rows: result}, nil
}

// Close releases the connection pool.
func (ds *DataSource) Close() error {
	if ds.pool != nil {
		ds.pool.Close()
	}
	return nil
}

// validateQuery ensures only SELECT statements are executed.
func validateQuery(query string) error {
	trimmed := strings.TrimSpace(strings.ToLower(query))
	if !strings.HasPrefix(trimmed, "select") && !strings.HasPrefix(trimmed, "with") {
		return fmt.Errorf("lens/postgres: only SELECT/WITH queries are allowed")
	}
	for _, kw := range []string{"drop ", "delete ", "insert ", "update ", "create ", "alter ", "truncate "} {
		if strings.Contains(trimmed, kw) {
			return fmt.Errorf("lens/postgres: query contains disallowed keyword: %s", strings.TrimSpace(kw))
		}
	}
	return nil
}

// pgTypeToString converts PostgreSQL type OIDs to friendly type names.
func pgTypeToString(oid uint32) string {
	switch oid {
	case 25, 1043, 1042: // text, varchar, bpchar
		return "string"
	case 21, 23, 20, 700, 701, 1700: // int2, int4, int8, float4, float8, numeric
		return "number"
	case 16: // bool
		return "boolean"
	case 1114, 1184, 1082, 1083: // timestamp, timestamptz, date, time
		return "timestamp"
	default:
		return "string"
	}
}
