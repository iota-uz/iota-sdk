package sql

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"sync"
	"time"
)

// validIdentifier validates SQL identifiers for safe dynamic query construction.
var validIdentifier = regexp.MustCompile(`^[a-zA-Z_][a-zA-Z0-9_]*$`)

// CacheKeyFunc extracts a cache key (typically tenant ID) from context.
type CacheKeyFunc func(context.Context) (string, error)

// SchemaListerOption configures a QueryExecutorSchemaLister.
type SchemaListerOption func(*QueryExecutorSchemaLister)

// WithCountCacheTTL sets the TTL for cached view row counts.
func WithCountCacheTTL(ttl time.Duration) SchemaListerOption {
	return func(l *QueryExecutorSchemaLister) { l.cacheTTL = ttl }
}

// WithCacheKeyFunc sets the function used to derive a per-tenant cache key.
// When nil, view counts are fetched on every call without caching.
func WithCacheKeyFunc(fn CacheKeyFunc) SchemaListerOption {
	return func(l *QueryExecutorSchemaLister) { l.cacheKeyFunc = fn }
}

type cachedCounts struct {
	counts    map[string]int64
	fetchedAt time.Time
}

// QueryExecutorSchemaLister adapts a QueryExecutor to implement SchemaLister
// by executing SQL queries to list tables.
type QueryExecutorSchemaLister struct {
	executor     QueryExecutor
	cacheTTL     time.Duration
	cacheKeyFunc CacheKeyFunc
	mu           sync.Mutex
	cache        map[string]*cachedCounts
}

// NewQueryExecutorSchemaLister creates a schema lister that uses a query executor.
func NewQueryExecutorSchemaLister(executor QueryExecutor, opts ...SchemaListerOption) SchemaLister {
	l := &QueryExecutorSchemaLister{
		executor: executor,
		cacheTTL: 10 * time.Minute,
		cache:    make(map[string]*cachedCounts),
	}
	for _, opt := range opts {
		opt(l)
	}
	return l
}

// SchemaList executes a query to list all tables and views.
func (l *QueryExecutorSchemaLister) SchemaList(ctx context.Context) ([]TableInfo, error) {
	query := `
		SELECT
			n.nspname AS schema,
			c.relname AS name,
			GREATEST(c.reltuples, 0)::bigint AS approximate_row_count,
			COALESCE(obj_description(c.oid, 'pg_class'), '') AS description,
			c.relkind::text
		FROM pg_class c
		JOIN pg_namespace n ON n.oid = c.relnamespace
		WHERE n.nspname = 'analytics'
		  AND c.relkind IN ('v', 'r', 'm')
		ORDER BY c.relname
	`

	result, err := l.executor.ExecuteQuery(ctx, query, nil, 10*time.Second)
	if err != nil {
		return nil, fmt.Errorf("failed to list schema: %w", err)
	}

	tables := make([]TableInfo, 0, len(result.Rows))
	var viewsNeedingCounts []string

	for _, row := range result.Rows {
		if len(row) < 5 {
			continue
		}

		schema, _ := row[0].(string)
		name, _ := row[1].(string)
		rowCount := parseIntColumn(row[2])
		description, _ := row[3].(string)
		relkind, _ := row[4].(string)

		tables = append(tables, TableInfo{
			Schema:      schema,
			Name:        name,
			RowCount:    rowCount,
			Description: description,
		})

		if relkind == "v" && rowCount == 0 {
			viewsNeedingCounts = append(viewsNeedingCounts, name)
		}
	}

	if len(viewsNeedingCounts) > 0 {
		counts := l.getViewCounts(ctx, viewsNeedingCounts)
		if counts != nil {
			for i := range tables {
				if c, ok := counts[tables[i].Name]; ok && c > 0 {
					tables[i].RowCount = c
				}
			}
		}
	}

	return tables, nil
}

