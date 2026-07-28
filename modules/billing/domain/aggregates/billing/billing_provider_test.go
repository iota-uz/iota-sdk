package billing

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

type payoutProviderStub struct{}

func (payoutProviderStub) CreatePayout(context.Context, CreatePayoutRequest) (*PayoutResult, error) {
	return &PayoutResult{Status: "created"}, nil
}

func (payoutProviderStub) ConfirmPayout(context.Context, ConfirmPayoutRequest) (*PayoutResult, error) {
	return &PayoutResult{Status: "completed"}, nil
}

func (payoutProviderStub) CheckPayout(context.Context, string) (*PayoutResult, error) {
	return &PayoutResult{Status: "processing"}, nil
}

func TestPayoutProviderCapability(t *testing.T) {
	t.Parallel()

	var provider PayoutProvider = payoutProviderStub{}
	result, err := provider.CreatePayout(context.Background(), CreatePayoutRequest{
		ExternalID:  "cashback-42",
		Amount:      125_000,
		Currency:    "UZS",
		Destination: "8600123412341234",
	})

	require.NoError(t, err)
	require.Equal(t, "created", result.Status)
	require.Equal(t, Uzum, Gateway("uzum"))
}
