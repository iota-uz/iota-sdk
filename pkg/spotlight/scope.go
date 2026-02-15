package spotlight

import (
	"context"
	"sync"
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
}

func NewInMemoryScopeStore() *InMemoryScopeStore {
	return &InMemoryScopeStore{defaultCfg: ScopeConfig{EnabledProviders: map[string]bool{}}}
}

func (s *InMemoryScopeStore) Resolve(_ context.Context, _ SearchRequest) (ScopeConfig, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	enabled := make(map[string]bool, len(s.defaultCfg.EnabledProviders))
	for key, value := range s.defaultCfg.EnabledProviders {
		enabled[key] = value
	}
	return ScopeConfig{EnabledProviders: enabled}, nil
}

func (s *InMemoryScopeStore) SetEnabled(providerID string, enabled bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.defaultCfg.EnabledProviders[providerID] = enabled
}
