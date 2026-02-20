package types

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
type Formatter interface {
	Format(payload any, opts FormatOptions) (string, error)
}

// FormatterFunc is a convenience adapter for using a plain function as a Formatter.
type FormatterFunc func(payload any, opts FormatOptions) (string, error)

// Format implements Formatter.
func (f FormatterFunc) Format(payload any, opts FormatOptions) (string, error) {
	return f(payload, opts)
}

// FormatterRegistry maps codec IDs to Formatters.
type FormatterRegistry interface {
	Get(codecID string) Formatter
}

// ToolResult carries a structured payload from a StructuredTool.
// The CodecID identifies which Formatter should render the payload.
type ToolResult struct {
	// CodecID identifies the formatter to use (e.g. "query-result", "schema-list").
	CodecID string
	// Payload is the structured data to be formatted.
	Payload any
	// Artifacts are typed output artifacts produced by the tool execution.
	Artifacts []ToolArtifact
}

// ToolArtifact describes a generated artifact emitted by a tool execution.
type ToolArtifact struct {
	Type        string         `json:"type"`
	Name        string         `json:"name"`
	Description string         `json:"description,omitempty"`
	MimeType    string         `json:"mime_type,omitempty"`
	URL         string         `json:"url,omitempty"`
	SizeBytes   int64          `json:"size_bytes,omitempty"`
	Metadata    map[string]any `json:"metadata,omitempty"`
}

// Codec IDs for built-in formatters.
const (
	CodecQueryResult     = "query-result"
	CodecExplainPlan     = "explain-plan"
	CodecSchemaList      = "schema-list"
	CodecSchemaDescribe  = "schema-describe"
	CodecSearchResults   = "search-results"
	CodecToolError       = "tool-error"
	CodecSQLDiagnosis    = "sql-diagnosis"
	CodecTime            = "time"
	CodecArtifactList    = "artifact-list"
	CodecArtifactContent = "artifact-content"
	CodecJSON            = "json"
)
