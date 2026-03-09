// Package runtime validates and executes Lens dashboard specs.
package runtime

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/iota-uz/iota-sdk/pkg/lens"
	"github.com/iota-uz/iota-sdk/pkg/lens/action"
	"github.com/iota-uz/iota-sdk/pkg/lens/datasource"
	"github.com/iota-uz/iota-sdk/pkg/lens/frame"
	"github.com/iota-uz/iota-sdk/pkg/lens/panel"
	"github.com/iota-uz/iota-sdk/pkg/lens/transform"
)

type DatasetResult struct {
	Frames   *frame.FrameSet
	Duration time.Duration
	Error    error
}

type PanelResult struct {
	Panel     panel.Spec
	Frames    *frame.FrameSet
	Duration  time.Duration
	Error     error
	Locale    string
	Timezone  string
	Variables map[string]any
}

type DashboardResult struct {
	Spec      lens.DashboardSpec
	Variables map[string]any
	Datasets  map[string]*DatasetResult
	Panels    map[string]*PanelResult
	Locale    string
	Timezone  string
	StartedAt time.Time
	Duration  time.Duration
}

type Runtime struct {
	Locale      string
	Timezone    string
	Request     url.Values
	Overrides   map[string]any
	DataSources map[string]datasource.DataSource
	Cache       Cache
}

type Cache interface {
	Get(key string) (*frame.FrameSet, bool)
	Set(key string, value *frame.FrameSet)
}

type memoryCache struct {
	mu    sync.RWMutex
	items map[string]*frame.FrameSet
}

func NewMemoryCache() Cache {
	return &memoryCache{items: make(map[string]*frame.FrameSet)}
}

func (m *memoryCache) Get(key string) (*frame.FrameSet, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	value, ok := m.items[key]
	if !ok {
		return nil, false
	}
	return value.Clone(), true
}

func (m *memoryCache) Set(key string, value *frame.FrameSet) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.items[key] = value.Clone()
}

func Execute(ctx context.Context, spec lens.DashboardSpec, rt Runtime) (*DashboardResult, error) {
	if err := Validate(spec); err != nil {
		return nil, err
	}
	startedAt := time.Now()
	if rt.Cache == nil {
		rt.Cache = NewMemoryCache()
	}
	variables, err := resolveVariables(spec.Variables, rt)
	if err != nil {
		return nil, err
	}

	result := &DashboardResult{
		Spec:      spec,
		Variables: variables,
		Datasets:  make(map[string]*DatasetResult, len(spec.Datasets)),
		Panels:    make(map[string]*PanelResult),
		Locale:    rt.Locale,
		Timezone:  rt.Timezone,
		StartedAt: startedAt,
	}

	state := executorState{
		spec:      spec,
		runtime:   rt,
		variables: variables,
		results:   result.Datasets,
		waiters:   make(map[string]*datasetPromise),
	}

	for _, row := range spec.Rows {
		for _, panelSpec := range row.Panels {
			if err := state.executePanel(ctx, panelSpec, result.Panels); err != nil {
				return nil, err
			}
		}
	}
	result.Duration = time.Since(startedAt)
	return result, nil
}

func Run(ctx context.Context, spec lens.DashboardSpec, rt Runtime) (*DashboardResult, error) {
	return Execute(ctx, spec, rt)
}

type executorState struct {
	spec      lens.DashboardSpec
	runtime   Runtime
	variables map[string]any
	results   map[string]*DatasetResult
	waiters   map[string]*datasetPromise
	mu        sync.Mutex
}

type datasetPromise struct {
	ready chan struct{}
}

func (s *executorState) executePanel(ctx context.Context, panelSpec panel.Spec, results map[string]*PanelResult) error {
	start := time.Now()
	if panelSpec.Kind == panel.KindTabs || panelSpec.Kind == panel.KindGrid || panelSpec.Kind == panel.KindSplit || panelSpec.Kind == panel.KindRepeat {
		for _, child := range panelSpec.Children {
			if err := s.executePanel(ctx, child, results); err != nil {
				return err
			}
		}
		results[panelSpec.ID] = &PanelResult{
			Panel:     panelSpec,
			Duration:  time.Since(start),
			Locale:    s.runtime.Locale,
			Timezone:  s.runtime.Timezone,
			Variables: s.variables,
		}
		return nil
	}
	frames, err := s.executeDataset(ctx, panelSpec.Dataset)
	if err == nil && len(panelSpec.Transforms) > 0 {
		frames, err = transform.Apply(frames, map[string]*frame.FrameSet{panelSpec.Dataset: frames}, panelSpec.Transforms)
	}
	results[panelSpec.ID] = &PanelResult{
		Panel:     panelSpec,
		Frames:    frames,
		Duration:  time.Since(start),
		Error:     err,
		Locale:    s.runtime.Locale,
		Timezone:  s.runtime.Timezone,
		Variables: s.variables,
	}
	return nil
}

