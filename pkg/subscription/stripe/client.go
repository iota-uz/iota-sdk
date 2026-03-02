// Package stripe provides Stripe integration helpers for subscription synchronization.
package stripe

import (
	"context"

	"github.com/iota-uz/iota-sdk/pkg/serrors"
	"github.com/stripe/stripe-go/v82"
)

type EntitlementsClient interface {
	ListActiveEntitlements(ctx context.Context, customerID string) ([]string, error)
}

type client struct {
	api *stripe.Client
}

func NewClient(secretKey string) EntitlementsClient {
	return &client{
		api: stripe.NewClient(secretKey),
	}
}

func (c *client) ListActiveEntitlements(ctx context.Context, customerID string) ([]string, error) {
	const op serrors.Op = "SubscriptionStripeClient.ListActiveEntitlements"

	params := &stripe.EntitlementsActiveEntitlementListParams{}
	params.Customer = stripe.String(customerID)
	params.AddExpand("data.feature")

	seen := make(map[string]struct{})
	features := make([]string, 0)
	for current, iterErr := range c.api.V1EntitlementsActiveEntitlements.List(ctx, params) {
		if iterErr != nil {
			return nil, serrors.E(op, iterErr)
		}
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
		if _, exists := seen[lookupKey]; exists {
			continue
		}
		seen[lookupKey] = struct{}{}
		features = append(features, lookupKey)
	}
	return features, nil
}
