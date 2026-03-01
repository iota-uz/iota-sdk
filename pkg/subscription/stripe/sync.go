package stripe

import (
	"context"
	"sort"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
	subrepo "github.com/iota-uz/iota-sdk/pkg/subscription/repository"
	"github.com/sirupsen/logrus"
)

type CacheInvalidator interface {
	InvalidateCache(ctx context.Context, tenantID uuid.UUID) error
}

type Config struct {
	SecretKey       string
	GracePeriodDays int
	DefaultPlan     string
}

type Service struct {
	cfg         Config
	repo        subrepo.Repository
	client      EntitlementsClient
	invalidator CacheInvalidator
	now         func() time.Time
	webhookSeen atomic.Uint64
	graceFlips  atomic.Uint64
}

func NewService(cfg Config, repo subrepo.Repository, invalidator CacheInvalidator, client EntitlementsClient) *Service {
	if repo == nil {
		panic("subscription stripe service requires repository")
	}
	if cfg.GracePeriodDays <= 0 {
		cfg.GracePeriodDays = 7
	}
	if cfg.DefaultPlan == "" {
		cfg.DefaultPlan = "FREE"
	}
	if client == nil {
		client = NewClient(cfg.SecretKey)
	}
	return &Service{
		cfg:         cfg,
		repo:        repo,
		client:      client,
		invalidator: invalidator,
		now:         func() time.Time { return time.Now().UTC() },
	}
}

func (s *Service) RefreshTenant(ctx context.Context, tenantID uuid.UUID) error {
	const op serrors.Op = "SubscriptionStripeService.RefreshTenant"

	entitlement, err := s.repo.GetEntitlement(ctx, tenantID)
	if err != nil {
		return serrors.E(op, err)
	}
	if entitlement.StripeCustomerID == nil || *entitlement.StripeCustomerID == "" {
		if s.invalidator != nil {
			if err := s.invalidator.InvalidateCache(ctx, tenantID); err != nil {
				return serrors.E(op, err)
			}
		}
		return nil
	}

	features, err := s.client.ListActiveEntitlements(ctx, *entitlement.StripeCustomerID)
	if err != nil {
		return serrors.E(op, err)
	}
	sort.Strings(features)

	now := s.now()
	entitlement.Features = features
	entitlement.LastSyncedAt = &now
	entitlement.UpdatedAt = now
	if err := s.repo.UpsertEntitlement(ctx, entitlement); err != nil {
		return serrors.E(op, err)
	}

	if s.invalidator != nil {
		if err := s.invalidator.InvalidateCache(ctx, tenantID); err != nil {
			return serrors.E(op, err)
		}
	}

	return nil
}

func (s *Service) setGracePeriod(ctx context.Context, tenantID uuid.UUID, active bool) error {
	const op serrors.Op = "SubscriptionStripeService.setGracePeriod"

	var endsAt *time.Time
	if active {
		t := s.now().AddDate(0, 0, s.cfg.GracePeriodDays)
		endsAt = &t
	}
	if err := s.repo.SetGracePeriod(ctx, tenantID, active, endsAt); err != nil {
		return serrors.E(op, err)
	}
	total := s.graceFlips.Add(1)
	logrus.WithFields(logrus.Fields{
		"tenant_id":                 tenantID.String(),
		"in_grace_period":           active,
		"grace_transition_total":    total,
		"subscription_component":    "stripe_sync",
		"configured_grace_day_span": s.cfg.GracePeriodDays,
	}).Info("Subscription grace period state changed")
	if s.invalidator != nil {
		if err := s.invalidator.InvalidateCache(ctx, tenantID); err != nil {
			return serrors.E(op, err)
		}
	}
	return nil
}
