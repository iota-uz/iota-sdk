package formatters

import (
	"github.com/iota-uz/iota-sdk/pkg/bichat/context"
)

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

// DefaultFormatterRegistry returns a FormatterRegistry pre-populated
// with all built-in formatters.
func DefaultFormatterRegistry() *context.FormatterRegistry {
	r := context.NewFormatterRegistry()

	r.Register(CodecQueryResult, NewQueryResultFormatter())
	r.Register(CodecExplainPlan, NewExplainPlanFormatter())
	r.Register(CodecSchemaList, NewSchemaListFormatter())
	r.Register(CodecSchemaDescribe, NewSchemaDescribeFormatter())
	r.Register(CodecToolError, NewToolErrorFormatter())
	r.Register(CodecSQLDiagnosis, NewToolErrorFormatter()) // reuses same formatter
	r.Register(CodecArtifactList, NewArtifactListFormatter())
	r.Register(CodecArtifactContent, NewArtifactContentFormatter())

	// JSON-based formatters
	jsonFmt := NewJSONFormatter()
	r.Register(CodecSearchResults, jsonFmt)
	r.Register(CodecTime, jsonFmt)
	r.Register(CodecJSON, jsonFmt)

	return r
}
