package sql

import (
	"context"
	"fmt"
	"regexp"
)

// QueryPolicy is a pluggable authorization hook evaluated for every query
// before it reaches the database. Consumers inject domain-specific rules
// (e.g. "accounting schema requires permission X") without coupling the
// SDK to their permission system.
//
// A Check that returns a non-nil error aborts the query. The error is
// surfaced to callers verbatim; wrap with whatever semantics the consumer
// prefers (unauthorized, forbidden, etc.).
type QueryPolicy interface {
	Check(ctx context.Context, sql string) error
}

// AllowAllPolicy is the default policy: every query passes. SDK ships this
// so SafeQueryExecutor works out of the box when no policy is configured.
type AllowAllPolicy struct{}

// Check always returns nil.
func (AllowAllPolicy) Check(context.Context, string) error { return nil }

// PolicyFunc adapts a plain function to the QueryPolicy interface.
type PolicyFunc func(ctx context.Context, sql string) error

// Check delegates to the underlying function.
func (f PolicyFunc) Check(ctx context.Context, sql string) error {
	if f == nil {
		return nil
	}
	return f(ctx, sql)
}

// NewSchemaGatedPolicy returns a QueryPolicy that blocks queries
// referencing the given schema unless `authz(ctx)` returns nil.
//
// The shape is generic: consumers supply the schema name (e.g.
// "accounting") and an authorization callback that consults their
// permission system. The SDK doesn't know about those permissions;
// it just executes the callback when the schema shows up.
//
// Detection is a case-insensitive regex on the raw query text. It
// catches bare-identifier, quoted-identifier, and schema-qualified
// references ("accounting.table", `"accounting"."table"`,
// `FROM accounting .table`). False negatives are possible against
// heavily obfuscated SQL — this is a permission gate, not a parser.
//
// NOTE: the regex runs before string-literal stripping, so a literal
// like `SELECT 'accounting.gl' AS x` will match. Errors on the side
// of refusal. Callers with stricter needs should layer their own
// parser-backed policy.
//
// When `schema` is empty the policy permits everything. When `authz`
// is nil and the schema is present, queries are blocked with a
// generic "access to schema X requires authorization" error.
func NewSchemaGatedPolicy(schema string, authz func(ctx context.Context) error) QueryPolicy {
	if schema == "" {
		return AllowAllPolicy{}
	}
	// Match the schema name as a whole identifier, optionally wrapped
	// in double quotes, followed by the qualifier dot. The
	// [^a-zA-Z0-9_] preceding boundary avoids matching a column named
	// `accounting_note`.
	pattern := regexp.MustCompile(
		`(?i)(^|[^a-zA-Z0-9_])"?` + regexp.QuoteMeta(schema) + `"?\s*\.`,
	)
	return PolicyFunc(func(ctx context.Context, sql string) error {
		if !pattern.MatchString(sql) {
			return nil
		}
		if authz == nil {
			return fmt.Errorf("access to schema %q requires authorization", schema)
		}
		return authz(ctx)
	})
}
