package composition

import (
	"embed"

	"github.com/benbjohnson/hashfs"
	"github.com/gorilla/mux"
	"github.com/iota-uz/iota-sdk/pkg/application"
	"github.com/iota-uz/iota-sdk/pkg/spotlight"
	"github.com/iota-uz/iota-sdk/pkg/types"
)

type navItemRegistrar interface {
	RegisterNavItems(items ...types.NavigationItem)
}

type controllerRegistrar interface {
	RegisterControllers(controllers ...application.Controller)
}

type hashFSRegistrar interface {
	RegisterHashFsAssets(fs ...*hashfs.FS)
}

type assetRegistrar interface {
	RegisterAssets(fs ...*embed.FS)
}

type localeRegistrar interface {
	RegisterLocaleFiles(fs ...*embed.FS)
}

type graphSchemaRegistrar interface {
	RegisterGraphSchema(schema application.GraphSchema)
}

type middlewareRegistrar interface {
	RegisterMiddleware(middleware ...mux.MiddlewareFunc)
}

type appletRegistrar interface {
	RegisterApplet(applet application.Applet) error
}

func syncApplication(app application.Application, container *Container) error {
	if app == nil || container == nil {
		return nil
	}
	appendLocaleFilesToApp(app, container.locales)
	appendNavItemsToApp(app, container.navItems)
	appendGraphSchemasToApp(app, container.graphSchemas)
	appendAssetsToApp(app, container.assets)
	appendHashFSAssetsToApp(app, container.hashFSAssets)
	appendQuickLinksToApp(app, container.quickLinks)
	appendSpotlightProvidersToApp(app, container.spotlightProviders)
	appendMiddlewareToApp(app, container.middleware)
	if err := appendAppletsToApp(app, container.applets); err != nil {
		return err
	}
	appendControllersToApp(app, container.controllers)
	return nil
}

func appendLocaleFilesToApp(app application.Application, locales []*embed.FS) {
	registrar, ok := app.(localeRegistrar)
	if !ok || len(locales) == 0 {
		return
	}
	registrar.RegisterLocaleFiles(locales...)
}

func appendNavItemsToApp(app application.Application, items []types.NavigationItem) {
	registrar, ok := app.(navItemRegistrar)
	if !ok || len(items) == 0 {
		return
	}
	registrar.RegisterNavItems(items...)
}

func appendGraphSchemasToApp(app application.Application, schemas []application.GraphSchema) {
	registrar, ok := app.(graphSchemaRegistrar)
	if !ok {
		return
	}
	for _, schema := range schemas {
		registrar.RegisterGraphSchema(schema)
	}
}

func appendAssetsToApp(app application.Application, assets []*embed.FS) {
	registrar, ok := app.(assetRegistrar)
	if !ok || len(assets) == 0 {
		return
	}
	registrar.RegisterAssets(assets...)
}

func appendHashFSAssetsToApp(app application.Application, assets []*hashfs.FS) {
	registrar, ok := app.(hashFSRegistrar)
	if !ok || len(assets) == 0 {
		return
	}
	registrar.RegisterHashFsAssets(assets...)
}

func appendQuickLinksToApp(app application.Application, quickLinks []*spotlight.QuickLink) {
	if len(quickLinks) == 0 {
		return
	}
	app.QuickLinks().Add(quickLinks...)
}

func appendSpotlightProvidersToApp(app application.Application, providers []spotlight.SearchProvider) {
	if len(providers) == 0 {
		return
	}
	for _, provider := range providers {
		app.Spotlight().RegisterProvider(provider)
	}
}

func appendMiddlewareToApp(app application.Application, middleware []mux.MiddlewareFunc) {
	registrar, ok := app.(middlewareRegistrar)
	if !ok || len(middleware) == 0 {
		return
	}
	registrar.RegisterMiddleware(middleware...)
}

func appendControllersToApp(app application.Application, controllers []application.Controller) {
	registrar, ok := app.(controllerRegistrar)
	if !ok || len(controllers) == 0 {
		return
	}
	registrar.RegisterControllers(controllers...)
}

func appendAppletsToApp(app application.Application, applets []application.Applet) error {
	registrar, ok := app.(appletRegistrar)
	if !ok {
		return nil
	}
	for _, applet := range applets {
		if err := registrar.RegisterApplet(applet); err != nil {
			return err
		}
	}
	return nil
}
