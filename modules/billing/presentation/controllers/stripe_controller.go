package controllers

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/gorilla/mux"
	"github.com/iota-uz/iota-sdk/modules/billing/domain/aggregates/billing"
	"github.com/iota-uz/iota-sdk/modules/billing/domain/aggregates/details"
	"github.com/iota-uz/iota-sdk/modules/billing/services"
	"github.com/iota-uz/iota-sdk/pkg/application"
	"github.com/iota-uz/iota-sdk/pkg/configuration"
	"github.com/iota-uz/iota-sdk/pkg/di"
	"github.com/sirupsen/logrus"
	"github.com/stripe/stripe-go/v82"
	"github.com/stripe/stripe-go/v82/webhook"
)

type StripeController struct {
	app            application.Application
	billingService *services.BillingService
	stripe         configuration.StripeOptions
	basePath       string
	hooks          []billing.StripeEventHook
	hookQueue      chan stripe.Event
}

func NewStripeController(
	app application.Application,
	stripeOpts configuration.StripeOptions,
	basePath string,
	hooks ...billing.StripeEventHook,
) application.Controller {
	controller := &StripeController{
		app:            app,
		billingService: app.Service(services.BillingService{}).(*services.BillingService),
		stripe:         stripeOpts,
		basePath:       basePath,
		hooks:          hooks,
		hookQueue:      make(chan stripe.Event, 64),
	}
	controller.startHookWorkers()
	return controller
}

func (c *StripeController) Register(r *mux.Router) {
	router := r.PathPrefix(c.basePath).Subrouter()
	router.HandleFunc("", di.H(c.Handle)).Methods(http.MethodPost)
}

func (c *StripeController) Key() string {
	return c.basePath
}