func (s *executorState) executeDataset(ctx context.Context, name string) (*frame.FrameSet, error) {
	s.mu.Lock()
	if existing, ok := s.results[name]; ok {
		s.mu.Unlock()
		return existing.Frames, existing.Error
	}
	if waiter, ok := s.waiters[name]; ok {
		s.mu.Unlock()
		select {
		case <-waiter.ready:
		case <-ctx.Done():
			return nil, ctx.Err()
		}
		s.mu.Lock()
		defer s.mu.Unlock()
		existing := s.results[name]
		return existing.Frames, existing.Error
	}
	waiter := &datasetPromise{ready: make(chan struct{})}
	s.waiters[name] = waiter
	s.mu.Unlock()

	defer func() {
		s.mu.Lock()
		close(waiter.ready)
		delete(s.waiters, name)
		s.mu.Unlock()
	}()

	spec, ok := s.findDataset(name)
	if !ok {
		err := fmt.Errorf("dataset %q not found", name)
		s.mu.Lock()
		s.results[name] = &DatasetResult{Error: err}
		s.mu.Unlock()
		return nil, err
	}

	start := time.Now()
	var (
		frames *frame.FrameSet
		err    error
	)
	switch spec.Kind {
	case lens.DatasetKindStatic:
		if spec.Static == nil {
			err = fmt.Errorf("dataset %q is missing static frames", spec.Name)
			break
		}
		frames = spec.Static.Clone()
	case lens.DatasetKindQuery:
		frames, err = s.runQueryDataset(ctx, spec)
	case lens.DatasetKindTransform, lens.DatasetKindJoin, lens.DatasetKindUnion, lens.DatasetKindFormula:
		frames, err = s.runDerivedDataset(ctx, spec)
	default:
		err = fmt.Errorf("unsupported dataset kind %q", spec.Kind)
	}
	if err == nil && len(spec.Transforms) > 0 && (spec.Kind == lens.DatasetKindStatic || spec.Kind == lens.DatasetKindQuery) {
		deps := make(map[string]*frame.FrameSet, len(spec.DependsOn))
		for _, dep := range spec.DependsOn {
			depFrames, depErr := s.executeDataset(ctx, dep)
			if depErr != nil {
				err = fmt.Errorf("dataset %q dependency %q failed: %w", spec.Name, dep, depErr)
				break
			}
			if depFrames != nil {
				deps[dep] = depFrames
			}
		}
		if err == nil {
			frames, err = transform.Apply(frames, deps, spec.Transforms)
		}
	}
	s.mu.Lock()
	s.results[name] = &DatasetResult{Frames: frames, Duration: time.Since(start), Error: err}
	s.mu.Unlock()
	return frames, err
}

func (s *executorState) runQueryDataset(ctx context.Context, spec lens.DatasetSpec) (*frame.FrameSet, error) {
	ds, ok := s.runtime.DataSources[spec.Source]
	if !ok {
		return nil, fmt.Errorf("datasource %q not configured", spec.Source)
	}
	request := datasource.QueryRequest{
		Source:    spec.Source,
		Text:      spec.Query.Text,
		Params:    resolveParams(spec.Query.Params, s.variables),
		Timezone:  s.runtime.Timezone,
		TimeRange: resolveDatasetTimeRange(s.spec.Variables, s.variables),
		MaxRows:   spec.Query.MaxRows,
		Kind:      spec.Query.Kind,
	}
	cacheKey := queryCacheKey(request)
	if cached, ok := s.runtime.Cache.Get(cacheKey); ok {
		return cached, nil
	}
	frames, err := ds.Run(ctx, request)
	if err != nil {
		return nil, err
	}
	s.runtime.Cache.Set(cacheKey, frames)
	return frames, nil
}

func (s *executorState) runDerivedDataset(ctx context.Context, spec lens.DatasetSpec) (*frame.FrameSet, error) {
	if len(spec.DependsOn) == 0 {
		return nil, fmt.Errorf("dataset %q has no dependencies", spec.Name)
	}
	base, err := s.executeDataset(ctx, spec.DependsOn[0])
	if err != nil {
		return nil, err
	}
	deps := make(map[string]*frame.FrameSet, len(spec.DependsOn))
	for _, dep := range spec.DependsOn {
		frames, depErr := s.executeDataset(ctx, dep)
		if depErr != nil {
			return nil, depErr
		}
		deps[dep] = frames
	}
	return transform.Apply(base, deps, spec.Transforms)
}

