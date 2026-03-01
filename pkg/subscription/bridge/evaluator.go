package bridge

import (
	"context"
	"fmt"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
	"github.com/iota-uz/iota-sdk/pkg/subscription"
	subrepo "github.com/iota-uz/iota-sdk/pkg/subscription/repository"
)

const decisionVersion = "policy"

type Evaluator struct {
	repo        subrepo.Repository
	plans       map[string]subscription.PlanDefinition
	defaultPlan string
	now         func() time.Time
}

func NewEvaluator(cfg subscription.Config, repo subrepo.Repository) (*Evaluator, error) {
	const op serrors.Op = "SubscriptionBridge.NewEvaluator"

	if repo == nil {
		return nil, serrors.E(op, fmt.Errorf("repository is required"))
	}
	cfg = normalizeConfig(cfg)
	plans, err := resolvePlans(cfg)
	if err != nil {
		return nil, serrors.E(op, err)
	}

	return &Evaluator{
		repo:        repo,
		plans:       plans,
		defaultPlan: cfg.DefaultPlan,
		now:         func() time.Time { return time.Now().UTC() },
	}, nil
}

func (e *Evaluator) CurrentPlan(ctx context.Context, subject subscription.Subject) (subscription.PlanInfo, error) {
	const op serrors.Op = "SubscriptionBridge.CurrentPlan"

	entitlement, err := e.entitlementForSubject(ctx, subject)
	if err != nil {
		return subscription.PlanInfo{}, serrors.E(op, err)
	}

	planID, plan := e.resolvePlan(entitlement.PlanID)
	return subscription.PlanInfo{
		ID:          planID,
		DisplayName: plan.DisplayName,
		Description: plan.Description,
	}, nil
}

func (e *Evaluator) EvaluateFeature(
	ctx context.Context,
	subject subscription.Subject,
	feature subscription.FeatureKey,
) (subscription.Decision, error) {
	const op serrors.Op = "SubscriptionBridge.EvaluateFeature"

	if strings.TrimSpace(string(feature)) == "" {
		return subscription.Decision{}, serrors.E(op, fmt.Errorf("feature is required"))
	}

	entitlement, err := e.entitlementForSubject(ctx, subject)
	if err != nil {
		return subscription.Decision{}, serrors.E(op, err)
	}

	planID, plan := e.resolvePlan(entitlement.PlanID)
	allowed := slices.Contains(plan.Features, string(feature)) || slices.Contains(entitlement.Features, string(feature))

	reason := "feature is not granted"
	if allowed {
		reason = "allowed by plan or entitlement"
	}
	if inGrace(entitlement, e.now()) {
		reason += " (grace period active)"
	}

	return subscription.Decision{
		Allowed: allowed,
		Subject: subject.Ref(),
		Feature: feature,
		PlanID:  planID,
		Reason:  reason,
		Version: decisionVersion,
	}, nil
}

func (e *Evaluator) EvaluateLimit(
	ctx context.Context,
	subject subscription.Subject,
	quota subscription.QuotaKey,
) (subscription.LimitDecision, error) {
	const op serrors.Op = "SubscriptionBridge.EvaluateLimit"

	normalizedQuota, err := subscription.NewQuotaKey(quota.Resource, quota.Dimension, quota.Window)
	if err != nil {
		return subscription.LimitDecision{}, serrors.E(op, err)
	}
	quota = normalizedQuota

	entitlement, err := e.entitlementForSubject(ctx, subject)
	if err != nil {
		return subscription.LimitDecision{}, serrors.E(op, err)
	}
	planID, plan := e.resolvePlan(entitlement.PlanID)

	limit := resolveLimit(entitlement, plan, quota)
	current := 0
	if strings.EqualFold(quota.Resource, "seats") {
		current = entitlement.CurrentSeats
	} else {
		current, err = e.repo.GetEntityCount(ctx, entitlement.TenantID, quota.Resource)
		if err != nil {
			return subscription.LimitDecision{}, serrors.E(op, err)
		}
	}

	allowed := true
	remaining := -1
	reason := "unlimited quota"
	if limit >= 0 {
		allowed = current < limit
		remaining = max(limit-current, 0)
		if allowed {
			reason = "within quota"
		} else {
			reason = "quota exceeded"
		}
	}
	if inGrace(entitlement, e.now()) {
		reason += " (grace period active)"
	}

	return subscription.LimitDecision{
		Allowed:   allowed,
		Subject:   subject.Ref(),
		Quota:     quota,
		Current:   current,
		Limit:     limit,
		Remaining: remaining,
		PlanID:    planID,
		Reason:    reason,
		Version:   decisionVersion,
	}, nil
}

func (e *Evaluator) entitlementForSubject(ctx context.Context, subject subscription.Subject) (*subrepo.Entitlement, error) {
	tenantID, err := tenantIDFromSubject(subject)
	if err != nil {
		return nil, err
	}

	entitlement, err := e.repo.GetEntitlement(ctx, tenantID)
	if err == nil {
		return entitlement, nil
	}
	if err != nil && err != subrepo.ErrEntitlementNotFound {
		return nil, err
	}

	now := e.now()
	return &subrepo.Entitlement{
		TenantID:     tenantID,
		PlanID:       e.defaultPlan,
		Features:     []string{},
		EntityLimits: map[string]int{},
		CreatedAt:    now,
		UpdatedAt:    now,
	}, nil
}

