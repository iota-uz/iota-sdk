package document

import (
	"container/list"
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"sync"
	"time"

	lenstheme "github.com/iota-uz/iota-sdk/pkg/lens/theme"
)

const defaultSnapshotTTL = 30 * time.Minute
const defaultMaxEntries = 128
const maxFramesPerSnapshot = 1024

var ErrSnapshotGone = errors.New("lens document snapshot is gone")

type Snapshot struct {
	ID        string
	Params    map[string]any
	Frames    map[FrameRef]Frame
	Levels    map[NodeKey]Level
	Panels    map[string]PanelCalculation
	CreatedAt time.Time
}

type SnapshotStore interface {
	Put(context.Context, *Snapshot) error
	Get(context.Context, string) (*Snapshot, error)
	Append(context.Context, string, map[FrameRef]Frame) error
	// PutPanel atomically stores or replaces one independently materialised
	// panel and its calculation provenance.
	PutPanel(context.Context, string, string, FrameRef, Frame, PanelCalculation) error
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

// NewMemoryStore returns a bounded store whose TTL slides on Get and Append.
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
	if len(cloned.Frames) > maxFramesPerSnapshot {
		return fmt.Errorf("snapshot cannot exceed %d frames", maxFramesPerSnapshot)
	}
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
	now := m.clock()
	if !entry.expiresAt.After(now) {
		m.remove(element)
		return nil, ErrSnapshotGone
	}
	entry.expiresAt = now.Add(m.ttl)
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
	now := m.clock()
	if !entry.expiresAt.After(now) {
		m.remove(element)
		return ErrSnapshotGone
	}
	if entry.snapshot.Frames == nil {
		entry.snapshot.Frames = make(map[FrameRef]Frame)
	}
	newFrames := 0
	for ref := range frames {
		if _, materialized := entry.snapshot.Frames[ref]; !materialized {
			newFrames++
		}
	}
	if len(entry.snapshot.Frames)+newFrames > maxFramesPerSnapshot {
		return fmt.Errorf("snapshot cannot exceed %d frames", maxFramesPerSnapshot)
	}
	for ref, frame := range frames {
		if _, materialized := entry.snapshot.Frames[ref]; materialized {
			continue
		}
		entry.snapshot.Frames[ref] = cloneFrame(frame)
	}
	entry.expiresAt = now.Add(m.ttl)
	m.lru.MoveToFront(element)
	return nil
}

func (m *memoryStore) PutPanel(
	ctx context.Context,
	id string,
	panelID string,
	ref FrameRef,
	frame Frame,
	calculation PanelCalculation,
) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	id = strings.TrimSpace(id)
	panelID = strings.TrimSpace(panelID)
	if panelID == "" || strings.TrimSpace(string(ref)) == "" {
		return fmt.Errorf("panel id and frame reference are required")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	element, ok := m.items[id]
	if !ok {
		return ErrSnapshotGone
	}
	entry := element.Value.(*memorySnapshot)
	now := m.clock()
	if !entry.expiresAt.After(now) {
		m.remove(element)
		return ErrSnapshotGone
	}
	if entry.snapshot.Frames == nil {
		entry.snapshot.Frames = make(map[FrameRef]Frame)
	}
	if entry.snapshot.Panels == nil {
		entry.snapshot.Panels = make(map[string]PanelCalculation)
	}
	entry.snapshot.Frames[ref] = cloneFrame(frame)
	entry.snapshot.Panels[panelID] = calculation
	entry.expiresAt = now.Add(m.ttl)
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
		Levels: make(map[NodeKey]Level, len(snapshot.Levels)), Panels: make(map[string]PanelCalculation, len(snapshot.Panels)),
	}
	for key, value := range snapshot.Params {
		result.Params[key] = cloneAny(value)
	}
	for ref, frame := range snapshot.Frames {
		result.Frames[ref] = cloneFrame(frame)
	}
	for key, level := range snapshot.Levels {
		result.Levels[key] = cloneLevel(level)
	}
	for panelID, calculation := range snapshot.Panels {
		result.Panels[panelID] = calculation
	}
	return result
}

func cloneFrame(frame Frame) Frame {
	result := Frame{Columns: append([]Column(nil), frame.Columns...), Rows: make([][]any, len(frame.Rows)), Children: cloneNodes(frame.Children)}
	for rowIndex, row := range frame.Rows {
		result.Rows[rowIndex] = make([]any, len(row))
		for columnIndex, value := range row {
			result.Rows[rowIndex][columnIndex] = cloneAny(value)
		}
	}
	return result
}

