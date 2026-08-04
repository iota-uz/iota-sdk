package composition

import (
	"embed"

	"github.com/benbjohnson/hashfs"
	"github.com/iota-uz/iota-sdk/pkg/application"
	"github.com/iota-uz/iota-sdk/pkg/types"
)

// The Add* helpers panic on a nil builder (a programmer error — the rest of
// the composition API panics on nil builders too) but treat an empty input
// slice as a no-op: attaching zero items is always valid and lets callers
// use variadic splats like `AddNavNodes(builder, optionalNodes...)` without
// guarding the caller side.

// AddNavNodes attaches one or more descriptor-backed navigation catalog nodes.
func AddNavNodes(builder *Builder, nodes ...application.NavNode) {
	if builder == nil {
		panic("composition: builder is nil")
	}
	if len(nodes) == 0 {
		return
	}
	captured := append([]application.NavNode(nil), nodes...)
	ContributeNavNodes(builder, func(*Container) ([]application.NavNode, error) {
		return captured, nil
	})
}

// AddNavWorkspaces attaches one or more sidebar workspace declarations.
func AddNavWorkspaces(builder *Builder, workspaces ...types.NavWorkspace) {
	if builder == nil {
		panic("composition: builder is nil")
	}
	if len(workspaces) == 0 {
		return
	}
	captured := append([]types.NavWorkspace(nil), workspaces...)
	ContributeNavWorkspaces(builder, func(*Container) ([]types.NavWorkspace, error) {
		return captured, nil
	})
}

// AddNavProviders attaches runtime navigation providers.
func AddNavProviders(builder *Builder, providers ...application.NavProvider) {
	if builder == nil {
		panic("composition: builder is nil")
	}
	if len(providers) == 0 {
		return
	}
	captured := append([]application.NavProvider(nil), providers...)
	ContributeNavProviders(builder, func(*Container) ([]application.NavProvider, error) {
		return captured, nil
	})
}

// RemoveNavItemsByKey removes contributed navigation items with matching
// stable keys after all nav contributions have materialized.
func RemoveNavItemsByKey(builder *Builder, keys ...string) {
	if builder == nil {
		panic("composition: builder is nil")
	}
	for _, key := range keys {
		if key == "" {
			continue
		}
		builder.navItemRemovals = append(builder.navItemRemovals, key)
	}
}

// ReplaceNavItemsByKey replaces contributed navigation items by their stable
// keys after all nav contributions have materialized.
func ReplaceNavItemsByKey(builder *Builder, items ...types.NavigationItem) {
	if builder == nil {
		panic("composition: builder is nil")
	}
	for _, item := range items {
		if item.Key == "" {
			continue
		}
		builder.navItemOverrides = append(builder.navItemOverrides, item)
	}
}

// AddHashFS attaches one or more hashfs.FS asset bundles.
func AddHashFS(builder *Builder, assets ...*hashfs.FS) {
	if builder == nil {
		panic("composition: builder is nil")
	}
	if len(assets) == 0 {
		return
	}
	captured := append([]*hashfs.FS(nil), assets...)
	ContributeHashFS(builder, func(*Container) ([]*hashfs.FS, error) {
		return captured, nil
	})
}

// AddAssets attaches one or more raw embed.FS asset bundles.
func AddAssets(builder *Builder, assets ...*embed.FS) {
	if builder == nil {
		panic("composition: builder is nil")
	}
	if len(assets) == 0 {
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
	if builder == nil {
		panic("composition: builder is nil")
	}
	if len(ctrls) == 0 {
		return
	}
	captured := append([]application.Controller(nil), ctrls...)
	ContributeControllers(builder, func(*Container) ([]application.Controller, error) {
		return captured, nil
	})
}
