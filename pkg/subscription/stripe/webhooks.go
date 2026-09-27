package stripe

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/iota-uz/iota-sdk/pkg/serrors"
	subrepo "github.com/iota-uz/iota-sdk/pkg/subscription/repository"
	"github.com/sirupsen/logrus"
	"github.com/stripe/stripe-go/v82"
)

func (s *Service) HandleStripeEvent(ctx context.Context, event stripe.Event) error {
	const op serrors.Op = "SubscriptionStripeService.HandleStripeEvent"
	total := s.webhookSeen.Add(1)
	logrus.WithFields(logrus.Fields{
		"event_id":             event.ID,
		"event_type":           event.Type,
		"webhook_events_total": total,
	}).Info("Subscription Stripe webhook received")

	if event.ID != "" {
		fresh, err := s.repo.TryMarkWebhookEventProcessed(ctx, event.ID, string(event.Type), 24*time.Hour)
		if err != nil {
			return serrors.E(op, err)
		}
		if !fresh {
			dedupTotal := s.webhookDup.Add(1)
			logrus.WithFields(logrus.Fields{
				"event_id":               event.ID,
				"event_type":             event.Type,
				"webhook_dedup_hits":     dedupTotal,
				"subscription_component": "stripe_sync",
			}).Info("Duplicate Stripe webhook skipped")
			return nil
		}
	}

	switch string(event.Type) {
	case "entitlements.active_entitlement_summary.updated":
		if err := s.handleEntitlementSummaryUpdated(ctx, event); err != nil {
			return serrors.E(op, err)
		}
	case "customer.subscription.created", "customer.subscription.updated", "customer.subscription.deleted":
		if err := s.handleSubscriptionEvent(ctx, event); err != nil {
			return serrors.E(op, err)
		}
	case "invoice.payment_succeeded":
		if err := s.handleInvoicePaymentSucceeded(ctx, event); err != nil {
			return serrors.E(op, err)
		}
	case "invoice.payment_failed":
		if err := s.handleInvoicePaymentFailed(ctx, event); err != nil {
			return serrors.E(op, err)
		}
	}

	return nil
}

func (s *Service) handleEntitlementSummaryUpdated(ctx context.Context, event stripe.Event) error {
	const op serrors.Op = "SubscriptionStripeService.handleEntitlementSummaryUpdated"

	var summary stripe.EntitlementsActiveEntitlementSummary
	if err := json.Unmarshal(event.Data.Raw, &summary); err != nil {
		return serrors.E(op, err)
	}
	if summary.Customer == "" {
		return nil
	}
	tenantID, err := s.repo.FindTenantByStripeCustomer(ctx, summary.Customer)
	if err != nil {
		if errors.Is(err, subrepo.ErrEntitlementNotFound) {
			return nil
		}
		return serrors.E(op, err)
	}
	return s.RefreshTenant(ctx, tenantID)
}

func (s *Service) handleSubscriptionEvent(ctx context.Context, event stripe.Event) error {
	const op serrors.Op = "SubscriptionStripeService.handleSubscriptionEvent"

	var sub stripe.Subscription
	if err := json.Unmarshal(event.Data.Raw, &sub); err != nil {
		return serrors.E(op, err)
	}

	customerID := ""
	if sub.Customer != nil {
		customerID = sub.Customer.ID
	}
	tenantID, err := s.resolveTenantID(ctx, sub.Metadata, customerID, sub.ID)
	if err != nil {
		if errors.Is(err, subrepo.ErrEntitlementNotFound) {
			return nil
		}
		return serrors.E(op, err)
	}

	if err := s.ensureEntitlement(ctx, tenantID); err != nil {
		return serrors.E(op, err)
	}

	if err := s.updateStripeRefs(ctx, tenantID, customerID, sub.ID); err != nil {
		if !errors.Is(err, subrepo.ErrEntitlementNotFound) {
			return serrors.E(op, err)
		}
	}
	if planID, ok := sub.Metadata["plan_id"]; ok && planID != "" {
		if err := s.repo.SetPlan(ctx, tenantID, planID); err != nil {
			if !errors.Is(err, subrepo.ErrEntitlementNotFound) {
				return serrors.E(op, err)
			}
		}
	}

	switch string(event.Type) {
	case "customer.subscription.deleted":
		if err := s.setGracePeriod(ctx, tenantID, true); err != nil {
			return serrors.E(op, err)
		}
	}

	return s.RefreshTenant(ctx, tenantID)
}

