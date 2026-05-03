package sql

import "time"

// BIChatDefaults returns the ExecutorOption bundle that matches the
// spotlight / bichat production wiring: 10-second query timeout, 200-row
// result cap, 12 KB max query length, and the supplied TenantResolver.
// Consumers apply these as a starting point and override specific knobs
// as needed:
//
//	opts := append(sql.BIChatDefaults(composables.UseTenantID),
//	    sql.WithQueryPolicy(myPolicy),
//	    sql.WithMaxResultRows(500), // override
//	)
//	executor := sql.NewSafeQueryExecutor(pool, opts...)
//
// Tuned for BI-style short, interactive queries where the LLM wants
// enough rows to reason about (~100s) but not enough to blow the
// model's context window, and where a single runaway query must not
// hang a request longer than ~10s.
func BIChatDefaults(resolver TenantResolver) []ExecutorOption {
	return []ExecutorOption{
		WithQueryTimeout(10 * time.Second),
		WithMaxResultRows(200),
		WithMaxQueryLength(12_000),
		WithTenantResolver(resolver),
	}
}
