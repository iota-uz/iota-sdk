package sql

import "github.com/iota-uz/iota-sdk/pkg/bichat/agents"

// Compile-time interface checks
var (
	_ agents.StructuredTool = (*SQLExecuteTool)(nil)
	_ agents.StructuredTool = (*SchemaListTool)(nil)
	_ agents.StructuredTool = (*SchemaDescribeTool)(nil)
)
