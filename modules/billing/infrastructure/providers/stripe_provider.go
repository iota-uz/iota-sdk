package providers

import (
	"context"
	"fmt"
	"github.com/iota-uz/iota-sdk/modules/billing/domain/aggregates/billing"
	"github.com/iota-uz/iota-sdk/modules/billing/domain/aggregates/details"
	"github.com/stripe/stripe-go/v82"
	"github.com/stripe/stripe-go/v82/checkout/session"
)

type StripeConfig struct {
	SecretKey string
}

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

func (s *stripeProvider) Gateway() billing.Gateway {
	return billing.Stripe
}

func (s *stripeProvider) Create(_ context.Context, t billing.Transaction) (billing.Transaction, error) {
	stripe.Key = s.config.SecretKey

	stripeDetails, err := toStripeDetails(t.Details())
	if err != nil {
		return nil, err
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
		return nil, err
	}

	stripeDetails = stripeDetails.
		SetSessionID(sess.ID).
		SetURL(sess.URL)

	t = t.SetDetails(stripeDetails)

	return t, nil
}

func (s *stripeProvider) Cancel(ctx context.Context, tx billing.Transaction) (billing.Transaction, error) {
	//TODO implement me
	panic("implement me")
}

func (s *stripeProvider) Refund(ctx context.Context, tx billing.Transaction, quantity float64) (billing.Transaction, error) {
	//TODO implement me
	panic("implement me")
}

func toStripeDetails(detailsObj details.Details) (details.StripeDetails, error) {
	stripeDetails, ok := detailsObj.(details.StripeDetails)
	if !ok {
		return nil, fmt.Errorf("failed to cast details to StripeDetails: invalid type %T", detailsObj)
	}
	return stripeDetails, nil
}