func (c *StripeController) Handle(
	r *http.Request,
	w http.ResponseWriter,
	logger *logrus.Entry,
) {
	logger.Info("Stripe webhook received")

	const MaxBodyBytes = int64(65536)
	r.Body = http.MaxBytesReader(w, r.Body, MaxBodyBytes)

	payload, err := io.ReadAll(r.Body)
	if err != nil {
		logger.WithError(err).Error("Error reading webhook body")
		w.WriteHeader(http.StatusServiceUnavailable)
		return
	}

	sigHeader := r.Header.Get("Stripe-Signature")
	event, err := webhook.ConstructEvent(payload, sigHeader, c.stripe.SigningSecret)
	if err != nil {
		logger.WithError(err).Error("Stripe signature verification failed")
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	ctx := r.Context()
	logger.WithField("event_type", event.Type).Info("Processing Stripe event")

	var handleErr error
	switch string(event.Type) {
	case "checkout.session.completed":
		handleErr = c.handleCheckoutCompleted(ctx, event, logger)
	case "invoice.created":
		handleErr = c.handleInvoiceCreated(ctx, event, logger)
	case "invoice.payment_succeeded":
		handleErr = c.invoicePaymentSucceeded(ctx, event, logger)
	case "invoice.payment_failed":
		handleErr = c.handleInvoicePaymentFailed(ctx, event, logger)
	default:
		logger.WithField("event_type", event.Type).Info("Unhandled Stripe event type")
	}
	c.dispatchHooksAsync(event, logger)

	if handleErr != nil {
		logger.WithError(handleErr).Error("Stripe webhook handler failed")
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

func (c *StripeController) dispatchHooksAsync(event stripe.Event, logger *logrus.Entry) {
	if len(c.hooks) == 0 {
		return
	}

	select {
	case c.hookQueue <- event:
		return
	default:
		logger.WithFields(logrus.Fields{
			"event_id":   event.ID,
			"event_type": event.Type,
		}).Warn("Stripe hook queue is full; dropping hook dispatch")
	}
}

func (c *StripeController) startHookWorkers() {
	if len(c.hooks) == 0 {
		return
	}

	const numWorkers = 8
	for i := 0; i < numWorkers; i++ {
		go func() {
			for evt := range c.hookQueue {
				c.runHookDispatch(evt)
			}
		}()
	}
}

func (c *StripeController) runHookDispatch(event stripe.Event) {
	logger := logrus.WithFields(logrus.Fields{
		"component":  "billing_stripe_webhook",
		"event_id":   event.ID,
		"event_type": event.Type,
	})
	defer func() {
		if rec := recover(); rec != nil {
			logger.WithFields(logrus.Fields{
				"event_type": event.Type,
				"panic":      rec,
			}).Error("Stripe hook dispatch panicked")
		}
	}()

	hookCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	c.dispatchHooks(hookCtx, event, logger)
}

func (c *StripeController) dispatchHooks(ctx context.Context, event stripe.Event, logger *logrus.Entry) {
	for idx, hook := range c.hooks {
		if hook == nil {
			continue
		}
		if err := hook.HandleStripeEvent(ctx, event); err != nil {
			logger.WithError(err).WithFields(logrus.Fields{
				"event_type": event.Type,
				"hook_index": idx,
			}).Warn("Stripe hook failed")
		}
	}
}

func (c *StripeController) handleCheckoutCompleted(ctx context.Context, event stripe.Event, logger *logrus.Entry) error {
	var session stripe.CheckoutSession
	if err := json.Unmarshal(event.Data.Raw, &session); err != nil {
		logger.WithError(err).Error("Failed to parse checkout session")
		return err
	}

	entities, err := c.billingService.GetByDetailsFields(
		ctx,
		billing.Stripe,
		[]billing.DetailsFieldFilter{
			{
				Path:     []string{"session_id"},
				Operator: billing.OpEqual,
				Value:    session.ID,
			},
		},
	)

	if err != nil || len(entities) != 1 {
		logger.WithError(err).WithField("session_id", session.ID).Error("Failed to find transaction by session ID")
		return err
	}

	entity := entities[0]

	stripeDetails, ok := entity.Details().(details.StripeDetails)
	if !ok {
		logger.Error("Details is not of type StripeDetails")
		return nil
	}

	if session.Customer != nil {
		stripeDetails = stripeDetails.SetCustomerID(session.Customer.ID)
	}
	if session.Subscription != nil {
		stripeDetails = stripeDetails.SetSubscriptionID(session.Subscription.ID)
	}
	if session.Invoice != nil {
		stripeDetails = stripeDetails.
			SetInvoiceID(session.Invoice.ID).
			SetBillingReason(string(session.Invoice.BillingReason))
	}

	entity = entity.
		SetStatus(billing.Completed).
		SetDetails(stripeDetails)

	entity, err = c.billingService.Save(ctx, entity)
	if err != nil {
		logger.WithError(err).WithField("session_id", session.ID).Error("Failed to update transaction after checkout completed")
		return err
	}

	// Invoke callback for notification (non-blocking)
	if err := c.billingService.InvokeCallback(ctx, entity); err != nil {
		logger.WithError(err).WithField("session_id", session.ID).Warn("Callback error on status change")
	}

	logger.WithField("session_id", session.ID).Info("Transaction updated from checkout.session.completed")
	return nil
}

func (c *StripeController) handleInvoiceCreated(ctx context.Context, event stripe.Event, logger *logrus.Entry) error {
	var invoice stripe.Invoice
	if err := json.Unmarshal(event.Data.Raw, &invoice); err != nil {
		logger.WithError(err).Error("Failed to parse invoice")
		return err
	}

	if invoice.BillingReason == stripe.InvoiceBillingReasonSubscriptionCreate {
		logger.WithField("invoice_id", invoice.ID).Info("Skipping invoice.created for subscription_create")
		return nil
	}

	if invoice.Customer == nil {
		logger.WithField("invoice_id", invoice.ID).Error("Missing customer in invoice")
		return nil
	}

	var subscriptionID string

	if invoice.Lines != nil && len(invoice.Lines.Data) > 0 {
		for _, line := range invoice.Lines.Data {
			if line.Parent != nil && line.Parent.SubscriptionItemDetails != nil {
				subscriptionID = line.Parent.SubscriptionItemDetails.Subscription
				break
			}
		}
	}

	if subscriptionID == "" {
		logger.WithField("invoice_id", invoice.ID).Error("Subscription ID not found in invoice lines")
		return nil
	}

	entities, err := c.billingService.GetByDetailsFields(ctx, billing.Stripe, []billing.DetailsFieldFilter{
		{
			Path:     []string{"subscription_id"},
			Operator: billing.OpEqual,
			Value:    subscriptionID,
		},
	})
	if err != nil || len(entities) == 0 {
		logger.WithError(err).WithField("subscription_id", subscriptionID).Error("Could not find previous transaction by subscription_id")
		return err
	}

	prevEntity := entities[0]
	prevDetails, ok := prevEntity.Details().(details.StripeDetails)
	if !ok {
		logger.Error("Previous details is not of type StripeDetails")
		return nil
	}

	clientRef := prevDetails.ClientReferenceID()

	stripeDetails := details.NewStripeDetails(clientRef).
		SetMode(prevDetails.Mode()).
		SetBillingReason(string(invoice.BillingReason)).
		SetInvoiceID(invoice.ID).
		SetSubscriptionID(subscriptionID).
		SetURL(invoice.HostedInvoiceURL)

	if invoice.Customer != nil {
		stripeDetails = stripeDetails.SetCustomerID(invoice.Customer.ID)
	}

	quantity := float64(invoice.AmountDue) / currencyDivisor(string(invoice.Currency))

	entity := billing.New(
		quantity,
		billing.Currency(strings.ToUpper(string(invoice.Currency))),
		billing.Stripe,
		stripeDetails,
		billing.WithTenantID(prevEntity.TenantID()),
	)

	if _, err := c.billingService.Save(ctx, entity); err != nil {
		logger.WithError(err).WithField("invoice_id", invoice.ID).Error("Failed to create transaction")
		return err
	}

	logger.WithFields(logrus.Fields{
		"invoice_id":      invoice.ID,
		"subscription_id": subscriptionID,
	}).Info("Transaction created from invoice.created")
	return nil
}

func (c *StripeController) invoicePaymentSucceeded(ctx context.Context, event stripe.Event, logger *logrus.Entry) error {
	var invoice stripe.Invoice
	if err := json.Unmarshal(event.Data.Raw, &invoice); err != nil {
		logger.WithError(err).Error("Failed to parse invoice")
		return err
	}

	if invoice.ID == "" {
		logger.Error("Missing invoice ID in invoice.payment_succeeded")
		return nil
	}

	entities, err := c.billingService.GetByDetailsFields(ctx, billing.Stripe, []billing.DetailsFieldFilter{
		{
			Path:     []string{"invoice_id"},
			Operator: billing.OpEqual,
			Value:    invoice.ID,
		},
	})
	if err != nil || len(entities) == 0 {
		logger.WithError(err).WithField("invoice_id", invoice.ID).Error("Could not find transaction by invoice_id")
		return err
	}

	entity := entities[0]

	stripeDetails, ok := entity.Details().(details.StripeDetails)
	if !ok {
		logger.Error("Details is not of type StripeDetails")
		return nil
	}

	stripeDetails = stripeDetails.
		SetInvoiceID(invoice.ID).
		SetBillingReason(string(invoice.BillingReason))

	if invoice.Customer != nil {
		stripeDetails = stripeDetails.SetCustomerID(invoice.Customer.ID)
	}

	oldStatus := entity.Status()
	entity = entity.
		SetStatus(billing.Completed).
		SetDetails(stripeDetails)

	entity, err = c.billingService.Save(ctx, entity)
	if err != nil {
		logger.WithError(err).WithField("invoice_id", invoice.ID).Error("Failed to update transaction on invoice.payment_succeeded")
		return err
	}

	// Invoke callback for notification (non-blocking)
	if err := c.billingService.InvokeCallback(ctx, entity); err != nil {
		logger.WithError(err).WithField("invoice_id", invoice.ID).Warn("Callback error on status change")
	}

	logger.WithFields(logrus.Fields{
		"invoice_id": invoice.ID,
		"old_status": oldStatus,
		"new_status": billing.Completed,
	}).Info("Transaction marked as paid")
	return nil
}

func (c *StripeController) handleInvoicePaymentFailed(ctx context.Context, event stripe.Event, logger *logrus.Entry) error {
	var invoice stripe.Invoice
	if err := json.Unmarshal(event.Data.Raw, &invoice); err != nil {
		logger.WithError(err).Error("Failed to parse invoice")
		return err
	}

	if invoice.ID == "" {
		logger.Error("Missing invoice ID in invoice.payment_failed")
		return nil
	}

	entities, err := c.billingService.GetByDetailsFields(ctx, billing.Stripe, []billing.DetailsFieldFilter{
		{
			Path:     []string{"invoice_id"},
			Operator: billing.OpEqual,
			Value:    invoice.ID,
		},
	})
	if err != nil || len(entities) == 0 {
		logger.WithError(err).WithField("invoice_id", invoice.ID).Error("Could not find transaction by invoice_id")
		return err
	}

	entity := entities[0]

	stripeDetails, ok := entity.Details().(details.StripeDetails)
	if !ok {
		logger.Error("Details is not of type StripeDetails")
		return nil
	}

	stripeDetails = stripeDetails.
		SetInvoiceID(invoice.ID).
		SetBillingReason(string(invoice.BillingReason))

	if invoice.Customer != nil {
		stripeDetails = stripeDetails.SetCustomerID(invoice.Customer.ID)
	}

	oldStatus := entity.Status()
	entity = entity.
		SetStatus(billing.Failed).
		SetDetails(stripeDetails)

	entity, err = c.billingService.Save(ctx, entity)
	if err != nil {
		logger.WithError(err).WithField("invoice_id", invoice.ID).Error("Failed to update transaction on invoice.payment_failed")
		return err
	}

	// Invoke callback for notification (non-blocking)
	if err := c.billingService.InvokeCallback(ctx, entity); err != nil {
		logger.WithError(err).WithField("invoice_id", invoice.ID).Warn("Callback error on status change")
	}

	logger.WithFields(logrus.Fields{
		"invoice_id": invoice.ID,
		"old_status": oldStatus,
		"new_status": billing.Failed,
	}).Info("Transaction marked as failed")
	return nil
}

// currencyDivisor returns the smallest currency unit divisor.
// Zero-decimal currencies (e.g. JPY, KRW) use 1; most others use 100.
func currencyDivisor(currency string) float64 {
	switch strings.ToUpper(currency) {
	case "BIF", "CLP", "DJF", "GNF", "JPY", "KMF", "KRW", "MGA",
		"PYG", "RWF", "UGX", "VND", "VUV", "XAF", "XOF", "XPF":
		return 1
	default:
		return 100
	}
}
