package spotlight

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

type registryOrderTestProvider struct {
	id         string
	priority   int
	entityType string
}

func (p registryOrderTestProvider) ProviderID() string {
	return p.id
}

func (p registryOrderTestProvider) Capabilities() ProviderCapabilities {
	return ProviderCapabilities{
		EntityTypes:   []string{p.entityType},
		IndexPriority: p.priority,
	}
}

func (p registryOrderTestProvider) StreamDocuments(context.Context, ProviderScope, DocumentBatchEmitter) error {
	return nil
}

func (p registryOrderTestProvider) Watch(context.Context, ProviderScope) (<-chan DocumentEvent, error) {
	ch := make(chan DocumentEvent)
	close(ch)
	return ch, nil
}

func TestProviderRegistryAllOrdersByPriorityThenID(t *testing.T) {
	registry := NewProviderRegistry()
	registry.Register(registryOrderTestProvider{id: "gamma", priority: 100, entityType: "gamma"})
	registry.Register(registryOrderTestProvider{id: "alpha", priority: 300, entityType: "alpha"})
	registry.Register(registryOrderTestProvider{id: "beta", priority: 300, entityType: "beta"})
	registry.Register(registryOrderTestProvider{id: "delta", priority: 50, entityType: "delta"})

	providers := registry.All()
	ids := make([]string, 0, len(providers))
	for _, provider := range providers {
		ids = append(ids, provider.ProviderID())
	}

	require.Equal(t, []string{"alpha", "beta", "gamma", "delta"}, ids)
}
