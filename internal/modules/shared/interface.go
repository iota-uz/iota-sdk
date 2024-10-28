package shared

import (
	"github.com/a-h/templ"
	"github.com/gorilla/mux"
	"github.com/iota-agency/iota-erp/internal/domain/entities/permission"
)

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
	NavigationItems() []NavigationItem
	Controllers() []Controller
	LocaleFiles() []string
}
