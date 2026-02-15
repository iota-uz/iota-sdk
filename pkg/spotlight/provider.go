package spotlight

import (
	"context"
	"sync"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
)

type ProviderCapabilities struct {
	SupportsWatch bool
	EntityTypes   []string
}

type ProviderScope struct {
	TenantID uuid.UUID
	Language string
	Query    string
	TopK     int
}

type SearchProvider interface {
	ProviderID() string
	Capabilities() ProviderCapabilities
	ListDocuments(ctx context.Context, scope ProviderScope) ([]SearchDocument, error)
	Watch(ctx context.Context, scope ProviderScope) (<-chan DocumentEvent, error)
}

type ProviderRegistry struct {
	mu        sync.RWMutex
	providers map[string]SearchProvider
}

func NewProviderRegistry() *ProviderRegistry {
	return &ProviderRegistry{providers: make(map[string]SearchProvider)}
}

func (r *ProviderRegistry) Register(provider SearchProvider) {
	if provider == nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.providers[provider.ProviderID()]; exists {
		logrus.WithField("provider_id", provider.ProviderID()).Warn("spotlight provider already registered, ignoring duplicate registration")
		return
	}
	r.providers[provider.ProviderID()] = provider
}

func (r *ProviderRegistry) Get(id string) (SearchProvider, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	p, ok := r.providers[id]
	return p, ok
}

func (r *ProviderRegistry) All() []SearchProvider {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]SearchProvider, 0, len(r.providers))
	for _, provider := range r.providers {
		out = append(out, provider)
	}
	return out
}
