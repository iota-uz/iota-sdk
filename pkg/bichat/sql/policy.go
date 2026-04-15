package sql

import "context"

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