func (s *Service) handleInvoicePaymentSucceeded(ctx context.Context, event stripe.Event) error {
	const op serrors.Op = "SubscriptionStripeService.handleInvoicePaymentSucceeded"

	var invoice stripe.Invoice
	if err := json.Unmarshal(event.Data.Raw, &invoice); err != nil {
		return serrors.E(op, err)
	}

	customerID := ""
	if invoice.Customer != nil {
		customerID = invoice.Customer.ID
	}
	subscriptionID := extractSubscriptionID(invoice)

	tenantID, err := s.resolveTenantID(ctx, invoice.Metadata, customerID, subscriptionID)
	if err != nil {
		if errors.Is(err, subrepo.ErrEntitlementNotFound) {
			return nil
		}
		return serrors.E(op, err)
	}

	if err := s.ensureEntitlement(ctx, tenantID); err != nil {
		return serrors.E(op, err)
	}

	if err := s.setGracePeriod(ctx, tenantID, false); err != nil {
		return serrors.E(op, err)
	}
	if err := s.updateStripeRefs(ctx, tenantID, customerID, subscriptionID); err != nil {
		if !errors.Is(err, subrepo.ErrEntitlementNotFound) {
			return serrors.E(op, err)
		}
	}
	return s.RefreshTenant(ctx, tenantID)
}

func (s *Service) handleInvoicePaymentFailed(ctx context.Context, event stripe.Event) error {
	const op serrors.Op = "SubscriptionStripeService.handleInvoicePaymentFailed"

	var invoice stripe.Invoice
	if err := json.Unmarshal(event.Data.Raw, &invoice); err != nil {
		return serrors.E(op, err)
	}

	customerID := ""
	if invoice.Customer != nil {
		customerID = invoice.Customer.ID
	}
	subscriptionID := extractSubscriptionID(invoice)

	tenantID, err := s.resolveTenantID(ctx, invoice.Metadata, customerID, subscriptionID)
	if err != nil {
		if errors.Is(err, subrepo.ErrEntitlementNotFound) {
			return nil
		}
		return serrors.E(op, err)
	}

	if err := s.ensureEntitlement(ctx, tenantID); err != nil {
		return serrors.E(op, err)
	}

	if err := s.setGracePeriod(ctx, tenantID, true); err != nil {
		return serrors.E(op, err)
	}
	if err := s.updateStripeRefs(ctx, tenantID, customerID, subscriptionID); err != nil {
		if !errors.Is(err, subrepo.ErrEntitlementNotFound) {
			return serrors.E(op, err)
		}
	}
	return s.RefreshTenant(ctx, tenantID)
}

func extractSubscriptionID(invoice stripe.Invoice) string {
	if invoice.Parent != nil &&
		invoice.Parent.SubscriptionDetails != nil &&
		invoice.Parent.SubscriptionDetails.Subscription != nil &&
		invoice.Parent.SubscriptionDetails.Subscription.ID != "" {
		return invoice.Parent.SubscriptionDetails.Subscription.ID
	}
	if invoice.Lines == nil || len(invoice.Lines.Data) == 0 {
		return ""
	}
	for _, line := range invoice.Lines.Data {
		if line.Parent == nil || line.Parent.SubscriptionItemDetails == nil {
			continue
		}
		if line.Parent.SubscriptionItemDetails.Subscription != "" {
			return line.Parent.SubscriptionItemDetails.Subscription
		}
	}
	return ""
}
