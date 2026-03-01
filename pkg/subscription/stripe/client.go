package stripe

import (
	"context"

	"github.com/iota-uz/iota-sdk/pkg/serrors"
	"github.com/stripe/stripe-go/v82"
	"github.com/stripe/stripe-go/v82/entitlements/activeentitlement"
)

type EntitlementsClient interface {
	ListActiveEntitlements(ctx context.Context, customerID string) ([]string, error)
}

type client struct {
	api activeentitlement.Client
}

func NewClient(secretKey string) EntitlementsClient {
	return &client{
		api: activeentitlement.Client{
			B:   stripe.GetBackendWithConfig(stripe.APIBackend, &stripe.BackendConfig{}),
			Key: secretKey,
		},
	}
}

func (c *client) ListActiveEntitlements(ctx context.Context, customerID string) ([]string, error) {
	const op serrors.Op = "SubscriptionStripeClient.ListActiveEntitlements"

	params := &stripe.EntitlementsActiveEntitlementListParams{}
	params.Customer = stripe.String(customerID)
	params.Context = ctx
	params.AddExpand("data.feature")

	iter := c.api.List(params)
	features := make([]string, 0)
	for iter.Next() {
		current := iter.EntitlementsActiveEntitlement()
		if current == nil {
			continue
		}
		lookupKey := current.LookupKey
		if lookupKey == "" && current.Feature != nil {
			lookupKey = current.Feature.LookupKey
		}
		if lookupKey == "" {
			continue
		}
		features = append(features, lookupKey)
	}
	if err := iter.Err(); err != nil {
		return nil, serrors.E(op, err)
	}
	return features, nil
}
