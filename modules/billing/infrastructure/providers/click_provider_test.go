package providers_test

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/iota-uz/iota-sdk/modules/billing/domain/aggregates/billing"
	"github.com/iota-uz/iota-sdk/modules/billing/domain/aggregates/details"
	"github.com/iota-uz/iota-sdk/modules/billing/infrastructure/providers"
)

const (
	testServiceID      int64 = 12345
	testMerchantUserID int64 = 6789
	testPaymentID      int64 = 987654321
	testSecretKey            = "click-secret"
)

func clickTransaction(t *testing.T, quantity float64, opts ...func(details.ClickDetails) details.ClickDetails) billing.Transaction {
	t.Helper()

	clickDetails := details.NewClickDetails("sale-1",
		details.ClickWithServiceID(testServiceID),
		details.ClickWithMerchantUserID(testMerchantUserID),
		details.ClickWithPaymentID(testPaymentID),
	)
	for _, opt := range opts {
		clickDetails = opt(clickDetails)
	}

	return billing.New(quantity, billing.UZS, billing.Click, clickDetails)
}

func clickProvider(t *testing.T, handler http.HandlerFunc) (billing.Provider, *httptest.Server) {
	t.Helper()

	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)

	return providers.NewClickProvider(providers.ClickConfig{
		ServiceID:      testServiceID,
		SecretKey:      testSecretKey,
		MerchantUserID: testMerchantUserID,
		APIURL:         server.URL,
		HTTPClient:     server.Client(),
	}), server
}

// The reversal endpoint identifies the payment entirely by URL, and the caller
// by a hand-rolled header. A mistake in either is invisible locally and shows up
// as somebody else's money not coming back, so both are asserted literally.
func TestClickRefundAddressesTheReversalEndpointAndSignsTheRequest(t *testing.T) {
	t.Parallel()

	var (
		gotMethod string
		gotPath   string
		gotAuth   string
	)
	provider, _ := clickProvider(t, func(w http.ResponseWriter, r *http.Request) {
		gotMethod, gotPath, gotAuth = r.Method, r.URL.Path, r.Header.Get("Auth")
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprintf(w, `{"error_code":0,"error_note":"Success","payment_id":%d}`, testPaymentID)
	})

	before := time.Now().Unix()
	transaction, err := provider.Refund(context.Background(), clickTransaction(t, 100_000), 100_000)
	after := time.Now().Unix()
	require.NoError(t, err)

	require.Equal(t, http.MethodDelete, gotMethod)
	require.Equal(t, fmt.Sprintf("/payment/reversal/%d/%d", testServiceID, testPaymentID), gotPath)

	parts := strings.Split(gotAuth, ":")
	require.Len(t, parts, 3, "Auth is merchant_user_id:digest:timestamp")
	require.Equal(t, strconv.FormatInt(testMerchantUserID, 10), parts[0])

	timestamp, err := strconv.ParseInt(parts[2], 10, 64)
	require.NoError(t, err)
	require.GreaterOrEqual(t, timestamp, before)
	require.LessOrEqual(t, timestamp, after)

	digest := sha1.Sum([]byte(parts[2] + testSecretKey)) //nolint:gosec // mirrors the Click contract under test
	require.Equal(t, hex.EncodeToString(digest[:]), parts[1])

	require.Equal(t, billing.Refunded, transaction.Status())
}

// A refusal by Click is an answer, not an outage: it must surface with the
// gateway's own code so the operator learns why, and it must not leave the
// transaction looking refunded.
func TestClickRefundSurfacesGatewayRefusalWithItsCode(t *testing.T) {
	t.Parallel()

	provider, _ := clickProvider(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprint(w, `{"error_code":-31007,"error_note":"Payment was rejected"}`)
	})

	transaction, err := provider.Refund(context.Background(), clickTransaction(t, 100_000), 100_000)
	require.Nil(t, transaction)

	var refusal providers.ClickReversalError
	require.ErrorAs(t, err, &refusal)
	require.Equal(t, int32(-31007), refusal.Code)
	require.Equal(t, "Payment was rejected", refusal.Note)
}

// The endpoint takes no amount. Accepting a smaller quantity would silently
// reverse the whole payment, so it is refused before the call is made.
func TestClickRefundRejectsAPartialAmountWithoutCallingTheGateway(t *testing.T) {
	t.Parallel()

	called := false
	provider, _ := clickProvider(t, func(w http.ResponseWriter, _ *http.Request) {
		called = true
		_, _ = fmt.Fprint(w, `{"error_code":0}`)
	})

	_, err := provider.Refund(context.Background(), clickTransaction(t, 100_000), 40_000)
	require.ErrorIs(t, err, providers.ErrClickPartialRefund)
	require.False(t, called, "a partial refund must not reach Click")
}

// More than was captured is not a reversal either: the endpoint sends back the
// captured amount, so accepting a larger figure would let the caller believe it
// returned money that never arrived.
func TestClickRefundRejectsAnAmountLargerThanWasCaptured(t *testing.T) {
	t.Parallel()

	called := false
	provider, _ := clickProvider(t, func(w http.ResponseWriter, _ *http.Request) {
		called = true
		_, _ = fmt.Fprint(w, `{"error_code":0}`)
	})

	_, err := provider.Refund(context.Background(), clickTransaction(t, 100_000), 140_000)
	require.ErrorIs(t, err, providers.ErrClickPartialRefund)
	require.False(t, called, "an amount that is not the captured one must not reach Click")
}

// Without the Complete callback there is no click_trans_id, and nothing was
// captured. Reversing "payment 0" would be a request about someone else's money.
func TestClickRefundRefusesATransactionThatNeverCompleted(t *testing.T) {
	t.Parallel()

	called := false
	provider, _ := clickProvider(t, func(w http.ResponseWriter, _ *http.Request) {
		called = true
		_, _ = fmt.Fprint(w, `{"error_code":0}`)
	})

	transaction := billing.New(100_000, billing.UZS, billing.Click, details.NewClickDetails("sale-1",
		details.ClickWithServiceID(testServiceID),
	))

	_, err := provider.Refund(context.Background(), transaction, 100_000)
	require.ErrorIs(t, err, providers.ErrClickPaymentIDMissing)
	require.False(t, called)
}

// An HTTP error page is not a reversal verdict. It must not be read as success
// and must not be read as a refusal either.
func TestClickRefundTreatsANonJSONErrorPageAsAFailure(t *testing.T) {
	t.Parallel()

	provider, _ := clickProvider(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
		_, _ = fmt.Fprint(w, "<html>502</html>")
	})

	transaction, err := provider.Refund(context.Background(), clickTransaction(t, 100_000), 100_000)
	require.Nil(t, transaction)
	require.Error(t, err)
	require.Contains(t, err.Error(), "502")

	// A transport failure is not a gateway refusal.
	require.NotErrorAs(t, err, &providers.ClickReversalError{})
}
