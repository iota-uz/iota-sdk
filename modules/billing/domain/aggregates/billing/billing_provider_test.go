package billing

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
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
	tests := []struct {
		name       string
		call       func(context.Context, PayoutProvider) (*PayoutResult, error)
		wantStatus string
	}{
		{
			name: "create",
			call: func(ctx context.Context, provider PayoutProvider) (*PayoutResult, error) {
				return provider.CreatePayout(ctx, CreatePayoutRequest{
					ExternalID:  "cashback-42",
					Amount:      125_000,
					Currency:    "UZS",
					Destination: "8600123412341234",
				})
			},
			wantStatus: "created",
		},
		{
			name: "confirm",
			call: func(ctx context.Context, provider PayoutProvider) (*PayoutResult, error) {
				return provider.ConfirmPayout(ctx, ConfirmPayoutRequest{
					ExternalID:        "cashback-42",
					ProviderPaymentID: "receipt-42",
				})
			},
			wantStatus: "completed",
		},
		{
			name: "check",
			call: func(ctx context.Context, provider PayoutProvider) (*PayoutResult, error) {
				return provider.CheckPayout(ctx, "cashback-42")
			},
			wantStatus: "processing",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := tt.call(context.Background(), provider)
			require.NoError(t, err)
			assert.Equal(t, tt.wantStatus, result.Status)
		})
	}
	assert.Equal(t, Uzum, Gateway("uzum"))
}
