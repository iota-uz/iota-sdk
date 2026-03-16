// Package drill provides backwards-compatible aliases for cube drill types.
//
// Deprecated: Use [cube.DrillContext] and [cube.ParseDrillContext] directly.
// This package exists only for migration convenience and will be removed in a
// future release.
package drill

import (
	"net/url"

	"github.com/iota-uz/iota-sdk/pkg/lens/cube"
)

// QueryFilter is an alias for [cube.QueryFilter].
//
// Deprecated: Use cube.QueryFilter directly.
const QueryFilter = cube.QueryFilter

// State is a thin wrapper around [cube.DrillContext].
//
// Deprecated: Use *cube.DrillContext directly.
type State = cube.DrillContext

// Parse parses drill filters from URL query values.
//
// Deprecated: Use [cube.ParseDrillContext] directly.
func Parse(values url.Values) *State {
	ctx := cube.ParseDrillContext(values)
	return &ctx
}

// Strip removes drill filter params from URL query values.
//
// Deprecated: Use [cube.Strip] directly.
func Strip(values url.Values) url.Values {
	return cube.Strip(values)
}
