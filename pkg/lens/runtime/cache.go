package runtime

import (
	"container/list"
	"context"
	"reflect"
	"strings"
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
		if frames != nil {
			out.Datasets[name] = frames.Clone()
		}
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
	Update(context.Context, string, time.Duration, func(*ExecutionSnapshot) *ExecutionSnapshot)
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

func (m *MemorySnapshotStore) Update(_ context.Context, key string, ttl time.Duration, update func(*ExecutionSnapshot) *ExecutionSnapshot) {
	if update == nil {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	var current *ExecutionSnapshot
	if element, ok := m.items[key]; ok {
		current = element.Value.(*memoryEntry).snapshot.Clone()
	}
	next := update(current)
	if next == nil {
		return
	}
	if ttl <= 0 {
		ttl = m.ttl
	}
	expiresAt := m.clock().Add(ttl)
	next = next.Clone()
	next.ExpiresAt = expiresAt
	if existing, ok := m.items[key]; ok {
		entry := existing.Value.(*memoryEntry)
		entry.snapshot, entry.expiresAt = next, expiresAt
		m.lru.MoveToFront(existing)
	} else {
		m.items[key] = m.lru.PushFront(&memoryEntry{key: key, snapshot: next, expiresAt: expiresAt})
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
		if namespace == "" || key == namespace || strings.HasPrefix(key, namespace+":") {
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
	cloned := cloneReflect(reflect.ValueOf(v))
	if !cloned.IsValid() {
		return nil
	}
	return cloned.Interface()
}

func cloneReflect(value reflect.Value) reflect.Value {
	if !value.IsValid() {
		return value
	}
	switch value.Kind() {
	case reflect.Interface:
		if value.IsNil() {
			return reflect.Zero(value.Type())
		}
		cloned := cloneReflect(value.Elem())
		out := reflect.New(value.Type()).Elem()
		out.Set(cloned)
		return out
	case reflect.Pointer:
		if value.IsNil() {
			return reflect.Zero(value.Type())
		}
		out := reflect.New(value.Type().Elem())
		out.Elem().Set(cloneReflect(value.Elem()))
		return out
	case reflect.Slice:
		if value.IsNil() {
			return reflect.Zero(value.Type())
		}
		out := reflect.MakeSlice(value.Type(), value.Len(), value.Len())
		for i := range value.Len() {
			out.Index(i).Set(cloneReflect(value.Index(i)))
		}
		return out
	case reflect.Array:
		out := reflect.New(value.Type()).Elem()
		for i := range value.Len() {
			out.Index(i).Set(cloneReflect(value.Index(i)))
		}
		return out
	case reflect.Map:
		if value.IsNil() {
			return reflect.Zero(value.Type())
		}
		out := reflect.MakeMapWithSize(value.Type(), value.Len())
		iter := value.MapRange()
		for iter.Next() {
			out.SetMapIndex(iter.Key(), cloneReflect(iter.Value()))
		}
		return out
	case reflect.Struct:
		for i := range value.NumField() {
			if value.Type().Field(i).PkgPath != "" {
				return value
			}
		}
		out := reflect.New(value.Type()).Elem()
		for i := range value.NumField() {
			out.Field(i).Set(cloneReflect(value.Field(i)))
		}
		return out
	case reflect.Invalid, reflect.Bool, reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr,
		reflect.Float32, reflect.Float64, reflect.Complex64, reflect.Complex128, reflect.Chan,
		reflect.Func, reflect.String, reflect.UnsafePointer:
		return value
	}
	return value
}
