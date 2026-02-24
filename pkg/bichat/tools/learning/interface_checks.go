package learning

import "github.com/iota-uz/iota-sdk/pkg/bichat/agents"

// Compile-time interface checks
var (
	_ agents.StructuredTool = (*SearchLearningsTool)(nil)
	_ agents.StructuredTool = (*SaveLearningTool)(nil)
	_ agents.StructuredTool = (*SearchValidatedQueriesTool)(nil)
	_ agents.StructuredTool = (*SaveValidatedQueryTool)(nil)
)
