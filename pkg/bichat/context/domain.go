package context

// DomainContext provides domain-specific context for agent system prompts.
// Implementations provide business context, timezone, and data source information
// that help the agent understand the domain it's operating in.
//
// This interface enables consumers to inject domain knowledge without modifying
// the core bichat module. For example, an insurance application can provide
// OSAGO/KASKO-specific business rules and table descriptions.
type DomainContext interface {
	// Timezone returns the user's timezone for date/time context.
	// Used for resolving relative dates like "last month" or "this quarter".
	// Example: "Asia/Tashkent", "UTC", "America/New_York"
	Timezone() string

	// DataSources returns descriptions of available data sources.
	// These help the agent understand what data is available and how to query it.
	DataSources() []DataSource

	// BusinessRules returns domain-specific rules and constraints as text.
	// This is included in the system prompt to guide agent behavior.
	// Example: "OSAGO policies require valid driver license. Premium is calculated based on..."
	BusinessRules() string
}

// DataSource describes a data source available to the agent.
// This could be a database table, API endpoint, or other data source.
type DataSource struct {
	// Name is the identifier used to reference this source (e.g., table name).
	Name string

	// Description explains what data this source contains.
	Description string

	// Type indicates the kind of data source (e.g., "table", "view", "api").
	Type string

	// Schema provides optional schema information (columns, types, etc.).
	Schema string
}

// EmptyDomainContext is a no-op implementation that returns empty values.
// Use this as a default when no domain context is configured.
type EmptyDomainContext struct{}

// NewEmptyDomainContext creates an empty domain context.
func NewEmptyDomainContext() *EmptyDomainContext {
	return &EmptyDomainContext{}
}

// Timezone returns UTC as the default timezone.
func (e *EmptyDomainContext) Timezone() string {
	return "UTC"
}

// DataSources returns an empty slice.
func (e *EmptyDomainContext) DataSources() []DataSource {
	return nil
}

// BusinessRules returns an empty string.
func (e *EmptyDomainContext) BusinessRules() string {
	return ""
}

// SimpleDomainContext is a basic implementation with configurable values.
// Use this for simple domain configurations or testing.
type SimpleDomainContext struct {
	timezone      string
	dataSources   []DataSource
	businessRules string
}

// SimpleDomainContextOption is a functional option for SimpleDomainContext.
type SimpleDomainContextOption func(*SimpleDomainContext)

// WithTimezone sets the timezone for the domain context.
func WithDomainTimezone(tz string) SimpleDomainContextOption {
	return func(c *SimpleDomainContext) {
		c.timezone = tz
	}
}

// WithDataSources sets the available data sources.
func WithDataSources(sources ...DataSource) SimpleDomainContextOption {
	return func(c *SimpleDomainContext) {
		c.dataSources = sources
	}
}

// WithBusinessRules sets the business rules text.
func WithBusinessRules(rules string) SimpleDomainContextOption {
	return func(c *SimpleDomainContext) {
		c.businessRules = rules
	}
}

// NewSimpleDomainContext creates a domain context with the given options.
//
// Example:
//
//	ctx := context.NewSimpleDomainContext(
//	    context.WithDomainTimezone("Asia/Tashkent"),
//	    context.WithDataSources(
//	        context.DataSource{Name: "policies", Description: "Insurance policies"},
//	        context.DataSource{Name: "claims", Description: "Insurance claims"},
//	    ),
//	    context.WithBusinessRules("OSAGO policies require valid driver license..."),
//	)
func NewSimpleDomainContext(opts ...SimpleDomainContextOption) *SimpleDomainContext {
	ctx := &SimpleDomainContext{
		timezone: "UTC",
	}

	for _, opt := range opts {
		opt(ctx)
	}

	return ctx
}

// Timezone returns the configured timezone.
func (c *SimpleDomainContext) Timezone() string {
	return c.timezone
}

// DataSources returns the configured data sources.
func (c *SimpleDomainContext) DataSources() []DataSource {
	return c.dataSources
}

// BusinessRules returns the configured business rules.
func (c *SimpleDomainContext) BusinessRules() string {
	return c.businessRules
}
