package subscription

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
)

const decisionSchemaVersion = "policy"

type Engine interface {
	UpsertGrant(ctx context.Context, grant Grant) error
	RevokeGrant(ctx context.Context, grantID string) error
	ListGrants(ctx context.Context, subject SubjectRef) ([]Grant, error)

	AssignPlan(ctx context.Context, subject SubjectRef, planID string) error
	CurrentPlan(ctx context.Context, subject Subject) (PlanInfo, error)

	EvaluateFeature(ctx context.Context, subject Subject, feature FeatureKey) (Decision, error)
	EvaluateLimit(ctx context.Context, subject Subject, quota QuotaKey) (LimitDecision, error)

	Reserve(ctx context.Context, subject Subject, quota QuotaKey, amount int, token string) (Reservation, error)
	Commit(ctx context.Context, reservationID string) error
	Release(ctx context.Context, reservationID string) error

	SetUsage(ctx context.Context, subject SubjectRef, quota QuotaKey, amount int) error
}

type Option func(*service)

func WithClock(now func() time.Time) Option {
	return func(s *service) {
		if now != nil {
			s.now = now
		}
	}
}

type service struct {
	cfg Config
	now func() time.Time

	mu sync.RWMutex

	plans map[string]PlanDefinition

	grants          map[string]Grant
	grantsBySubject map[string]map[string]struct{}

	usage              map[string]int
	reservations       map[string]*Reservation
	reservationByToken map[string]string
}

func NewService(cfg Config, opts ...Option) (Engine, error) {
	const op serrors.Op = "SubscriptionEngine.NewService"

	cfg = cfg.normalized()
	plans, err := resolvePlans(cfg)
	if err != nil {
		return nil, serrors.E(op, err)
	}

	svc := &service{
		cfg:                cfg,
		now:                func() time.Time { return time.Now().UTC() },
		plans:              plans,
		grants:             make(map[string]Grant),
		grantsBySubject:    make(map[string]map[string]struct{}),
		usage:              make(map[string]int),
		reservations:       make(map[string]*Reservation),
		reservationByToken: make(map[string]string),
	}
	for _, opt := range opts {
		if opt != nil {
			opt(svc)
		}
	}
	return svc, nil
}

func (s *service) UpsertGrant(_ context.Context, grant Grant) error {
	const op serrors.Op = "SubscriptionEngine.UpsertGrant"

	if strings.TrimSpace(grant.ID) == "" {
		return serrors.E(op, ErrGrantIDRequired)
	}
	if err := validateSubjectRef(grant.Subject); err != nil {
		return serrors.E(op, err)
	}
	if grant.PlanID != "" {
		if _, ok := s.plans[grant.PlanID]; !ok {
			return serrors.E(op, ErrPlanNotFound)
		}
	}

	now := s.now()
	s.mu.Lock()
	defer s.mu.Unlock()

	existing, exists := s.grants[grant.ID]
	if exists && subjectRefKey(existing.Subject) != subjectRefKey(grant.Subject) {
		s.detachGrantLocked(existing.Subject, grant.ID)
	}

	if grant.Kind == "" {
		grant.Kind = GrantKindOverride
	}
	if grant.Version <= 0 {
		grant.Version = 1
	}
	if grant.Features == nil {
		grant.Features = map[FeatureKey]GrantEffect{}
	}
	if grant.Quotas == nil {
		grant.Quotas = map[string]QuotaRule{}
	}
	grant.Features = copyFeatureRules(grant.Features)
	grant.Quotas = copyQuotaRules(grant.Quotas)
	grant.Metadata = copyStringMap(grant.Metadata)
	if grant.CreatedAt.IsZero() {
		grant.CreatedAt = now
	}
	grant.UpdatedAt = now

	s.grants[grant.ID] = grant
	s.attachGrantLocked(grant.Subject, grant.ID)
	return nil
}

