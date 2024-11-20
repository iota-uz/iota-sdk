package modules

import (
	"embed"
	"github.com/benbjohnson/hashfs"
	"github.com/iota-agency/iota-sdk/pkg/shared"
	"github.com/iota-agency/iota-sdk/pkg/types"
)

type ModuleRegistry struct {
	modules         []shared.Module
	controllers     []shared.ControllerConstructor
	navigationItems []types.NavigationItem
	assets          []*hashfs.FS
	localeFiles     []*embed.FS
	migrationDirs   []*embed.FS
}

func (m *ModuleRegistry) RegisterModules(modules ...shared.Module) {
	m.modules = append(m.modules, modules...)
	for _, module := range modules {
		m.controllers = append(m.controllers, module.Controllers()...)
		m.assets = append(m.assets, module.Assets())
		localeFs := module.LocaleFiles()
		if localeFs != nil {
			m.localeFiles = append(m.localeFiles, localeFs)
		}
		migrationsFs := module.MigrationDirs()
		if migrationsFs != nil {
			m.migrationDirs = append(m.migrationDirs, migrationsFs)
		}
	}
}

func (m *ModuleRegistry) Modules() []shared.Module {
	return m.modules
}

func (m *ModuleRegistry) Controllers() []shared.ControllerConstructor {
	return m.controllers
}

func (m *ModuleRegistry) NavigationItems() []types.NavigationItem {
	return m.navigationItems
}

func (m *ModuleRegistry) Assets() []*hashfs.FS {
	return m.assets
}

func (m *ModuleRegistry) LocaleFiles() []*embed.FS {
	return m.localeFiles
}

func (m *ModuleRegistry) MigrationDirs() []*embed.FS {
	return m.migrationDirs
}
