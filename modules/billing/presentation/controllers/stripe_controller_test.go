package controllers

import (
	"context"
	"errors"
	"testing"

	"github.com/iota-uz/iota-sdk/modules/billing/domain/aggregates/billing"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stripe/stripe-go/v82"
)

type testStripeHook struct {
	calls int
	err   error
}

func (h *testStripeHook) HandleStripeEvent(_ context.Context, _ stripe.Event) error {
	h.calls++
	return h.err
}

var _ billing.StripeEventHook = (*testStripeHook)(nil)

func TestStripeController_dispatchHooks(t *testing.T) {
	t.Parallel()

	okHook := &testStripeHook{}
	failHook := &testStripeHook{err: errors.New("hook failure")}

	controller := &StripeController{
		hooks: []billing.StripeEventHook{okHook, failHook},
	}

	event := stripe.Event{Type: "invoice.payment_succeeded"}
	logger := logrus.New().WithField("test", true)

	controller.dispatchHooks(context.Background(), event, logger)

	assert.Equal(t, 1, okHook.calls)
	assert.Equal(t, 1, failHook.calls)
}
