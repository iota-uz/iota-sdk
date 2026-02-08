package formatters

import (
	"sync"

	"github.com/iota-uz/iota-sdk/pkg/bichat/context"
	"github.com/iota-uz/iota-sdk/pkg/bichat/types"
)

var (
	defaultRegistry     *context.FormatterRegistry
	defaultRegistryOnce sync.Once
)

// DefaultFormatterRegistry returns a shared FormatterRegistry pre-populated
// with all built-in formatters. The registry is created once and reused.
func DefaultFormatterRegistry() *context.FormatterRegistry {
	defaultRegistryOnce.Do(func() {
		r := context.NewFormatterRegistry()

		r.Register(types.CodecQueryResult, NewQueryResultFormatter())
		r.Register(types.CodecExplainPlan, NewExplainPlanFormatter())
		r.Register(types.CodecSchemaList, NewSchemaListFormatter())
		r.Register(types.CodecSchemaDescribe, NewSchemaDescribeFormatter())
		r.Register(types.CodecToolError, NewToolErrorFormatter())
		r.Register(types.CodecSQLDiagnosis, NewToolErrorFormatter()) // reuses same formatter
		r.Register(types.CodecArtifactList, NewArtifactListFormatter())
		r.Register(types.CodecArtifactContent, NewArtifactContentFormatter())

		// JSON-based formatters
		jsonFmt := NewJSONFormatter()
		r.Register(types.CodecSearchResults, jsonFmt)
		r.Register(types.CodecTime, jsonFmt)
		r.Register(types.CodecJSON, jsonFmt)

		defaultRegistry = r
	})
	return defaultRegistry
}
