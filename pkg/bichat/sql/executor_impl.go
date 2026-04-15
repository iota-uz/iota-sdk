package sql

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Defaults applied when an option is omitted or set to a non-positive value.
const (
	DefaultMaxQueryLength = 50_000
	DefaultQueryTimeout   = 30 * time.Second
	DefaultMaxResultRows  = 50_000
)

// Sentinel errors. Callers can errors.Is against them to map to HTTP / RPC
// status codes without parsing strings.
var (
	ErrEmptyQuery       = errors.New("query is required")
	ErrQueryTooLong     = errors.New("query exceeds maximum length")
	ErrWriteOperation   = errors.New("write operations are not allowed")
	ErrDangerousPattern = errors.New("query contains disallowed patterns")
	ErrNotReadOnly      = errors.New("query must start with SELECT, WITH, or VALUES")
)

// commentLineRE strips `-- foo` comments before validation. Multiline `/* */`
// comments are stripped by blockCommentRE. Both run before uppercase
// normalization so the keyword-prefix Contains scan is reliable.
var (
	commentLineRE  = regexp.MustCompile(`--[^\n]*`)
	blockCommentRE = regexp.MustCompile(`/\*[\s\S]*?\*/`)
)

// SafeQueryExecutor is the canonical pgx-backed QueryExecutor. It enforces
// read-only semantics, applies result/length caps, scopes execution to the
// caller's tenant via SetTenantContext, and lets consumers attach a
// pluggable QueryPolicy for domain-specific authorization.
//
// Construction: NewSafeQueryExecutor(pool, opts...). Defaults match the
// previous EAI QueryExecutorService values so the lift is behavior-preserving.
type SafeQueryExecutor struct {
	pool *pgxpool.Pool

	maxQueryLength int
	queryTimeout   time.Duration
	maxResultRows  int
	policy         QueryPolicy
	tenantResolver TenantResolver
	statementCapMS int64 // SET LOCAL statement_timeout. 0 disables.
}

// ExecutorOption configures a SafeQueryExecutor at construction.
type ExecutorOption func(*SafeQueryExecutor)

// WithMaxQueryLength caps the raw SQL byte length before any execution.
// Non-positive values are ignored.
func WithMaxQueryLength(n int) ExecutorOption {
	return func(e *SafeQueryExecutor) {
		if n > 0 {
			e.maxQueryLength = n
		}
	}
}

// WithQueryTimeout sets the default per-query timeout. Callers may pass a
// non-zero timeout to ExecuteQuery to override on a per-call basis.
func WithQueryTimeout(d time.Duration) ExecutorOption {
	return func(e *SafeQueryExecutor) {
		if d > 0 {
			e.queryTimeout = d
		}
	}
}

// WithMaxResultRows caps the number of rows returned. Excess rows are
// dropped and QueryResult.Truncated is set to true.
func WithMaxResultRows(n int) ExecutorOption {
	return func(e *SafeQueryExecutor) {
		if n > 0 {
			e.maxResultRows = n
		}
	}
}

// WithQueryPolicy installs a permission/authorization hook. Default is
// AllowAllPolicy.
func WithQueryPolicy(p QueryPolicy) ExecutorOption {
	return func(e *SafeQueryExecutor) {
		if p != nil {
			e.policy = p
		}
	}
}

// WithTenantResolver supplies the tenant-id resolver used to bind
// app.tenant_id inside each transaction. Default is NoTenantResolver
// (skips set_config). Wire composables.UseTenantID to inherit the SDK's
// context-based tenant routing.
func WithTenantResolver(r TenantResolver) ExecutorOption {
	return func(e *SafeQueryExecutor) {
		if r != nil {
			e.tenantResolver = r
		}
	}
}

