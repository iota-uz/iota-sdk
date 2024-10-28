package controllers

import (
	"github.com/iota-agency/iota-erp/internal/modules/shared"
	"net/http"

	"github.com/a-h/templ"
	"github.com/gorilla/mux"
	"github.com/iota-agency/iota-erp/internal/app/services"
	"github.com/iota-agency/iota-erp/internal/presentation/templates/pages/home"
	"github.com/iota-agency/iota-erp/pkg/composables"
)

type HomeController struct {
	app *services.Application
}

func NewHomeController(app *services.Application) shared.Controller {
	return &HomeController{
		app: app,
	}
}

func (c *HomeController) Register(r *mux.Router) {
	r.HandleFunc("/", c.Home).Methods(http.MethodGet)
	//router := r.Methods(http.MethodGet).Subrouter()
	//router.HandleFunc("", c.Home)
	//router.Use(middleware.RequireAuthorization())
}

func (c *HomeController) Home(w http.ResponseWriter, r *http.Request) {
	pageCtx, err := composables.UsePageCtx(
		r, composables.NewPageData("Home.Meta.Title", ""),
	)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	templ.Handler(home.Index(pageCtx), templ.WithStreaming()).ServeHTTP(w, r)
}
