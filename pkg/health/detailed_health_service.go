package health

import (
	"context"
	"sync"
	"time"
)

type CheckFunc func(ctx context.Context) HealthCheck

type DetailedHealthService interface {
	GetDetailedHealth(ctx context.Context) *DetailedHealth
}

type DetailedHealthServiceConfig struct {
	Checks       map[string]CheckFunc
	Capabilities CapabilityService
	CacheTTL     time.Duration
}

type detailedHealthServiceImpl struct {
	checks       map[string]CheckFunc
	capabilities CapabilityService
	cacheTTL     time.Duration
	cacheMu      sync.RWMutex
	cacheAt      time.Time
	cacheValue   *DetailedHealth
}

func NewDetailedHealthService(cfg DetailedHealthServiceConfig) DetailedHealthService {
	checks := cfg.Checks
	if checks == nil {
		checks = map[string]CheckFunc{}
	}
	capabilities := cfg.Capabilities
	if capabilities == nil {
		capabilities = NewCapabilityService(nil)
	}

	cacheTTL := cfg.CacheTTL
	if cacheTTL <= 0 {
		cacheTTL = 3 * time.Second
	}

	return &detailedHealthServiceImpl{
		checks:       checks,
		capabilities: capabilities,
		cacheTTL:     cacheTTL,
	}
}

func (s *detailedHealthServiceImpl) GetDetailedHealth(ctx context.Context) *DetailedHealth {
	if cached := s.loadFromCache(); cached != nil {
		return cached
	}

	checks := make(map[string]HealthCheck, len(s.checks))
	for key, check := range s.checks {
		if check == nil {
			continue
		}
		checks[key] = check(ctx)
	}

	health := &DetailedHealth{
		Status:       aggregateStatus(checks),
		Timestamp:    time.Now().UTC(),
		Checks:       checks,
		Capabilities: s.capabilities.GetCapabilities(ctx),
	}

	s.storeInCache(health)
	return cloneDetailedHealth(health)
}

func aggregateStatus(checks map[string]HealthCheck) Status {
	status := StatusHealthy
	for _, check := range checks {
		if check.Status == StatusDown {
			return StatusDown
		}
		if check.Status == StatusDegraded {
			status = StatusDegraded
		}
	}

	return status
}

func (s *detailedHealthServiceImpl) loadFromCache() *DetailedHealth {
	s.cacheMu.RLock()
	defer s.cacheMu.RUnlock()

	if s.cacheValue == nil {
		return nil
	}
	if time.Since(s.cacheAt) > s.cacheTTL {
		return nil
	}

	return cloneDetailedHealth(s.cacheValue)
}

func (s *detailedHealthServiceImpl) storeInCache(health *DetailedHealth) {
	s.cacheMu.Lock()
	defer s.cacheMu.Unlock()

	s.cacheAt = time.Now()
	s.cacheValue = cloneDetailedHealth(health)
}

func cloneDetailedHealth(health *DetailedHealth) *DetailedHealth {
	if health == nil {
		return nil
	}

	checks := make(map[string]HealthCheck, len(health.Checks))
	for key, check := range health.Checks {
		cloned := check
		if check.Details != nil {
			cloned.Details = make(map[string]any, len(check.Details))
			for detailKey, detailValue := range check.Details {
				cloned.Details[detailKey] = detailValue
			}
		}
		checks[key] = cloned
	}

	capabilities := make([]Capability, len(health.Capabilities))
	copy(capabilities, health.Capabilities)

	return &DetailedHealth{
		Status:       health.Status,
		Timestamp:    health.Timestamp,
		Checks:       checks,
		Capabilities: capabilities,
	}
}
