package application

import (
	"context"
	"github.com/gorilla/mux"
	"github.com/iota-agency/iota-sdk/pkg/types"
	"github.com/nicksnyder/go-i18n/v2/i18n"
)

type ControllerConstructor func(app *ApplicationImpl) Controller
type SeedFunc func(ctx context.Context, app *ApplicationImpl) error

type Controller interface {
	Register(r *mux.Router)
}

type Module interface {
	Name() string
	Seed(ctx context.Context, app Application) error
	Register(app Application) error
	NavigationItems(localizer *i18n.Localizer) []types.NavigationItem
	//Controllers() []ControllerConstructor
	//Assets() *embed.FS
	//Templates() *embed.FS
	//MigrationDirs() *embed.FS
	//LocaleFiles() *embed.FS
}
