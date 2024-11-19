package shared

import (
	"context"
	"github.com/benbjohnson/hashfs"
	"github.com/gorilla/mux"
	"github.com/iota-agency/iota-sdk/internal/application"
	"github.com/iota-agency/iota-sdk/internal/types"
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
	Assets() *hashfs.FS
	MigrationDirs() []string
	LocaleFiles() []string
}
