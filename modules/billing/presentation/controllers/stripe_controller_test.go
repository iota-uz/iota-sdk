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

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/modules/billing/domain/aggregates/billing"
	"github.com/iota-uz/iota-sdk/modules/billing/domain/aggregates/details"
	"github.com/iota-uz/iota-sdk/modules/billing/ports"
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

var _ ports.StripeEventHook = (*testStripeHook)(nil)

type testBillingRepo struct {
	getByDetailsFields func(
		ctx context.Context,
		gateway billing.Gateway,
		filters []billing.DetailsFieldFilter,
	) ([]billing.Transaction, error)
}

func (r *testBillingRepo) Count(_ context.Context, _ *billing.FindParams) (int64, error) {
	return 0, nil
}

func (r *testBillingRepo) GetPaginated(_ context.Context, _ *billing.FindParams) ([]billing.Transaction, error) {
	return nil, nil
}

func (r *testBillingRepo) GetByID(_ context.Context, _ uuid.UUID) (billing.Transaction, error) {
	return nil, errors.New("not implemented")
}

func (r *testBillingRepo) GetByDetailsFields(
	ctx context.Context,
	gateway billing.Gateway,
	filters []billing.DetailsFieldFilter,
) ([]billing.Transaction, error) {
	if r.getByDetailsFields == nil {
		return nil, nil
	}
	return r.getByDetailsFields(ctx, gateway, filters)
}

func (r *testBillingRepo) GetAll(_ context.Context) ([]billing.Transaction, error) {
	return nil, nil
}

func (r *testBillingRepo) Save(_ context.Context, tx billing.Transaction) (billing.Transaction, error) {
	return tx, nil
}

func (r *testBillingRepo) Delete(_ context.Context, _ uuid.UUID) error {
	return nil
}

func TestStripeController_dispatchHooks(t *testing.T) {
	t.Parallel()

	okHook := &testStripeHook{}
	failHook := &testStripeHook{err: errors.New("hook failure")}

	controller := &StripeController{
		hooks: []ports.StripeEventHook{okHook, failHook},
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
			hooks:          []ports.StripeEventHook{&testStripeHook{}},
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

func TestStripeController_Handle_Returns500_OnLookupFailures(t *testing.T) {
	t.Parallel()

	secret := "whsec_test_secret"

	stripeTx := func(clientReferenceID string) billing.Transaction {
		return billing.New(
			10,
			billing.USD,
			billing.Stripe,
			details.NewStripeDetails(clientReferenceID),
		)
	}

	tests := []struct {
		name       string
		payload    map[string]any
		entities   []billing.Transaction
		err        error
		wantStatus int
	}{
		{
			name: "checkout completed with no transactions returns 500",
			payload: map[string]any{
				"id":     "evt_checkout_no_rows",
				"object": "event",
				"type":   "checkout.session.completed",
				"data": map[string]any{
					"object": map[string]any{"id": "cs_test_1"},
				},
			},
			entities:   []billing.Transaction{},
			wantStatus: http.StatusInternalServerError,
		},
		{
			name: "checkout completed with multiple transactions returns 500",
			payload: map[string]any{
				"id":     "evt_checkout_multi_rows",
				"object": "event",
				"type":   "checkout.session.completed",
				"data": map[string]any{
					"object": map[string]any{"id": "cs_test_2"},
				},
			},
			entities: []billing.Transaction{
				stripeTx("ref-1"),
				stripeTx("ref-2"),
			},
			wantStatus: http.StatusInternalServerError,
		},
		{
			name: "checkout completed with invalid details type returns 500",
			payload: map[string]any{
				"id":     "evt_checkout_bad_details",
				"object": "event",
				"type":   "checkout.session.completed",
				"data": map[string]any{
					"object": map[string]any{"id": "cs_test_3"},
				},
			},
			entities: []billing.Transaction{
				billing.New(
					10,
					billing.USD,
					billing.Stripe,
					details.NewCashDetails(),
				),
			},
			wantStatus: http.StatusInternalServerError,
		},
		{
			name: "invoice payment succeeded with no transactions returns 500",
			payload: map[string]any{
				"id":     "evt_invoice_no_rows",
				"object": "event",
				"type":   "invoice.payment_succeeded",
				"data": map[string]any{
					"object": map[string]any{"id": "in_test_1"},
				},
			},
			entities:   []billing.Transaction{},
			wantStatus: http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			repo := &testBillingRepo{
				getByDetailsFields: func(_ context.Context, _ billing.Gateway, _ []billing.DetailsFieldFilter) ([]billing.Transaction, error) {
					return tt.entities, tt.err
				},
			}
			controller := &StripeController{
				billingService: services.NewBillingService(repo, nil, nil),
				stripe:         configuration.StripeOptions{SigningSecret: secret},
			}
			logger := logrus.New().WithField("test", true)

			req := newSignedWebhookRequest(t, secret, tt.payload)
			res := httptest.NewRecorder()

			controller.Handle(req, res, logger)
			assert.Equal(t, tt.wantStatus, res.Code)
		})
	}
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

func TestCurrencyDivisor_Scenarios(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		currency string
		want     float64
	}{
		{name: "zero-decimal JPY", currency: "JPY", want: 1},
		{name: "three-decimal BHD", currency: "BHD", want: 1000},
		{name: "three-decimal JOD", currency: "JOD", want: 1000},
		{name: "three-decimal KWD", currency: "KWD", want: 1000},
		{name: "three-decimal OMR", currency: "OMR", want: 1000},
		{name: "three-decimal TND", currency: "TND", want: 1000},
		{name: "default USD", currency: "USD", want: 100},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.InDelta(t, tt.want, currencyDivisor(tt.currency), 0)
		})
	}
}

func TestTransactionLookupErrors_Scenarios(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		err      error
		contains string
	}{
		{
			name:     "transaction not found",
			err:      transactionNotFoundError("invoice_id", "in_123"),
			contains: "invoice_id=in_123",
		},
		{
			name:     "unexpected transaction count",
			err:      unexpectedTransactionCountError("session_id", "cs_123", 1, 0),
			contains: "expected 1 transaction(s)",
		},
		{
			name:     "invalid stripe details type",
			err:      invalidStripeDetailsTypeError("not-details"),
			contains: "got string",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			require.Error(t, tt.err)
			assert.Contains(t, tt.err.Error(), tt.contains)
		})
	}
}
