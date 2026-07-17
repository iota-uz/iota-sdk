package runtime

import (
	"container/list"
	"context"
	"sync"
	"sync/atomic"
	"time"

	"github.com/iota-uz/iota-sdk/pkg/lens/frame"
)

// ExecutionSnapshot is the immutable, clone-safe result of resolving a Lens
// execution identity. It is shared by render, fragments, drill and export.
type ExecutionSnapshot struct {
	ID              string
	SpecFingerprint string
	Variables       map[string]any
	DataScope       string
	Locale          string
	Timezone        string
	Datasets        map[string]*frame.FrameSet
	Provenance      map[string]DatasetProvenance
	CreatedAt       time.Time
	ExpiresAt       time.Time
}

type DatasetProvenance struct {
	Dataset   string
	Source    string
	DependsOn []string
	Duration  time.Duration
}

func (s *ExecutionSnapshot) Clone() *ExecutionSnapshot {
	if s == nil {
		return nil
	}
	out := *s
	out.Variables = cloneMap(s.Variables)
	out.Datasets = make(map[string]*frame.FrameSet, len(s.Datasets))
	for name, frames := range s.Datasets {
		out.Datasets[name] = frames.Clone()
	}
	out.Provenance = make(map[string]DatasetProvenance, len(s.Provenance))
	for name, item := range s.Provenance {
		item.DependsOn = append([]string(nil), item.DependsOn...)
		out.Provenance[name] = item
	}
	return &out
}

type SnapshotStore interface {
	Load(context.Context, string) (*ExecutionSnapshot, bool)
	Save(context.Context, string, *ExecutionSnapshot, time.Duration)
	Invalidate(context.Context, string)
	Stats() CacheStats
}

type CacheStats struct{ Hits, Misses, Stores, Evictions, Expirations uint64 }

type MemoryStoreOptions struct {
	TTL        time.Duration
	MaxEntries int
	Clock      func() time.Time
}

type memoryEntry struct {
	key       string
	snapshot  *ExecutionSnapshot
	expiresAt time.Time
}
type MemorySnapshotStore struct {
	mu          sync.Mutex
	items       map[string]*list.Element
	lru         *list.List
	ttl         time.Duration
	max         int
	clock       func() time.Time
	hits        atomic.Uint64
	misses      atomic.Uint64
	stores      atomic.Uint64
	evictions   atomic.Uint64
	expirations atomic.Uint64
}

func NewMemorySnapshotStore(opts MemoryStoreOptions) *MemorySnapshotStore {
	if opts.TTL <= 0 {
		opts.TTL = 5 * time.Minute
	}
	if opts.MaxEntries <= 0 {
		opts.MaxEntries = 128
	}
	if opts.Clock == nil {
		opts.Clock = time.Now
	}
	return &MemorySnapshotStore{items: map[string]*list.Element{}, lru: list.New(), ttl: opts.TTL, max: opts.MaxEntries, clock: opts.Clock}
}

func (m *MemorySnapshotStore) Load(_ context.Context, key string) (*ExecutionSnapshot, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	element, ok := m.items[key]
	if !ok {
		m.misses.Add(1)
		return nil, false
	}
	entry := element.Value.(*memoryEntry)
	if !entry.expiresAt.After(m.clock()) {
		m.remove(element)
		m.expirations.Add(1)
		m.misses.Add(1)
		return nil, false
	}
	m.lru.MoveToFront(element)
	m.hits.Add(1)
	return entry.snapshot.Clone(), true
}

func (m *MemorySnapshotStore) Save(_ context.Context, key string, snapshot *ExecutionSnapshot, ttl time.Duration) {
	if snapshot == nil {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if ttl <= 0 {
		ttl = m.ttl
	}
	expiresAt := m.clock().Add(ttl)
	cloned := snapshot.Clone()
	cloned.ExpiresAt = expiresAt
	if existing, ok := m.items[key]; ok {
		entry := existing.Value.(*memoryEntry)
		entry.snapshot = cloned
		entry.expiresAt = expiresAt
		m.lru.MoveToFront(existing)
	} else {
		m.items[key] = m.lru.PushFront(&memoryEntry{key: key, snapshot: cloned, expiresAt: expiresAt})
	}
	m.stores.Add(1)
	for len(m.items) > m.max {
		m.remove(m.lru.Back())
		m.evictions.Add(1)
	}
}

func (m *MemorySnapshotStore) Invalidate(_ context.Context, namespace string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for key, element := range m.items {
		if namespace == "" || len(key) >= len(namespace) && key[:len(namespace)] == namespace {
			m.remove(element)
		}
	}
}
func (m *MemorySnapshotStore) remove(element *list.Element) {
	if element == nil {
		return
	}
	delete(m.items, element.Value.(*memoryEntry).key)
	m.lru.Remove(element)
}
func (m *MemorySnapshotStore) Stats() CacheStats {
	return CacheStats{m.hits.Load(), m.misses.Load(), m.stores.Load(), m.evictions.Load(), m.expirations.Load()}
}

func cloneMap(in map[string]any) map[string]any {
	out := make(map[string]any, len(in))
	for k, v := range in {
		out[k] = cloneValue(v)
	}
	return out
}
func cloneValue(v any) any {
	switch value := v.(type) {
	case []string:
		return append([]string(nil), value...)
	case []any:
		return append([]any(nil), value...)
	case map[string]any:
		return cloneMap(value)
	default:
		return value
	}
}
