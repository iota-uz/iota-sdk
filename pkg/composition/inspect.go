package composition

import (
	"github.com/iota-uz/iota-sdk/pkg/spotlight"
	"github.com/iota-uz/iota-sdk/pkg/types"
)

// StaticContributions captures the static, container-independent contributions
// a component declares during Build. It is populated by InspectStatic and is
// primarily useful for asserting Build-time wiring (e.g. whether a component
// contributes navigation under a given set of options) without standing up a
// full container or database.
type StaticContributions struct {
	NavItems   []types.NavigationItem
	QuickLinks []*spotlight.QuickLink
}

// InspectStatic runs a component's Build against a fresh builder and returns its
// static nav-item and quick-link contributions, without instantiating providers
// or touching a database.
//
// It is intended for components whose nav-item/quick-link factories are static
// (e.g. registered via AddNavItems / AddQuickLinks, which ignore the container).
// Such factories are evaluated with a nil container. A container-dependent
// factory (e.g. one that calls Resolve) will fail against the nil container and
// that error is returned, so InspectStatic must not be used to obtain fully
// materialized output — use Compile for that. Returns the component's Build
// error, or the first factory error, if any.
func InspectStatic(component Component) (StaticContributions, error) {
	var out StaticContributions
	if component == nil {
		return out, nil
	}
	descriptor := normalizeDescriptor(component)
	builder := newBuilder(BuildContext{}, descriptor)
	if err := component.Build(builder); err != nil {
		return out, err
	}
	for _, entry := range builder.navItemFactories {
		values, err := entry.factory(nil)
		if err != nil {
			return out, err
		}
		out.NavItems = append(out.NavItems, values...)
	}
	for _, entry := range builder.quickLinkFactories {
		values, err := entry.factory(nil)
		if err != nil {
			return out, err
		}
		out.QuickLinks = append(out.QuickLinks, values...)
	}
	return out, nil
}
