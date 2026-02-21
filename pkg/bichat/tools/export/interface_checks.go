package export

import "github.com/iota-uz/iota-sdk/pkg/bichat/agents"

// Compile-time interface checks
var (
	_ agents.StructuredTool = (*RenderTableTool)(nil)
	_ agents.StructuredTool = (*ExportToExcelTool)(nil)
	_ agents.StructuredTool = (*ExportToPDFTool)(nil)
	_ agents.StructuredTool = (*ExportQueryToExcelTool)(nil)
)
