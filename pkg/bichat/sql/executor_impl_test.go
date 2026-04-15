package sql

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestSafeQueryExecutor_Defaults(t *testing.T) {
	e := NewSafeQueryExecutor(nil)

	if e.maxQueryLength != DefaultMaxQueryLength {
		t.Fatalf("maxQueryLength: got %d, want %d", e.maxQueryLength, DefaultMaxQueryLength)
	}
	if e.queryTimeout != DefaultQueryTimeout {
		t.Fatalf("queryTimeout: got %v, want %v", e.queryTimeout, DefaultQueryTimeout)
	}
	if e.maxResultRows != DefaultMaxResultRows {
		t.Fatalf("maxResultRows: got %d, want %d", e.maxResultRows, DefaultMaxResultRows)
	}
	if e.statementCapMS != DefaultQueryTimeout.Milliseconds() {
		t.Fatalf("statementCapMS: got %d, want %d", e.statementCapMS, DefaultQueryTimeout.Milliseconds())
	}
	if _, ok := e.policy.(AllowAllPolicy); !ok {
		t.Fatalf("default policy: got %T, want AllowAllPolicy", e.policy)
	}
}

func TestSafeQueryExecutor_Options(t *testing.T) {
	customPolicy := PolicyFunc(func(context.Context, string) error { return nil })
	e := NewSafeQueryExecutor(nil,
		WithMaxQueryLength(100),
		WithQueryTimeout(5*time.Second),
		WithMaxResultRows(10),
		WithQueryPolicy(customPolicy),
		WithStatementTimeoutCap(2*time.Second),
	)

	if e.maxQueryLength != 100 {
		t.Fatalf("maxQueryLength override failed: %d", e.maxQueryLength)
	}
	if e.queryTimeout != 5*time.Second {
		t.Fatalf("queryTimeout override failed: %v", e.queryTimeout)
	}
	if e.maxResultRows != 10 {
		t.Fatalf("maxResultRows override failed: %d", e.maxResultRows)
	}
	if e.statementCapMS != 2000 {
		t.Fatalf("statementCapMS override failed: %d", e.statementCapMS)
	}
}

func TestSafeQueryExecutor_OptionsIgnoreNonPositive(t *testing.T) {
	e := NewSafeQueryExecutor(nil,
		WithMaxQueryLength(0),
		WithQueryTimeout(0),
		WithMaxResultRows(-1),
		WithQueryPolicy(nil),
	)
	if e.maxQueryLength != DefaultMaxQueryLength {
		t.Fatal("zero maxQueryLength should be ignored")
	}
	if e.queryTimeout != DefaultQueryTimeout {
		t.Fatal("zero queryTimeout should be ignored")
	}
	if e.maxResultRows != DefaultMaxResultRows {
		t.Fatal("negative maxResultRows should be ignored")
	}
	if _, ok := e.policy.(AllowAllPolicy); !ok {
		t.Fatal("nil policy should be ignored")
	}
}

func TestSafeQueryExecutor_StatementTimeoutCapDisabled(t *testing.T) {
	e := NewSafeQueryExecutor(nil, WithStatementTimeoutCap(0))
	if e.statementCapMS != 0 {
		t.Fatalf("explicit zero should disable: got %d", e.statementCapMS)
	}
}

func TestValidateQuery_Empty(t *testing.T) {
	e := NewSafeQueryExecutor(nil)
	err := e.ValidateQuery(context.Background(), "")
	if !errors.Is(err, ErrEmptyQuery) {
		t.Fatalf("want ErrEmptyQuery, got %v", err)
	}
}

func TestValidateQuery_TooLong(t *testing.T) {
	e := NewSafeQueryExecutor(nil, WithMaxQueryLength(20))
	err := e.ValidateQuery(context.Background(), strings.Repeat("a", 21))
	if !errors.Is(err, ErrQueryTooLong) {
		t.Fatalf("want ErrQueryTooLong, got %v", err)
	}
}

