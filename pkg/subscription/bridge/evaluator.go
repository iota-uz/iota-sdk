package bridge

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
	"github.com/iota-uz/iota-sdk/pkg/subscription"
	subrepo "github.com/iota-uz/iota-sdk/pkg/subscription/repository"
)

const decisionVersion = "policy"

var _ subscription.FeatureEvaluator = (*Evaluator)(nil)
var _ subscription.LimitEvaluator = (*Evaluator)(nil)
var _ subscription.PlanResolver = (*Evaluator)(nil)

type Evaluator struct {
	repo        subrepo.Repository
	plans       map[string]subscription.PlanDefinition
	defaultPlan string
	now         func() time.Time
}

type Option func(*Evaluator)

func WithClock(now func() time.Time) Option {
	return func(e *Evaluator) {
		if now != nil {
			e.now = now
		}
	}
}

func NewEvaluator(cfg subscription.Config, repo subrepo.Repository, opts ...Option) (*Evaluator, error) {
	const op serrors.Op = "SubscriptionBridge.NewEvaluator"

	if repo == nil {
		return nil, serrors.E(op, fmt.Errorf("repository is required"))
	}
	plans, err := subscription.ResolvePlans(cfg)
	if err != nil {
		return nil, serrors.E(op, err)
	}
	defaultPlan := cfg.DefaultPlan
	if strings.TrimSpace(defaultPlan) == "" {
		defaultPlan = "FREE"
	}

	evaluator := &Evaluator{
		repo:        repo,
		plans:       plans,
		defaultPlan: defaultPlan,
		now:         func() time.Time { return time.Now().UTC() },
	}
	for _, opt := range opts {
		if opt != nil {
			opt(evaluator)
		}
	}
	return evaluator, nil
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
	if err != nil && !errors.Is(err, subrepo.ErrEntitlementNotFound) {
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
