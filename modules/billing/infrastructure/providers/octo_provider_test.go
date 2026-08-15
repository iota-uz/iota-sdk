package providers_test

import (
	"context"
	"testing"

	"github.com/iota-uz/iota-sdk/modules/billing/domain/aggregates/billing"
	"github.com/iota-uz/iota-sdk/modules/billing/domain/aggregates/details"
	"github.com/iota-uz/iota-sdk/modules/billing/infrastructure/providers"
	"github.com/iota-uz/iota-sdk/pkg/middleware"
	octoapi "github.com/iota-uz/octo"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
)

// octoProvider is built with a nil-safe log transport and a config that names
// no host: every test here asserts that Cancel reaches no network at all, so a
// call would fail rather than hit a stub, which is the stronger statement.
func octoProvider(t *testing.T) billing.Provider {
	t.Helper()

	return providers.NewOctoProvider(
		providers.OctoConfig{OctoShopID: 1, OctoSecret: "secret"},
		middleware.NewLogTransport(logrus.New(), nil, false, false, "octo-test"),
	)
}

func octoTransaction(status string, opts ...billing.Option) billing.Transaction {
	return billing.New(
		1920.0, billing.UZS, billing.Octo,
		details.NewOctoDetails("sale-1",
			details.OctoWithAutoCapture(false),
			details.OctoWithStatus(status),
		),
		opts...,
	)
}

// Cancelling a payment nobody has captured is the merchant withdrawing its
// willingness to take the money. Octo has no endpoint for that, and none is
// needed: the status is what the callback reads when it answers Octo's request
// for a verdict.
//
// What would make this falsely green: asserting only that Cancel returns no
// error. The status is the whole mechanism — without it the link stays live.
func TestOctoCancelAbandonsAPaymentThatWasNeverCaptured(t *testing.T) {
	t.Parallel()

	for _, status := range []string{
		octoapi.CreatedStatus,
		octoapi.WaitUserActionStatus,
		octoapi.WaitingForCaptureStatus,
	} {
		t.Run(status, func(t *testing.T) {
			t.Parallel()

			cancelled, err := octoProvider(t).Cancel(context.Background(), octoTransaction(status))
			require.NoError(t, err)
			require.Equal(t, billing.Canceled, cancelled.Status())
			require.False(t, cancelled.Status().IsActive())
		})
	}
}

// `succeeded` means Octo took the money. Marking that transaction cancelled
// would leave captured funds recorded as never taken, and the refund that
// should have been asked for never happens.
func TestOctoCancelRefusesACapturedPayment(t *testing.T) {
	t.Parallel()

	_, err := octoProvider(t).Cancel(context.Background(), octoTransaction(octoapi.SucceededStatus))
	require.ErrorIs(t, err, providers.ErrOctoCancelAfterCapture)
}

// The billing status is consulted as well as Octo's own, because a transaction
// can be marked Completed by the status check that runs after the callback
// while its stored octo status is still the one the notification carried.
func TestOctoCancelRefusesATransactionAlreadyCompletedOnOurSide(t *testing.T) {
	t.Parallel()

	completed := octoTransaction(octoapi.WaitingForCaptureStatus, billing.WithStatus(billing.Completed))

	_, err := octoProvider(t).Cancel(context.Background(), completed)
	require.ErrorIs(t, err, providers.ErrOctoCancelAfterCapture)
}
