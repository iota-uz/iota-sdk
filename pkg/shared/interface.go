package shared

import (
	"context"
	"embed"
	"github.com/gorilla/mux"
	"github.com/iota-agency/iota-sdk/pkg/application"
	"github.com/iota-agency/iota-sdk/pkg/types"
	"github.com/nicksnyder/go-i18n/v2/i18n"
)

type ControllerConstructor func(app *application.Application) Controller
type SeedFunc func(ctx context.Context, app *application.Application) error

type Controller interface {
	Register(r *mux.Router)
}

type Module interface {
	Name() string
	Seed(ctx context.Context, app *application.Application) error
	Register(app *application.Application) error
	NavigationItems(localizer *i18n.Localizer) []types.NavigationItem
	Controllers() []ControllerConstructor
	Assets() *embed.FS
	MigrationDirs() *embed.FS
	LocaleFiles() *embed.FS
}