func (s *executorState) findDataset(name string) (lens.DatasetSpec, bool) {
	for _, dataset := range s.spec.Datasets {
		if dataset.Name == name {
			return dataset, true
		}
	}
	return lens.DatasetSpec{}, false
}

func resolveVariables(specs []lens.VariableSpec, rt Runtime) (map[string]any, error) {
	values := make(map[string]any, len(specs))
	for _, spec := range specs {
		if override, ok := rt.Overrides[spec.Name]; ok {
			values[spec.Name] = override
			continue
		}
		switch spec.Kind {
		case lens.VariableDateRange:
			values[spec.Name] = resolveDateRange(spec, rt.Request)
		case lens.VariableToggle:
			raw := rt.Request.Get(spec.Name)
			values[spec.Name] = raw == "true" || raw == "1"
		case lens.VariableSingleSelect, lens.VariableMultiSelect, lens.VariableText, lens.VariableNumber:
			if raw := rt.Request.Get(spec.Name); raw != "" {
				values[spec.Name] = raw
			} else {
				values[spec.Name] = spec.Default
			}
		}
	}
	return values, nil
}

func resolveDateRange(spec lens.VariableSpec, values url.Values) lens.DateRangeValue {
	rawMode := values.Get(spec.Name)
	if rawMode == "all" && spec.AllowAllTime {
		return lens.DateRangeValue{Mode: "all"}
	}
	startRaw := values.Get(spec.Name + "_start")
	endRaw := values.Get(spec.Name + "_end")
	if startRaw != "" && endRaw != "" {
		start, startErr := time.Parse("2006-01-02", startRaw)
		end, endErr := time.Parse("2006-01-02", endRaw)
		if startErr == nil && endErr == nil {
			end = end.Add(24*time.Hour - time.Nanosecond)
			return lens.DateRangeValue{Mode: "bounded", Start: &start, End: &end}
		}
	}
	if defaultValue, ok := spec.Default.(lens.DateRangeValue); ok && defaultValue.Mode == "all" {
		return defaultValue
	}
	now := time.Now().UTC()
	start := now.Add(-spec.DefaultDuration)
	return lens.DateRangeValue{Mode: "default", Start: &start, End: &now}
}

func resolveParams(specs map[string]lens.ParamValue, variables map[string]any) map[string]any {
	if len(specs) == 0 {
		return nil
	}
	out := make(map[string]any, len(specs))
	for key, spec := range specs {
		if spec.Variable != "" {
			out[key] = variables[spec.Variable]
			continue
		}
		out[key] = spec.Literal
	}
	return out
}

func resolveDatasetTimeRange(specs []lens.VariableSpec, variables map[string]any) datasource.TimeRange {
	for _, spec := range specs {
		value, ok := variables[spec.Name]
		if !ok {
			continue
		}
		if timeRange := lens.ResolveTimeRange(value); timeRange.Mode != "" {
			return timeRange
		}
	}
	return datasource.TimeRange{}
}

func queryCacheKey(req datasource.QueryRequest) string {
	payload, err := json.Marshal(req)
	if err != nil {
		sum := sha256.Sum256([]byte(fmt.Sprintf("%#v", req)))
		return fmt.Sprintf("%x", sum[:16])
	}
	sum := sha256.Sum256(payload)
	return fmt.Sprintf("%x", sum[:16])
}

func Validate(spec lens.DashboardSpec) error {
	datasets := make(map[string]lens.DatasetSpec, len(spec.Datasets))
	for _, dataset := range spec.Datasets {
		if dataset.Name == "" {
			return fmt.Errorf("dataset name is required")
		}
		switch dataset.Kind {
		case lens.DatasetKindStatic:
			if dataset.Static == nil {
				return fmt.Errorf("dataset %s is missing static frames", dataset.Name)
			}
		case lens.DatasetKindQuery:
			if dataset.Query == nil {
				return fmt.Errorf("dataset %s is missing query spec", dataset.Name)
			}
			if strings.TrimSpace(dataset.Source) == "" {
				return fmt.Errorf("dataset %s is missing datasource", dataset.Name)
			}
			if strings.TrimSpace(dataset.Query.Text) == "" {
				return fmt.Errorf("dataset %s is missing query text", dataset.Name)
			}
		case lens.DatasetKindTransform, lens.DatasetKindJoin, lens.DatasetKindUnion, lens.DatasetKindFormula:
			// Derived dataset kinds are validated through dependency graph checks below.
		}
		if _, exists := datasets[dataset.Name]; exists {
			return fmt.Errorf("duplicate dataset %s", dataset.Name)
		}
		datasets[dataset.Name] = dataset
	}
	visiting := make(map[string]bool, len(datasets))
	visited := make(map[string]bool, len(datasets))
	var visit func(name string) error
	visit = func(name string) error {
		if visited[name] {
			return nil
		}
		if visiting[name] {
			return fmt.Errorf("dataset cycle detected at %s", name)
		}
		visiting[name] = true
		dataset, ok := datasets[name]
		if !ok {
			return fmt.Errorf("dataset %s not found", name)
		}
		for _, dep := range dataset.DependsOn {
			if err := visit(dep); err != nil {
				return err
			}
		}
		visiting[name] = false
		visited[name] = true
		return nil
	}
	for name := range datasets {
		if err := visit(name); err != nil {
			return err
		}
	}
	panelIDs := make(map[string]struct{})
	for _, row := range spec.Rows {
		for _, panelSpec := range row.Panels {
			if err := validatePanel(panelSpec, datasets, panelIDs); err != nil {
				return err
			}
		}
	}
	return nil
}

