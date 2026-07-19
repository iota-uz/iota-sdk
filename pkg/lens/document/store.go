package document

import (
	"container/list"
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"sync"
	"time"
)

const defaultSnapshotTTL = 30 * time.Minute
const defaultMaxEntries = 128

var ErrSnapshotGone = errors.New("lens document snapshot is gone")

type Snapshot struct {
	ID        string
	Params    map[string]any
	Frames    map[FrameRef]Frame
	CreatedAt time.Time
}

type SnapshotStore interface {
	Put(context.Context, *Snapshot) error
	Get(context.Context, string) (*Snapshot, error)
	Append(context.Context, string, map[FrameRef]Frame) error
}

type memoryStore struct {
	mu         sync.Mutex
	ttl        time.Duration
	maxEntries int
	clock      func() time.Time
	items      map[string]*list.Element
	lru        *list.List
}

type memorySnapshot struct {
	snapshot  *Snapshot
	expiresAt time.Time
}

func NewMemoryStore(ttl time.Duration, maxEntries int) SnapshotStore {
	if ttl <= 0 {
		ttl = defaultSnapshotTTL
	}
	if maxEntries <= 0 {
		maxEntries = defaultMaxEntries
	}
	return &memoryStore{
		ttl: ttl, maxEntries: maxEntries, clock: time.Now,
		items: make(map[string]*list.Element), lru: list.New(),
	}
}

func (m *memoryStore) Put(ctx context.Context, snapshot *Snapshot) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if snapshot == nil {
		return fmt.Errorf("snapshot is required")
	}
	id := strings.TrimSpace(snapshot.ID)
	if id == "" {
		return fmt.Errorf("snapshot id is required")
	}
	cloned := cloneSnapshot(snapshot)
	cloned.ID = id
	now := m.clock()
	if cloned.CreatedAt.IsZero() {
		cloned.CreatedAt = now
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	if existing, ok := m.items[id]; ok {
		entry := existing.Value.(*memorySnapshot)
		entry.snapshot = cloned
		entry.expiresAt = now.Add(m.ttl)
		m.lru.MoveToFront(existing)
		return nil
	}
	element := m.lru.PushFront(&memorySnapshot{snapshot: cloned, expiresAt: now.Add(m.ttl)})
	m.items[id] = element
	for len(m.items) > m.maxEntries {
		m.remove(m.lru.Back())
	}
	return nil
}

func (m *memoryStore) Get(ctx context.Context, id string) (*Snapshot, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	id = strings.TrimSpace(id)
	m.mu.Lock()
	defer m.mu.Unlock()
	element, ok := m.items[id]
	if !ok {
		return nil, ErrSnapshotGone
	}
	entry := element.Value.(*memorySnapshot)
	if !entry.expiresAt.After(m.clock()) {
		m.remove(element)
		return nil, ErrSnapshotGone
	}
	m.lru.MoveToFront(element)
	return cloneSnapshot(entry.snapshot), nil
}

func (m *memoryStore) Append(ctx context.Context, id string, frames map[FrameRef]Frame) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	id = strings.TrimSpace(id)
	m.mu.Lock()
	defer m.mu.Unlock()
	element, ok := m.items[id]
	if !ok {
		return ErrSnapshotGone
	}
	entry := element.Value.(*memorySnapshot)
	if !entry.expiresAt.After(m.clock()) {
		m.remove(element)
		return ErrSnapshotGone
	}
	if entry.snapshot.Frames == nil {
		entry.snapshot.Frames = make(map[FrameRef]Frame)
	}
	for ref, frame := range frames {
		if _, materialized := entry.snapshot.Frames[ref]; materialized {
			continue
		}
		entry.snapshot.Frames[ref] = cloneFrame(frame)
	}
	m.lru.MoveToFront(element)
	return nil
}

func (m *memoryStore) remove(element *list.Element) {
	if element == nil {
		return
	}
	entry := element.Value.(*memorySnapshot)
	delete(m.items, entry.snapshot.ID)
	m.lru.Remove(element)
}

func cloneSnapshot(snapshot *Snapshot) *Snapshot {
	if snapshot == nil {
		return nil
	}
	result := &Snapshot{
		ID: snapshot.ID, CreatedAt: snapshot.CreatedAt,
		Params: make(map[string]any, len(snapshot.Params)), Frames: make(map[FrameRef]Frame, len(snapshot.Frames)),
	}
	for key, value := range snapshot.Params {
		result.Params[key] = cloneAny(value)
	}
	for ref, frame := range snapshot.Frames {
		result.Frames[ref] = cloneFrame(frame)
	}
	return result
}

func cloneFrame(frame Frame) Frame {
	result := Frame{Columns: append([]Column(nil), frame.Columns...), Rows: make([][]any, len(frame.Rows))}
	for rowIndex, row := range frame.Rows {
		result.Rows[rowIndex] = make([]any, len(row))
		for columnIndex, value := range row {
			result.Rows[rowIndex][columnIndex] = cloneAny(value)
		}
	}
	return result
}

func cloneAny(value any) any {
	if value == nil {
		return nil
	}
	return cloneReflect(reflect.ValueOf(value)).Interface()
}

func cloneReflect(value reflect.Value) reflect.Value {
	if !value.IsValid() {
		return reflect.ValueOf((*any)(nil)).Elem()
	}
	if value.Type() == reflect.TypeOf(time.Time{}) {
		return value
	}
	switch value.Kind() {
	case reflect.Interface:
		if value.IsNil() {
			return reflect.Zero(value.Type())
		}
		cloned := cloneReflect(value.Elem())
		result := reflect.New(value.Type()).Elem()
		result.Set(cloned)
		return result
	case reflect.Pointer:
		if value.IsNil() {
			return reflect.Zero(value.Type())
		}
		result := reflect.New(value.Type().Elem())
		result.Elem().Set(cloneReflect(value.Elem()))
		return result
	case reflect.Map:
		if value.IsNil() {
			return reflect.Zero(value.Type())
		}
		result := reflect.MakeMapWithSize(value.Type(), value.Len())
		iterator := value.MapRange()
		for iterator.Next() {
			result.SetMapIndex(cloneReflect(iterator.Key()), cloneReflect(iterator.Value()))
		}
		return result
	case reflect.Slice:
		if value.IsNil() {
			return reflect.Zero(value.Type())
		}
		result := reflect.MakeSlice(value.Type(), value.Len(), value.Len())
		for index := 0; index < value.Len(); index++ {
			result.Index(index).Set(cloneReflect(value.Index(index)))
		}
		return result
	case reflect.Array:
		result := reflect.New(value.Type()).Elem()
		for index := 0; index < value.Len(); index++ {
			result.Index(index).Set(cloneReflect(value.Index(index)))
		}
		return result
	case reflect.Struct:
		result := reflect.New(value.Type()).Elem()
		result.Set(value)
		for index := 0; index < value.NumField(); index++ {
			if result.Field(index).CanSet() && value.Type().Field(index).IsExported() {
				result.Field(index).Set(cloneReflect(value.Field(index)))
			}
		}
		return result
	default:
		return value
	}
}

func cloneStrings(values map[string]string) map[string]string {
	if values == nil {
		return nil
	}
	result := make(map[string]string, len(values))
	for key, value := range values {
		result[key] = value
	}
	return result
}

func cloneTheme(theme Theme) Theme {
	result := Theme{Palette: cloneStrings(theme.Palette), Series: cloneStrings(theme.Series)}
	if result.Palette == nil {
		result.Palette = make(map[string]string)
	}
	if result.Series == nil {
		result.Series = make(map[string]string)
	}
	return result
}
