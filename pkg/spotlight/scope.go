package spotlight

import (
	"context"
	"sync"

	"github.com/google/uuid"
)

type ScopeConfig struct {
	EnabledProviders map[string]bool
}

type ScopeStore interface {
	Resolve(ctx context.Context, req SearchRequest) (ScopeConfig, error)
}

type InMemoryScopeStore struct {
	mu         sync.RWMutex
	defaultCfg ScopeConfig
	tenantCfg  map[uuid.UUID]ScopeConfig
}

func NewInMemoryScopeStore() *InMemoryScopeStore {
	return &InMemoryScopeStore{
		defaultCfg: ScopeConfig{EnabledProviders: map[string]bool{}},
		tenantCfg:  make(map[uuid.UUID]ScopeConfig),
	}
}

func (s *InMemoryScopeStore) Resolve(_ context.Context, req SearchRequest) (ScopeConfig, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	cfg := s.defaultCfg
	if tenantCfg, ok := s.tenantCfg[req.TenantID]; ok {
		cfg = tenantCfg
	}
	enabled := make(map[string]bool, len(cfg.EnabledProviders))
	for key, value := range cfg.EnabledProviders {
		enabled[key] = value
	}
	return ScopeConfig{EnabledProviders: enabled}, nil
}

func (s *InMemoryScopeStore) SetEnabled(providerID string, enabled bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.defaultCfg.EnabledProviders[providerID] = enabled
}

func (s *InMemoryScopeStore) SetEnabledForTenant(tenantID uuid.UUID, providerID string, enabled bool) {
	if tenantID == uuid.Nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	cfg, ok := s.tenantCfg[tenantID]
	if !ok {
		cfg = ScopeConfig{EnabledProviders: map[string]bool{}}
	}
	if cfg.EnabledProviders == nil {
		cfg.EnabledProviders = map[string]bool{}
	}
	cfg.EnabledProviders[providerID] = enabled
	s.tenantCfg[tenantID] = cfg
}