func validatePanel(spec panel.Spec, datasets map[string]lens.DatasetSpec, panelIDs map[string]struct{}) error {
	if strings.TrimSpace(spec.ID) == "" {
		return fmt.Errorf("panel id is required")
	}
	if _, exists := panelIDs[spec.ID]; exists {
		return fmt.Errorf("duplicate panel %s", spec.ID)
	}
	panelIDs[spec.ID] = struct{}{}
	switch spec.Kind {
	case panel.KindTabs, panel.KindGrid, panel.KindSplit, panel.KindRepeat:
		for _, child := range spec.Children {
			if err := validatePanel(child, datasets, panelIDs); err != nil {
				return err
			}
		}
		return nil
	case panel.KindStat, panel.KindTimeSeries, panel.KindBar, panel.KindHorizontalBar, panel.KindStackedBar, panel.KindPie, panel.KindDonut, panel.KindTable, panel.KindGauge:
		// Leaf panels continue through dataset and field validation below.
	}
	if spec.Dataset == "" {
		return fmt.Errorf("panel %s is missing dataset", spec.ID)
	}
	if _, ok := datasets[spec.Dataset]; !ok {
		return fmt.Errorf("panel %s references missing dataset %s", spec.ID, spec.Dataset)
	}
	if spec.Kind != panel.KindTable && strings.TrimSpace(spec.Fields.Value) == "" {
		return fmt.Errorf("panel %s is missing value field", spec.ID)
	}
	switch spec.Kind {
	case panel.KindStat, panel.KindTable, panel.KindTabs, panel.KindGrid, panel.KindSplit, panel.KindRepeat:
		// These panel kinds do not require label/category validation here.
	case panel.KindBar, panel.KindHorizontalBar, panel.KindPie, panel.KindDonut, panel.KindGauge:
		if strings.TrimSpace(spec.Fields.Label) == "" && strings.TrimSpace(spec.Fields.Category) == "" {
			return fmt.Errorf("panel %s requires label or category field", spec.ID)
		}
	case panel.KindStackedBar, panel.KindTimeSeries:
		if strings.TrimSpace(spec.Fields.Category) == "" {
			return fmt.Errorf("panel %s requires category field", spec.ID)
		}
		if strings.TrimSpace(spec.Fields.Series) == "" {
			return fmt.Errorf("panel %s requires series field", spec.ID)
		}
	}
	if spec.Action != nil && strings.TrimSpace(spec.Action.URL) == "" && spec.Action.Kind != action.KindEmitEvent {
		return fmt.Errorf("panel %s action requires url", spec.ID)
	}
	if spec.Action != nil {
		if spec.Action.Kind == action.KindEmitEvent && strings.TrimSpace(spec.Action.Event) == "" {
			return fmt.Errorf("panel %s emit event action requires event name", spec.ID)
		}
		if spec.Action.Kind == action.KindHtmxSwap && strings.TrimSpace(spec.Action.Target) == "" {
			return fmt.Errorf("panel %s htmx action requires target", spec.ID)
		}
		for _, param := range spec.Action.Params {
			if err := validateValueSource(spec.ID, param.Name, param.Source); err != nil {
				return err
			}
		}
		for name, source := range spec.Action.Payload {
			if err := validateValueSource(spec.ID, name, source); err != nil {
				return err
			}
		}
	}
	return nil
}

func validateValueSource(panelID, name string, source action.ValueSource) error {
	switch source.Kind {
	case action.SourceField, action.SourceVariable, action.SourcePoint:
		if strings.TrimSpace(source.Name) == "" {
			return fmt.Errorf("panel %s action value %s requires source name", panelID, name)
		}
	case action.SourceLiteral:
		if source.Value == nil {
			return fmt.Errorf("panel %s action value %s requires literal value", panelID, name)
		}
	default:
		return fmt.Errorf("panel %s action value %s has unsupported source kind %q", panelID, name, source.Kind)
	}
	return nil
}
