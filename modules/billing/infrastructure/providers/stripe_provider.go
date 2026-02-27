package providers

import (
	"context"
	"fmt"
	"math"

	"github.com/iota-uz/iota-sdk/modules/billing/domain/aggregates/billing"
	"github.com/iota-uz/iota-sdk/modules/billing/domain/aggregates/details"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
	"github.com/stripe/stripe-go/v82"
	"github.com/stripe/stripe-go/v82/checkout/session"
	"github.com/stripe/stripe-go/v82/refund"
	"github.com/stripe/stripe-go/v82/subscription"
)

type StripeConfig struct {
	SecretKey string
}

// NewStripeProvider creates a new Stripe provider with the given configuration.
func NewStripeProvider(
	config StripeConfig,
) billing.Provider {
	return &stripeProvider{
		config: config,
	}
}

type stripeProvider struct {
	config StripeConfig
}

// Gateway returns the Stripe gateway.
func (s *stripeProvider) Gateway() billing.Gateway {
	return billing.Stripe
}

// Create generates a Stripe checkout session.
func (s *stripeProvider) Create(_ context.Context, t billing.Transaction) (billing.Transaction, error) {
	const op serrors.Op = "stripeProvider.Create"
	stripe.Key = s.config.SecretKey

	stripeDetails, err := toStripeDetails(t.Details())
	if err != nil {
		return nil, serrors.E(op, err)
	}

	lineItems := make([]*stripe.CheckoutSessionLineItemParams, len(stripeDetails.Items()))
	for i, item := range stripeDetails.Items() {
		lineItems[i] = &stripe.CheckoutSessionLineItemParams{
			Price:    stripe.String(item.PriceID()),
			Quantity: stripe.Int64(item.Quantity()),
		}

		if item.AdjustableQuantity() != nil {
			lineItems[i].AdjustableQuantity = &stripe.CheckoutSessionLineItemAdjustableQuantityParams{
				Enabled: stripe.Bool(item.AdjustableQuantity().Enabled()),
				Minimum: stripe.Int64(item.AdjustableQuantity().Minimum()),
				Maximum: stripe.Int64(item.AdjustableQuantity().Maximum()),
			}
		}
	}

	params := &stripe.CheckoutSessionParams{
		SuccessURL:        stripe.String(stripeDetails.SuccessURL()),
		CancelURL:         stripe.String(stripeDetails.CancelURL()),
		ClientReferenceID: stripe.String(stripeDetails.ClientReferenceID()),
		Mode:              stripe.String(stripeDetails.Mode()),
		LineItems:         lineItems,
	}

	if stripeDetails.Mode() == "subscription" && stripeDetails.SubscriptionData() != nil {
		params.SubscriptionData = &stripe.CheckoutSessionSubscriptionDataParams{}

		if stripeDetails.SubscriptionData().Description() != "" {
			params.SubscriptionData.Description = stripe.String(stripeDetails.SubscriptionData().Description())
		}
		if stripeDetails.SubscriptionData().TrialPeriodDays() != 0 {
			params.SubscriptionData.TrialPeriodDays = stripe.Int64(stripeDetails.SubscriptionData().TrialPeriodDays())
		}
	}

	sess, err := session.New(params)
	if err != nil {
		return nil, serrors.E(op, serrors.Internal, err)
	}

	stripeDetails = stripeDetails.
		SetSessionID(sess.ID).
		SetURL(sess.URL)

	if sess.PaymentIntent != nil {
		stripeDetails = stripeDetails.SetPaymentIntentID(sess.PaymentIntent.ID)
	}

	t = t.SetDetails(stripeDetails)

	return t, nil
}

// Cancel expires a Stripe session or cancels a subscription.
func (s *stripeProvider) Cancel(_ context.Context, t billing.Transaction) (billing.Transaction, error) {
	const op serrors.Op = "stripeProvider.Cancel"
	stripe.Key = s.config.SecretKey

	stripeDetails, err := toStripeDetails(t.Details())
	if err != nil {
		return nil, serrors.E(op, err)
	}

	if stripeDetails.SubscriptionID() != "" {
		params := &stripe.SubscriptionParams{
			CancelAtPeriodEnd: stripe.Bool(true),
		}
		_, err := subscription.Update(stripeDetails.SubscriptionID(), params)
		if err != nil {
			return nil, serrors.E(op, serrors.Internal, err)
		}
		return t.SetStatus(billing.Canceled), nil
	}

	if stripeDetails.SessionID() != "" {
		_, err := session.Expire(stripeDetails.SessionID(), nil)
		if err != nil {
			return nil, serrors.E(op, serrors.Internal, err)
		}
		return t.SetStatus(billing.Canceled), nil
	}

	return nil, serrors.E(op, serrors.Invalid, "cannot cancel: neither subscription_id nor session_id found in stripe details")
}

// Refund processes a full or partial refund for Stripe.
func (s *stripeProvider) Refund(_ context.Context, t billing.Transaction, amount float64) (billing.Transaction, error) {
	const op serrors.Op = "stripeProvider.Refund"
	stripe.Key = s.config.SecretKey

	stripeDetails, err := toStripeDetails(t.Details())
	if err != nil {
		return nil, serrors.E(op, err)
	}

	if stripeDetails.PaymentIntentID() == "" {
		return nil, serrors.E(op, serrors.Invalid, "cannot refund: payment_intent_id not found in stripe details")
	}

	if amount <= 0 {
		return nil, serrors.E(op, serrors.Invalid, "refund amount must be positive")
	}

	if amount > t.Amount().Quantity()+0.001 {
		return nil, serrors.E(op, serrors.Invalid, fmt.Sprintf("invalid refund amount: %f. Amount exceeds transaction total: %f", amount, t.Amount().Quantity()))
	}

	params := &stripe.RefundParams{
		PaymentIntent: stripe.String(stripeDetails.PaymentIntentID()),
		Amount:        stripe.Int64(int64(math.Round(amount * 100))),
	}

	_, err = refund.New(params)
	if err != nil {
		return nil, serrors.E(op, serrors.Internal, err)
	}

	if amount >= t.Amount().Quantity()-0.001 {
		return t.SetStatus(billing.Refunded), nil
	}

	return t.SetStatus(billing.PartiallyRefunded), nil
}

func toStripeDetails(detailsObj details.Details) (details.StripeDetails, error) {
	stripeDetails, ok := detailsObj.(details.StripeDetails)
	if !ok {
		return nil, serrors.E(serrors.Invalid, fmt.Sprintf("failed to cast details to StripeDetails: invalid type %T", detailsObj))
	}
	return stripeDetails, nil
}
