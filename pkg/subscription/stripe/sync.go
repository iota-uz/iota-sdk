package stripe

import (
	"context"
	"sort"
	"time"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
	subrepo "github.com/iota-uz/iota-sdk/pkg/subscription/repository"
)

type CacheInvalidator interface {
	InvalidateCache(ctx context.Context, tenantID uuid.UUID) error
}

type Config struct {
	SecretKey       string
	GracePeriodDays int
	DefaultTier     string
}

type Service struct {
	cfg         Config
	repo        subrepo.Repository
	client      EntitlementsClient
	invalidator CacheInvalidator
	now         func() time.Time
}

func NewService(cfg Config, repo subrepo.Repository, invalidator CacheInvalidator, client EntitlementsClient) *Service {
	if cfg.GracePeriodDays <= 0 {
		cfg.GracePeriodDays = 7
	}
	if cfg.DefaultTier == "" {
		cfg.DefaultTier = "FREE"
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
			_ = s.invalidator.InvalidateCache(ctx, tenantID)
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
	var endsAt *time.Time
	if active {
		t := s.now().AddDate(0, 0, s.cfg.GracePeriodDays)
		endsAt = &t
	}
	if err := s.repo.SetGracePeriod(ctx, tenantID, active, endsAt); err != nil {
		return err
	}
	if s.invalidator != nil {
		if err := s.invalidator.InvalidateCache(ctx, tenantID); err != nil {
			return err
		}
	}
	return nil
}
