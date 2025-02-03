package controllers

import (
	"net/http"

	"github.com/gorilla/mux"
	cpassproviders "github.com/iota-uz/iota-sdk/modules/crm/infrastructure/cpass-providers"
	"github.com/iota-uz/iota-sdk/pkg/application"
	"github.com/iota-uz/iota-sdk/pkg/middleware"
)

func NewTwilioController(
	app application.Application,
	twilioProvider cpassproviders.Provider,
) *TwillioController {
	return &TwillioController{
		app:            app,
		twilioProvider: twilioProvider,
	}
}

type TwillioController struct {
	app            application.Application
	twilioProvider cpassproviders.Provider
}

func (c *TwillioController) Register(r *mux.Router) {
	subRouter := r.PathPrefix("/twilio").Subrouter()
	subRouter.Use(
		middleware.WithTransaction(),
	)
	webhookHandler := c.twilioProvider.WebhookHandler(c.app.EventPublisher())
	subRouter.HandleFunc("", webhookHandler).Methods(http.MethodPost)
}

func (c *TwillioController) Key() string {
	return "TwillioController"
}
