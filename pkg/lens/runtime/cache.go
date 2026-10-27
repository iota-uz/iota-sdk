package runtime

import (
	"container/list"
	"context"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/shopspring/decimal"

	"github.com/iota-uz/iota-sdk/pkg/lens/frame"
	"github.com/iota-uz/iota-sdk/pkg/lens/resource"
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
	// Update atomically supplies a clone-safe snapshot to update and transfers
	// ownership of the callback result to the store.
	Update(context.Context, string, time.Duration, func(*ExecutionSnapshot) *ExecutionSnapshot)
	Invalidate(context.Context, string)
	Stats() CacheStats
}

type CacheStats struct{ Hits, Misses, Stores, Evictions, Expirations, Rejections uint64 }

type MemoryStoreOptions struct {
	Budget        *resource.Budget
	MaxEntryBytes int64
	TTL           time.Duration
	MaxEntries    int
	Clock         func() time.Time
}

type memoryEntry struct {
	reservation *resource.Reservation
	key         string
	snapshot    *ExecutionSnapshot
	expiresAt   time.Time
}
type MemorySnapshotStore struct {
	budget        *resource.Budget
	maxEntryBytes int64
	mu            sync.Mutex
	items         map[string]*list.Element
	lru           *list.List
	ttl           time.Duration
	max           int
	clock         func() time.Time
	rejections    atomic.Uint64
	hits          atomic.Uint64
	misses        atomic.Uint64
	stores        atomic.Uint64
	evictions     atomic.Uint64
	expirations   atomic.Uint64
}

// PayloadLimit lets runtime preflights use the same policy as retention.
func (m *MemorySnapshotStore) PayloadLimit() int64 { return m.maxEntryBytes }

func NewMemorySnapshotStore(opts MemoryStoreOptions) *MemorySnapshotStore {
	if opts.Budget == nil {
		opts.Budget = resource.NewBudget(resource.DefaultLimit)
	}
	if opts.MaxEntryBytes <= 0 {
		opts.MaxEntryBytes = resource.DefaultEntryLimit
	}
	if opts.TTL <= 0 {
		opts.TTL = 5 * time.Minute
	}
	if opts.MaxEntries <= 0 {
		opts.MaxEntries = 128
	}
	if opts.Clock == nil {
		opts.Clock = time.Now
	}
	return &MemorySnapshotStore{budget: opts.Budget, maxEntryBytes: opts.MaxEntryBytes, items: map[string]*list.Element{}, lru: list.New(), ttl: opts.TTL, max: opts.MaxEntries, clock: opts.Clock}
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

// Save preserves the interface's best-effort cache contract. SaveChecked reports admission errors.
func (m *MemorySnapshotStore) Save(ctx context.Context, key string, snapshot *ExecutionSnapshot, ttl time.Duration) {
	if err := m.SaveChecked(ctx, key, snapshot, ttl); err != nil {
		m.rejections.Add(1)
	}
}
func (m *MemorySnapshotStore) SaveChecked(ctx context.Context, key string, snapshot *ExecutionSnapshot, ttl time.Duration) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if snapshot == nil {
		return nil
	}
	size, err := estimateExecutionSnapshot(snapshot, m.maxEntryBytes)
	if err != nil {
		return err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.saveLocked(key, snapshot, ttl, size)
}
func (m *MemorySnapshotStore) saveLocked(key string, snapshot *ExecutionSnapshot, ttl time.Duration, size int64) error {
	for _, element := range m.items {
		if !element.Value.(*memoryEntry).expiresAt.After(m.clock()) {
			m.remove(element)
			m.expirations.Add(1)
		}
	}
	var previous *resource.Reservation
	if element := m.items[key]; element != nil {
		previous = element.Value.(*memoryEntry).reservation
	}
	if size > m.budget.Limit() {
		return resource.NewLimitError("entry bytes", size, m.budget.Limit())
	}
	reservation, err := m.reserveLocked(previous, size, m.items[key])
	if err != nil {
		return err
	}
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
		entry.reservation = reservation
		m.lru.MoveToFront(existing)
	} else {
		m.items[key] = m.lru.PushFront(&memoryEntry{key: key, snapshot: cloned, expiresAt: expiresAt, reservation: reservation})
	}
	m.stores.Add(1)
	for len(m.items) > m.max {
		m.remove(m.lru.Back())
		m.evictions.Add(1)
	}
	return nil
}
func (m *MemorySnapshotStore) Update(ctx context.Context, key string, ttl time.Duration, update func(*ExecutionSnapshot) *ExecutionSnapshot) {
	if err := m.UpdateChecked(ctx, key, ttl, update); err != nil {
		m.rejections.Add(1)
	}
}

