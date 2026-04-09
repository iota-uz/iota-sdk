package composition

import "github.com/iota-uz/iota-sdk/pkg/application"

type ApplyOptions struct {
	IncludeControllers bool
}

func Apply(app application.Application, container *Container, opts ApplyOptions) error {
	if app == nil || container == nil {
		return nil
	}

	if locales := container.LocaleFiles(); len(locales) > 0 {
		app.RegisterLocaleFiles(locales...)
	}
	if items := container.NavItems(); len(items) > 0 {
		app.RegisterNavItems(items...)
	}
	if schemas := container.GraphSchemas(); len(schemas) > 0 {
		for _, schema := range schemas {
			app.RegisterGraphSchema(schema)
		}
	}
	if assets := container.Assets(); len(assets) > 0 {
		app.RegisterAssets(assets...)
	}
	if hashFSAssets := container.HashFSAssets(); len(hashFSAssets) > 0 {
		app.RegisterHashFsAssets(hashFSAssets...)
	}
	if quickLinks := container.QuickLinks(); len(quickLinks) > 0 {
		app.QuickLinks().Add(quickLinks...)
	}
	if providers := container.SpotlightProviders(); len(providers) > 0 {
		for _, provider := range providers {
			app.Spotlight().RegisterProvider(provider)
		}
	}
	if middleware := container.Middleware(); len(middleware) > 0 {
		app.RegisterMiddleware(middleware...)
	}
	if applets := container.Applets(); len(applets) > 0 {
		for _, applet := range applets {
			if err := app.RegisterApplet(applet); err != nil {
				return err
			}
		}
	}
	if opts.IncludeControllers {
		if controllers := container.Controllers(); len(controllers) > 0 {
			app.RegisterControllers(controllers...)
		}
	}

	return nil
}