// getViewCounts returns estimated row counts for views, using cache when available.
func (l *QueryExecutorSchemaLister) getViewCounts(ctx context.Context, views []string) map[string]int64 {
	// Try cache first
	if l.cacheKeyFunc != nil {
		key, err := l.cacheKeyFunc(ctx)
		if err == nil {
			l.mu.Lock()
			cached, ok := l.cache[key]
			l.mu.Unlock()

			if ok && time.Since(cached.fetchedAt) < l.cacheTTL {
				return cached.counts
			}
		}
	}

	counts := l.fetchViewCounts(ctx, views)
	if counts == nil {
		return nil
	}

	// Store in cache
	if l.cacheKeyFunc != nil {
		key, err := l.cacheKeyFunc(ctx)
		if err == nil {
			l.mu.Lock()
			l.cache[key] = &cachedCounts{counts: counts, fetchedAt: time.Now()}
			l.mu.Unlock()
		}
	}

	return counts
}

// fetchViewCounts runs a batch count(*) query for the given views.
func (l *QueryExecutorSchemaLister) fetchViewCounts(ctx context.Context, views []string) map[string]int64 {
	const limit = 1_000_001

	parts := make([]string, 0, len(views))
	for _, name := range views {
		if !validIdentifier.MatchString(name) {
			continue
		}
		parts = append(parts, fmt.Sprintf(
			"SELECT '%s'::text AS name, count(*)::bigint AS cnt FROM (SELECT 1 FROM analytics.%s LIMIT %d) t",
			name, name, limit,
		))
	}
	if len(parts) == 0 {
		return nil
	}

	batchSQL := strings.Join(parts, " UNION ALL ")
	result, err := l.executor.ExecuteQuery(ctx, batchSQL, nil, 30*time.Second)
	if err != nil {
		return nil // graceful degradation
	}

	counts := make(map[string]int64, len(result.Rows))
	for _, row := range result.Rows {
		if len(row) < 2 {
			continue
		}
		name, _ := row[0].(string)
		cnt := parseIntColumn(row[1])
		counts[name] = cnt
	}
	return counts
}

func parseIntColumn(v any) int64 {
	switch n := v.(type) {
	case int64:
		return n
	case int:
		return int64(n)
	case int32:
		return int64(n)
	case float64:
		return int64(n)
	default:
		return 0
	}
}

// QueryExecutorSchemaDescriber adapts a QueryExecutor to implement SchemaDescriber
// by executing SQL queries to describe table schemas.
type QueryExecutorSchemaDescriber struct {
	executor QueryExecutor
}

// NewQueryExecutorSchemaDescriber creates a schema describer that uses a query executor.
func NewQueryExecutorSchemaDescriber(executor QueryExecutor) SchemaDescriber {
	return &QueryExecutorSchemaDescriber{executor: executor}
}

// SchemaDescribe executes queries to get detailed schema information.
func (d *QueryExecutorSchemaDescriber) SchemaDescribe(ctx context.Context, tableName string) (*TableSchema, error) {
	// Query column information
	columnsQuery := `
		SELECT
			column_name,
			data_type,
			is_nullable,
			column_default,
			character_maximum_length,
			numeric_precision,
			numeric_scale
		FROM information_schema.columns
		WHERE table_schema = 'analytics' AND table_name = $1
		ORDER BY ordinal_position
	`

	columnsResult, err := d.executor.ExecuteQuery(ctx, columnsQuery, []any{tableName}, 10*time.Second)
	if err != nil {
		return nil, fmt.Errorf("failed to describe columns: %w", err)
	}

	columns := make([]ColumnInfo, 0, len(columnsResult.Rows))
	for _, row := range columnsResult.Rows {
		if len(row) < 7 {
			continue
		}

		colName, _ := row[0].(string)
		dataType, _ := row[1].(string)
		isNullable, _ := row[2].(string)
		colDefault := row[3]
		// Skip length/precision/scale for now

		var defaultValue *string
		if colDefault != nil {
			if s, ok := colDefault.(string); ok {
				defaultValue = &s
			}
		}

		columns = append(columns, ColumnInfo{
			Name:         colName,
			Type:         dataType,
			Nullable:     isNullable == "YES",
			DefaultValue: defaultValue,
		})
	}

	return &TableSchema{
		Name:    tableName,
		Schema:  "analytics",
		Columns: columns,
	}, nil
}
