package controllers

import (
	"context"

	"github.com/stripe/stripe-go/v82"
)

// StripeEventHook allows non-billing components to observe raw Stripe events
// received by the billing webhook controller.
type StripeEventHook interface {
	HandleStripeEvent(ctx context.Context, event stripe.Event) error
}
