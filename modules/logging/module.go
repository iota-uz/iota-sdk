package logging

import (
	"embed"

	"github.com/iota-uz/iota-sdk/modules/logging/permissions"
	"github.com/iota-uz/iota-sdk/pkg/application"
)

//go:embed presentation/locales/*.json
var localeFiles embed.FS

//go:embed infrastructure/persistence/schema/crm-schema.sql
var migrationFiles embed.FS

func NewModule() application.Module {
	return &Module{}
}

type Module struct {
}

func (m *Module) Register(app application.Application) error {
	app.RBAC().Register(permissions.Permissions...)
	app.RegisterLocaleFiles(&localeFiles)
	app.RegisterMigrationDirs(&migrationFiles)
	return nil
}

func (m *Module) Name() string {
	return "crm"
}
