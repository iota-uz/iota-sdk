package billing

import (
	"context"

	"github.com/stripe/stripe-go/v82"
)

type StripeEventHook interface {
	HandleStripeEvent(ctx context.Context, event stripe.Event) error
}
