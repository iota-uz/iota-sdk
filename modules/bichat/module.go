package bichat

import (
	"embed"

	"github.com/iota-uz/iota-sdk/pkg/application"
)

//go:embed presentation/locales/*.json
var LocaleFiles embed.FS

//go:embed infrastructure/persistence/schema/bichat-schema.sql
var MigrationFiles embed.FS

// NewModule creates a BiChat module for schema and locale registration.
//
// For full BiChat functionality, you must:
// 1. Create a ModuleConfig using NewModuleConfig() in config.go
// 2. Implement required dependencies (Model, ChatRepository, etc.)
// 3. Register controllers and services in your application setup
//
// See CLAUDE.md for complete configuration examples.
func NewModule() application.Module {
	return &Module{}
}

type Module struct{}

func (m *Module) Register(app application.Application) error {
	// Register BiChat applet for React app integration
	if err := app.RegisterApplet(&BiChatApplet{}); err != nil {
		return err
	}

	// Register database schema
	app.Migrations().RegisterSchema(&MigrationFiles)

	// Register translation files
	app.RegisterLocaleFiles(&LocaleFiles)

	return nil
}

func (m *Module) Name() string {
	return "bichat"
}
