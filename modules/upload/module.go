package upload

import (
	"context"
	"embed"

	"github.com/iota-agency/iota-sdk/modules/upload/controllers"
	"github.com/iota-agency/iota-sdk/modules/upload/permissions"
	"github.com/iota-agency/iota-sdk/modules/upload/persistence"
	"github.com/iota-agency/iota-sdk/modules/upload/services"
	"github.com/iota-agency/iota-sdk/pkg/application"
	"github.com/iota-agency/iota-sdk/pkg/types"
	"github.com/nicksnyder/go-i18n/v2/i18n"
)

//go:embed migrations/*.sql
var migrationFiles embed.FS

type Module struct{}

func NewModule() application.Module {
	return &Module{}
}

func (m *Module) Register(app application.Application) error {
	fsStorage, err := persistence.NewFSStorage()
	if err != nil {
		return err
	}
	uploadService := services.NewUploadService(
		persistence.NewUploadRepository(),
		fsStorage,
		app.EventPublisher(),
	)
	app.RegisterService(uploadService)
	app.RegisterMigrationDirs(&migrationFiles)
	app.RegisterPermissions(
		permissions.UploadRead,
		permissions.UploadCreate,
		permissions.UploadUpdate,
		permissions.UploadDelete,
	)
	app.RegisterControllers(
		controllers.NewUploadController(app),
	)
	return nil
}

func (m *Module) Seed(ctx context.Context, app application.Application) error {
	return nil
}

func (m *Module) Name() string {
	return "upload"
}

func (m *Module) NavigationItems(localizer *i18n.Localizer) []types.NavigationItem {
	return []types.NavigationItem{}
}
