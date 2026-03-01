package stripe

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
	"github.com/iota-uz/iota-sdk/pkg/subscription/repository"
	"github.com/sirupsen/logrus"
)

func (s *Service) resolveTenantID(ctx context.Context, metadata map[string]string, customerID, subscriptionID string) (uuid.UUID, error) {
	const op serrors.Op = "SubscriptionStripeService.resolveTenantID"

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
		if !errors.Is(err, repository.ErrEntitlementNotFound) {
			return uuid.Nil, serrors.E(op, err)
		}
	}
	if subscriptionID != "" {
		tenantID, err := s.repo.FindTenantByStripeSubscription(ctx, subscriptionID)
		if err == nil {
			return tenantID, nil
		}
		if !errors.Is(err, repository.ErrEntitlementNotFound) {
			return uuid.Nil, serrors.E(op, err)
		}
	}
	return uuid.Nil, repository.ErrEntitlementNotFound
}

// ensureEntitlement creates a default entitlement for the tenant if one does
// not already exist. This is needed when a Stripe webhook arrives before the
// seed has run for a tenant (e.g. checkout completed before first login).
func (s *Service) ensureEntitlement(ctx context.Context, tenantID uuid.UUID) error {
	const op serrors.Op = "SubscriptionStripeService.ensureEntitlement"

	_, err := s.repo.GetEntitlement(ctx, tenantID)
	if err == nil {
		return nil
	}
	if !errors.Is(err, repository.ErrEntitlementNotFound) {
		return serrors.E(op, err)
	}

	now := s.now()
	if err := s.repo.UpsertEntitlement(ctx, &repository.Entitlement{
		TenantID:     tenantID,
		PlanID:       s.cfg.DefaultPlan,
		Features:     []string{},
		EntityLimits: map[string]int{},
		CreatedAt:    now,
		UpdatedAt:    now,
	}); err != nil {
		return serrors.E(op, err)
	}

	logrus.WithField("tenant_id", tenantID.String()).Info("Auto-provisioned default subscription entitlement")
	return nil
}

func (s *Service) updateStripeRefs(ctx context.Context, tenantID uuid.UUID, customerID, subscriptionID string) error {
	const op serrors.Op = "SubscriptionStripeService.updateStripeRefs"

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
	if customerPtr == nil || subscriptionPtr == nil {
		existing, err := s.repo.GetEntitlement(ctx, tenantID)
		if err != nil {
			return serrors.E(op, err)
		}
		if customerPtr == nil {
			customerPtr = existing.StripeCustomerID
		}
		if subscriptionPtr == nil {
			subscriptionPtr = existing.StripeSubscriptionID
		}
	}
	if customerPtr == nil && subscriptionPtr == nil {
		return nil
	}
	if err := s.repo.SetStripeReferences(ctx, tenantID, customerPtr, subscriptionPtr); err != nil {
		return serrors.E(op, err)
	}
	if s.invalidator != nil {
		if err := s.invalidator.InvalidateCache(ctx, tenantID); err != nil {
			return serrors.E(op, err)
		}
	}
	return nil
}