// WithStatementTimeoutCap mirrors the per-query timeout into a Postgres
// SET LOCAL statement_timeout so a runaway query is killed by the server
// even if the client context is leaked. Default is to mirror queryTimeout.
// Set 0 to disable the SET LOCAL entirely.
func WithStatementTimeoutCap(d time.Duration) ExecutorOption {
	return func(e *SafeQueryExecutor) {
		if d < 0 {
			return
		}
		e.statementCapMS = d.Milliseconds()
	}
}

// NewSafeQueryExecutor constructs an executor bound to pool. Pool is
// required in production; passing nil does not fail at construction but
// the first ExecuteQuery / ExplainQuery call returns a "pool is nil"
// error. The deferred failure lets test contexts construct an executor
// before wiring the pool — production callers should still pass a
// live pool.
func NewSafeQueryExecutor(pool *pgxpool.Pool, opts ...ExecutorOption) *SafeQueryExecutor {
	e := &SafeQueryExecutor{
		pool:           pool,
		maxQueryLength: DefaultMaxQueryLength,
		queryTimeout:   DefaultQueryTimeout,
		maxResultRows:  DefaultMaxResultRows,
		policy:         AllowAllPolicy{},
		tenantResolver: NoTenantResolver,
	}
	e.statementCapMS = e.queryTimeout.Milliseconds()
	for _, opt := range opts {
		opt(e)
	}
	return e
}

// Compile-time check: SafeQueryExecutor satisfies QueryExecutor.
var _ QueryExecutor = (*SafeQueryExecutor)(nil)

// ValidateQuery applies structural and policy checks without executing
// the query. Used by ExecuteQuery / ExplainQuery internally; exported so
// callers (e.g. a UI dry-run) can validate without spinning a tx.
func (e *SafeQueryExecutor) ValidateQuery(ctx context.Context, query string) error {
	if query == "" {
		return fmt.Errorf("sql.SafeQueryExecutor.ValidateQuery: %w", ErrEmptyQuery)
	}
	if len(query) > e.maxQueryLength {
		return fmt.Errorf("sql.SafeQueryExecutor.ValidateQuery: %w (limit %d, got %d)", ErrQueryTooLong, e.maxQueryLength, len(query))
	}

	normalized := normalizeQuery(query)
	// Specific scans first so callers get the most informative error:
	// DROP, INSERT, etc. report ErrWriteOperation rather than the
	// broader ErrNotReadOnly.
	if isWriteOperation(normalized) {
		return fmt.Errorf("sql.SafeQueryExecutor.ValidateQuery: %w", ErrWriteOperation)
	}
	if containsDangerousPatterns(normalized) {
		return fmt.Errorf("sql.SafeQueryExecutor.ValidateQuery: %w", ErrDangerousPattern)
	}
	// Positive allowlist: the first real keyword must be SELECT,
	// WITH, or VALUES. Rejects SHOW, SET, DO, RESET, etc. — read-only
	// from the server's perspective but outside the executor's
	// read-data contract.
	if !isReadOnlyStatement(normalized) {
		return fmt.Errorf("sql.SafeQueryExecutor.ValidateQuery: %w", ErrNotReadOnly)
	}
	if err := e.policy.Check(ctx, query); err != nil {
		return fmt.Errorf("sql.SafeQueryExecutor.ValidateQuery: policy: %w", err)
	}
	return nil
}

