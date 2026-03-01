package billing

import (
	"context"
	"testing"

	"github.com/iota-uz/iota-sdk/modules/billing/presentation/controllers"
	"github.com/stretchr/testify/require"
	"github.com/stripe/stripe-go/v82"
)

type noopStripeHook struct{}

func (noopStripeHook) HandleStripeEvent(context.Context, stripe.Event) error {
	return nil
}

var _ controllers.StripeEventHook = noopStripeHook{}

func TestNewModule_WithStripeEventHooks(t *testing.T) {
	t.Parallel()

	hook := noopStripeHook{}
	module := NewModule(WithStripeEventHooks(hook))

	typed, ok := module.(*Module)
	require.True(t, ok)
	require.Len(t, typed.stripeHooks, 1)
}
