package subscription

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
	subrepo "github.com/iota-uz/iota-sdk/pkg/subscription/repository"
	subpostgres "github.com/iota-uz/iota-sdk/pkg/subscription/repository/postgres"
	substripe "github.com/iota-uz/iota-sdk/pkg/subscription/stripe"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/sirupsen/logrus"
	"github.com/stripe/stripe-go/v82"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

type EntitlementService interface {
	HasFeature(ctx context.Context, tenantID uuid.UUID, feature string) (bool, error)
	HasFeatures(ctx context.Context, tenantID uuid.UUID, features ...string) (map[string]bool, error)
	GetFeatures(ctx context.Context, tenantID uuid.UUID) ([]string, error)

	CheckLimit(ctx context.Context, tenantID uuid.UUID, entityType string) (*LimitResult, error)
	GetLimits(ctx context.Context, tenantID uuid.UUID) (map[string]Limit, error)
	IncrementCount(ctx context.Context, tenantID uuid.UUID, entityType string) error
	DecrementCount(ctx context.Context, tenantID uuid.UUID, entityType string) error
	SetCount(ctx context.Context, tenantID uuid.UUID, entityType string, count int) error

	CheckSeatLimit(ctx context.Context, tenantID uuid.UUID) (*LimitResult, error)
	AddSeat(ctx context.Context, tenantID uuid.UUID) error
	RemoveSeat(ctx context.Context, tenantID uuid.UUID) error

	GetTier(ctx context.Context, tenantID uuid.UUID) (*TierInfo, error)
	GetAllTiers(ctx context.Context) ([]TierDefinition, error)

	IsInGracePeriod(ctx context.Context, tenantID uuid.UUID) (bool, *time.Time, error)
	StartGracePeriod(ctx context.Context, tenantID uuid.UUID) error
	ClearGracePeriod(ctx context.Context, tenantID uuid.UUID) error

	RefreshEntitlements(ctx context.Context, tenantID uuid.UUID) error
	InvalidateCache(ctx context.Context, tenantID uuid.UUID) error
}

type StripeEventHandler interface {
	HandleStripeEvent(ctx context.Context, event stripe.Event) error
}

type StripeSyncer interface {
	RefreshTenant(ctx context.Context, tenantID uuid.UUID) error
}

type Option func(*service)

func WithRepository(repo subrepo.Repository) Option {
	return func(s *service) {
		s.repo = repo
	}
}

func WithCache(cache Cache) Option {
	return func(s *service) {
		s.cache = cache
	}
}

func WithStripeSyncer(syncer StripeSyncer) Option {
	return func(s *service) {
		s.syncer = syncer
	}
}

func WithStripeEventHandler(handler StripeEventHandler) Option {
	return func(s *service) {
		s.stripeHandler = handler
	}
}

type resolvedTier struct {
	Definition TierDefinition
	Features   map[string]struct{}
	Limits     map[string]int
	SeatLimit  *int
}

type cachedEntitlement struct {
	Tier         string            `json:"tier"`
	Features     []string          `json:"features"`
	EntityLimits map[string]int    `json:"entity_limits"`
	SeatLimit    *int              `json:"seat_limit"`
	InGrace      bool              `json:"in_grace"`
	GraceEndsAt  *time.Time        `json:"grace_ends_at"`
	ExpiresAt    *time.Time        `json:"expires_at"`
	RawFeatures  []string          `json:"raw_features"`
	RawLimits    map[string]int    `json:"raw_limits"`
	UpdatedAt    time.Time         `json:"updated_at"`
	LastSyncedAt *time.Time        `json:"last_synced_at"`
	StripeMeta   map[string]string `json:"stripe_meta,omitempty"`
}

type service struct {
	cfg           Config
	repo          subrepo.Repository
	cache         Cache
	tiers         map[string]resolvedTier
	tierList      []TierDefinition
	cacheTTL      time.Duration
	syncer        StripeSyncer
	stripeHandler StripeEventHandler
	now           func() time.Time
	cacheHits     atomic.Uint64
	cacheMisses   atomic.Uint64
	graceUpdates  atomic.Uint64
}