// ExecuteQuery satisfies the QueryExecutor interface. timeout > 0 overrides
// the executor's configured queryTimeout for this call only; pass 0 to use
// the default. Rows are scanned via FormatValue; the result is truncated
// (with Truncated=true) once maxResultRows is reached.
func (e *SafeQueryExecutor) ExecuteQuery(ctx context.Context, query string, params []any, timeout time.Duration) (*QueryResult, error) {
	if err := e.ValidateQuery(ctx, query); err != nil {
		return nil, err
	}

	effective := e.resolveTimeout(timeout)
	queryCtx, cancel := context.WithTimeout(ctx, effective)
	defer cancel()

	var result *QueryResult
	err := e.withTenantTx(queryCtx, effective, func(tx pgx.Tx) error {
		start := time.Now()

		rows, err := tx.Query(queryCtx, query, params...)
		if err != nil {
			return fmt.Errorf("query execution failed: %w", err)
		}
		defer rows.Close()

		fds := rows.FieldDescriptions()
		columnNames := make([]string, len(fds))
		columnTypes := make([]string, len(fds))
		for i, fd := range fds {
			columnNames[i] = fd.Name
			columnTypes[i] = PgOIDToColumnType(fd.DataTypeOID)
		}

		out := make([][]any, 0, 64)
		truncated := false

		for rows.Next() {
			if len(out) >= e.maxResultRows {
				truncated = true
				break
			}
			values, err := rows.Values()
			if err != nil {
				return fmt.Errorf("scan row: %w", err)
			}
			row := make([]any, len(values))
			for i, val := range values {
				row[i] = FormatValue(val)
			}
			out = append(out, row)
		}

		if err := rows.Err(); err != nil {
			return fmt.Errorf("iterate rows: %w", err)
		}

		result = &QueryResult{
			Columns:     columnNames,
			ColumnTypes: columnTypes,
			Rows:        out,
			RowCount:    len(out),
			Truncated:   truncated,
			Duration:    time.Since(start),
			SQL:         query,
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("sql.SafeQueryExecutor.ExecuteQuery: %w", err)
	}
	return result, nil
}

// ExplainQuery returns the JSON execution plan for a query. The query is
// validated and tenant-scoped exactly like ExecuteQuery; the EXPLAIN itself
// is wrapped server-side as `EXPLAIN (FORMAT JSON, ANALYZE) ...`.
//
// Note: ANALYZE means the query actually runs, so caps (timeout, tenant
// scope) apply. Use this on dev / staging or with row-bounded queries.
func (e *SafeQueryExecutor) ExplainQuery(ctx context.Context, query string) (string, error) {
	if err := e.ValidateQuery(ctx, query); err != nil {
		return "", err
	}

	effective := e.resolveTimeout(0)
	queryCtx, cancel := context.WithTimeout(ctx, effective)
	defer cancel()

	wrapped := "EXPLAIN (FORMAT JSON, ANALYZE) " + query

	var planLines []string
	err := e.withTenantTx(queryCtx, effective, func(tx pgx.Tx) error {
		rows, err := tx.Query(queryCtx, wrapped)
		if err != nil {
			return fmt.Errorf("explain query failed: %w", err)
		}
		defer rows.Close()

		for rows.Next() {
			var line string
			if err := rows.Scan(&line); err != nil {
				return fmt.Errorf("scan plan line: %w", err)
			}
			planLines = append(planLines, line)
		}
		if err := rows.Err(); err != nil {
			return fmt.Errorf("iterate plan rows: %w", err)
		}
		if len(planLines) == 0 {
			return errors.New("no execution plan returned")
		}
		return nil
	})
	if err != nil {
		return "", fmt.Errorf("sql.SafeQueryExecutor.ExplainQuery: %w", err)
	}
	return strings.Join(planLines, "\n"), nil
}

// resolveTimeout picks the per-call timeout when supplied (>0), else the
// executor's configured queryTimeout.
func (e *SafeQueryExecutor) resolveTimeout(perCall time.Duration) time.Duration {
	if perCall > 0 {
		return perCall
	}
	return e.queryTimeout
}

// withTenantTx opens a read-only transaction, applies SetTenantContext,
// optionally sets a SQL-level statement_timeout, runs fn, and rolls back.
// Read-only tx never commits, even on success — there's nothing to persist.
func (e *SafeQueryExecutor) withTenantTx(ctx context.Context, timeout time.Duration, fn func(tx pgx.Tx) error) error {
	if e.pool == nil {
		return fmt.Errorf("sql.SafeQueryExecutor: pool is nil (executor misconfigured)")
	}
	tx, err := e.pool.BeginTx(ctx, pgx.TxOptions{AccessMode: pgx.ReadOnly})
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	// Use a cancel-free context for rollback so a client timeout/deadline
	// still lets us return the connection to the pool cleanly. The server
	// would auto-rollback on close anyway, but explicit is cheaper.
	defer func() { _ = tx.Rollback(context.WithoutCancel(ctx)) }()

	tenantID, err := e.tenantResolver(ctx)
	if err != nil {
		return fmt.Errorf("resolve tenant: %w", err)
	}
	if err := SetTenantContext(ctx, tx, tenantID); err != nil {
		return err
	}

	if e.statementCapMS > 0 {
		// Cap to the smaller of the configured statement cap and the
		// resolved per-call timeout. set_config takes a string; the unit
		// "ms" makes statement_timeout interpretation explicit.
		ms := e.statementCapMS
		if perCallMS := timeout.Milliseconds(); perCallMS > 0 && perCallMS < ms {
			ms = perCallMS
		}
		if _, err := tx.Exec(ctx, fmt.Sprintf("SET LOCAL statement_timeout = '%dms'", ms)); err != nil {
			return fmt.Errorf("set statement_timeout: %w", err)
		}
	}

	return fn(tx)
}

// normalizeQuery strips comments, uppercases keywords, and collapses
// whitespace so naive Contains scans against the write/dangerous lists
// don't trip on formatting.
//
// Ordering matters:
//
//  1. Strip literals first. A literal like `'--'` or `$$INSERT...$$`
//     must not be mistaken for a real comment or keyword. Running
//     comment stripping first would let `SELECT '--'; INSERT ...`
//     collapse into `SELECT ' `, hiding the INSERT from validation.
//  2. Strip comments. Both replacements substitute a single space
//     rather than the empty string: `UPDATE/*x*/foo` must not
//     collapse to `UPDATEfoo`, which would slip past the `UPDATE `
//     prefix check in isWriteOperation.
//  3. Uppercase + collapse whitespace so naive Contains scans against
//     the write/dangerous lists match regardless of source casing.
//
// Literal stripping runs before uppercasing because dollar-tag matching
// is case sensitive.
func normalizeQuery(query string) string {
	query = stripSQLLiterals(query)
	query = commentLineRE.ReplaceAllString(query, " ")
	query = blockCommentRE.ReplaceAllString(query, " ")
	query = strings.ToUpper(query)
	query = strings.Join(strings.Fields(query), " ")
	return query
}

// stripSQLLiterals blanks single-quoted strings, dollar-quoted strings,
// and double-quoted identifiers so validation scans only real tokens.
// Each literal is replaced with a single space to preserve token
// boundaries (so `'X'Y` stays separable).
func stripSQLLiterals(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for i := 0; i < len(s); {
		c := s[i]
		switch {
		case c == '\'':
			// Single-quoted string. `''` is an embedded quote.
			b.WriteByte(' ')
			i++
			for i < len(s) {
				if s[i] == '\'' {
					if i+1 < len(s) && s[i+1] == '\'' {
						i += 2
						continue
					}
					i++
					break
				}
				i++
			}
		case c == '"':
			// Double-quoted identifier. `""` is an embedded quote.
			b.WriteByte(' ')
			i++
			for i < len(s) {
				if s[i] == '"' {
					if i+1 < len(s) && s[i+1] == '"' {
						i += 2
						continue
					}
					i++
					break
				}
				i++
			}
		case c == '$':
			// Potential dollar-quoted literal: $tag$ ... $tag$ or $$ ... $$.
			j := i + 1
			for j < len(s) && (s[j] == '_' || isAlphaNumByte(s[j])) {
				j++
			}
			if j < len(s) && s[j] == '$' {
				tag := s[i : j+1]
				if end := strings.Index(s[j+1:], tag); end >= 0 {
					b.WriteByte(' ')
					i = j + 1 + end + len(tag)
					continue
				}
			}
			// Not a dollar-quoted literal; keep the character.
			b.WriteByte(c)
			i++
		default:
			b.WriteByte(c)
			i++
		}
	}
	return b.String()
}

func isAlphaNumByte(c byte) bool {
	return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9')
}

// readOnlyStatementPrefixes are the allow-listed first keywords for
// ValidateQuery. EXPLAIN is included because tools wrap user queries
// with it (e.g. `EXPLAIN (FORMAT TEXT) SELECT ...`); the inner write
// keywords are still caught by the blocklist that runs before this
// allowlist.
var readOnlyStatementPrefixes = []string{
	"SELECT ",
	"WITH ",
	"VALUES ",
	"EXPLAIN ",
	// Single-statement edge cases — parenthesised CTEs and VALUES.
	"(",
}

// isReadOnlyStatement checks that the first real keyword is one of the
// allow-listed read-only statement starters. `normalized` is already
// uppercase and has comments + literals stripped.
func isReadOnlyStatement(normalized string) bool {
	trimmed := strings.TrimLeft(normalized, " \t\n\r;")
	for _, p := range readOnlyStatementPrefixes {
		if strings.HasPrefix(trimmed, p) {
			return true
		}
	}
	// Bare single-token queries like "SELECT" (no trailing space) also
	// need to be permitted for ValidateQuery to behave like a strict
	// parser — but they're caught later by the write-op scan which
	// looks for keyword prefixes followed by space. Treat an exact
	// token match (no trailing content) as read-only.
	switch trimmed {
	case "SELECT", "WITH", "VALUES", "EXPLAIN":
		return true
	}
	return false
}

// writeOperationPrefixes intentionally include a trailing space so we don't
// false-match identifiers like "DELETED" (column name) or "DROPDOWN".
var writeOperationPrefixes = []string{
	"INSERT ",
	"UPDATE ",
	"DELETE ",
	"DROP ",
	"CREATE ",
	"ALTER ",
	"TRUNCATE ",
	"GRANT ",
	"REVOKE ",
	"CALL ",
}

func isWriteOperation(normalized string) bool {
	for _, p := range writeOperationPrefixes {
		if strings.Contains(normalized, p) {
			return true
		}
	}
	return false
}

var dangerousPatterns = []string{
	"EXEC ",
	"EXECUTE ",
	"LOAD DATA",
	"INTO OUTFILE",
	"INTO DUMPFILE",
	"PRAGMA",
	"ATTACHDATABASE",
	// COPY reads/writes the client protocol. Even under a read-only tx,
	// `COPY (SELECT ...) TO PROGRAM '...'` on a privileged role can exfil
	// or shell out. Rely on role grants as primary defence; this is belt.
	"COPY ",
	// set_config() inside a SELECT can rewrite `app.tenant_id` mid-
	// transaction, bypassing the executor's tenant binding and the RLS
	// policies that key off current_setting('app.tenant_id'). Reject
	// any call to set_config regardless of the is_local flag.
	"SET_CONFIG(",
	// SET ROLE / SET SESSION AUTHORIZATION switch the effective role
	// and would also bypass role-scoped RLS policies. SET LOCAL is
	// similarly blocked because read-only tx permits it.
	"SET ROLE ",
	"SET LOCAL ROLE ",
	"SET SESSION AUTHORIZATION",
	// Server-side file / large-object reads are a privilege-escalation
	// surface even under read-only tx on roles with the filesystem
	// grants. Rely on role grants as primary; block as belt.
	"PG_READ_SERVER_FILES(",
	"PG_READ_BINARY_FILE(",
	"LO_EXPORT(",
	"LO_IMPORT(",
}

func containsDangerousPatterns(normalized string) bool {
	for _, p := range dangerousPatterns {
		if strings.Contains(normalized, p) {
			return true
		}
	}
	return false
}