func (s *service) RevokeGrant(_ context.Context, grantID string) error {
	const op serrors.Op = "SubscriptionEngine.RevokeGrant"

	if strings.TrimSpace(grantID) == "" {
		return serrors.E(op, ErrGrantIDRequired)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	grant, ok := s.grants[grantID]
	if !ok {
		return serrors.E(op, ErrGrantNotFound)
	}
	delete(s.grants, grantID)
	s.detachGrantLocked(grant.Subject, grantID)
	return nil
}

func (s *service) ListGrants(_ context.Context, subject SubjectRef) ([]Grant, error) {
	const op serrors.Op = "SubscriptionEngine.ListGrants"

	if err := validateSubjectRef(subject); err != nil {
		return nil, serrors.E(op, err)
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	ids := s.grantsBySubject[subjectRefKey(subject)]
	grants := make([]Grant, 0, len(ids))
	for grantID := range ids {
		grant, ok := s.grants[grantID]
		if !ok {
			continue
		}
		grants = append(grants, cloneGrant(grant))
	}
	sort.SliceStable(grants, func(i, j int) bool {
		if grants[i].UpdatedAt.Equal(grants[j].UpdatedAt) {
			return grants[i].ID < grants[j].ID
		}
		return grants[i].UpdatedAt.After(grants[j].UpdatedAt)
	})
	return grants, nil
}

func (s *service) AssignPlan(ctx context.Context, subject SubjectRef, planID string) error {
	const op serrors.Op = "SubscriptionEngine.AssignPlan"

	if err := validateSubjectRef(subject); err != nil {
		return serrors.E(op, err)
	}
	if _, ok := s.plans[planID]; !ok {
		return serrors.E(op, ErrPlanNotFound)
	}

	grant := Grant{
		ID:      fmt.Sprintf("plan:%s:%s", subject.Scope, subject.ID.String()),
		Kind:    GrantKindPlan,
		Subject: subject,
		PlanID:  planID,
	}
	if err := s.UpsertGrant(ctx, grant); err != nil {
		return serrors.E(op, err)
	}
	return nil
}

func (s *service) CurrentPlan(_ context.Context, subject Subject) (PlanInfo, error) {
	const op serrors.Op = "SubscriptionEngine.CurrentPlan"

	if err := validateSubject(subject); err != nil {
		return PlanInfo{}, serrors.E(op, err)
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	planID, planDef, _ := s.resolvePlanLocked(subject)
	return PlanInfo{
		ID:          planID,
		DisplayName: planDef.DisplayName,
		Description: planDef.Description,
	}, nil
}

func (s *service) EvaluateFeature(_ context.Context, subject Subject, feature FeatureKey) (Decision, error) {
	const op serrors.Op = "SubscriptionEngine.EvaluateFeature"

	if err := validateSubject(subject); err != nil {
		return Decision{}, serrors.E(op, err)
	}
	if strings.TrimSpace(string(feature)) == "" {
		return Decision{}, serrors.E(op, fmt.Errorf("feature is required"))
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	planID, planDef, planSource := s.resolvePlanLocked(subject)
	grants := s.applicableGrantsLocked(subject)
	for _, grant := range grants {
		effect, ok := grant.Features[feature]
		if !ok {
			continue
		}
		src := grantToSource(grant, "feature")
		if effect == GrantEffectDeny {
			return Decision{
				Allowed: false,
				Subject: subject.Ref(),
				Feature: feature,
				PlanID:  planID,
				Reason:  "explicitly denied by grant",
				Version: decisionSchemaVersion,
				Sources: []DecisionSource{src},
			}, nil
		}
		return Decision{
			Allowed: true,
			Subject: subject.Ref(),
			Feature: feature,
			PlanID:  planID,
			Reason:  "explicitly allowed by grant",
			Version: decisionSchemaVersion,
			Sources: []DecisionSource{src},
		}, nil
	}

	if containsString(planDef.Features, string(feature)) {
		return Decision{
			Allowed: true,
			Subject: subject.Ref(),
			Feature: feature,
			PlanID:  planID,
			Reason:  "allowed by plan",
			Version: decisionSchemaVersion,
			Sources: []DecisionSource{planSource},
		}, nil
	}
	return Decision{
		Allowed: false,
		Subject: subject.Ref(),
		Feature: feature,
		PlanID:  planID,
		Reason:  "feature is not granted",
		Version: decisionSchemaVersion,
		Sources: []DecisionSource{planSource},
	}, nil
}

func (s *service) EvaluateLimit(_ context.Context, subject Subject, quota QuotaKey) (LimitDecision, error) {
	const op serrors.Op = "SubscriptionEngine.EvaluateLimit"

	if err := validateSubject(subject); err != nil {
		return LimitDecision{}, serrors.E(op, err)
	}
	if err := validateQuota(quota); err != nil {
		return LimitDecision{}, serrors.E(op, err)
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.evaluateLimitLocked(subject, quota), nil
}

func (s *service) Reserve(_ context.Context, subject Subject, quota QuotaKey, amount int, token string) (Reservation, error) {
	const op serrors.Op = "SubscriptionEngine.Reserve"

	if err := validateSubject(subject); err != nil {
		return Reservation{}, serrors.E(op, err)
	}
	if err := validateQuota(quota); err != nil {
		return Reservation{}, serrors.E(op, err)
	}
	if amount <= 0 {
		return Reservation{}, serrors.E(op, fmt.Errorf("reservation amount must be positive"))
	}
	if strings.TrimSpace(token) == "" {
		return Reservation{}, serrors.E(op, fmt.Errorf("reservation token is required"))
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	s.cleanupExpiredReservationsLocked()

	if existingID, ok := s.reservationByToken[token]; ok {
		existing, exists := s.reservations[existingID]
		if !exists {
			delete(s.reservationByToken, token)
		} else if existing.Subject == subject.Ref() && existing.Quota == quota && existing.Amount == amount {
			return cloneReservation(*existing), nil
		} else {
			return Reservation{}, serrors.E(op, fmt.Errorf("reservation token conflict"))
		}
	}

	limitDecision := s.evaluateLimitLocked(subject, quota)
	if limitDecision.Limit >= 0 && limitDecision.Current+amount > limitDecision.Limit {
		return Reservation{}, serrors.E(op, ErrLimitExceeded{
			Quota:   quota,
			Current: limitDecision.Current,
			Limit:   limitDecision.Limit,
		})
	}

	now := s.now()
	reservation := Reservation{
		ID:         uuid.NewString(),
		Token:      token,
		Subject:    subject.Ref(),
		Quota:      quota,
		Amount:     amount,
		Status:     ReservationPending,
		ReservedAt: now,
		ExpiresAt:  now.Add(s.cfg.ReservationTTL),
	}

	s.reservations[reservation.ID] = &reservation
	s.reservationByToken[token] = reservation.ID
	return cloneReservation(reservation), nil
}

func (s *service) Commit(_ context.Context, reservationID string) error {
	const op serrors.Op = "SubscriptionEngine.Commit"

	if strings.TrimSpace(reservationID) == "" {
		return serrors.E(op, ErrReservationNotFound)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	s.cleanupExpiredReservationsLocked()

	reservation, ok := s.reservations[reservationID]
	if !ok {
		return serrors.E(op, ErrReservationNotFound)
	}
	switch reservation.Status {
	case ReservationCommitted:
		return nil
	case ReservationReleased, ReservationExpired:
		return serrors.E(op, fmt.Errorf("reservation is not committable: %s", reservation.Status))
	}

	now := s.now()
	reservation.Status = ReservationCommitted
	reservation.CommittedAt = &now
	usageKey := usageKeyFor(reservation.Subject, reservation.Quota)
	s.usage[usageKey] += reservation.Amount
	return nil
}

func (s *service) Release(_ context.Context, reservationID string) error {
	const op serrors.Op = "SubscriptionEngine.Release"

	if strings.TrimSpace(reservationID) == "" {
		return serrors.E(op, ErrReservationNotFound)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	reservation, ok := s.reservations[reservationID]
	if !ok {
		return serrors.E(op, ErrReservationNotFound)
	}
	if reservation.Status == ReservationReleased {
		return nil
	}
	now := s.now()
	if reservation.Status == ReservationCommitted {
		usageKey := usageKeyFor(reservation.Subject, reservation.Quota)
		s.usage[usageKey] = max(s.usage[usageKey]-reservation.Amount, 0)
	}
	reservation.Status = ReservationReleased
	reservation.ReleasedAt = &now
	return nil
}

func (s *service) SetUsage(_ context.Context, subject SubjectRef, quota QuotaKey, amount int) error {
	const op serrors.Op = "SubscriptionEngine.SetUsage"

	if err := validateSubjectRef(subject); err != nil {
		return serrors.E(op, err)
	}
	if err := validateQuota(quota); err != nil {
		return serrors.E(op, err)
	}
	if amount < 0 {
		return serrors.E(op, fmt.Errorf("usage cannot be negative"))
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	s.usage[usageKeyFor(subject, quota)] = amount
	return nil
}

func (s *service) evaluateLimitLocked(subject Subject, quota QuotaKey) LimitDecision {
	planID, planDef, planSource := s.resolvePlanLocked(subject)

	limit := -1
	sources := []DecisionSource{planSource}
	if planLimit, ok := planLimitForQuota(planDef, quota); ok {
		limit = planLimit
	}

	for _, grant := range s.applicableGrantsLocked(subject) {
		rule, ok := matchQuotaRule(grant, quota)
		if !ok {
			continue
		}
		sources = append(sources, grantToSource(grant, "quota"))
		if rule.Effect == GrantEffectDeny {
			limit = 0
			break
		}
		mode := rule.Mode
		if mode == "" {
			mode = QuotaModeAbsolute
		}
		switch mode {
		case QuotaModeAdditive:
			if limit < 0 || rule.Limit < 0 {
				limit = -1
				continue
			}
			limit += rule.Limit
		default:
			limit = rule.Limit
		}
	}

	current := s.currentUsageLocked(subject.Ref(), quota)
	if limit >= 0 {
		current += s.pendingUsageLocked(subject.Ref(), quota)
	}

	remaining := -1
	allowed := true
	reason := "unlimited quota"
	if limit >= 0 {
		remaining = max(limit-current, 0)
		allowed = current < limit
		if allowed {
			reason = "within quota"
		} else {
			reason = "quota exceeded"
		}
	}

	return LimitDecision{
		Allowed:   allowed,
		Subject:   subject.Ref(),
		Quota:     quota,
		Current:   current,
		Limit:     limit,
		Remaining: remaining,
		PlanID:    planID,
		Reason:    reason,
		Version:   decisionSchemaVersion,
		Sources:   sources,
	}
}

func (s *service) resolvePlanLocked(subject Subject) (string, PlanDefinition, DecisionSource) {
	for _, grant := range s.applicableGrantsLocked(subject) {
		if grant.PlanID == "" {
			continue
		}
		plan, ok := s.plans[grant.PlanID]
		if !ok {
			continue
		}
		return grant.PlanID, plan, grantToSource(grant, "assigned plan")
	}

	plan, ok := s.plans[s.cfg.DefaultPlan]
	if !ok {
		for planID, fallback := range s.plans {
			return planID, fallback, DecisionSource{
				GrantID:  "plan:fallback",
				Kind:     GrantKindDefault,
				Priority: kindPriority(GrantKindDefault),
				Reason:   "fallback default plan",
			}
		}
	}
	return s.cfg.DefaultPlan, plan, DecisionSource{
		GrantID:  "plan:default",
		Kind:     GrantKindDefault,
		Priority: kindPriority(GrantKindDefault),
		Reason:   "configured default plan",
	}
}

func (s *service) applicableGrantsLocked(subject Subject) []Grant {
	refs := make([]SubjectRef, 0, len(subject.Parents)+2)
	refs = append(refs, subject.Ref())
	refs = append(refs, subject.Parents...)
	refs = append(refs, SubjectRef{Scope: ScopeGlobal, ID: uuid.Nil})

	now := s.now()
	seen := make(map[string]struct{})
	grants := make([]Grant, 0)
	for _, ref := range refs {
		ids := s.grantsBySubject[subjectRefKey(ref)]
		for grantID := range ids {
			if _, ok := seen[grantID]; ok {
				continue
			}
			seen[grantID] = struct{}{}
			grant, ok := s.grants[grantID]
			if !ok {
				continue
			}
			if grant.ExpiresAt != nil && now.After(*grant.ExpiresAt) {
				continue
			}
			grants = append(grants, grant)
		}
	}

	sort.SliceStable(grants, func(i, j int) bool {
		pi := effectivePriority(grants[i])
		pj := effectivePriority(grants[j])
		if pi == pj {
			if grants[i].UpdatedAt.Equal(grants[j].UpdatedAt) {
				return grants[i].ID < grants[j].ID
			}
			return grants[i].UpdatedAt.After(grants[j].UpdatedAt)
		}
		return pi > pj
	})
	return grants
}

func (s *service) currentUsageLocked(subject SubjectRef, quota QuotaKey) int {
	return s.usage[usageKeyFor(subject, quota)]
}

func (s *service) pendingUsageLocked(subject SubjectRef, quota QuotaKey) int {
	total := 0
	now := s.now()
	for _, reservation := range s.reservations {
		if reservation.Status != ReservationPending {
			continue
		}
		if now.After(reservation.ExpiresAt) {
			continue
		}
		if reservation.Subject == subject && reservation.Quota == quota {
			total += reservation.Amount
		}
	}
	return total
}

func (s *service) cleanupExpiredReservationsLocked() {
	now := s.now()
	for _, reservation := range s.reservations {
		if reservation.Status != ReservationPending {
			continue
		}
		if now.After(reservation.ExpiresAt) {
			reservation.Status = ReservationExpired
			reservation.ReleasedAt = &now
		}
	}
}

func (s *service) attachGrantLocked(subject SubjectRef, grantID string) {
	key := subjectRefKey(subject)
	if _, ok := s.grantsBySubject[key]; !ok {
		s.grantsBySubject[key] = map[string]struct{}{}
	}
	s.grantsBySubject[key][grantID] = struct{}{}
}

func (s *service) detachGrantLocked(subject SubjectRef, grantID string) {
	key := subjectRefKey(subject)
	if _, ok := s.grantsBySubject[key]; !ok {
		return
	}
	delete(s.grantsBySubject[key], grantID)
	if len(s.grantsBySubject[key]) == 0 {
		delete(s.grantsBySubject, key)
	}
}

func resolvePlans(cfg Config) (map[string]PlanDefinition, error) {
	const op serrors.Op = "SubscriptionEngine.resolvePlans"

	byPlan := make(map[string]PlanDefinition, len(cfg.Plans))
	for _, plan := range cfg.Plans {
		if strings.TrimSpace(plan.PlanID) == "" {
			continue
		}
		if plan.EntityLimits == nil {
			plan.EntityLimits = map[string]int{}
		}
		byPlan[plan.PlanID] = plan
	}
	if _, ok := byPlan[cfg.DefaultPlan]; !ok {
		byPlan[cfg.DefaultPlan] = PlanDefinition{
			PlanID:       cfg.DefaultPlan,
			DisplayName:  cfg.DefaultPlan,
			EntityLimits: map[string]int{},
			Features:     []string{},
		}
	}

	resolved := make(map[string]PlanDefinition, len(byPlan))
	visiting := make(map[string]bool, len(byPlan))

	var dfs func(string) (PlanDefinition, error)
	dfs = func(planID string) (PlanDefinition, error) {
		if plan, ok := resolved[planID]; ok {
			return plan, nil
		}
		if visiting[planID] {
			return PlanDefinition{}, serrors.E(op, fmt.Errorf("plan inheritance cycle detected: %s", planID))
		}

		current, ok := byPlan[planID]
		if !ok {
			return PlanDefinition{}, serrors.E(op, fmt.Errorf("plan not found: %s", planID))
		}
		visiting[planID] = true

		featureSet := map[string]struct{}{}
		limits := map[string]int{}
		var seatLimit *int
		if current.ParentPlanID != "" {
			parent, err := dfs(current.ParentPlanID)
			if err != nil {
				return PlanDefinition{}, err
			}
			for _, feature := range parent.Features {
				featureSet[feature] = struct{}{}
			}
			for key, value := range parent.EntityLimits {
				limits[key] = value
			}
			seatLimit = cloneIntPtr(parent.SeatLimit)
		}
		for _, feature := range current.Features {
			featureSet[feature] = struct{}{}
		}
		for key, value := range current.EntityLimits {
			limits[key] = value
		}
		if current.SeatLimit != nil {
			seatLimit = cloneIntPtr(current.SeatLimit)
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

	for planID := range byPlan {
		if _, err := dfs(planID); err != nil {
			return nil, serrors.E(op, err)
		}
	}
	return resolved, nil
}

func planLimitForQuota(plan PlanDefinition, quota QuotaKey) (int, bool) {
	if strings.EqualFold(quota.Resource, "seats") && plan.SeatLimit != nil {
		return *plan.SeatLimit, true
	}

	if plan.EntityLimits == nil {
		return 0, false
	}
	if v, ok := plan.EntityLimits[quota.String()]; ok {
		return v, true
	}
	if v, ok := plan.EntityLimits[quota.Resource]; ok {
		return v, true
	}
	return 0, false
}

func matchQuotaRule(grant Grant, quota QuotaKey) (QuotaRule, bool) {
	if grant.Quotas == nil {
		return QuotaRule{}, false
	}
	if rule, ok := grant.Quotas[quota.String()]; ok {
		return rule, true
	}
	if rule, ok := grant.Quotas[quota.Resource]; ok {
		return rule, true
	}
	return QuotaRule{}, false
}

func grantToSource(grant Grant, reason string) DecisionSource {
	return DecisionSource{
		GrantID:  grant.ID,
		Kind:     grant.Kind,
		Priority: effectivePriority(grant),
		Reason:   reason,
	}
}

func effectivePriority(grant Grant) int {
	if grant.Priority != 0 {
		return grant.Priority
	}
	return kindPriority(grant.Kind)
}

func kindPriority(kind GrantKind) int {
	switch kind {
	case GrantKindDeny:
		return 600
	case GrantKindOverride:
		return 500
	case GrantKindAddOn:
		return 400
	case GrantKindPromo:
		return 300
	case GrantKindPlan:
		return 200
	default:
		return 100
	}
}

func usageKeyFor(subject SubjectRef, quota QuotaKey) string {
	return fmt.Sprintf("%s|%s|%s", subjectRefKey(subject), quota.String(), "usage")
}

func subjectRefKey(subject SubjectRef) string {
	return fmt.Sprintf("%s|%s", subject.Scope, subject.ID.String())
}

func validateSubject(subject Subject) error {
	return validateSubjectRef(subject.Ref())
}

func validateSubjectRef(subject SubjectRef) error {
	if subject.Scope == "" {
		return ErrSubjectRequired
	}
	if subject.Scope != ScopeGlobal && subject.ID == uuid.Nil {
		return ErrSubjectRequired
	}
	return nil
}

func validateQuota(quota QuotaKey) error {
	if strings.TrimSpace(quota.Resource) == "" {
		return ErrQuotaInvalid
	}
	if quota.Window == "" {
		quota.Window = WindowNone
	}
	return nil
}

func cloneGrant(grant Grant) Grant {
	out := grant
	out.Features = copyFeatureRules(grant.Features)
	out.Quotas = copyQuotaRules(grant.Quotas)
	out.Metadata = copyStringMap(grant.Metadata)
	out.ExpiresAt = cloneTimePtr(grant.ExpiresAt)
	return out
}

func cloneReservation(res Reservation) Reservation {
	out := res
	out.CommittedAt = cloneTimePtr(res.CommittedAt)
	out.ReleasedAt = cloneTimePtr(res.ReleasedAt)
	return out
}

func copyFeatureRules(in map[FeatureKey]GrantEffect) map[FeatureKey]GrantEffect {
	if len(in) == 0 {
		return map[FeatureKey]GrantEffect{}
	}
	out := make(map[FeatureKey]GrantEffect, len(in))
	for key, value := range in {
		out[key] = value
	}
	return out
}

func copyQuotaRules(in map[string]QuotaRule) map[string]QuotaRule {
	if len(in) == 0 {
		return map[string]QuotaRule{}
	}
	out := make(map[string]QuotaRule, len(in))
	for key, value := range in {
		out[key] = value
	}
	return out
}

func copyStringMap(in map[string]string) map[string]string {
	if len(in) == 0 {
		return map[string]string{}
	}
	out := make(map[string]string, len(in))
	for key, value := range in {
		out[key] = value
	}
	return out
}

func cloneTimePtr(t *time.Time) *time.Time {
	if t == nil {
		return nil
	}
	val := *t
	return &val
}

func cloneIntPtr(v *int) *int {
	if v == nil {
		return nil
	}
	val := *v
	return &val
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}
