package controllers

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"sync"

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
	mutex          sync.Mutex
}

func NewStripeController(
	app application.Application,
	stripe configuration.StripeOptions,
	basePath string,
) application.Controller {
	return &StripeController{
		app:            app,
		billingService: app.Service(services.BillingService{}).(*services.BillingService),
		stripe:         stripe,
		basePath:       basePath,
		mutex:          sync.Mutex{},
	}
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
	c.mutex.Lock()
	defer c.mutex.Unlock()

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

	switch string(event.Type) {
	case "checkout.session.completed":
		c.handleCheckoutCompleted(ctx, event, logger)
	case "invoice.created":
		c.handleInvoiceCreated(ctx, event, logger)
	case "invoice.payment_succeeded":
		c.invoicePaymentSucceeded(ctx, event, logger)
	case "invoice.payment_failed":
		c.handleInvoicePaymentFailed(ctx, event, logger)
	default:
		logger.WithField("event_type", event.Type).Info("Unhandled Stripe event type")
	}

	w.WriteHeader(http.StatusOK)
}

func (c *StripeController) handleCheckoutCompleted(ctx context.Context, event stripe.Event, logger *logrus.Entry) {
	var session stripe.CheckoutSession
	if err := json.Unmarshal(event.Data.Raw, &session); err != nil {
		logger.WithError(err).Error("Failed to parse checkout session")
		return
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
		return
	}

	entity := entities[0]

	stripeDetails, ok := entity.Details().(details.StripeDetails)
	if !ok {
		logger.Error("Details is not of type StripeDetails")
		return
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

	if _, err := c.billingService.Save(ctx, entity); err != nil {
		logger.WithError(err).WithField("session_id", session.ID).Error("Failed to update transaction after checkout completed")
		return
	}

	logger.WithField("session_id", session.ID).Info("Transaction updated from checkout.session.completed")
}

func (c *StripeController) handleInvoiceCreated(ctx context.Context, event stripe.Event, logger *logrus.Entry) {
	var invoice stripe.Invoice
	if err := json.Unmarshal(event.Data.Raw, &invoice); err != nil {
		logger.WithError(err).Error("Failed to parse invoice")
		return
	}

	if invoice.BillingReason == stripe.InvoiceBillingReasonSubscriptionCreate {
		logger.WithField("invoice_id", invoice.ID).Info("Skipping invoice.created for subscription_create")
		return
	}

	if invoice.Customer == nil {
		logger.WithField("invoice_id", invoice.ID).Error("Missing customer in invoice")
		return
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
		return
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
		return
	}

	prevEntity := entities[0]
	prevDetails, ok := prevEntity.Details().(details.StripeDetails)
	if !ok {
		logger.Error("Previous details is not of type StripeDetails")
		return
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

	quantity := float64(invoice.AmountDue) / 100

	entity := billing.New(
		quantity,
		billing.Currency(strings.ToUpper(string(invoice.Currency))),
		billing.Stripe,
		stripeDetails,
		billing.WithTenantID(prevEntity.TenantID()),
	)

	if _, err := c.billingService.Save(ctx, entity); err != nil {
		logger.WithError(err).WithField("invoice_id", invoice.ID).Error("Failed to create transaction")
		return
	}

	logger.WithFields(logrus.Fields{
		"invoice_id":      invoice.ID,
		"subscription_id": subscriptionID,
	}).Info("Transaction created from invoice.created")
}

func (c *StripeController) invoicePaymentSucceeded(ctx context.Context, event stripe.Event, logger *logrus.Entry) {
	var invoice stripe.Invoice
	if err := json.Unmarshal(event.Data.Raw, &invoice); err != nil {
		logger.WithError(err).Error("Failed to parse invoice")
		return
	}

	if invoice.ID == "" {
		logger.Error("Missing invoice ID in invoice.payment_succeeded")
		return
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
		return
	}

	entity := entities[0]

	stripeDetails, ok := entity.Details().(details.StripeDetails)
	if !ok {
		logger.Error("Details is not of type StripeDetails")
		return
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

	if _, err := c.billingService.Save(ctx, entity); err != nil {
		logger.WithError(err).WithField("invoice_id", invoice.ID).Error("Failed to update transaction on invoice.payment_succeeded")
		return
	}

	logger.WithFields(logrus.Fields{
		"invoice_id": invoice.ID,
		"old_status": oldStatus,
		"new_status": billing.Completed,
	}).Info("Transaction marked as paid")
}

func (c *StripeController) handleInvoicePaymentFailed(ctx context.Context, event stripe.Event, logger *logrus.Entry) {
	var invoice stripe.Invoice
	if err := json.Unmarshal(event.Data.Raw, &invoice); err != nil {
		logger.WithError(err).Error("Failed to parse invoice")
		return
	}

	if invoice.ID == "" {
		logger.Error("Missing invoice ID in invoice.payment_failed")
		return
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
		return
	}

	entity := entities[0]

	stripeDetails, ok := entity.Details().(details.StripeDetails)
	if !ok {
		logger.Error("Details is not of type StripeDetails")
		return
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

	if _, err := c.billingService.Save(ctx, entity); err != nil {
		logger.WithError(err).WithField("invoice_id", invoice.ID).Error("Failed to update transaction on invoice.payment_failed")
		return
	}

	logger.WithFields(logrus.Fields{
		"invoice_id": invoice.ID,
		"old_status": oldStatus,
		"new_status": billing.Failed,
	}).Info("Transaction marked as failed")
}
