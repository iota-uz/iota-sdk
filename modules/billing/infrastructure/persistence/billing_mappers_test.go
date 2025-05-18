package persistence_test

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/iota-uz/iota-sdk/modules/billing/domain/aggregates/billing"
	"github.com/iota-uz/iota-sdk/modules/billing/domain/aggregates/details"
	"github.com/iota-uz/iota-sdk/modules/billing/infrastructure/persistence"
	"github.com/iota-uz/iota-sdk/modules/billing/infrastructure/persistence/models"
)

func TestTransactionMapping(t *testing.T) {
	t.Helper()
	t.Parallel()

	id := uuid.New()
	now := time.Now().UTC().Truncate(time.Second)

	tests := []struct {
		name     string
		details  details.Details
		gateway  billing.Gateway
		validate func(t *testing.T, d details.Details)
	}{
		{
			name:    "ClickDetails",
			gateway: billing.Click,
			details: details.NewClickDetails(
				"https://example.com",
				details.ClickWithServiceID(100),
				details.ClickWithMerchantID(200),
				details.ClickWithMerchantUserID(300),
				details.ClickWithMerchantPrepareID(400),
				details.ClickWithMerchantConfirmID(500),
				details.ClickWithPayDocId(600),
				details.ClickWithPaymentID(700),
				details.ClickWithPaymentStatus(1),
				details.ClickWithSignTime("2025-01-01 12:00:00"),
				details.ClickWithSignString("signed"),
				details.ClickWithErrorCode(0),
				details.ClickWithErrorNote("OK"),
				details.ClickWithLink("https://example.com"),
				details.ClickWithParams(map[string]any{"key": "value"}),
			),
			validate: func(t *testing.T, d details.Details) {
				t.Helper()
				click := d.(details.ClickDetails)
				assert.Equal(t, "https://example.com", click.Link())
				assert.Equal(t, int64(100), click.ServiceID())
				assert.Equal(t, "signed", click.SignString())
				assert.Equal(t, map[string]any{"key": "value"}, click.Params())
			},
		},
		//{
		//	name:    "PaymeDetails",
		//	gateway: billing.Payme,
		//	details: details.NewPaymeDetails(),
		//	validate: func(t *testing.T, d details.Details) {
		//		t.Helper()
		//	},
		//},
		//{
		//	name:    "OctoDetails",
		//	gateway: billing.Octo,
		//	details: details.NewOctoDetails(),
		//	validate: func(t *testing.T, d details.Details) {
		//		t.Helper()
		//	},
		//},
		//{
		//	name:    "StripeDetails",
		//	gateway: billing.Stripe,
		//	details: details.NewStripeDetails(),
		//	validate: func(t *testing.T, d details.Details) {
		//		t.Helper()
		//	},
		//},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			original := billing.New(
				99.9,
				billing.USD,
				tt.gateway,
				tt.details,
				billing.WithID(id),
				billing.WithStatus(billing.Completed),
				billing.WithCreatedAt(now),
				billing.WithUpdatedAt(now),
			)

			// Serialize to DB model
			dbModel, err := persistence.ToDBTransaction(original)
			require.NoError(t, err)
			require.Equal(t, id.String(), dbModel.ID)
			require.Equal(t, "completed", dbModel.Status)
			require.Equal(t, "USD", dbModel.Currency)
			require.Equal(t, string(tt.gateway), dbModel.Gateway)
			require.WithinDuration(t, now, dbModel.CreatedAt, time.Second)
			require.WithinDuration(t, now, dbModel.UpdatedAt, time.Second)

			// Deserialize from DB model
			parsed, err := persistence.ToDomainTransaction(dbModel)
			require.NoError(t, err)

			assert.Equal(t, original.ID(), parsed.ID())
			assert.Equal(t, original.Status(), parsed.Status())
			assert.InEpsilon(t, original.Amount().Quantity(), parsed.Amount().Quantity(), 0.0001)
			assert.Equal(t, original.Amount().Currency(), parsed.Amount().Currency())
			assert.Equal(t, original.Gateway(), parsed.Gateway())

			// Additional details check
			tt.validate(t, parsed.Details())
		})
	}
}

func TestToDomainTransaction_InvalidUUID(t *testing.T) {
	t.Helper()
	t.Parallel()

	dbModel := &models.Transaction{
		ID:        "not-a-uuid",
		Status:    "created",
		Quantity:  10,
		Currency:  "USD",
		Gateway:   "click",
		Details:   json.RawMessage(`{}`),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	_, err := persistence.ToDomainTransaction(dbModel)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid UUID")
}

func TestToDomainTransaction_InvalidJSON(t *testing.T) {
	t.Helper()
	t.Parallel()

	dbModel := &models.Transaction{
		ID:        uuid.New().String(),
		Status:    "created",
		Quantity:  10,
		Currency:  "USD",
		Gateway:   "click",
		Details:   json.RawMessage(`{invalid-json}`),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	_, err := persistence.ToDomainTransaction(dbModel)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to parse details")
}