func (e *Evaluator) resolvePlan(planID string) (string, subscription.PlanDefinition) {
	if planID != "" {
		if plan, ok := e.plans[planID]; ok {
			return planID, plan
		}
	}
	if plan, ok := e.plans[e.defaultPlan]; ok {
		return e.defaultPlan, plan
	}
	return e.defaultPlan, subscription.PlanDefinition{
		PlanID:       e.defaultPlan,
		DisplayName:  e.defaultPlan,
		EntityLimits: map[string]int{},
		Features:     []string{},
	}
}

func resolveLimit(entitlement *subrepo.Entitlement, plan subscription.PlanDefinition, quota subscription.QuotaKey) int {
	if strings.EqualFold(quota.Resource, "seats") {
		if entitlement.SeatLimit != nil {
			return *entitlement.SeatLimit
		}
		if plan.SeatLimit != nil {
			return *plan.SeatLimit
		}
		return -1
	}

	if v, ok := entitlement.EntityLimits[quota.String()]; ok {
		return v
	}
	if v, ok := entitlement.EntityLimits[quota.Resource]; ok {
		return v
	}
	if v, ok := plan.EntityLimits[quota.String()]; ok {
		return v
	}
	if v, ok := plan.EntityLimits[quota.Resource]; ok {
		return v
	}
	return -1
}

func tenantIDFromSubject(subject subscription.Subject) (uuid.UUID, error) {
	if subject.Scope == subscription.ScopeTenant && subject.ID != uuid.Nil {
		return subject.ID, nil
	}
	for _, parent := range subject.Parents {
		if parent.Scope == subscription.ScopeTenant && parent.ID != uuid.Nil {
			return parent.ID, nil
		}
	}
	return uuid.Nil, fmt.Errorf("tenant subject is required")
}

func inGrace(entitlement *subrepo.Entitlement, now time.Time) bool {
	if entitlement == nil || !entitlement.InGracePeriod {
		return false
	}
	if entitlement.GracePeriodEndsAt == nil {
		return true
	}
	return now.Before(*entitlement.GracePeriodEndsAt)
}

func normalizeConfig(cfg subscription.Config) subscription.Config {
	out := cfg
	if out.DefaultPlan == "" {
		out.DefaultPlan = "FREE"
	}
	return out
}

func resolvePlans(cfg subscription.Config) (map[string]subscription.PlanDefinition, error) {
	plans := make(map[string]subscription.PlanDefinition, len(cfg.Plans))
	for _, plan := range cfg.Plans {
		if strings.TrimSpace(plan.PlanID) == "" {
			continue
		}
		if plan.EntityLimits == nil {
			plan.EntityLimits = map[string]int{}
		}
		plans[plan.PlanID] = plan
	}
	if _, ok := plans[cfg.DefaultPlan]; !ok {
		plans[cfg.DefaultPlan] = subscription.PlanDefinition{
			PlanID:       cfg.DefaultPlan,
			DisplayName:  cfg.DefaultPlan,
			EntityLimits: map[string]int{},
			Features:     []string{},
		}
	}

	resolved := make(map[string]subscription.PlanDefinition, len(plans))
	visiting := make(map[string]bool, len(plans))

	var dfs func(string) (subscription.PlanDefinition, error)
	dfs = func(planID string) (subscription.PlanDefinition, error) {
		if plan, ok := resolved[planID]; ok {
			return plan, nil
		}
		if visiting[planID] {
			return subscription.PlanDefinition{}, fmt.Errorf("plan inheritance cycle detected: %s", planID)
		}

		current, ok := plans[planID]
		if !ok {
			return subscription.PlanDefinition{}, fmt.Errorf("plan not found: %s", planID)
		}
		visiting[planID] = true

		featureSet := map[string]struct{}{}
		limits := map[string]int{}
		var seatLimit *int
		if current.ParentPlanID != "" {
			parent, err := dfs(current.ParentPlanID)
			if err != nil {
				return subscription.PlanDefinition{}, err
			}
			for _, feature := range parent.Features {
				featureSet[feature] = struct{}{}
			}
			for key, value := range parent.EntityLimits {
				limits[key] = value
			}
			if parent.SeatLimit != nil {
				parentLimit := *parent.SeatLimit
				seatLimit = &parentLimit
			}
		}
		for _, feature := range current.Features {
			featureSet[feature] = struct{}{}
		}
		for key, value := range current.EntityLimits {
			limits[key] = value
		}
		if current.SeatLimit != nil {
			currentLimit := *current.SeatLimit
			seatLimit = &currentLimit
		}

		features := make([]string, 0, len(featureSet))
		for feature := range featureSet {
			features = append(features, feature)
		}
		sort.Strings(features)

		merged := current
		merged.PlanID = planID
		merged.Features = features
		merged.EntityLimits = limits
		merged.SeatLimit = seatLimit

		visiting[planID] = false
		resolved[planID] = merged
		return merged, nil
	}

	for planID := range plans {
		if _, err := dfs(planID); err != nil {
			return nil, err
		}
	}

	return resolved, nil
}
