package controllers

import (
	"context"
	"net/http"

	"github.com/gorilla/mux"
	cpassproviders "github.com/iota-uz/iota-sdk/modules/crm/infrastructure/cpass-providers"
	"github.com/iota-uz/iota-sdk/pkg/application"
	"github.com/iota-uz/iota-sdk/pkg/composables"
)

func NewTwilioController(
	app application.Application,
	twilioProvider *cpassproviders.TwilioProvider,
) *TwillioController {
	return &TwillioController{
		app:            app,
		twilioProvider: twilioProvider,
	}
}

type TwillioController struct {
	app            application.Application
	twilioProvider *cpassproviders.TwilioProvider
}

func (c *TwillioController) Register(r *mux.Router) {
	subRouter := r.PathPrefix("/twilio").Subrouter()
	webhookHandler := c.twilioProvider.WebhookHandler(c.app.EventPublisher())
	
	// Wrap the webhook handler with transaction support
	wrappedHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		err := composables.InTx(r.Context(), func(txCtx context.Context) error {
			// Create a new request with the transaction context
			rWithTx := r.WithContext(txCtx)
			// Use a custom ResponseWriter that captures the status
			// to determine if we should commit or rollback
			webhookHandler(w, rWithTx)
			return nil
		})
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	})
	
	subRouter.HandleFunc("", wrappedHandler).Methods(http.MethodPost)
}

func (c *TwillioController) Key() string {
	return "TwillioController"
}