func TestValidateQuery_RejectsWriteOperations(t *testing.T) {
	e := NewSafeQueryExecutor(nil)

	cases := []string{
		"INSERT INTO foo VALUES (1)",
		"insert into foo values (1)", // case
		"  UPDATE foo SET a=1",       // leading whitespace
		"DELETE FROM foo WHERE id=1",
		"DROP TABLE foo",
		"CREATE TABLE foo(id int)",
		"ALTER TABLE foo ADD col int",
		"TRUNCATE foo",
		"GRANT SELECT ON foo TO bar",
		"REVOKE SELECT ON foo FROM bar",
		"CALL myproc()",
		"/* hidden */ INSERT INTO foo VALUES(1)",        // block comment masked
		"-- DELETE FROM foo\nINSERT INTO foo VALUES(1)", // line comment masked write op revealed
		"UPDATE/*x*/foo SET a=1",                        // block-comment boundary — regression
		"DELETE--x\nfoo WHERE id=1",                     // line-comment boundary — regression
	}
	for _, sql := range cases {
		t.Run(sql, func(t *testing.T) {
			err := e.ValidateQuery(context.Background(), sql)
			if !errors.Is(err, ErrWriteOperation) {
				t.Fatalf("want ErrWriteOperation, got %v", err)
			}
		})
	}
}

func TestValidateQuery_AllowsCommentMaskedWrite(t *testing.T) {
	e := NewSafeQueryExecutor(nil)
	// Confirm comment-stripping does NOT mask a real write op as a SELECT.
	// Here, the actual statement is SELECT; the comment contains INSERT but
	// it's stripped before scanning.
	err := e.ValidateQuery(context.Background(), "SELECT 1 -- INSERT INTO foo VALUES(1)")
	if err != nil {
		t.Fatalf("comment-only INSERT should not block: %v", err)
	}
}

func TestValidateQuery_RejectsDangerousPatterns(t *testing.T) {
	e := NewSafeQueryExecutor(nil)
	cases := []string{
		"SELECT 1; EXEC sys.sp_who",
		"SELECT 1; EXECUTE my_proc",
		"SELECT * FROM foo INTO OUTFILE '/tmp/out'",
		"SELECT * FROM foo INTO DUMPFILE '/tmp/out'",
		"LOAD DATA INFILE '/etc/passwd'",
		"PRAGMA table_info(foo)",
		"ATTACHDATABASE 'evil.db'",
		"COPY (SELECT * FROM foo) TO PROGRAM '/bin/sh'",
	}
	for _, sql := range cases {
		t.Run(sql, func(t *testing.T) {
			err := e.ValidateQuery(context.Background(), sql)
			if !errors.Is(err, ErrDangerousPattern) {
				t.Fatalf("want ErrDangerousPattern, got %v", err)
			}
		})
	}
}

func TestValidateQuery_AllowsReadOnly(t *testing.T) {
	e := NewSafeQueryExecutor(nil)
	cases := []string{
		"SELECT 1",
		"WITH t AS (SELECT 1) SELECT * FROM t",
		"SELECT * FROM public.users WHERE deleted_at IS NULL",
		"VALUES (1), (2), (3)",
		// Literals containing banned keywords must not be flagged.
		"SELECT 'INSERT INTO foo' AS col",
		"SELECT 'DROP TABLE x' FROM public.users",
		`SELECT $$DELETE FROM foo$$ AS msg`,
		`SELECT $tag$TRUNCATE foo$tag$ AS msg`,
		`SELECT "column with DROP" FROM public.weird`,
	}
	for _, sql := range cases {
		if err := e.ValidateQuery(context.Background(), sql); err != nil {
			t.Fatalf("read query rejected: %s -> %v", sql, err)
		}
	}
}

func TestValidateQuery_RejectsNonReadOnlyStatements(t *testing.T) {
	e := NewSafeQueryExecutor(nil)
	cases := []string{
		"SHOW tables",
		"SET search_path TO foo",
		"DO $$ BEGIN PERFORM 1; END $$",
		"RESET statement_timeout",
	}
	for _, sql := range cases {
		err := e.ValidateQuery(context.Background(), sql)
		if !errors.Is(err, ErrNotReadOnly) {
			t.Fatalf("want ErrNotReadOnly for %q, got %v", sql, err)
		}
	}
}

