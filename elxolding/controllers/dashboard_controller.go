package controllers

import (
	"github.com/iota-agency/iota-sdk/elxolding/services"
	"github.com/iota-agency/iota-sdk/elxolding/templates/pages/dashboard"
	"github.com/iota-agency/iota-sdk/internal/application"
	"github.com/iota-agency/iota-sdk/internal/modules/shared"
	"github.com/iota-agency/iota-sdk/internal/modules/shared/middleware"
	"github.com/iota-agency/iota-sdk/internal/types"
	"net/http"
	"strconv"

	"github.com/gorilla/mux"
	"github.com/iota-agency/iota-sdk/pkg/composables"
)

func NewDashboardController(app *application.Application) shared.Controller {
	return &DashboardController{
		app:              app,
		dashboardService: app.Service(services.DashboardService{}).(*services.DashboardService),
	}
}

type DashboardController struct {
	app              *application.Application
	dashboardService *services.DashboardService
}

func (c *DashboardController) Register(r *mux.Router) {
	router := r.Methods(http.MethodGet).Subrouter()
	router.Use(middleware.RequireAuthorization())
	router.HandleFunc("/", c.Get)
}

func (c *DashboardController) Get(w http.ResponseWriter, r *http.Request) {
	pageCtx, err := composables.UsePageCtx(r, types.NewPageData("Dashboard.Meta.Title", ""))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	stats, err := c.dashboardService.GetStats(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	props := &dashboard.IndexPageProps{
		PageContext:    pageCtx,
		PositionsCount: strconv.FormatInt(stats.PositionsCount, 10),
		ProductsCount:  strconv.FormatInt(stats.ProductsCount, 10),
		Depth:          strconv.FormatFloat(stats.Depth, 'f', 2, 64),
		OrdersCount:    strconv.FormatInt(stats.OrdersCount, 10),
	}
	if err := dashboard.Index(props).Render(r.Context(), w); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}
