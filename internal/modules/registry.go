package modules

import (
	"github.com/benbjohnson/hashfs"
	"github.com/iota-agency/iota-erp/internal/modules/shared"
	"github.com/iota-agency/iota-erp/internal/types"
)

type ModuleRegistry struct {
	modules         []shared.Module
	controllers     []shared.ControllerConstructor
	navigationItems []types.NavigationItem
	assets          []*hashfs.FS
	localeFiles     []string
	migrationDirs   []string
}

func (m *ModuleRegistry) RegisterModules(modules ...shared.Module) {
	m.modules = append(m.modules, modules...)
	for _, module := range modules {
		m.controllers = append(m.controllers, module.Controllers()...)
		m.assets = append(m.assets, module.Assets())
		m.localeFiles = append(m.localeFiles, module.LocaleFiles()...)
		m.migrationDirs = append(m.migrationDirs, module.MigrationDirs()...)
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

func (m *ModuleRegistry) LocaleFiles() []string {
	return m.localeFiles
}

func (m *ModuleRegistry) MigrationDirs() []string {
	return m.migrationDirs
}
