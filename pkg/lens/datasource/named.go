package datasource

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/iota-uz/iota-sdk/pkg/lens/frame"
)

// NamedQuery resolves a host-registered domain query. The request carries the
// same frozen variables, time range, locale, and data scope-derived parameters
// as every other Lens datasource request; Text is the stable registration key.
type NamedQuery func(context.Context, QueryRequest) (*frame.FrameSet, error)

// NamedDataSource exposes domain queries to Lens without putting dashboard
// orchestration or SQL in the dashboard declaration. It is safe to share
// across requests; registration is normally completed during application
// startup.
type NamedDataSource struct {
	mu      sync.RWMutex
	queries map[string]NamedQuery
}

func NewNamedDataSource() *NamedDataSource {
	return &NamedDataSource{queries: make(map[string]NamedQuery)}
}

func (d *NamedDataSource) Register(name string, query NamedQuery) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return fmt.Errorf("named query key is required")
	}
	if query == nil {
		return fmt.Errorf("named query %q has no handler", name)
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	if _, exists := d.queries[name]; exists {
		return fmt.Errorf("named query %q is already registered", name)
	}
	d.queries[name] = query
	return nil
}

func (d *NamedDataSource) Capabilities() CapabilitySet {
	return CapabilitySet{
		CapabilityParameterizedQueries: true,
		CapabilityTimeouts:             true,
		CapabilityQueryKinds:           true,
	}
}

func (d *NamedDataSource) Run(ctx context.Context, req QueryRequest) (*frame.FrameSet, error) {
	if req.Kind != QueryKindNamed {
		return nil, fmt.Errorf("named datasource does not support query kind %q", req.Kind)
	}
	name := strings.TrimSpace(req.Text)
	d.mu.RLock()
	query := d.queries[name]
	d.mu.RUnlock()
	if query == nil {
		return nil, fmt.Errorf("named query %q is not registered", name)
	}
	return query(ctx, req)
}
