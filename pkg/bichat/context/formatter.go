package context

import "github.com/iota-uz/iota-sdk/pkg/bichat/agents"

// FormatOptions controls how a formatter renders structured data.
type FormatOptions struct {
	// MaxRows caps the number of data rows in tabular output (0 = no limit).
	MaxRows int
	// MaxCellWidth caps individual cell length in tabular output (0 = no limit).
	MaxCellWidth int
	// MaxOutputTokens is a soft token budget hint for the formatted output (0 = no limit).
	MaxOutputTokens int
}

// DefaultFormatOptions returns sensible defaults for tool output formatting.
func DefaultFormatOptions() FormatOptions {
	return FormatOptions{
		MaxRows:      25,
		MaxCellWidth: 80,
	}
}

// Formatter converts a structured payload into an LLM-readable string.
// Each formatter is registered against a codec ID so the executor can
// look up the correct formatter for a given ToolResult.
type Formatter interface {
	// Format renders the payload as a human/LLM-readable string.
	// The payload type must match what the formatter expects (e.g. QueryResultPayload).
	Format(payload any, opts FormatOptions) (string, error)
}

// FormatterFunc is a convenience adapter for using a plain function as a Formatter.
type FormatterFunc func(payload any, opts FormatOptions) (string, error)

// Format implements Formatter.
func (f FormatterFunc) Format(payload any, opts FormatOptions) (string, error) {
	return f(payload, opts)
}

// FormatterRegistry maps codec IDs to Formatters.
// The executor uses this to convert StructuredTool results to strings.
type FormatterRegistry struct {
	formatters map[string]Formatter
}

// NewFormatterRegistry creates an empty FormatterRegistry.
func NewFormatterRegistry() *FormatterRegistry {
	return &FormatterRegistry{
		formatters: make(map[string]Formatter),
	}
}

// Register adds a formatter for the given codec ID.
// If a formatter is already registered for that ID, it is replaced.
func (r *FormatterRegistry) Register(codecID string, f Formatter) {
	r.formatters[codecID] = f
}

// Get returns the formatter for the given codec ID, or nil if none is registered.
func (r *FormatterRegistry) Get(codecID string) Formatter {
	return r.formatters[codecID]
}

// Has returns true if a formatter is registered for the given codec ID.
func (r *FormatterRegistry) Has(codecID string) bool {
	_, ok := r.formatters[codecID]
	return ok
}

// formatterAdapter adapts a context.Formatter to agents.Formatter interface.
type formatterAdapter struct {
	f Formatter
}

func (a *formatterAdapter) Format(payload any, opts agents.FormatOptions) (string, error) {
	// Convert agents.FormatOptions to context.FormatOptions
	contextOpts := FormatOptions{
		MaxRows:         opts.MaxRows,
		MaxCellWidth:    opts.MaxCellWidth,
		MaxOutputTokens: opts.MaxOutputTokens,
	}
	return a.f.Format(payload, contextOpts)
}

// registryAdapter adapts context.FormatterRegistry to agents.FormatterRegistry interface.
type registryAdapter struct {
	r *FormatterRegistry
}

func (a *registryAdapter) Get(codecID string) agents.Formatter {
	f := a.r.Get(codecID)
	if f == nil {
		return nil
	}
	return &formatterAdapter{f: f}
}

// AsAgentsRegistry returns an agents.FormatterRegistry adapter for this registry.
// This allows the context.FormatterRegistry to be used with the agents package
// without creating a circular dependency.
func (r *FormatterRegistry) AsAgentsRegistry() agents.FormatterRegistry {
	return &registryAdapter{r: r}
}
