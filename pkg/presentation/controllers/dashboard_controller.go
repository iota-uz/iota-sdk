package controllers

import (
	"github.com/iota-agency/iota-sdk/pkg/application"
	"github.com/iota-agency/iota-sdk/pkg/middleware"
	"github.com/iota-agency/iota-sdk/pkg/presentation/templates/pages/dashboard"
	"github.com/iota-agency/iota-sdk/pkg/services"
	"github.com/iota-agency/iota-sdk/pkg/types"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/iota-agency/iota-sdk/pkg/composables"
)

func NewDashboardController(app application.Application) application.Controller {
	return &DashboardController{
		app: app,
	}
}

type DashboardController struct {
	app application.Application
}

func (c *DashboardController) Register(r *mux.Router) {
	router := r.Methods(http.MethodGet).Subrouter()
	router.Use(
		middleware.WithTransaction(),
		middleware.Authorize(c.app.Service(services.AuthService{}).(*services.AuthService)),
		middleware.RequireAuthorization(),
		middleware.ProvideUser(c.app.Service(services.UserService{}).(*services.UserService)),
		middleware.Tabs(c.app.Service(services.TabService{}).(*services.TabService)),
		middleware.WithLocalizer(c.app.Bundle()),
		middleware.NavItems(c.app),
	)
	router.HandleFunc("/", c.Get)
}

func (c *DashboardController) Get(w http.ResponseWriter, r *http.Request) {
	pageCtx, err := composables.UsePageCtx(r, types.NewPageData("Dashboard.Meta.Title", ""))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	props := &dashboard.IndexPageProps{
		PageContext: pageCtx,
	}
	if err := dashboard.Index(props).Render(r.Context(), w); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}
