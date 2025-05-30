package controllers

import (
	"context"
	"encoding/json"
	"github.com/gorilla/mux"
	"github.com/iota-uz/iota-sdk/modules/billing/domain/aggregates/billing"
	"github.com/iota-uz/iota-sdk/modules/billing/domain/aggregates/details"
	"github.com/iota-uz/iota-sdk/modules/billing/services"
	"github.com/iota-uz/iota-sdk/pkg/application"
	"github.com/iota-uz/iota-sdk/pkg/configuration"
	"github.com/stripe/stripe-go/v82"
	"github.com/stripe/stripe-go/v82/webhook"
	"io"
	"log"
	"net/http"
	"strings"
	"sync"
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
) *StripeController {
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
	router.HandleFunc("", c.Handle).Methods(http.MethodPost)
}

func (c *StripeController) Key() string {
	return c.basePath
}

func (c *StripeController) Handle(w http.ResponseWriter, r *http.Request) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	const MaxBodyBytes = int64(65536)
	r.Body = http.MaxBytesReader(w, r.Body, MaxBodyBytes)

	payload, err := io.ReadAll(r.Body)
	if err != nil {
		log.Printf("Error reading body: %v", err)
		w.WriteHeader(http.StatusServiceUnavailable)
		return
	}

	sigHeader := r.Header.Get("Stripe-Signature")
	event, err := webhook.ConstructEvent(payload, sigHeader, c.stripe.SigningSecret)
	if err != nil {
		log.Printf("Signature verification failed: %v", err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	ctx := r.Context()
	switch string(event.Type) {
	case "checkout.session.completed":
		c.handleCheckoutCompleted(ctx, event)
	case "invoice.created":
		c.handleInvoiceCreated(ctx, event)
	case "invoice.payment_succeeded":
		c.invoicePaymentSucceeded(ctx, event)
	case "invoice.payment_failed":
		c.handleInvoicePaymentFailed(ctx, event)
	default:
		log.Printf("Unhandled event type: %s", event.Type)
	}

	w.WriteHeader(http.StatusOK)
}

func (c *StripeController) handleCheckoutCompleted(ctx context.Context, event stripe.Event) {
	var session stripe.CheckoutSession
	if err := json.Unmarshal(event.Data.Raw, &session); err != nil {
		log.Printf("Failed to parse checkout session: %v", err)
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
		log.Printf("Failed to find transaction by session ID: %v", err)
		return
	}

	entity := entities[0]

	stripeDetails, ok := entity.Details().(details.StripeDetails)
	if !ok {
		log.Printf("Details is not of type StripeDetails")
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

	if _, err := c.billingService.Update(ctx, entity); err != nil {
		log.Printf("Failed to update transaction after checkout completed: %v", err)
		return
	}

	log.Printf("Transaction updated from checkout.session.completed: %s", session.ID)
}

func (c *StripeController) handleInvoiceCreated(ctx context.Context, event stripe.Event) {
	var invoice stripe.Invoice
	if err := json.Unmarshal(event.Data.Raw, &invoice); err != nil {
		log.Printf("Failed to parse invoice: %v", err)
		return
	}

	if invoice.BillingReason == stripe.InvoiceBillingReasonSubscriptionCreate {
		log.Println("Skipping invoice.created for subscription_create")
		return
	}

	if invoice.Customer == nil {
		log.Printf("Missing customer in invoice: %s", invoice.ID)
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
		log.Printf("Subscription ID not found in invoice lines: %s", invoice.ID)
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
		log.Printf("Could not find previous transaction by subscription_id: %v", err)
		return
	}

	prevEntity := entities[0]
	prevDetails, ok := prevEntity.Details().(details.StripeDetails)
	if !ok {
		log.Printf("Previous details is not of type StripeDetails")
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

	if _, err := c.billingService.Update(ctx, entity); err != nil {
		log.Printf("Failed to update transaction after checkout completed: %v", err)
		return
	}
}

func (c *StripeController) invoicePaymentSucceeded(ctx context.Context, event stripe.Event) {
	var invoice stripe.Invoice
	if err := json.Unmarshal(event.Data.Raw, &invoice); err != nil {
		log.Printf("Failed to parse invoice: %v", err)
		return
	}

	if invoice.ID == "" {
		log.Printf("Missing invoice ID in invoice.payment_succeeded")
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
		log.Printf("Could not find transaction by invoice_id: %v", err)
		return
	}

	entity := entities[0]

	stripeDetails, ok := entity.Details().(details.StripeDetails)
	if !ok {
		log.Printf("Details is not of type StripeDetails")
		return
	}

	stripeDetails = stripeDetails.
		SetInvoiceID(invoice.ID).
		SetBillingReason(string(invoice.BillingReason))

	if invoice.Customer != nil {
		stripeDetails = stripeDetails.SetCustomerID(invoice.Customer.ID)
	}

	entity = entity.
		SetStatus(billing.Completed).
		SetDetails(stripeDetails)

	if _, err := c.billingService.Update(ctx, entity); err != nil {
		log.Printf("Failed to update transaction on invoice.payment_succeeded: %v", err)
		return
	}

	log.Printf("Transaction marked as paid: invoice %s", invoice.ID)
}

func (c *StripeController) handleInvoicePaymentFailed(ctx context.Context, event stripe.Event) {
	var invoice stripe.Invoice
	if err := json.Unmarshal(event.Data.Raw, &invoice); err != nil {
		log.Printf("Failed to parse invoice: %v", err)
		return
	}

	if invoice.ID == "" {
		log.Printf("Missing invoice ID in invoice.payment_failed")
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
		log.Printf("Could not find transaction by invoice_id: %v", err)
		return
	}

	entity := entities[0]

	stripeDetails, ok := entity.Details().(details.StripeDetails)
	if !ok {
		log.Printf("Details is not of type StripeDetails")
		return
	}

	stripeDetails = stripeDetails.
		SetInvoiceID(invoice.ID).
		SetBillingReason(string(invoice.BillingReason))

	if invoice.Customer != nil {
		stripeDetails = stripeDetails.SetCustomerID(invoice.Customer.ID)
	}

	entity = entity.
		SetStatus(billing.Failed).
		SetDetails(stripeDetails)

	if _, err := c.billingService.Update(ctx, entity); err != nil {
		log.Printf("Failed to update transaction on invoice.payment_failed: %v", err)
		return
	}

	log.Printf("Transaction marked as failed: invoice %s", invoice.ID)
}
