package controllers

import (
	"github.com/gorilla/mux"
	"net/http"

	"github.com/a-h/templ"
	"github.com/iota-agency/iota-erp/internal/app/services"
	"github.com/iota-agency/iota-erp/internal/presentation/templates/pages/home"
	"github.com/iota-agency/iota-erp/pkg/composables"
)

type HomeController struct {
	app *services.Application
}

func NewHomeController(app *services.Application) Controller {
	return &HomeController{
		app: app,
	}
}

func (c *HomeController) Register(r *mux.Router) {
	r.HandleFunc("/", c.Home).Methods(http.MethodGet)
}

func (c *HomeController) Home(w http.ResponseWriter, r *http.Request) {
	pageCtx, err := composables.UsePageCtx(r, &composables.PageData{
		Title: "Home.Meta.Title",
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	templ.Handler(home.Index(pageCtx), templ.WithStreaming()).ServeHTTP(w, r)
}
