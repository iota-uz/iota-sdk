package controllers

import (
	"encoding/json"
	"github.com/gorilla/mux"
	"github.com/iota-uz/iota-sdk/modules/billing/services"
	"github.com/iota-uz/iota-sdk/pkg/application"
	"github.com/iota-uz/iota-sdk/pkg/configuration"
	"log"
	"net/http"
)

type OctoController struct {
	app            application.Application
	billingService *services.BillingService
	octo           configuration.OctoOptions
	basePath       string
}

func NewOctoController(
	app application.Application,
	octo configuration.OctoOptions,
	basePath string,
) application.Controller {
	return &OctoController{
		app:            app,
		billingService: app.Service(services.BillingService{}).(*services.BillingService),
		octo:           octo,
		basePath:       basePath,
	}
}

func (c *OctoController) Register(r *mux.Router) {
	router := r.PathPrefix(c.basePath).Subrouter()
	router.HandleFunc("", c.Handle).Methods(http.MethodPost)
}

func (c *OctoController) Key() string {
	return c.basePath
}

func (c *OctoController) Handle(w http.ResponseWriter, r *http.Request) {
	var body interface{}

	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}

	log.Printf("Received JSON: %+v\n", body)

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(map[string]string{"status": "ok"}); err != nil {
		log.Printf("failed to write response: %v", err)
	}
}
