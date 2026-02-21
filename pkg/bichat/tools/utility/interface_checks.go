package utility

import "github.com/iota-uz/iota-sdk/pkg/bichat/agents"

// Compile-time interface checks
var _ agents.StructuredTool = (*GetCurrentTimeTool)(nil)
var _ agents.StructuredTool = (*WebFetchTool)(nil)
