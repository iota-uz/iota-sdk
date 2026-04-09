package composition

import (
	"embed"

	"github.com/benbjohnson/hashfs"
	"github.com/gorilla/mux"
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

// AddSchemas attaches one or more application.GraphSchema entries.
func AddSchemas(builder *Builder, schemas ...application.GraphSchema) {
	if builder == nil || len(schemas) == 0 {
		return
	}
	captured := append([]application.GraphSchema(nil), schemas...)
	ContributeSchemas(builder, func(*Container) ([]application.GraphSchema, error) {
		return captured, nil
	})
}

// AddSpotlightProviders attaches one or more spotlight search providers.
func AddSpotlightProviders(builder *Builder, providers ...spotlight.SearchProvider) {
	if builder == nil || len(providers) == 0 {
		return
	}
	captured := append([]spotlight.SearchProvider(nil), providers...)
	ContributeSpotlightProviders(builder, func(*Container) ([]spotlight.SearchProvider, error) {
		return captured, nil
	})
}

// AddMiddleware attaches one or more mux middlewares.
func AddMiddleware(builder *Builder, mws ...mux.MiddlewareFunc) {
	if builder == nil || len(mws) == 0 {
		return
	}
	captured := append([]mux.MiddlewareFunc(nil), mws...)
	ContributeMiddleware(builder, func(*Container) ([]mux.MiddlewareFunc, error) {
		return captured, nil
	})
}

// AddControllers attaches pre-built controllers without a closure. Useful when
// the controllers are constructed eagerly inside Build (after their typed deps
// have been resolved through Use[T]) or are stateless.
func AddControllers(builder *Builder, controllers ...application.Controller) {
	if builder == nil || len(controllers) == 0 {
		return
	}
	captured := append([]application.Controller(nil), controllers...)
	ContributeControllers(builder, func(*Container) ([]application.Controller, error) {
		return captured, nil
	})
}

// AddApplets attaches pre-built applets.
func AddApplets(builder *Builder, applets ...application.Applet) {
	if builder == nil || len(applets) == 0 {
		return
	}
	captured := append([]application.Applet(nil), applets...)
	ContributeApplets(builder, func(*Container) ([]application.Applet, error) {
		return captured, nil
	})
}

// AddHook attaches a single hook value (no factory closure).
func AddHook(builder *Builder, hook Hook) {
	if builder == nil || hook.Start == nil {
		return
	}
	ContributeHooks(builder, func(*Container) ([]Hook, error) {
		return []Hook{hook}, nil
	})
}
