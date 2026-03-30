// Package spotlight provides this package.
package spotlight

import (
	"context"
	"sort"
	"sync"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
)

type ProviderCapabilities struct {
	SupportsWatch bool
	EntityTypes   []string
	IndexPriority int
}

const ProviderStreamBatchSize = 500

type ProviderScope struct {
	TenantID uuid.UUID
	Language string
	Query    string
	TopK     int
}

type DocumentBatchEmitter func([]SearchDocument) error

type SearchProvider interface {
	ProviderID() string
	Capabilities() ProviderCapabilities
	StreamDocuments(ctx context.Context, scope ProviderScope, emit DocumentBatchEmitter) error
	Watch(ctx context.Context, scope ProviderScope) (<-chan DocumentEvent, error)
}

func CollectDocumentStream(_ context.Context, streamer func(DocumentBatchEmitter) error) ([]SearchDocument, error) {
	var out []SearchDocument
	if err := streamer(func(batch []SearchDocument) error {
		out = append(out, batch...)
		return nil
	}); err != nil {
		return nil, err
	}
	return out, nil
}

func CollectDocuments(ctx context.Context, provider SearchProvider, scope ProviderScope) ([]SearchDocument, error) {
	if provider == nil {
		return nil, nil
	}
	return CollectDocumentStream(ctx, func(emit DocumentBatchEmitter) error {
		return provider.StreamDocuments(ctx, scope, emit)
	})
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
	sort.SliceStable(out, func(i, j int) bool {
		left := out[i]
		right := out[j]
		leftPriority := left.Capabilities().IndexPriority
		rightPriority := right.Capabilities().IndexPriority
		if leftPriority != rightPriority {
			return leftPriority > rightPriority
		}
		return left.ProviderID() < right.ProviderID()
	})
	return out
}
