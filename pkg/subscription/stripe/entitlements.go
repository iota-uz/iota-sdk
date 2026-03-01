package stripe

import (
	"context"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/pkg/subscription/repository"
)

func (s *Service) resolveTenantID(ctx context.Context, metadata map[string]string, customerID, subscriptionID string) (uuid.UUID, error) {
	if metadata != nil {
		if tenantRaw, ok := metadata["tenant_id"]; ok && tenantRaw != "" {
			tenantID, err := uuid.Parse(tenantRaw)
			if err == nil {
				return tenantID, nil
			}
		}
	}
	if customerID != "" {
		tenantID, err := s.repo.FindTenantByStripeCustomer(ctx, customerID)
		if err == nil {
			return tenantID, nil
		}
	}
	if subscriptionID != "" {
		tenantID, err := s.repo.FindTenantByStripeSubscription(ctx, subscriptionID)
		if err == nil {
			return tenantID, nil
		}
	}
	return uuid.Nil, repository.ErrEntitlementNotFound
}

func (s *Service) updateStripeRefs(ctx context.Context, tenantID uuid.UUID, customerID, subscriptionID string) error {
	var customerPtr *string
	if customerID != "" {
		customerPtr = &customerID
	}
	var subscriptionPtr *string
	if subscriptionID != "" {
		subscriptionPtr = &subscriptionID
	}
	if customerPtr == nil && subscriptionPtr == nil {
		return nil
	}
	if err := s.repo.SetStripeReferences(ctx, tenantID, customerPtr, subscriptionPtr); err != nil {
		return err
	}
	if s.invalidator != nil {
		return s.invalidator.InvalidateCache(ctx, tenantID)
	}
	return nil
}
