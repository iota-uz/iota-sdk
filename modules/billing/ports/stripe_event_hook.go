// Package ports defines integration contracts for the billing module.
package ports

import (
	"context"

	"github.com/stripe/stripe-go/v82"
)

// StripeEventHook allows external components to observe raw Stripe events
// received by the billing webhook controller.
type StripeEventHook interface {
	HandleStripeEvent(ctx context.Context, event stripe.Event) error
}
