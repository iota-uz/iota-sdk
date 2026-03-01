package subscription

import (
	"context"
	"sort"
	"time"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
)

func (s *service) HasFeature(ctx context.Context, tenantID uuid.UUID, feature string) (bool, error) {
	state, err := s.entitlementState(ctx, tenantID)
	if err != nil {
		return false, err
	}
	for _, key := range state.Features {
		if key == feature {
			return true, nil
		}
	}
	return false, nil
}

func (s *service) HasFeatures(ctx context.Context, tenantID uuid.UUID, features ...string) (map[string]bool, error) {
	state, err := s.entitlementState(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	available := make(map[string]struct{}, len(state.Features))
	for _, feature := range state.Features {
		available[feature] = struct{}{}
	}
	result := make(map[string]bool, len(features))
	for _, feature := range features {
		_, ok := available[feature]
		result[feature] = ok
	}
	return result, nil
}

func (s *service) GetFeatures(ctx context.Context, tenantID uuid.UUID) ([]string, error) {
	state, err := s.entitlementState(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	features := make([]string, len(state.Features))
	copy(features, state.Features)
	sort.Strings(features)
	return features, nil
}

func (s *service) GetTier(ctx context.Context, tenantID uuid.UUID) (*TierInfo, error) {
	state, err := s.entitlementState(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	resolved, err := s.resolveTier(state.Tier)
	if err != nil {
		return nil, err
	}
	limits := make(map[string]int, len(state.EntityLimits))
	for key, value := range state.EntityLimits {
		limits[key] = value
	}
	return &TierInfo{
		Tier:        state.Tier,
		DisplayName: resolved.Definition.DisplayName,
		Features:    append([]string{}, state.Features...),
		Limits:      limits,
		SeatLimit:   state.SeatLimit,
		ExpiresAt:   state.ExpiresAt,
		InGrace:     state.InGrace,
		GraceEndsAt: state.GraceEndsAt,
	}, nil
}

func (s *service) GetAllTiers(_ context.Context) ([]TierDefinition, error) {
	tiers := make([]TierDefinition, len(s.tierList))
	copy(tiers, s.tierList)
	return tiers, nil
}

func (s *service) IsInGracePeriod(ctx context.Context, tenantID uuid.UUID) (bool, *time.Time, error) {
	state, err := s.entitlementState(ctx, tenantID)
	if err != nil {
		return false, nil, err
	}
	if state.GraceEndsAt != nil && s.now().After(*state.GraceEndsAt) {
		return false, state.GraceEndsAt, nil
	}
	return state.InGrace, state.GraceEndsAt, nil
}

func (s *service) StartGracePeriod(ctx context.Context, tenantID uuid.UUID) error {
	graceEndsAt := s.now().AddDate(0, 0, s.cfg.GracePeriodDays)
	if err := s.repo.SetGracePeriod(ctx, tenantID, true, &graceEndsAt); err != nil {
		return err
	}
	total := s.graceUpdates.Add(1)
	logrus.WithFields(logrus.Fields{
		"tenant_id":              tenantID.String(),
		"in_grace_period":        true,
		"grace_period_ends_at":   graceEndsAt,
		"grace_updates_total":    total,
		"subscription_component": "service",
	}).Info("Subscription grace period updated")
	return s.InvalidateCache(ctx, tenantID)
}

func (s *service) ClearGracePeriod(ctx context.Context, tenantID uuid.UUID) error {
	if err := s.repo.SetGracePeriod(ctx, tenantID, false, nil); err != nil {
		return err
	}
	total := s.graceUpdates.Add(1)
	logrus.WithFields(logrus.Fields{
		"tenant_id":              tenantID.String(),
		"in_grace_period":        false,
		"grace_updates_total":    total,
		"subscription_component": "service",
	}).Info("Subscription grace period updated")
	return s.InvalidateCache(ctx, tenantID)
}

func (s *service) RefreshEntitlements(ctx context.Context, tenantID uuid.UUID) error {
	if s.syncer != nil {
		if err := s.syncer.RefreshTenant(ctx, tenantID); err != nil {
			return err
		}
	}
	return s.InvalidateCache(ctx, tenantID)
}

func (s *service) InvalidateCache(ctx context.Context, tenantID uuid.UUID) error {
	return s.cache.Delete(ctx, tenantCacheKey(tenantID))
}