func cloneLevel(level Level) Level {
	level.Path = append(NodePath(nil), level.Path...)
	level.Children = cloneNodes(level.Children)
	level.Perspectives = append([]PerspectiveRef(nil), level.Perspectives...)
	if level.DynamicChildren != nil {
		declaration := *level.DynamicChildren
		if declaration.Target != nil {
			target := *declaration.Target
			declaration.Target = &target
		}
		declaration.Action = cloneAction(declaration.Action)
		level.DynamicChildren = &declaration
	}
	if level.Presentation != nil {
		presentation := *level.Presentation
		presentation.Sortable = cloneBool(level.Presentation.Sortable)
		presentation.Expandable = cloneBool(level.Presentation.Expandable)
		presentation.Exportable = cloneBool(level.Presentation.Exportable)
		level.Presentation = &presentation
	}
	if level.Status != nil {
		status := *level.Status
		level.Status = &status
	}
	if level.Source != nil {
		source := *level.Source
		source.Columns = make([]TableColumn, len(level.Source.Columns))
		for index, column := range level.Source.Columns {
			column.Action = cloneAction(column.Action)
			source.Columns[index] = column
		}
		if level.Source.Format != nil {
			source.Format = make(map[string]FieldFormat, len(level.Source.Format))
			for field, format := range level.Source.Format {
				if format.Precision != nil {
					precision := *format.Precision
					format.Precision = &precision
				}
				source.Format[field] = format
			}
		}
		level.Source = &source
	}
	return level
}

func cloneBool(value *bool) *bool {
	if value == nil {
		return nil
	}
	cloned := *value
	return &cloned
}

func cloneNodes(nodes []Node) []Node {
	result := make([]Node, len(nodes))
	for index, node := range nodes {
		node.Path = append(NodePath(nil), node.Path...)
		node.Action = cloneAction(node.Action)
		result[index] = node
	}
	return result
}

func cloneAction(source *Action) *Action {
	if source == nil {
		return nil
	}
	result := *source
	// make (not append to a nil slice) so an EMPTY-but-non-nil Params survives the
	// clone as []ActionParam{} rather than collapsing to nil: append([]T(nil))
	// with zero elements returns nil, which would marshal as `params: null` and
	// fail the wire contract's `params: array` (e.g. the dynamic explorer child
	// actions, which carry no params).
	result.Params = make([]ActionParam, len(source.Params))
	copy(result.Params, source.Params)
	result.Payload = make(map[string]Source, len(source.Payload))
	for key, value := range source.Payload {
		result.Payload[key] = value
	}
	if source.URLSource != nil {
		urlSource := *source.URLSource
		result.URLSource = &urlSource
	}
	if source.DrawerKey != nil {
		drawerKey := *source.DrawerKey
		result.DrawerKey = &drawerKey
	}
	if source.Filter != nil {
		filter := *source.Filter
		result.Filter = &filter
	}
	return &result
}

func cloneAny(value any) any {
	switch typed := value.(type) {
	case nil, bool, string,
		int, int8, int16, int32, int64,
		uint, uint8, uint16, uint32, uint64,
		float32, float64, time.Time:
		return typed
	case []any:
		if typed == nil {
			return []any(nil)
		}
		result := make([]any, len(typed))
		for index, item := range typed {
			result[index] = cloneAny(item)
		}
		return result
	case map[string]any:
		if typed == nil {
			return map[string]any(nil)
		}
		result := make(map[string]any, len(typed))
		for key, item := range typed {
			result[key] = cloneAny(item)
		}
		return result
	default:
		return typed
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

func cloneFilters(filters []Filter) []Filter {
	if len(filters) == 0 {
		return nil
	}
	result := make([]Filter, len(filters))
	for index, filter := range filters {
		cloned := filter
		if filter.Period != nil {
			period := *filter.Period
			period.Presets = slices.Clone(filter.Period.Presets)
			cloned.Period = &period
		}
		if filter.Facet != nil {
			facet := *filter.Facet
			facet.Selections = slices.Clone(filter.Facet.Selections)
			cloned.Facet = &facet
		}
		if filter.Compare != nil {
			comparison := *filter.Compare
			cloned.Compare = &comparison
		}
		result[index] = cloned
	}
	return result
}

func cloneTheme(theme Theme) Theme {
	result := Theme{Palette: cloneStrings(theme.Palette), Series: cloneStrings(theme.Series), DebounceMS: theme.DebounceMS}
	if result.DebounceMS <= 0 {
		result.DebounceMS = lenstheme.DebounceMs
	}
	if result.Palette == nil {
		result.Palette = make(map[string]string)
	}
	if result.Series == nil {
		result.Series = make(map[string]string)
	}
	return result
}
