package composition

import (
	"embed"

	"github.com/benbjohnson/hashfs"
	"github.com/iota-uz/iota-sdk/pkg/application"
	"github.com/iota-uz/iota-sdk/pkg/spotlight"
	"github.com/iota-uz/iota-sdk/pkg/types"
)

// AddLocales attaches one or more locale embeds without requiring a closure.
// Equivalent to ContributeLocales(b, func(*Container) ([]*embed.FS, error) {
// return locales, nil }) but with zero ceremony.
func AddLocales(builder *Builder, locales ...*embed.FS) {
	if builder == nil || len(locales) == 0 {
		return
	}
	captured := append([]*embed.FS(nil), locales...)
	ContributeLocales(builder, func(*Container) ([]*embed.FS, error) {
		return captured, nil
	})
}

// AddNavItems attaches one or more navigation items.
func AddNavItems(builder *Builder, items ...types.NavigationItem) {
	if builder == nil || len(items) == 0 {
		return
	}
	captured := append([]types.NavigationItem(nil), items...)
	ContributeNavItems(builder, func(*Container) ([]types.NavigationItem, error) {
		return captured, nil
	})
}

// AddHashFS attaches one or more hashfs.FS asset bundles.
func AddHashFS(builder *Builder, assets ...*hashfs.FS) {
	if builder == nil || len(assets) == 0 {
		return
	}
	captured := append([]*hashfs.FS(nil), assets...)
	ContributeHashFS(builder, func(*Container) ([]*hashfs.FS, error) {
		return captured, nil
	})
}

// AddQuickLinks attaches one or more spotlight quick links.
func AddQuickLinks(builder *Builder, links ...*spotlight.QuickLink) {
	if builder == nil || len(links) == 0 {
		return
	}
	captured := append([]*spotlight.QuickLink(nil), links...)
	ContributeQuickLinks(builder, func(*Container) ([]*spotlight.QuickLink, error) {
		return captured, nil
	})
}

// AddAssets attaches one or more raw embed.FS asset bundles.
func AddAssets(builder *Builder, assets ...*embed.FS) {
	if builder == nil || len(assets) == 0 {
		return
	}
	captured := append([]*embed.FS(nil), assets...)
	ContributeAssets(builder, func(*Container) ([]*embed.FS, error) {
		return captured, nil
	})
}

// AddControllers attaches pre-built controllers without a closure. Useful
// when a component can construct its controllers eagerly inside Build —
// typically because the controllers are stateless wrappers around typed
// configuration values that live on the component itself.
func AddControllers(builder *Builder, ctrls ...application.Controller) {
	if builder == nil || len(ctrls) == 0 {
		return
	}
	captured := append([]application.Controller(nil), ctrls...)
	ContributeControllers(builder, func(*Container) ([]application.Controller, error) {
		return captured, nil
	})
}