func NewService(cfg Config, db *pgxpool.Pool, opts ...Option) (EntitlementService, error) {
	cfg = cfg.normalized()

	tiers, tierList, err := resolveTiers(cfg)
	if err != nil {
		return nil, err
	}

	svc := &service{
		cfg:      cfg,
		repo:     subpostgres.NewRepository(db),
		cache:    NewMemoryCache(),
		tiers:    tiers,
		tierList: tierList,
		cacheTTL: cfg.cacheTTL(),
		now:      func() time.Time { return time.Now().UTC() },
	}

	switch strings.ToLower(cfg.Cache.Type) {
	case "", "memory":
		svc.cache = NewMemoryCache()
	case "redis":
		redisCache, err := NewRedisCache(cfg.Cache.RedisURL)
		if err != nil {
			return nil, err
		}
		svc.cache = redisCache
	default:
		return nil, fmt.Errorf("unsupported subscription cache type: %s", cfg.Cache.Type)
	}

	for _, opt := range opts {
		opt(svc)
	}

	if svc.stripeHandler == nil && cfg.Stripe.SecretKey != "" {
		stripeSvc := substripe.NewService(
			substripe.Config{
				SecretKey:       cfg.Stripe.SecretKey,
				GracePeriodDays: cfg.GracePeriodDays,
				DefaultTier:     cfg.DefaultTier,
			},
			svc.repo,
			svc,
			nil,
		)
		svc.stripeHandler = stripeSvc
		if svc.syncer == nil {
			svc.syncer = stripeSvc
		}
	}

	if err := svc.repo.UpsertPlans(context.Background(), toPlanModels(tierList)); err != nil {
		return nil, err
	}

	return svc, nil
}

func (s *service) HandleStripeEvent(ctx context.Context, event stripe.Event) error {
	if s.stripeHandler == nil {
		return nil
	}
	return s.stripeHandler.HandleStripeEvent(ctx, event)
}

func (s *service) resolveTier(tier string) (resolvedTier, error) {
	if tier == "" {
		tier = s.cfg.DefaultTier
	}
	resolved, ok := s.tiers[tier]
	if !ok {
		return resolvedTier{}, serrors.E("subscription.resolveTier", serrors.NotFound, ErrTierNotConfigured)
	}
	return resolved, nil
}

func (s *service) loadCachedEntitlement(ctx context.Context, tenantID uuid.UUID) (*cachedEntitlement, error) {
	payload, ok, err := s.cache.Get(ctx, tenantCacheKey(tenantID))
	if err != nil {
		return nil, err
	}
	if !ok {
		misses := s.cacheMisses.Add(1)
		logrus.WithFields(logrus.Fields{
			"tenant_id":          tenantID.String(),
			"cache_miss_total":   misses,
			"cache_lookup_scope": "subscription_entitlements",
		}).Debug("Subscription cache miss")
		return nil, nil
	}
	hits := s.cacheHits.Add(1)
	logrus.WithFields(logrus.Fields{
		"tenant_id":          tenantID.String(),
		"cache_hit_total":    hits,
		"cache_lookup_scope": "subscription_entitlements",
	}).Debug("Subscription cache hit")
	var entry cachedEntitlement
	if err := json.Unmarshal(payload, &entry); err != nil {
		return nil, err
	}
	return &entry, nil
}

func (s *service) storeCachedEntitlement(ctx context.Context, tenantID uuid.UUID, entry *cachedEntitlement) error {
	payload, err := json.Marshal(entry)
	if err != nil {
		return err
	}
	return s.cache.Set(ctx, tenantCacheKey(tenantID), payload, s.cacheTTL)
}

func (s *service) ensureEntitlement(ctx context.Context, tenantID uuid.UUID) (*subrepo.Entitlement, error) {
	entitlement, err := s.repo.GetEntitlement(ctx, tenantID)
	if err == nil {
		return entitlement, nil
	}
	if err != subrepo.ErrEntitlementNotFound {
		return nil, err
	}

	now := s.now()
	entitlement = &subrepo.Entitlement{
		TenantID:      tenantID,
		Tier:          s.cfg.DefaultTier,
		Features:      []string{},
		EntityLimits:  map[string]int{},
		CurrentSeats:  0,
		InGracePeriod: false,
		CreatedAt:     now,
		UpdatedAt:     now,
	}
	if err := s.repo.UpsertEntitlement(ctx, entitlement); err != nil {
		return nil, err
	}
	return entitlement, nil
}

