package persistence_test

import (
	"encoding/json"
	"github.com/iota-uz/iota-sdk/pkg/composables"
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
	f := setupTest(t)

	id := uuid.New()
	now := time.Now().UTC().Truncate(time.Second)

	tenant, err := composables.UseTenantID(f.ctx)
	require.NoError(t, err)

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
		{
			name:    "PaymeDetails",
			gateway: billing.Payme,
			details: details.NewPaymeDetails(
				"trans-001",
				details.PaymeWithMerchantID("merchant-xyz"),
				details.PaymeWithID("internal-id-001"),
				details.PaymeWithState(1),
				details.PaymeWithTime(1747834339199),
				details.PaymeWithCreatedTime(1747834339199),
				details.PaymeWithPerformTime(1747834339200),
				details.PaymeWithCancelTime(0),
				details.PaymeWithAccount(map[string]any{"order_id": "123"}),
				details.PaymeWithReceivers([]details.PaymeReceiver{
					details.NewPaymeReceiver("receiver-1", 100.0),
					details.NewPaymeReceiver("receiver-2", 200.0),
				}),
				details.PaymeWithAdditional(map[string]any{"extra": "info"}),
				details.PaymeWithReason(0),
				details.PaymeWithErrorCode(0),
				details.PaymeWithLink("https://payme.uz/paylink"),
				details.PaymeWithParams(map[string]any{"param": "value"}),
			),
			validate: func(t *testing.T, d details.Details) {
				t.Helper()
				payme := d.(details.PaymeDetails)
				assert.Equal(t, "merchant-xyz", payme.MerchantID())
				assert.Equal(t, "internal-id-001", payme.ID())
				assert.Equal(t, "trans-001", payme.Transaction())
				assert.Equal(t, int32(1), payme.State())
				assert.Equal(t, int64(1747834339199), payme.Time())
				assert.Equal(t, map[string]any{"order_id": "123"}, payme.Account())
				assert.Equal(t, map[string]any{"param": "value"}, payme.Params())
				assert.Equal(t, map[string]any{"extra": "info"}, payme.Additional())
				assert.Equal(t, "https://payme.uz/paylink", payme.Link())
				assert.Len(t, payme.Receivers(), 2)
				assert.Equal(t, "receiver-1", payme.Receivers()[0].ID())
				assert.InEpsilon(t, 100.0, payme.Receivers()[0].Amount(), 0.0001)
			},
		},
		{
			name:    "OctoDetails",
			gateway: billing.Octo,
			details: details.NewOctoDetails(
				"test-1",
				details.OctoWithOctoShopId(12345),
				details.OctoWithOctoPaymentUUID("uuid-abc-123"),
				details.OctoWithInitTime("2025-06-02T10:00:00Z"),
				details.OctoWithAutoCapture(true),
				details.OctoWithTest(false),
				details.OctoWithStatus("pending"),
				details.OctoWithDescription("Test transaction"),
				details.OctoWithCardType("VISA"),
				details.OctoWithCardCountry("US"),
				details.OctoWithCardIsPhysical(true),
				details.OctoWithCardMaskedPan("403200** **** 0000"),
				details.OctoWithRrn("rrn-xyz-987"),
				details.OctoWithRiskLevel(2),
				details.OctoWithRefundedSum(1.0),
				details.OctoWithTransferSum(965.0),
				details.OctoWithReturnUrl("https://example.com/return"),
				details.OctoWithNotifyUrl("https://example.com/notify"),
				details.OctoWithOctoPayUrl("https://octo.uz/pay/uuid-abc-123"),
				details.OctoWithSignature("F70F089D6EB66E34C8540149E32D0AC7C8A9500A"),
				details.OctoWithHashKey("2135b7e1-15bc-4a3c-930d-85b5493053b4"),
				details.OctoWithPayedTime("2025-06-02T10:01:00Z"),
				details.OctoWithError(0),
				details.OctoWithErrMessage(""),
			),
			validate: func(t *testing.T, d details.Details) {
				t.Helper()
				octo := d.(details.OctoDetails)
				assert.Equal(t, int32(12345), octo.OctoShopId())
				assert.Equal(t, "test-1", octo.ShopTransactionId())
				assert.Equal(t, "uuid-abc-123", octo.OctoPaymentUUID())
				assert.Equal(t, "2025-06-02T10:00:00Z", octo.InitTime())
				assert.True(t, octo.AutoCapture())
				assert.False(t, octo.Test())
				assert.Equal(t, "pending", octo.Status())
				assert.Equal(t, "Test transaction", octo.Description())
				assert.Equal(t, "VISA", octo.CardType())
				assert.Equal(t, "US", octo.CardCountry())
				assert.True(t, octo.CardIsPhysical())
				assert.Equal(t, "403200** **** 0000", octo.CardMaskedPan())
				assert.Equal(t, "rrn-xyz-987", octo.Rrn())
				assert.Equal(t, int32(2), octo.RiskLevel())
				assert.InEpsilon(t, 1.0, octo.RefundedSum(), 0.0001)
				assert.InEpsilon(t, 965.0, octo.TransferSum(), 0.0001)
				assert.Equal(t, "https://example.com/return", octo.ReturnUrl())
				assert.Equal(t, "https://example.com/notify", octo.NotifyUrl())
				assert.Equal(t, "https://octo.uz/pay/uuid-abc-123", octo.OctoPayUrl())
				assert.Equal(t, "F70F089D6EB66E34C8540149E32D0AC7C8A9500A", octo.Signature())
				assert.Equal(t, "2135b7e1-15bc-4a3c-930d-85b5493053b4", octo.HashKey())
				assert.Equal(t, "2025-06-02T10:01:00Z", octo.PayedTime())
				assert.Equal(t, int32(0), octo.Error())
				assert.Equal(t, "", octo.ErrMessage())
			},
		},
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
				billing.UZS,
				tt.gateway,
				tt.details,
				billing.WithTenantID(tenant),
				billing.WithID(id),
				billing.WithStatus(billing.Completed),
				billing.WithCreatedAt(now),
				billing.WithUpdatedAt(now),
			)

			// Serialize to DB model
			dbModel, err := persistence.ToDBTransaction(original)
			require.NoError(t, err)
			require.Equal(t, id.String(), dbModel.ID)
			require.Equal(t, tenant.String(), dbModel.TenantID)
			require.Equal(t, "completed", dbModel.Status)
			require.Equal(t, "UZS", dbModel.Currency)
			require.Equal(t, string(tt.gateway), dbModel.Gateway)
			require.WithinDuration(t, now, dbModel.CreatedAt, time.Second)
			require.WithinDuration(t, now, dbModel.UpdatedAt, time.Second)

			// Deserialize from DB model
			parsed, err := persistence.ToDomainTransaction(dbModel)
			require.NoError(t, err)

			assert.Equal(t, original.ID(), parsed.ID())
			assert.Equal(t, original.TenantID(), parsed.TenantID())
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
		TenantID:  uuid.New().String(),
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
