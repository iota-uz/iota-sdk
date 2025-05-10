package controllers

import (
	"net/http"

	"github.com/gorilla/mux"
	"github.com/iota-uz/iota-sdk/pkg/application"
)

func NewHealthController(app application.Application) application.Controller {
	return &HealthController{
		app: app,
	}
}

type HealthController struct {
	app application.Application
}

func (c *HealthController) Key() string {
	return "/health"
}

func (c *HealthController) Register(r *mux.Router) {
	router := r.Methods(http.MethodGet).Subrouter()
	router.HandleFunc("/health", c.Get)
}

func (c *HealthController) Get(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status":"ok"}`))
}