func TestValidateQuery_RejectsTenantEscalation(t *testing.T) {
	// Regression: a malicious payload could rewrite app.tenant_id
	// mid-transaction via set_config(), bypassing the executor's
	// tenant binding. The dangerous-pattern scan must reject it
	// regardless of how the call is buried in a CTE or subquery.
	e := NewSafeQueryExecutor(nil)
	cases := []string{
		"SELECT set_config('app.tenant_id', 'other-uuid', true), * FROM t",
		"WITH s AS MATERIALIZED (SELECT set_config('app.tenant_id', 'other', true)) SELECT * FROM s CROSS JOIN t",
		"SELECT SET_CONFIG('app.tenant_id', 'other-uuid', true) FROM t",
		"SELECT * FROM t; SET ROLE postgres",
		"SELECT * FROM t; SET SESSION AUTHORIZATION 'postgres'",
		"SELECT pg_read_server_files('/etc/passwd', 0, 100)",
		"SELECT lo_export(16387, '/tmp/x')",
	}
	for _, sql := range cases {
		err := e.ValidateQuery(context.Background(), sql)
		if !errors.Is(err, ErrDangerousPattern) {
			t.Fatalf("want ErrDangerousPattern for %q, got %v", sql, err)
		}
	}
}

func TestValidateQuery_AllowsExplain(t *testing.T) {
	// EXPLAIN-wrapped reads are a valid tool path (sql_execute
	// explain_plan=true). The allowlist must let them through; the
	// blocklist still catches write statements even when buried
	// inside EXPLAIN.
	e := NewSafeQueryExecutor(nil)
	readCases := []string{
		"EXPLAIN SELECT 1",
		"EXPLAIN (FORMAT JSON) SELECT * FROM public.users",
		"EXPLAIN ANALYZE SELECT count(*) FROM public.users",
	}
	for _, sql := range readCases {
		if err := e.ValidateQuery(context.Background(), sql); err != nil {
			t.Fatalf("EXPLAIN read rejected: %q -> %v", sql, err)
		}
	}
	writeCases := []string{
		"EXPLAIN INSERT INTO x VALUES (1)",
		"EXPLAIN UPDATE x SET a = 1",
	}
	for _, sql := range writeCases {
		err := e.ValidateQuery(context.Background(), sql)
		if !errors.Is(err, ErrWriteOperation) {
			t.Fatalf("EXPLAIN over write must still reject: %q -> %v", sql, err)
		}
	}
}

func TestValidateQuery_PolicyHookInvoked(t *testing.T) {
	denied := errors.New("policy denied")
	called := false
	policy := PolicyFunc(func(ctx context.Context, sql string) error {
		called = true
		return denied
	})

	e := NewSafeQueryExecutor(nil, WithQueryPolicy(policy))
	err := e.ValidateQuery(context.Background(), "SELECT 1")
	if !called {
		t.Fatal("policy.Check not invoked")
	}
	if !errors.Is(err, denied) {
		t.Fatalf("want wrapped denied, got %v", err)
	}
}

func TestValidateQuery_PolicySkippedOnStructuralFailure(t *testing.T) {
	called := false
	policy := PolicyFunc(func(context.Context, string) error {
		called = true
		return nil
	})

	e := NewSafeQueryExecutor(nil, WithQueryPolicy(policy))
	_ = e.ValidateQuery(context.Background(), "DROP TABLE foo")
	if called {
		t.Fatal("policy.Check should be skipped when write-op rejection fires first")
	}
}

func TestResolveTimeout(t *testing.T) {
	e := NewSafeQueryExecutor(nil, WithQueryTimeout(7*time.Second))

	if got := e.resolveTimeout(0); got != 7*time.Second {
		t.Fatalf("default fallback: got %v, want 7s", got)
	}
	if got := e.resolveTimeout(2 * time.Second); got != 2*time.Second {
		t.Fatalf("per-call override: got %v, want 2s", got)
	}
}

func TestNormalizeQuery(t *testing.T) {
	cases := map[string]string{
		"select 1":                         "SELECT 1",
		"SELECT 1 -- trailing":             "SELECT 1",
		"SELECT /* mid */ 1":               "SELECT 1",
		"  select\n  1  \n":                "SELECT 1",
		"/*x*/INSERT/*y*/ INTO foo VALUES": "INSERT INTO FOO VALUES",
	}
	for in, want := range cases {
		if got := normalizeQuery(in); got != want {
			t.Errorf("normalizeQuery(%q) = %q, want %q", in, got, want)
		}
	}
}
