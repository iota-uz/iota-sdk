package upload

import (
	"context"
	"embed"

	"github.com/benbjohnson/hashfs"
	"github.com/iota-agency/iota-sdk/modules/upload/controllers"
	"github.com/iota-agency/iota-sdk/modules/upload/permissions"
	"github.com/iota-agency/iota-sdk/modules/upload/persistence"
	"github.com/iota-agency/iota-sdk/modules/upload/services"
	"github.com/iota-agency/iota-sdk/pkg/application"
	"github.com/iota-agency/iota-sdk/pkg/shared"
	"github.com/iota-agency/iota-sdk/pkg/types"
	"github.com/nicksnyder/go-i18n/v2/i18n"
)

//go:embed migrations/*.sql
var migrationFiles embed.FS

type Module struct {
	baseFilePath string
}

func NewModule(baseFilePath string) shared.Module {
	return &Module{
		baseFilePath: baseFilePath,
	}
}

func (m *Module) Register(app *application.Application) error {
	fsStorage, err := persistence.NewFSStorage(m.baseFilePath)
	if err != nil {
		return err
	}
	uploadService := services.NewUploadService(persistence.NewUploadRepository(), fsStorage, app.EventPublisher)
	app.RegisterService(uploadService)
	app.Rbac.Register(
		permissions.UploadRead,
		permissions.UploadCreate,
		permissions.UploadDelete,
		permissions.UploadUpdate,
	)

	return nil
}

func (m *Module) MigrationDirs() *embed.FS {
	return &migrationFiles
}

func (m *Module) Assets() *hashfs.FS {
	return nil
	// return assets.FS
}

func (m *Module) Seed(ctx context.Context, app *application.Application) error {
	return nil
}

func (m *Module) Name() string {
	return "upload"
}

func (m *Module) NavigationItems(localizer *i18n.Localizer) []types.NavigationItem {
	return []types.NavigationItem{}
}

func (m *Module) Controllers() []shared.ControllerConstructor {
	return []shared.ControllerConstructor{controllers.NewUploadController}
}

func (m *Module) LocaleFiles() *embed.FS {
	return nil
}
