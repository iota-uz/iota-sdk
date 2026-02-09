package dataset

import (
	"fmt"
	"sort"
	"strings"
	"sync"
)

type Registry struct {
	mu       sync.RWMutex
	datasets map[string]Dataset
}

func NewRegistry(datasets ...Dataset) (*Registry, error) {
	r := &Registry{
		datasets: make(map[string]Dataset, len(datasets)),
	}
	if err := r.Register(datasets...); err != nil {
		return nil, err
	}
	return r, nil
}

func MustNewRegistry(datasets ...Dataset) *Registry {
	r, err := NewRegistry(datasets...)
	if err != nil {
		panic(err)
	}
	return r
}

func DefaultRegistry() *Registry {
	return MustNewRegistry(newAnalyticsBaselineDataset())
}

func (r *Registry) Get(datasetID string) (Dataset, error) {
	if r == nil {
		return nil, fmt.Errorf("registry is nil")
	}
	key := strings.TrimSpace(datasetID)
	if key == "" {
		return nil, fmt.Errorf("dataset id is required")
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	ds, ok := r.datasets[key]
	if !ok {
		return nil, fmt.Errorf("dataset %q is not registered", key)
	}
	return ds, nil
}

func (r *Registry) Register(datasets ...Dataset) error {
	if r == nil {
		return fmt.Errorf("registry is nil")
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if r.datasets == nil {
		r.datasets = make(map[string]Dataset, len(datasets))
	}

	for _, ds := range datasets {
		if ds == nil {
			return fmt.Errorf("dataset is nil")
		}
		id := strings.TrimSpace(ds.ID())
		if id == "" {
			return fmt.Errorf("dataset id is required")
		}
		if _, exists := r.datasets[id]; exists {
			return fmt.Errorf("dataset %q already registered", id)
		}
		r.datasets[id] = ds
	}

	return nil
}

func (r *Registry) IDs() []string {
	if r == nil {
		return nil
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	ids := make([]string, 0, len(r.datasets))
	for id := range r.datasets {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}