// UpdateChecked isolates even callback-retained aliases and rejects oversized replacements atomically.
func (m *MemorySnapshotStore) UpdateChecked(ctx context.Context, key string, ttl time.Duration, update func(*ExecutionSnapshot) *ExecutionSnapshot) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if update == nil {
		return nil
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	var current *ExecutionSnapshot
	if element := m.items[key]; element != nil {
		entry := element.Value.(*memoryEntry)
		if entry.expiresAt.After(m.clock()) {
			current = entry.snapshot.Clone()
		} else {
			m.remove(element)
		}
	}
	next := update(current)
	if next == nil {
		return nil
	}
	size, err := estimateExecutionSnapshot(next, m.maxEntryBytes)
	if err != nil {
		return err
	}
	return m.saveLocked(key, next, ttl, size)
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
	element.Value.(*memoryEntry).reservation.Release()
	delete(m.items, element.Value.(*memoryEntry).key)
	m.lru.Remove(element)
}

func (m *MemorySnapshotStore) reserveLocked(previous *resource.Reservation, size int64, keep *list.Element) (*resource.Reservation, error) {
	credits := []*resource.Reservation{previous}
	reservation, err := m.budget.ReplaceMany(credits, size)
	var victims []*list.Element
	for candidate := m.lru.Back(); err != nil && candidate != nil; candidate = candidate.Prev() {
		if candidate == keep {
			continue
		}
		victims = append(victims, candidate)
		credits = append(credits, candidate.Value.(*memoryEntry).reservation)
		reservation, err = m.budget.ReplaceMany(credits, size)
	}
	if err != nil {
		return nil, err
	}
	for _, victim := range victims {
		m.remove(victim)
		m.evictions.Add(1)
	}
	return reservation, nil
}
func (m *MemorySnapshotStore) Stats() CacheStats {
	return CacheStats{m.hits.Load(), m.misses.Load(), m.stores.Load(), m.evictions.Load(), m.expirations.Load(), m.rejections.Load()}
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
	if value.Type() == reflect.TypeFor[frame.Frame]() {
		original := value.Interface().(frame.Frame)
		return reflect.ValueOf(*original.Clone())
	}
	if value.Type() == reflect.TypeFor[decimal.Decimal]() {
		return reflect.ValueOf(value.Interface().(decimal.Decimal).Copy())
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
			if value.Type().Field(i).PkgPath != "" && !value.Field(i).IsZero() {
				return value
			}
		}
		out := reflect.New(value.Type()).Elem()
		for i := range value.NumField() {
			if out.Field(i).CanSet() {
				out.Field(i).Set(cloneReflect(value.Field(i)))
			}
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

func estimateExecutionSnapshot(snapshot *ExecutionSnapshot, limit int64) (int64, error) {
	type frameView struct {
		Name          string
		Fields        []frame.Field
		RowCount      int
		Meta          frame.FrameMeta
		IndexOverhead int64
	}
	datasets := make(map[string][]frameView, len(snapshot.Datasets))
	for name, set := range snapshot.Datasets {
		if set == nil {
			continue
		}
		for _, fr := range set.Frames {
			if fr != nil {
				datasets[name] = append(datasets[name], frameView{fr.Name, fr.Fields, fr.RowCount, fr.Meta, int64(len(fr.Fields)) * 64})
			}
		}
	}
	return resource.Estimate(struct {
		ID, SpecFingerprint, DataScope, Locale, Timezone string
		Variables                                        map[string]any
		Datasets                                         map[string][]frameView
		Provenance                                       map[string]DatasetProvenance
		CreatedAt, ExpiresAt                             time.Time
	}{snapshot.ID, snapshot.SpecFingerprint, snapshot.DataScope, snapshot.Locale, snapshot.Timezone, snapshot.Variables, datasets, snapshot.Provenance, snapshot.CreatedAt, snapshot.ExpiresAt}, limit)
}
