package stripe

import (
	"context"
	"fmt"
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
	repo        subrepo.StripeAwareRepository
	client      EntitlementsClient
	invalidator CacheInvalidator
	now         func() time.Time
	webhookSeen atomic.Uint64
	webhookDup  atomic.Uint64
	graceFlips  atomic.Uint64
}

func NewService(
	cfg Config,
	repo subrepo.StripeAwareRepository,
	invalidator CacheInvalidator,
	client EntitlementsClient,
) (*Service, error) {
	if repo == nil {
		return nil, fmt.Errorf("subscription stripe service requires repository")
	}
	if client == nil && cfg.SecretKey == "" {
		return nil, fmt.Errorf("subscription stripe service requires secret key when client is nil")
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
	}, nil
}

func (s *Service) RefreshTenant(ctx context.Context, tenantID uuid.UUID) error {
	const op serrors.Op = "SubscriptionStripeService.RefreshTenant"

	refs, err := s.repo.GetStripeReferences(ctx, tenantID)
	if err != nil {
		return serrors.E(op, err)
	}
	if refs.CustomerID == nil || *refs.CustomerID == "" {
		if s.invalidator != nil {
			if err := s.invalidator.InvalidateCache(ctx, tenantID); err != nil {
				return serrors.E(op, err)
			}
		}
		return nil
	}

	features, err := s.client.ListActiveEntitlements(ctx, *refs.CustomerID)
	if err != nil {
		return serrors.E(op, err)
	}
	sort.Strings(features)

	now := s.now()
	if err := s.repo.UpdateFeaturesAndSync(ctx, tenantID, features, now); err != nil {
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
