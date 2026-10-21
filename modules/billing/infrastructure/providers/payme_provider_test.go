package providers_test

import (
	"context"
	"encoding/base64"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/iota-uz/iota-sdk/modules/billing/domain/aggregates/billing"
	"github.com/iota-uz/iota-sdk/modules/billing/domain/aggregates/details"
	"github.com/iota-uz/iota-sdk/modules/billing/infrastructure/providers"
	paymeapi "github.com/iota-uz/payme"
)

func paymeProvider() billing.Provider {
	return providers.NewPaymeProvider(providers.PaymeConfig{
		URL:        "https://checkout.test.paycom.uz",
		MerchantID: "merchant-1",
		User:       "Paycom",
		SecretKey:  "secret",
	})
}

func paymeTransaction(t *testing.T, opts ...details.PaymeOption) billing.Transaction {
	t.Helper()

	opts = append([]details.PaymeOption{
		details.PaymeWithAccount(map[string]any{"order_id": "policy:1"}),
	}, opts...)

	return billing.New(1920.0, billing.UZS, billing.Payme, details.NewPaymeDetails("policy:1", opts...))
}

// Cancelling a minted checkout is what a merchant does when the customer moves
// to another gateway, and the whole point of it is the state the transaction
// leaves behind: PaymeController.create picks an account's transaction through
// activePaymeTransaction, so only a status outside the active set makes the old
// link stop being payable. Asserting the details' state alone would pass while
// the transaction stayed active and the customer kept two live links.
func TestPaymeCancelTakesTheTransactionOutOfTheActiveSet(t *testing.T) {
	t.Parallel()

	created, err := paymeProvider().Create(context.Background(), paymeTransaction(t))
	require.NoError(t, err)
	require.Equal(t, int32(paymeapi.TransactionStateCreated), created.Details().(details.PaymeDetails).State())

	cancelled, err := paymeProvider().Cancel(context.Background(), created)
	require.NoError(t, err)

	require.Equal(t, billing.Canceled, cancelled.Status())
	require.False(t, cancelled.Status().IsActive())

	cancelledDetails := cancelled.Details().(details.PaymeDetails)
	require.Equal(t, int32(paymeapi.TransactionStateCancelledBeforeCompletion), cancelledDetails.State())
	require.NotZero(t, cancelledDetails.CancelTime())
	// Payme's reason codes say why Payme cancelled something. A merchant
	// abandoning its own checkout is not one of them, and inventing the nearest
	// code would record a Payme decision that was never made.
	require.Zero(t, cancelledDetails.Reason())
}

// A cancellation that arrives after the customer paid must not be answered by
// marking captured money as never taken: that is a refund, and Payme gives it
// its own operation and its own reason code.
func TestPaymeCancelRefusesACompletedTransaction(t *testing.T) {
	t.Parallel()

	completed := paymeTransaction(t, details.PaymeWithState(paymeapi.TransactionStateCompleted)).
		SetStatus(billing.Completed)

	_, err := paymeProvider().Cancel(context.Background(), completed)
	require.ErrorIs(t, err, providers.ErrPaymeCancelAfterCompletion)
}

// A two-decimal transaction must mint the same integer tiyin amount in the
// checkout. This would be falsely green if it checked only stored transaction
// details instead of decoding the link handed to the customer.
func TestPaymeCheckoutPreservesMinorAmount(t *testing.T) {
	t.Parallel()
	for _, minor := range []int64{7405760, 100, 101, 199, 9257200} {
		t.Run(strconv.FormatInt(minor, 10), func(t *testing.T) {
			t.Parallel()
			transaction := paymeTransaction(t).SetAmount(float64(minor)/100, billing.UZS)
			created, err := paymeProvider().Create(context.Background(), transaction)
			require.NoError(t, err)
			link := created.Details().(details.PaymeDetails).Link()
			payload, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(link, "https://checkout.test.paycom.uz/"))
			require.NoError(t, err)
			fields := map[string]string{}
			for _, part := range strings.Split(string(payload), ";") {
				key, value, ok := strings.Cut(part, "=")
				require.True(t, ok)
				fields[key] = value
			}
			require.Equal(t, strconv.FormatInt(minor, 10), fields["a"])
		})
	}
}
