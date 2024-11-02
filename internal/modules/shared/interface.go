package shared

import (
	"context"
	"github.com/a-h/templ"
	"github.com/gorilla/mux"
	"github.com/iota-agency/iota-erp/internal/app/services"
	"github.com/iota-agency/iota-erp/internal/domain/entities/permission"
)

type ControllerConstructor func(app *services.Application) Controller

type Controller interface {
	Register(r *mux.Router)
}

type NavigationItem struct {
	Name        string
	Href        string
	Children    []NavigationItem
	Icon        templ.Component
	Permissions []permission.Permission
}

type Module interface {
	Name() string
	Seed(ctx context.Context) error
	NavigationItems() []NavigationItem
	Controllers() []ControllerConstructor
	LocaleFiles() []string
}