func (s *service) entitlementState(ctx context.Context, tenantID uuid.UUID) (*cachedEntitlement, error) {
	cached, err := s.loadCachedEntitlement(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	if cached != nil {
		return cached, nil
	}

	entitlement, err := s.ensureEntitlement(ctx, tenantID)
	if err != nil {
		return nil, err
	}

	resolved, err := s.resolveTier(entitlement.Tier)
	if err != nil {
		return nil, err
	}

	featureSet := make(map[string]struct{}, len(resolved.Features)+len(entitlement.Features))
	for key := range resolved.Features {
		featureSet[key] = struct{}{}
	}
	for _, key := range entitlement.Features {
		featureSet[key] = struct{}{}
	}
	features := make([]string, 0, len(featureSet))
	for key := range featureSet {
		features = append(features, key)
	}
	sort.Strings(features)

	limits := make(map[string]int, len(resolved.Limits)+len(entitlement.EntityLimits))
	for key, val := range resolved.Limits {
		limits[key] = val
	}
	for key, val := range entitlement.EntityLimits {
		limits[key] = val
	}

	seatLimit := resolved.SeatLimit
	if entitlement.SeatLimit != nil {
		seatLimit = entitlement.SeatLimit
	}

	entry := &cachedEntitlement{
		Tier:         entitlement.Tier,
		Features:     features,
		EntityLimits: limits,
		SeatLimit:    seatLimit,
		InGrace:      entitlement.InGracePeriod,
		GraceEndsAt:  entitlement.GracePeriodEndsAt,
		ExpiresAt:    entitlement.StripeSubscriptionEnd,
		RawFeatures:  entitlement.Features,
		RawLimits:    entitlement.EntityLimits,
		UpdatedAt:    entitlement.UpdatedAt,
		LastSyncedAt: entitlement.LastSyncedAt,
	}
	if err := s.storeCachedEntitlement(ctx, tenantID, entry); err != nil {
		return nil, err
	}
	return entry, nil
}

func resolveTiers(cfg Config) (map[string]resolvedTier, []TierDefinition, error) {
	byTier := make(map[string]TierDefinition, len(cfg.Tiers))
	for _, tier := range cfg.Tiers {
		if tier.Tier == "" {
			continue
		}
		if tier.EntityLimits == nil {
			tier.EntityLimits = map[string]int{}
		}
		byTier[tier.Tier] = tier
	}
	if _, ok := byTier[cfg.DefaultTier]; !ok {
		byTier[cfg.DefaultTier] = TierDefinition{
			Tier:         cfg.DefaultTier,
			DisplayName:  cases.Title(language.English).String(strings.ToLower(cfg.DefaultTier)),
			EntityLimits: map[string]int{},
			Features:     []string{},
		}
	}

	resolved := make(map[string]resolvedTier, len(byTier))
	visiting := make(map[string]bool, len(byTier))

	var dfs func(string) (resolvedTier, error)
	dfs = func(tierKey string) (resolvedTier, error) {
		if tier, ok := resolved[tierKey]; ok {
			return tier, nil
		}
		if visiting[tierKey] {
			return resolvedTier{}, fmt.Errorf("subscription tier inheritance cycle detected: %s", tierKey)
		}
		tierDef, ok := byTier[tierKey]
		if !ok {
			return resolvedTier{}, fmt.Errorf("subscription tier not found: %s", tierKey)
		}
		visiting[tierKey] = true

		featureSet := make(map[string]struct{})
		limits := make(map[string]int)
		var seatLimit *int
		if tierDef.ParentTier != "" {
			parent, err := dfs(tierDef.ParentTier)
			if err != nil {
				return resolvedTier{}, err
			}
			for key := range parent.Features {
				featureSet[key] = struct{}{}
			}
			for key, value := range parent.Limits {
				limits[key] = value
			}
			seatLimit = parent.SeatLimit
		}
		for _, feature := range tierDef.Features {
			featureSet[feature] = struct{}{}
		}
		for key, value := range tierDef.EntityLimits {
			limits[key] = value
		}
		if tierDef.SeatLimit != nil {
			seatLimit = tierDef.SeatLimit
		}

		visiting[tierKey] = false
		resolvedTierDef := resolvedTier{
			Definition: tierDef,
			Features:   featureSet,
			Limits:     limits,
			SeatLimit:  seatLimit,
		}
		resolved[tierKey] = resolvedTierDef
		return resolvedTierDef, nil
	}

	for key := range byTier {
		if _, err := dfs(key); err != nil {
			return nil, nil, err
		}
	}

	list := make([]TierDefinition, 0, len(resolved))
	for key, tier := range resolved {
		def := tier.Definition
		def.Features = make([]string, 0, len(tier.Features))
		for feature := range tier.Features {
			def.Features = append(def.Features, feature)
		}
		sort.Strings(def.Features)
		def.EntityLimits = make(map[string]int, len(tier.Limits))
		for limitKey, value := range tier.Limits {
			def.EntityLimits[limitKey] = value
		}
		def.SeatLimit = tier.SeatLimit
		def.Tier = key
		list = append(list, def)
	}
	sort.SliceStable(list, func(i, j int) bool {
		if list[i].DisplayOrder == list[j].DisplayOrder {
			return list[i].Tier < list[j].Tier
		}
		return list[i].DisplayOrder < list[j].DisplayOrder
	})

	return resolved, list, nil
}

func toPlanModels(tiers []TierDefinition) []subrepo.Plan {
	plans := make([]subrepo.Plan, 0, len(tiers))
	for _, tier := range tiers {
		plans = append(plans, subrepo.Plan{
			Tier:         tier.Tier,
			DisplayName:  tier.DisplayName,
			Description:  tier.Description,
			PriceCents:   tier.PriceCents,
			Interval:     tier.Interval,
			Features:     tier.Features,
			EntityLimits: tier.EntityLimits,
			SeatLimit:    tier.SeatLimit,
			DisplayOrder: tier.DisplayOrder,
		})
	}
	return plans
}
