// Package sqlcap adapts the SDK's read-only SafeQueryExecutor into a pykernel
// sql() capability — the data path for the Ali analyst REPL
// (iota-uz/eai#3110). All tenant scoping, the ai_readonly role, the
// accounting-schema guard, the timeout and the row limit come from the
// executor; this is a thin marshaling shim. The capability is read-only, so
// pykernel allows it in both plan and apply runs.
package sqlcap

import (
	"context"
	"fmt"
	"time"

	bichatsql "github.com/iota-uz/iota-sdk/pkg/bichat/sql"
	"github.com/iota-uz/iota-sdk/pkg/pykernel"
)

// DefaultName is the Python function name the capability is exposed under.
const DefaultName = "sql"

// DefaultTimeout is used when no WithTimeout option is given.
const DefaultTimeout = 30 * time.Second

// Option configures the capability.
type Option func(*config)

type config struct {
	name    string
	timeout time.Duration
}

// WithTimeout sets the per-query timeout passed to the executor.
func WithTimeout(d time.Duration) Option { return func(c *config) { c.timeout = d } }

// WithName overrides the exposed Python function name (default "sql").
func WithName(name string) Option { return func(c *config) { c.name = name } }

// New returns the read-only sql() capability backed by executor. The kernel
// calls it as sql(query, params=None) and receives a list of row dicts.
func New(executor bichatsql.QueryExecutor, opts ...Option) pykernel.Capability {
	cfg := config{name: DefaultName, timeout: DefaultTimeout}
	for _, o := range opts {
		o(&cfg)
	}
	sig := pykernel.CapabilitySignature{
		Params: []pykernel.ParamSpec{
			{Name: "query", Type: "str", Required: true},
			{Name: "params", Type: "list", Required: false},
		},
		Returns: "list[dict]",
		Doc:     "Run a read-only SQL query against the internal database and return rows as a list of dicts.",
	}
	return pykernel.CapabilityFunc(cfg.name, pykernel.AccessRead, sig,
		func(ctx context.Context, args pykernel.CallArgs) (any, error) {
			query, ok := args["query"].(string)
			if !ok || query == "" {
				return nil, fmt.Errorf("sql: 'query' must be a non-empty string")
			}
			params, err := toParams(args["params"])
			if err != nil {
				return nil, err
			}
			res, err := executor.ExecuteQuery(ctx, query, params, cfg.timeout)
			if err != nil {
				return nil, err
			}
			return res.AllMaps(), nil
		})
}

// toParams normalizes the optional params argument (a JSON array or absent)
// into a []any for the executor.
func toParams(v any) ([]any, error) {
	switch p := v.(type) {
	case nil:
		return nil, nil
	case []any:
		return p, nil
	default:
		return nil, fmt.Errorf("sql: 'params' must be a list")
	}
}
