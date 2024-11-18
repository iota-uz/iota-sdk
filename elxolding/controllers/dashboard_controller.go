package controllers

import (
	"github.com/iota-agency/iota-erp/elxolding/templates/pages/dashboard"
	"github.com/iota-agency/iota-erp/internal/application"
	"github.com/iota-agency/iota-erp/internal/modules/shared"
	"github.com/iota-agency/iota-erp/internal/services"
	"github.com/iota-agency/iota-erp/internal/types"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/iota-agency/iota-erp/pkg/composables"
)

func NewDashboardController(app *application.Application) shared.Controller {
	return &DashboardController{
		app:         app,
		userService: app.Service(services.UserService{}).(*services.UserService),
	}
}

type DashboardController struct {
	app         *application.Application
	userService *services.UserService
}

func (c *DashboardController) Register(r *mux.Router) {
	r.HandleFunc("/", c.Get).Methods(http.MethodGet)
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
