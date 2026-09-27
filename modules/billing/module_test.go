package billing

import (
	"context"
	"testing"

	"github.com/iota-uz/iota-sdk/modules/billing/ports"
	"github.com/stretchr/testify/require"
	"github.com/stripe/stripe-go/v82"
)

type noopStripeHook struct{}

func (noopStripeHook) HandleStripeEvent(context.Context, stripe.Event) error {
	return nil
}

var _ ports.StripeEventHook = noopStripeHook{}

func TestNewComponent_WithStripeEventHooks(t *testing.T) {
	t.Parallel()

	hook := noopStripeHook{}
	billingComponent := NewComponent(WithStripeEventHooks(hook))

	typed, ok := billingComponent.(*component)
	require.True(t, ok)
	require.Len(t, typed.stripeHooks, 1)
}
