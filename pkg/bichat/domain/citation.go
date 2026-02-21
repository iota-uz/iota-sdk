package domain

import "github.com/iota-uz/iota-sdk/pkg/bichat/types"

// Citation is an alias for types.Citation.
// All citation handling is consolidated in pkg/bichat/types.
// Use this alias to avoid import changes in domain-layer callers.
type Citation = types.Citation

// NewCitation creates a new Citation with the given source metadata.
// Re-exported from types for domain-layer convenience.
var NewCitation = types.NewCitation
