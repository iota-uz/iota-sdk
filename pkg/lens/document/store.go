package document

import (
	"container/list"
	"context"
	"errors"
	"fmt"
	"reflect"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/iota-uz/iota-sdk/pkg/lens/resource"
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

type SnapshotToucher interface {
	Touch(context.Context, string) error
}
type SnapshotDeleter interface {
	Delete(context.Context, string) error
}
type MemoryStoreOptions struct {
	TTL           time.Duration
	MaxEntries    int
	Budget        *resource.Budget
	MaxEntryBytes int64
}
type memoryStore struct {
	budget        *resource.Budget
	maxEntryBytes int64

	mu         sync.Mutex
	ttl        time.Duration
	maxEntries int
	clock      func() time.Time
	items      map[string]*list.Element
	lru        *list.List
}

type memorySnapshot struct {
	reservation *resource.Reservation
	snapshot    *Snapshot
	expiresAt   time.Time
}

// NewMemoryStore returns a bounded store whose TTL slides on Get and Append.
func NewMemoryStore(ttl time.Duration, maxEntries int) SnapshotStore {
	return NewMemoryStoreWithOptions(MemoryStoreOptions{TTL: ttl, MaxEntries: maxEntries})
}
func NewMemoryStoreWithOptions(opts MemoryStoreOptions) SnapshotStore {
	ttl, maxEntries := opts.TTL, opts.MaxEntries
	if opts.Budget == nil {
		opts.Budget = resource.NewBudget(resource.DefaultLimit)
	}
	if opts.MaxEntryBytes <= 0 {
		opts.MaxEntryBytes = resource.DefaultEntryLimit
	}

	if ttl <= 0 {
		ttl = defaultSnapshotTTL
	}
	if maxEntries <= 0 {
		maxEntries = defaultMaxEntries
	}
	return &memoryStore{
		ttl: ttl, maxEntries: maxEntries, clock: time.Now, budget: opts.Budget, maxEntryBytes: opts.MaxEntryBytes,
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
	size, err := resource.Estimate(snapshot, m.maxEntryBytes)
	if err != nil {
		return err
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
	for _, element := range m.items {
		if !element.Value.(*memorySnapshot).expiresAt.After(now) {
			m.remove(element)
		}
	}
	var previous *resource.Reservation
	if element := m.items[id]; element != nil {
		previous = element.Value.(*memorySnapshot).reservation
	}
	if size > m.budget.Limit() {
		return resource.NewLimitError("entry bytes", size, m.budget.Limit())
	}
	reservation, err := m.budget.Replace(previous, size)
	for err != nil && previous == nil && m.lru.Len() > 0 {
		m.remove(m.lru.Back())
		reservation, err = m.budget.Reserve(size)
	}
	if err != nil {
		return err
	}
	if existing, ok := m.items[id]; ok {
		entry := existing.Value.(*memorySnapshot)
		entry.reservation = reservation
		entry.snapshot = cloned
		entry.expiresAt = now.Add(m.ttl)
		m.lru.MoveToFront(existing)
		return nil
	}
	element := m.lru.PushFront(&memorySnapshot{snapshot: cloned, expiresAt: now.Add(m.ttl), reservation: reservation})
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
	if _, err := resource.Estimate(frames, m.maxEntryBytes); err != nil {
		return err
	}
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
	candidate := cloneSnapshot(entry.snapshot)
	if candidate.Frames == nil {
		candidate.Frames = make(map[FrameRef]Frame)
	}
	newFrames := 0
	for ref := range frames {
		if _, materialized := candidate.Frames[ref]; !materialized {
			newFrames++
		}
	}
	if len(candidate.Frames)+newFrames > maxFramesPerSnapshot {
		return fmt.Errorf("snapshot cannot exceed %d frames", maxFramesPerSnapshot)
	}
	for ref, frame := range frames {
		if _, materialized := candidate.Frames[ref]; materialized {
			continue
		}
		candidate.Frames[ref] = cloneFrame(frame)
	}
	if len(candidate.Frames) > maxFramesPerSnapshot {
		return fmt.Errorf("snapshot cannot exceed %d frames", maxFramesPerSnapshot)
	}
	size, err := resource.Estimate(candidate, m.maxEntryBytes)
	if err != nil {
		return err
	}
	reservation, err := m.budget.Replace(entry.reservation, size)
	if err != nil {
		return err
	}
	entry.reservation = reservation
	entry.snapshot = candidate
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
	if _, err := resource.Estimate(frame, m.maxEntryBytes); err != nil {
		return err
	}
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
	candidate := cloneSnapshot(entry.snapshot)
	if candidate.Frames == nil {
		candidate.Frames = make(map[FrameRef]Frame)
	}
	if candidate.Panels == nil {
		candidate.Panels = make(map[string]PanelCalculation)
	}
	candidate.Frames[ref] = cloneFrame(frame)
	candidate.Panels[panelID] = calculation
	if len(candidate.Frames) > maxFramesPerSnapshot {
		return fmt.Errorf("snapshot cannot exceed %d frames", maxFramesPerSnapshot)
	}
	size, err := resource.Estimate(candidate, m.maxEntryBytes)
	if err != nil {
		return err
	}
	reservation, err := m.budget.Replace(entry.reservation, size)
	if err != nil {
		return err
	}
	entry.reservation = reservation
	entry.snapshot = candidate
	entry.expiresAt = now.Add(m.ttl)

	m.lru.MoveToFront(element)
	return nil
}

func (m *memoryStore) remove(element *list.Element) {
	if element == nil {
		return
	}
	entry := element.Value.(*memorySnapshot)
	entry.reservation.Release()
	delete(m.items, entry.snapshot.ID)
	m.lru.Remove(element)
}

func (m *memoryStore) Delete(ctx context.Context, id string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.remove(m.items[strings.TrimSpace(id)])
	return nil
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
	result := frame
	result.Columns = slices.Clone(frame.Columns)
	result.Rows = make([][]any, len(frame.Rows))
	result.Children = cloneNodes(frame.Children)
	result.Colors = slices.Clone(frame.Colors)
	if frame.Total != nil {
		total := *frame.Total
		result.Total = &total
	}
	if frame.Presentation != nil {
		presentation := *frame.Presentation
		presentation.Sortable = cloneBool(presentation.Sortable)
		presentation.Expandable = cloneBool(presentation.Expandable)
		presentation.Exportable = cloneBool(presentation.Exportable)
		presentation.LineSeries = slices.Clone(presentation.LineSeries)
		result.Presentation = &presentation
	}
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
	v := cloneWireValue(reflect.ValueOf(value))
	if !v.IsValid() {
		return nil
	}
	return v.Interface()
}
func cloneWireValue(v reflect.Value) reflect.Value {
	if !v.IsValid() {
		return v
	}
	if v.Type() == reflect.TypeFor[time.Time]() {
		return v
	}
	switch v.Kind() {
	case reflect.Interface:
		if v.IsNil() {
			return reflect.Zero(v.Type())
		}
		out := reflect.New(v.Type()).Elem()
		out.Set(cloneWireValue(v.Elem()))
		return out
	case reflect.Pointer:
		if v.IsNil() {
			return reflect.Zero(v.Type())
		}
		out := reflect.New(v.Type().Elem())
		out.Elem().Set(cloneWireValue(v.Elem()))
		return out
	case reflect.Map:
		if v.IsNil() {
			return reflect.Zero(v.Type())
		}
		out := reflect.MakeMapWithSize(v.Type(), v.Len())
		it := v.MapRange()
		for it.Next() {
			out.SetMapIndex(it.Key(), cloneWireValue(it.Value()))
		}
		return out
	case reflect.Slice:
		if v.IsNil() {
			return reflect.Zero(v.Type())
		}
		out := reflect.MakeSlice(v.Type(), v.Len(), v.Len())
		for i := 0; i < v.Len(); i++ {
			out.Index(i).Set(cloneWireValue(v.Index(i)))
		}
		return out
	case reflect.Array:
		out := reflect.New(v.Type()).Elem()
		for i := 0; i < v.Len(); i++ {
			out.Index(i).Set(cloneWireValue(v.Index(i)))
		}
		return out
	case reflect.Struct:
		out := reflect.New(v.Type()).Elem()
		for i := 0; i < v.NumField(); i++ {
			if !out.Field(i).CanSet() {
				return v
			}
			out.Field(i).Set(cloneWireValue(v.Field(i)))
		}
		return out
	default:
		return v
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
		if filter.Placement != nil {
			placement := *filter.Placement
			cloned.Placement = &placement
		}
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
		if filter.Segmented != nil {
			segmented := *filter.Segmented
			segmented.Options = slices.Clone(filter.Segmented.Options)
			cloned.Segmented = &segmented
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

func (m *memoryStore) Touch(ctx context.Context, id string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	element := m.items[strings.TrimSpace(id)]
	if element == nil {
		return ErrSnapshotGone
	}
	entry := element.Value.(*memorySnapshot)
	now := m.clock()
	if !entry.expiresAt.After(now) {
		m.remove(element)
		return ErrSnapshotGone
	}
	entry.expiresAt = now.Add(m.ttl)
	m.lru.MoveToFront(element)
	return nil
}
