package controllers

import (
	"net/http"

	"github.com/a-h/templ"
	"github.com/iota-uz/iota-sdk/modules/core/presentation/templates/pages/dashboard"
	"github.com/iota-uz/iota-sdk/pkg/application"
	"github.com/iota-uz/iota-sdk/pkg/middleware"

	"github.com/gorilla/mux"
)

func NewDashboardController(app application.Application) application.Controller {
	return &DashboardController{
		app: app,
	}
}

type DashboardController struct {
	app application.Application
}

func (c *DashboardController) Key() string {
	return "/"
}

func (c *DashboardController) Register(r *mux.Router) {
	router := r.Methods(http.MethodGet).Subrouter()
	router.Use(
		middleware.Authorize(),
		middleware.RedirectNotAuthenticated(),
		middleware.ProvideUser(),
		middleware.Tabs(),
		middleware.ProvideLocalizer(c.app.Bundle()),
		middleware.NavItems(),
		middleware.WithPageContext(),
	)
	router.HandleFunc("/", c.Get)
}

func (c *DashboardController) Get(w http.ResponseWriter, r *http.Request) {
	props := &dashboard.IndexPageProps{}
	templ.Handler(dashboard.Index(props)).ServeHTTP(w, r)
}
