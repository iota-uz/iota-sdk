package controllers

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/iota-uz/iota-sdk/modules/billing/services"
	"github.com/iota-uz/iota-sdk/pkg/configuration"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stripe/stripe-go/v82"
	"github.com/stripe/stripe-go/v82/webhook"
)

type testStripeHook struct {
	calls int
	err   error
}

func (h *testStripeHook) HandleStripeEvent(_ context.Context, _ stripe.Event) error {
	h.calls++
	return h.err
}

var _ StripeEventHook = (*testStripeHook)(nil)

func TestStripeController_dispatchHooks(t *testing.T) {
	t.Parallel()

	okHook := &testStripeHook{}
	failHook := &testStripeHook{err: errors.New("hook failure")}

	controller := &StripeController{
		hooks: []StripeEventHook{okHook, failHook},
	}

	event := stripe.Event{Type: "invoice.payment_succeeded"}
	logger := logrus.New().WithField("test", true)

	controller.dispatchHooks(context.Background(), event, logger)

	assert.Equal(t, 1, okHook.calls)
	assert.Equal(t, 1, failHook.calls)
}

func TestStripeController_Handle_WebhookFlow(t *testing.T) {
	t.Parallel()

	secret := "whsec_test_secret"
	logger := logrus.New().WithField("test", true)

	t.Run("returns 200 for valid signed event", func(t *testing.T) {
		controller := &StripeController{
			billingService: &services.BillingService{},
			stripe:         configuration.StripeOptions{SigningSecret: secret},
		}

		eventPayload := map[string]any{
			"id":     "evt_ok_1",
			"object": "event",
			"type":   "product.created",
			"data": map[string]any{
				"object": map[string]any{"id": "prod_1"},
			},
		}
		req := newSignedWebhookRequest(t, secret, eventPayload)
		res := httptest.NewRecorder()

		controller.Handle(req, res, logger)
		assert.Equal(t, http.StatusOK, res.Code)
	})

	t.Run("returns 400 for invalid signature", func(t *testing.T) {
		controller := &StripeController{
			billingService: &services.BillingService{},
			stripe:         configuration.StripeOptions{SigningSecret: secret},
		}

		eventPayload := map[string]any{
			"id":     "evt_bad_sig",
			"object": "event",
			"type":   "product.created",
			"data": map[string]any{
				"object": map[string]any{"id": "prod_2"},
			},
		}
		body, err := json.Marshal(eventPayload)
		require.NoError(t, err)
		req := httptest.NewRequest(http.MethodPost, "/billing/stripe", bytes.NewReader(body))
		req.Header.Set("Stripe-Signature", "t=1,v1=invalid")
		res := httptest.NewRecorder()

		controller.Handle(req, res, logger)
		assert.Equal(t, http.StatusBadRequest, res.Code)
	})

	t.Run("returns 500 for handler error", func(t *testing.T) {
		controller := &StripeController{
			billingService: &services.BillingService{},
			stripe:         configuration.StripeOptions{SigningSecret: secret},
		}

		// checkout.session.completed path attempts to unmarshal Data.Raw into
		// stripe.CheckoutSession; a numeric `id` forces a parse error.
		eventPayload := map[string]any{
			"id":     "evt_handler_err",
			"object": "event",
			"type":   "checkout.session.completed",
			"data": map[string]any{
				"object": map[string]any{
					"id": 123,
				},
			},
		}
		req := newSignedWebhookRequest(t, secret, eventPayload)
		res := httptest.NewRecorder()

		controller.Handle(req, res, logger)
		assert.Equal(t, http.StatusInternalServerError, res.Code)
	})

	t.Run("enqueues hook dispatch on valid event", func(t *testing.T) {
		controller := &StripeController{
			billingService: &services.BillingService{},
			stripe:         configuration.StripeOptions{SigningSecret: secret},
			hooks:          []StripeEventHook{&testStripeHook{}},
			hookQueue:      make(chan stripe.Event, 1),
		}

		eventPayload := map[string]any{
			"id":     "evt_hook_1",
			"object": "event",
			"type":   "product.created",
			"data": map[string]any{
				"object": map[string]any{"id": "prod_3"},
			},
		}
		req := newSignedWebhookRequest(t, secret, eventPayload)
		res := httptest.NewRecorder()

		controller.Handle(req, res, logger)
		require.Equal(t, http.StatusOK, res.Code)

		select {
		case evt := <-controller.hookQueue:
			assert.Equal(t, stripe.EventType("product.created"), evt.Type)
		case <-time.After(time.Second):
			t.Fatal("expected event to be enqueued for hook dispatch")
		}
	})
}

func newSignedWebhookRequest(t *testing.T, secret string, payload map[string]any) *http.Request {
	t.Helper()

	if _, ok := payload["api_version"]; !ok {
		payload["api_version"] = stripe.APIVersion
	}

	body, err := json.Marshal(payload)
	require.NoError(t, err)
	signed := webhook.GenerateTestSignedPayload(&webhook.UnsignedPayload{
		Payload:   body,
		Secret:    secret,
		Timestamp: time.Now(),
	})

	req := httptest.NewRequest(http.MethodPost, "/billing/stripe", bytes.NewReader(signed.Payload))
	req.Header.Set("Stripe-Signature", signed.Header)
	return req
}
