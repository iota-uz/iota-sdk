// Package sql provides this package.
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

// maxCacheEntries limits the cache map size; stale entries are evicted when exceeded.
const maxCacheEntries = 256

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

// WithSchemaAllowlist restricts which schemas SchemaList scans. Required
// in production wiring — the default is empty, which makes SchemaList
// return no rows (intentional: forces every consumer to declare what
// the LLM may see).
//
// View row-count enrichment (the secondary count(*) query) only runs when
// the allowlist contains exactly one schema — multi-schema enumeration
// would conflate same-named views across schemas in the cache map.
func WithSchemaAllowlist(schemas []string) SchemaListerOption {
	return func(l *QueryExecutorSchemaLister) {
		l.allowlist = append([]string(nil), schemas...)
	}
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
	allowlist    []string
	mu           sync.Mutex
	cache        map[string]*cachedCounts
}

// NewQueryExecutorSchemaLister creates a schema lister that uses a query executor.
// Pass WithSchemaAllowlist to declare which schemas the LLM may enumerate;
// the default is empty (no schemas visible).
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

// SchemaList executes a query to list all tables and views the current
// Postgres role can SELECT from. The allowlist filters by schema; the
// has_table_privilege check filters by per-relation grant so a restricted
// role (e.g. ai_readonly) sees only what it can actually read.
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
		WHERE n.nspname = ANY($1)
		  AND c.relkind IN ('v', 'r', 'm')
		  AND has_table_privilege(current_user, c.oid, 'SELECT')
		ORDER BY n.nspname, c.relname
	`

	result, err := l.executor.ExecuteQuery(ctx, query, []any{l.allowlist}, 10*time.Second)
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

	// View row-count enrichment only when allowlist is single-schema:
	// multi-schema would require qualifying cache keys by schema, and the
	// secondary SQL builder would have to track schema per view too.
	// EAI's only consumer queries one schema at a time, so this path is
	// preserved unchanged for it.
	if len(viewsNeedingCounts) > 0 && len(l.allowlist) == 1 {
		counts := l.getViewCounts(ctx, l.allowlist[0], viewsNeedingCounts)
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
func (l *QueryExecutorSchemaLister) getViewCounts(ctx context.Context, schema string, views []string) map[string]int64 {
	// Compute cache key once to avoid redundant calls and potential inconsistency.
	var cacheKey string
	var cacheEnabled bool
	if l.cacheKeyFunc != nil {
		key, err := l.cacheKeyFunc(ctx)
		if err == nil {
			cacheKey = key
			cacheEnabled = true
		}
	}

	// Try cache first
	if cacheEnabled {
		l.mu.Lock()
		cached, ok := l.cache[cacheKey]
		// Evict stale entries while holding the lock.
		if len(l.cache) > maxCacheEntries {
			l.evictStaleLocked()
		}
		l.mu.Unlock()

		if ok && time.Since(cached.fetchedAt) < l.cacheTTL {
			return cached.counts
		}
	}

	counts := l.fetchViewCounts(ctx, schema, views)
	if counts == nil {
		return nil
	}

	// Store in cache
	if cacheEnabled {
		l.mu.Lock()
		l.cache[cacheKey] = &cachedCounts{counts: counts, fetchedAt: time.Now()}
		l.mu.Unlock()
	}

	return counts
}

// evictStaleLocked removes entries older than cacheTTL. Must be called with l.mu held.
func (l *QueryExecutorSchemaLister) evictStaleLocked() {
	for k, v := range l.cache {
		if time.Since(v.fetchedAt) >= l.cacheTTL {
			delete(l.cache, k)
		}
	}
}

// fetchViewCounts runs a batch count(*) query for the given views.
func (l *QueryExecutorSchemaLister) fetchViewCounts(ctx context.Context, schema string, views []string) map[string]int64 {
	const limit = 1_000_001

	if !validIdentifier.MatchString(schema) {
		return nil
	}

	parts := make([]string, 0, len(views))
	for _, name := range views {
		if !validIdentifier.MatchString(name) {
			continue
		}
		// Identifiers are double-quoted so mixed-case names like
		// `"SalesByDay"` are preserved (unquoted SQL folds them to
		// lowercase and the view lookup silently 404s). name matched
		// validIdentifier above, so no quote-escaping is needed.
		parts = append(parts, fmt.Sprintf(
			`SELECT '%s'::text AS name, count(*)::bigint AS cnt FROM (SELECT 1 FROM "%s"."%s" LIMIT %d) t`,
			name, schema, name, limit,
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

// containsString reports whether needle is present in haystack. Small
// helper used by SchemaDescribe's allowlist-enforcement check.
func containsString(haystack []string, needle string) bool {
	for _, h := range haystack {
		if h == needle {
			return true
		}
	}
	return false
}

func parseIntColumn(v any) int64 {
	switch n := v.(type) {
	case int64:
		return n
	case int:
		return int64(n)
	case int32:
		return int64(n)
	case int16:
		return int64(n)
	case int8:
		return int64(n)
	case uint64:
		return int64(n)
	case float64:
		return int64(n)
	default:
		return 0
	}
}

// SchemaDescriberOption configures a QueryExecutorSchemaDescriber.
type SchemaDescriberOption func(*QueryExecutorSchemaDescriber)

// WithDescribeSchemaAllowlist restricts the schemas the describer will look
// up tables in. The first match wins. Required in production wiring — the
// default is empty, which makes Describe reject every lookup.
func WithDescribeSchemaAllowlist(schemas []string) SchemaDescriberOption {
	return func(d *QueryExecutorSchemaDescriber) {
		d.allowlist = append([]string(nil), schemas...)
	}
}

// QueryExecutorSchemaDescriber adapts a QueryExecutor to implement SchemaDescriber
// by executing SQL queries to describe table schemas.
type QueryExecutorSchemaDescriber struct {
	executor  QueryExecutor
	allowlist []string
}

// NewQueryExecutorSchemaDescriber creates a schema describer that uses a query executor.
// Pass WithDescribeSchemaAllowlist to declare which schemas the LLM may
// describe; the default is empty (no schemas visible).
func NewQueryExecutorSchemaDescriber(executor QueryExecutor, opts ...SchemaDescriberOption) SchemaDescriber {
	d := &QueryExecutorSchemaDescriber{
		executor: executor,
	}
	for _, opt := range opts {
		opt(d)
	}
	return d
}

// SchemaDescribe executes queries to get detailed schema information.
//
// Accepts either a bare name ("users") or a schema-qualified reference
// ("public.users"). A qualified reference PINS the lookup to that
// schema — but only when the schema is present in the allowlist.
// Qualifying with a schema outside the allowlist returns an error rather
// than silently widening access. Pinning is the disambiguation path when
// two allow-listed schemas hold a same-named table.
//
// When a bare name matches in multiple allow-listed schemas, the earliest
// position in the allowlist wins (via array_position) so results are
// deterministic across planner choices.
//
// Column metadata is read from pg_attribute + pg_type so we can join
// pg_description on (objoid, objsubid) to surface column-level COMMENT ON
// values. Table-level description comes from the same pg_description join
// at objsubid=0.
func (d *QueryExecutorSchemaDescriber) SchemaDescribe(ctx context.Context, tableName string) (*TableSchema, error) {
	schemaFilter := d.allowlist
	bareName := tableName
	if idx := strings.Index(tableName, "."); idx > 0 && idx < len(tableName)-1 {
		pinned := tableName[:idx]
		if !containsString(d.allowlist, pinned) {
			return nil, fmt.Errorf("schema %q is not in the describer allowlist", pinned)
		}
		schemaFilter = []string{pinned}
		bareName = tableName[idx+1:]
	}

	// Table description (one row) — also asserts the table exists in an
	// allowed schema. Done as a separate small query to keep the column
	// query simple. ORDER BY array_position makes the tiebreak for
	// same-named tables in multiple schemas deterministic: earliest
	// allowlist entry wins.
	tableQuery := `
		SELECT
			n.nspname AS schema,
			COALESCE(obj_description(c.oid, 'pg_class'), '') AS description
		FROM pg_class c
		JOIN pg_namespace n ON n.oid = c.relnamespace
		WHERE c.relname = $1
		  AND n.nspname = ANY($2)
		  AND has_table_privilege(current_user, c.oid, 'SELECT')
		ORDER BY array_position($2::text[], n.nspname), n.nspname
		LIMIT 1
	`
	tableRes, err := d.executor.ExecuteQuery(ctx, tableQuery, []any{bareName, schemaFilter}, 10*time.Second)
	if err != nil {
		return nil, fmt.Errorf("failed to look up table: %w", err)
	}
	if len(tableRes.Rows) == 0 {
		return nil, fmt.Errorf("table %q not found in allowed schemas", tableName)
	}
	resolvedSchema, _ := tableRes.Rows[0][0].(string)
	tableDescription, _ := tableRes.Rows[0][1].(string)

	columnsQuery := `
		SELECT
			a.attname                                       AS column_name,
			format_type(a.atttypid, a.atttypmod)            AS data_type,
			NOT a.attnotnull                                AS is_nullable,
			pg_get_expr(ad.adbin, ad.adrelid)               AS column_default,
			COALESCE(d.description, '')                     AS description
		FROM pg_attribute a
		JOIN pg_class c    ON a.attrelid = c.oid
		JOIN pg_namespace n ON n.oid = c.relnamespace
		LEFT JOIN pg_attrdef ad
		       ON ad.adrelid = a.attrelid AND ad.adnum = a.attnum
		LEFT JOIN pg_description d
		       ON d.objoid = c.oid
		      AND d.objsubid = a.attnum
		      AND d.classoid = 'pg_class'::regclass
		WHERE c.relname = $1
		  AND n.nspname = $2
		  AND a.attnum > 0
		  AND NOT a.attisdropped
		ORDER BY a.attnum
	`

	columnsResult, err := d.executor.ExecuteQuery(ctx, columnsQuery, []any{bareName, resolvedSchema}, 10*time.Second)
	if err != nil {
		return nil, fmt.Errorf("failed to describe columns: %w", err)
	}

	columns := make([]ColumnInfo, 0, len(columnsResult.Rows))
	for _, row := range columnsResult.Rows {
		if len(row) < 5 {
			continue
		}

		colName, _ := row[0].(string)
		dataType, _ := row[1].(string)
		nullable, _ := row[2].(bool)
		colDefault := row[3]
		colDescription, _ := row[4].(string)

		var defaultValue *string
		if colDefault != nil {
			if s, ok := colDefault.(string); ok {
				defaultValue = &s
			}
		}

		columns = append(columns, ColumnInfo{
			Name:         colName,
			Type:         dataType,
			Nullable:     nullable,
			DefaultValue: defaultValue,
			Description:  colDescription,
		})
	}

	return &TableSchema{
		Name:        bareName,
		Schema:      resolvedSchema,
		Description: tableDescription,
		Columns:     columns,
	}, nil
}
