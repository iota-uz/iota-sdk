package artifacts

import "github.com/iota-uz/iota-sdk/pkg/bichat/agents"

// Compile-time interface checks
var _ agents.StructuredTool = (*ArtifactReaderTool)(nil)
