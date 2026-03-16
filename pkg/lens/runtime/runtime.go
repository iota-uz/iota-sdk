// Package runtime validates and executes Lens dashboard specs.
package runtime

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/iota-uz/iota-sdk/pkg/lens"
	"github.com/iota-uz/iota-sdk/pkg/lens/action"
	"github.com/iota-uz/iota-sdk/pkg/lens/datasource"
	"github.com/iota-uz/iota-sdk/pkg/lens/drill"
	"github.com/iota-uz/iota-sdk/pkg/lens/filter"
	"github.com/iota-uz/iota-sdk/pkg/lens/frame"
	"github.com/iota-uz/iota-sdk/pkg/lens/panel"
	"github.com/iota-uz/iota-sdk/pkg/lens/transform"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
	"github.com/sirupsen/logrus"
	"golang.org/x/sync/errgroup"
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
	Request   url.Values
	Drill     *drill.State
}

type DashboardResult struct {
	Spec      lens.DashboardSpec
	Variables map[string]any
	Filters   filter.Model
	Plan      ExecutionPlan
	Datasets  map[string]*DatasetResult
	Panels    map[string]*PanelResult
	Locale    string
	Timezone  string
	Request   url.Values
	Drill     *drill.State
	StartedAt time.Time
	Duration  time.Duration
}

type Result = DashboardResult

type ExecutionPlan struct {
	DatasetStages []ExecutionStage
	Panels        []string
}

type ExecutionStage struct {
	Level    int
	Datasets []string
}

type Request struct {
	Locale      string
	Timezone    string
	Path        string
	Request     url.Values
	Overrides   map[string]any
	DataSources map[string]datasource.DataSource
	Cache       Cache
}

type Runtime = Request

type Scope struct {
	PanelIDs []string
}

func DashboardScope() Scope {
	return Scope{}
}

func PanelsScope(panelIDs ...string) Scope {
	return Scope{PanelIDs: append([]string(nil), panelIDs...)}
}

func PanelScope(panelID string) Scope {
	return Scope{PanelIDs: []string{panelID}}
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

func Run(ctx context.Context, spec lens.DashboardSpec, req Request) (*DashboardResult, error) {
	return run(ctx, spec, req, DashboardScope())
}

func RunScope(ctx context.Context, spec lens.DashboardSpec, req Request, scope Scope) (*DashboardResult, error) {
	return run(ctx, spec, req, scope)
}

func run(ctx context.Context, spec lens.DashboardSpec, req Request, scope Scope) (*DashboardResult, error) {
	op := serrors.Op("lens/runtime.Run")
	if err := Validate(spec); err != nil {
		return nil, serrors.E(op, err)
	}
	startedAt := time.Now()
	if req.Cache == nil {
		req.Cache = NewMemoryCache()
	}
	variables, err := resolveVariables(spec.Variables, req)
	if err != nil {
		return nil, serrors.E(op, err)
	}
	internalPlan, err := compileExecutionPlan(spec, scope.PanelIDs)
	if err != nil {
		return nil, serrors.E(op, err)
	}

	result := &DashboardResult{
		Spec:      spec,
		Variables: variables,
		Filters:   filter.Build(spec.Variables, variables),
		Plan:      internalPlan.view,
		Datasets:  make(map[string]*DatasetResult, len(spec.Datasets)),
		Panels:    make(map[string]*PanelResult),
		Locale:    req.Locale,
		Timezone:  req.Timezone,
		Request:   cloneValues(req.Request),
		Drill:     drill.Parse(req.Path, req.Request, spec.Title),
		StartedAt: startedAt,
	}

	state := plannedExecutor{
		spec:      spec,
		runtime:   req,
		variables: variables,
	}

	if err := state.executeDatasets(ctx, internalPlan.datasetStages, result.Datasets); err != nil {
		return nil, serrors.E(op, err)
	}
	if err := state.executePanels(ctx, internalPlan.panels, result.Datasets, result.Panels); err != nil {
		return nil, serrors.E(op, err)
	}
	result.Duration = time.Since(startedAt)
	return result, nil
}

type plannedExecutor struct {
	spec      lens.DashboardSpec
	runtime   Request
	variables map[string]any
}

type executionPlan struct {
	view          ExecutionPlan
	datasetStages [][]lens.DatasetSpec
	panels        []panel.Spec
}

func Plan(spec lens.DashboardSpec, scope Scope) (ExecutionPlan, error) {
	op := serrors.Op("lens/runtime.Plan")
	if err := Validate(spec); err != nil {
		return ExecutionPlan{}, serrors.E(op, err)
	}
	plan, err := compileExecutionPlan(spec, scope.PanelIDs)
	if err != nil {
		return ExecutionPlan{}, serrors.E(op, err)
	}
	return plan.view, nil
}

func compileExecutionPlan(spec lens.DashboardSpec, panelIDs []string) (executionPlan, error) {
	datasets := indexDatasets(spec.Datasets)
	targetPanels, err := scopedPanels(spec, panelIDs)
	if err != nil {
		return executionPlan{}, err
	}
	required := requiredDatasetNames(targetPanels, datasets)
	stageMap := make(map[int][]string)
	depthMemo := make(map[string]int, len(required))
	visiting := make(map[string]bool, len(required))
	for _, name := range required {
		depth, err := datasetDepth(name, datasets, depthMemo, visiting)
		if err != nil {
			return executionPlan{}, err
		}
		stageMap[depth] = append(stageMap[depth], name)
	}

	depths := make([]int, 0, len(stageMap))
	for depth := range stageMap {
		depths = append(depths, depth)
	}
	slices.Sort(depths)

	view := ExecutionPlan{
		DatasetStages: make([]ExecutionStage, 0, len(depths)),
		Panels:        make([]string, 0),
	}
	internal := executionPlan{
		view:          view,
		datasetStages: make([][]lens.DatasetSpec, 0, len(depths)),
		panels:        targetPanels,
	}
	for _, panelSpec := range internal.panels {
		internal.view.Panels = append(internal.view.Panels, panelSpec.ID)
	}
	for _, depth := range depths {
		names := append([]string(nil), stageMap[depth]...)
		slices.Sort(names)
		stage := ExecutionStage{Level: depth, Datasets: append([]string(nil), names...)}
		internal.view.DatasetStages = append(internal.view.DatasetStages, stage)

		specs := make([]lens.DatasetSpec, 0, len(names))
		for _, name := range names {
			specs = append(specs, datasets[name])
		}
		internal.datasetStages = append(internal.datasetStages, specs)
	}
	return internal, nil
}

func scopedPanels(spec lens.DashboardSpec, panelIDs []string) ([]panel.Spec, error) {
	if len(panelIDs) == 0 {
		return lens.FlattenPanels(spec), nil
	}
	panels := make([]panel.Spec, 0, len(panelIDs))
	seen := make(map[string]struct{}, len(panelIDs))
	for _, panelID := range panelIDs {
		if strings.TrimSpace(panelID) == "" {
			continue
		}
		root, ok := lens.FindPanel(spec, panelID)
		if !ok {
			return nil, fmt.Errorf("panel %q not found", panelID)
		}
		for _, panelSpec := range lens.FlattenPanels(lens.DashboardSpec{
			Rows: []lens.RowSpec{{Panels: []panel.Spec{root}}},
		}) {
			if _, exists := seen[panelSpec.ID]; exists {
				continue
			}
			seen[panelSpec.ID] = struct{}{}
			panels = append(panels, panelSpec)
		}
	}
	return panels, nil
}

func (s *plannedExecutor) executeDatasets(ctx context.Context, stages [][]lens.DatasetSpec, results map[string]*DatasetResult) error {
	for _, stage := range stages {
		if err := ctx.Err(); err != nil {
			return err
		}
		stageResults := make(map[string]*DatasetResult, len(stage))
		var mu sync.Mutex
		group, groupCtx := errgroup.WithContext(ctx)
		for _, datasetSpec := range stage {
			group.Go(func() error {
				start := time.Now()
				frames, err := s.executeDatasetSpec(groupCtx, datasetSpec, results)
				mu.Lock()
				stageResults[datasetSpec.Name] = &DatasetResult{
					Frames:   frames,
					Duration: time.Since(start),
					Error:    err,
				}
				mu.Unlock()
				if err := groupCtx.Err(); err != nil {
					return err
				}
				return nil
			})
		}
		if err := group.Wait(); err != nil {
			return err
		}
		for name, result := range stageResults {
			results[name] = result
		}
	}
	return nil
}

func (s *plannedExecutor) executePanels(ctx context.Context, panels []panel.Spec, datasets map[string]*DatasetResult, results map[string]*PanelResult) error {
	var mu sync.Mutex
	group, groupCtx := errgroup.WithContext(ctx)
	for _, panelSpec := range panels {
		group.Go(func() error {
			start := time.Now()
			panelResult := &PanelResult{
				Panel:     panelSpec,
				Duration:  time.Since(start),
				Locale:    s.runtime.Locale,
				Timezone:  s.runtime.Timezone,
				Variables: s.variables,
				Request:   cloneValues(s.runtime.Request),
				Drill:     drill.Parse(s.runtime.Path, s.runtime.Request, s.spec.Title),
			}
			if isCompositePanel(panelSpec.Kind) {
				mu.Lock()
				results[panelSpec.ID] = panelResult
				mu.Unlock()
				return nil
			}

			datasetResult, ok := datasets[panelSpec.Dataset]
			if !ok {
				panelResult.Error = fmt.Errorf("dataset %q not found", panelSpec.Dataset)
			} else if datasetResult.Error != nil {
				panelResult.Error = datasetResult.Error
			} else {
				panelResult.Frames = datasetResult.Frames
				if len(panelSpec.Transforms) > 0 {
					panelResult.Frames, panelResult.Error = transform.Apply(
						datasetResult.Frames,
						map[string]*frame.FrameSet{panelSpec.Dataset: datasetResult.Frames},
						panelSpec.Transforms,
					)
				}
				if panelResult.Error == nil {
					panelResult.Error = validatePanelFrames(panelSpec, panelResult.Frames)
				}
			}
			panelResult.Duration = time.Since(start)
			if panelResult.Error != nil {
				logPanelFailure(panelSpec, s.runtime, panelResult.Error)
			}
			mu.Lock()
			results[panelSpec.ID] = panelResult
			mu.Unlock()
			if err := groupCtx.Err(); err != nil {
				return err
			}
			return nil
		})
	}
	return group.Wait()
}

func logPanelFailure(spec panel.Spec, req Request, err error) {
	if err == nil {
		return
	}
	logrus.WithFields(logrus.Fields{
		"panel_id":    spec.ID,
		"panel_title": spec.Title,
		"panel_kind":  string(spec.Kind),
		"dataset":     spec.Dataset,
		"path":        req.Path,
		"locale":      req.Locale,
	}).WithError(err).Error("lens panel execution failed")
}

func (s *plannedExecutor) executeDatasetSpec(ctx context.Context, spec lens.DatasetSpec, results map[string]*DatasetResult) (*frame.FrameSet, error) {
	switch spec.Kind {
	case lens.DatasetKindStatic:
		if spec.Static == nil {
			return nil, fmt.Errorf("dataset %q is missing static frames", spec.Name)
		}
		frames := spec.Static.Clone()
		if len(spec.Transforms) == 0 {
			return frames, nil
		}
		deps, err := resolveDependencyFrames(spec.Name, spec.DependsOn, results)
		if err != nil {
			return nil, err
		}
		return transform.Apply(frames, deps, spec.Transforms)
	case lens.DatasetKindQuery:
		frames, err := s.runQueryDataset(ctx, spec)
		if err != nil {
			return nil, err
		}
		if len(spec.Transforms) == 0 {
			return frames, nil
		}
		deps, depErr := resolveDependencyFrames(spec.Name, spec.DependsOn, results)
		if depErr != nil {
			return nil, depErr
		}
		return transform.Apply(frames, deps, spec.Transforms)
	case lens.DatasetKindTransform, lens.DatasetKindJoin, lens.DatasetKindUnion, lens.DatasetKindFormula:
		return s.runDerivedDataset(spec, results)
	default:
		return nil, fmt.Errorf("unsupported dataset kind %q", spec.Kind)
	}
}

func (s *plannedExecutor) runQueryDataset(ctx context.Context, spec lens.DatasetSpec) (*frame.FrameSet, error) {
	ds, ok := s.runtime.DataSources[spec.Source]
	if !ok {
		return nil, fmt.Errorf("datasource %q not configured", spec.Source)
	}
	request := datasource.QueryRequest{
		Source:    spec.Source,
		Text:      spec.Query.Text,
		Params:    resolveParams(spec.Query.Params, s.variables),
		Timezone:  s.runtime.Timezone,
		Locale:    s.runtime.Locale,
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

func (s *plannedExecutor) runDerivedDataset(spec lens.DatasetSpec, results map[string]*DatasetResult) (*frame.FrameSet, error) {
	if len(spec.DependsOn) == 0 {
		return nil, fmt.Errorf("dataset %q has no dependencies", spec.Name)
	}
	baseResult, ok := results[spec.DependsOn[0]]
	if !ok {
		return nil, fmt.Errorf("dataset %q dependency %q not found", spec.Name, spec.DependsOn[0])
	}
	if baseResult.Error != nil {
		return nil, fmt.Errorf("dataset %q dependency %q failed: %w", spec.Name, spec.DependsOn[0], baseResult.Error)
	}
	deps, err := resolveDependencyFrames(spec.Name, spec.DependsOn, results)
	if err != nil {
		return nil, err
	}
	return transform.Apply(baseResult.Frames, deps, spec.Transforms)
}

func indexDatasets(specs []lens.DatasetSpec) map[string]lens.DatasetSpec {
	datasets := make(map[string]lens.DatasetSpec, len(specs))
	for _, dataset := range specs {
		datasets[dataset.Name] = dataset
	}
	return datasets
}

func requiredDatasetNames(panels []panel.Spec, datasets map[string]lens.DatasetSpec) []string {
	seen := make(map[string]struct{})
	for _, panelSpec := range panels {
		if isCompositePanel(panelSpec.Kind) || strings.TrimSpace(panelSpec.Dataset) == "" {
			continue
		}
		markRequiredDataset(panelSpec.Dataset, datasets, seen)
	}
	names := make([]string, 0, len(seen))
	for name := range seen {
		names = append(names, name)
	}
	return names
}

func markRequiredDataset(name string, datasets map[string]lens.DatasetSpec, seen map[string]struct{}) {
	if name == "" {
		return
	}
	if _, ok := seen[name]; ok {
		return
	}
	seen[name] = struct{}{}
	spec, ok := datasets[name]
	if !ok {
		return
	}
	for _, dep := range spec.DependsOn {
		markRequiredDataset(dep, datasets, seen)
	}
}

func datasetDepth(name string, datasets map[string]lens.DatasetSpec, memo map[string]int, visiting map[string]bool) (int, error) {
	if depth, ok := memo[name]; ok {
		return depth, nil
	}
	if visiting[name] {
		return 0, fmt.Errorf("dataset cycle detected at %s", name)
	}
	spec, ok := datasets[name]
	if !ok {
		return 0, fmt.Errorf("dataset %q not found", name)
	}
	visiting[name] = true
	defer delete(visiting, name)
	maxDepth := 0
	for _, dep := range spec.DependsOn {
		depth, err := datasetDepth(dep, datasets, memo, visiting)
		if err != nil {
			return 0, err
		}
		if depth+1 > maxDepth {
			maxDepth = depth + 1
		}
	}
	memo[name] = maxDepth
	return maxDepth, nil
}

func isCompositePanel(kind panel.Kind) bool {
	return kind == panel.KindTabs || kind == panel.KindGrid || kind == panel.KindSplit || kind == panel.KindRepeat
}

func resolveDependencyFrames(name string, dependencies []string, results map[string]*DatasetResult) (map[string]*frame.FrameSet, error) {
	deps := make(map[string]*frame.FrameSet, len(dependencies))
	for _, dep := range dependencies {
		result, ok := results[dep]
		if !ok {
			return nil, fmt.Errorf("dataset %q dependency %q not found", name, dep)
		}
		if result.Error != nil {
			return nil, fmt.Errorf("dataset %q dependency %q failed: %w", name, dep, result.Error)
		}
		if result.Frames != nil {
			deps[dep] = result.Frames
		}
	}
	return deps, nil
}

func resolveVariables(specs []lens.VariableSpec, rt Request) (map[string]any, error) {
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
			raw := strings.TrimSpace(requestValue(rt.Request, spec.Name, spec.RequestKeys...))
			if raw == "" {
				values[spec.Name] = spec.Default
				continue
			}
			values[spec.Name] = raw == "true" || raw == "1" || raw == "on"
		case lens.VariableNumber:
			if raw := requestValue(rt.Request, spec.Name, spec.RequestKeys...); raw != "" {
				parsed, err := strconv.ParseFloat(strings.TrimSpace(raw), 64)
				if err != nil {
					values[spec.Name] = spec.Default
				} else {
					values[spec.Name] = parsed
				}
			} else {
				values[spec.Name] = spec.Default
			}
		case lens.VariableMultiSelect:
			if raw := requestValues(rt.Request, spec.Name, spec.RequestKeys...); len(raw) > 0 {
				values[spec.Name] = splitMultiSelectValues(raw)
			} else {
				values[spec.Name] = spec.Default
			}
		case lens.VariableSingleSelect, lens.VariableText:
			if raw := requestValue(rt.Request, spec.Name, spec.RequestKeys...); raw != "" {
				values[spec.Name] = raw
			} else {
				values[spec.Name] = spec.Default
			}
		}
	}
	return values, nil
}

func splitMultiSelectValues(raw []string) []string {
	values := make([]string, 0, len(raw))
	for _, item := range raw {
		for _, candidate := range strings.Split(item, ",") {
			trimmed := strings.TrimSpace(candidate)
			if trimmed == "" {
				continue
			}
			values = append(values, trimmed)
		}
	}
	if len(values) == 0 {
		return nil
	}
	return values
}

func resolveDateRange(spec lens.VariableSpec, values url.Values) lens.DateRangeValue {
	rawMode := requestValue(values, spec.Name, spec.RequestKeys...)
	if rawMode == "all" && spec.AllowAllTime {
		return lens.DateRangeValue{Mode: "all"}
	}
	startKey, endKey := dateRangeRequestKeys(spec)
	startRaw := values.Get(startKey)
	endRaw := values.Get(endKey)
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

func requestValue(values url.Values, name string, aliases ...string) string {
	keys := append([]string{name}, aliases...)
	for _, key := range keys {
		if trimmed := strings.TrimSpace(values.Get(key)); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func requestValues(values url.Values, name string, aliases ...string) []string {
	keys := append([]string{name}, aliases...)
	for _, key := range keys {
		if raw := values[key]; len(raw) > 0 {
			return raw
		}
	}
	return nil
}

func cloneValues(values url.Values) url.Values {
	if values == nil {
		return nil
	}
	cloned := make(url.Values, len(values))
	for key, items := range values {
		cloned[key] = append([]string(nil), items...)
	}
	return cloned
}

func dateRangeRequestKeys(spec lens.VariableSpec) (string, string) {
	if len(spec.RequestKeys) >= 3 {
		return spec.RequestKeys[1], spec.RequestKeys[2]
	}
	if len(spec.RequestKeys) >= 2 {
		return spec.RequestKeys[0], spec.RequestKeys[1]
	}
	return spec.Name + "_start", spec.Name + "_end"
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
	op := serrors.Op("lens/runtime.Validate")
	invalid := func(format string, args ...any) error {
		return serrors.E(op, fmt.Errorf(format, args...))
	}
	wrap := func(err error) error {
		if err == nil {
			return nil
		}
		return serrors.E(op, err)
	}
	datasets := make(map[string]lens.DatasetSpec, len(spec.Datasets))
	for _, dataset := range spec.Datasets {
		if dataset.Name == "" {
			return invalid("dataset name is required")
		}
		switch dataset.Kind {
		case lens.DatasetKindStatic:
			if dataset.Static == nil {
				return invalid("dataset %s is missing static frames", dataset.Name)
			}
		case lens.DatasetKindQuery:
			if dataset.Query == nil {
				return invalid("dataset %s is missing query spec", dataset.Name)
			}
			if strings.TrimSpace(dataset.Source) == "" {
				return invalid("dataset %s is missing datasource", dataset.Name)
			}
			if strings.TrimSpace(dataset.Query.Text) == "" {
				return invalid("dataset %s is missing query text", dataset.Name)
			}
		case lens.DatasetKindTransform, lens.DatasetKindJoin, lens.DatasetKindUnion, lens.DatasetKindFormula:
			// Derived dataset kinds are validated through dependency graph checks below.
		default:
			return invalid("dataset %s has unsupported kind %q", dataset.Name, dataset.Kind)
		}
		if _, exists := datasets[dataset.Name]; exists {
			return invalid("duplicate dataset %s", dataset.Name)
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
			return invalid("dataset cycle detected at %s", name)
		}
		visiting[name] = true
		dataset, ok := datasets[name]
		if !ok {
			return invalid("dataset %s not found", name)
		}
		for _, dep := range dataset.DependsOn {
			if err := visit(dep); err != nil {
				return wrap(err)
			}
		}
		visiting[name] = false
		visited[name] = true
		return nil
	}
	for name := range datasets {
		if err := visit(name); err != nil {
			return wrap(err)
		}
	}
	panelIDs := make(map[string]struct{})
	for _, row := range spec.Rows {
		for _, panelSpec := range row.Panels {
			if err := validatePanel(panelSpec, datasets, panelIDs); err != nil {
				return wrap(err)
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
	default:
		return fmt.Errorf("panel %s has unsupported kind %q", spec.ID, spec.Kind)
	}
	if spec.Dataset == "" {
		return fmt.Errorf("panel %s is missing dataset", spec.ID)
	}
	if _, ok := datasets[spec.Dataset]; !ok {
		return fmt.Errorf("panel %s references missing dataset %s", spec.ID, spec.Dataset)
	}
	if spec.Kind != panel.KindTable && spec.Fields.Value.Empty() {
		return fmt.Errorf("panel %s is missing value field", spec.ID)
	}
	switch spec.Kind {
	case panel.KindStat, panel.KindTable, panel.KindTabs, panel.KindGrid, panel.KindSplit, panel.KindRepeat:
		// These panel kinds do not require label/category validation here.
	case panel.KindBar, panel.KindHorizontalBar, panel.KindPie, panel.KindDonut, panel.KindGauge:
		if spec.Fields.Label.Empty() && spec.Fields.Category.Empty() {
			return fmt.Errorf("panel %s requires label or category field", spec.ID)
		}
	case panel.KindStackedBar, panel.KindTimeSeries:
		if spec.Fields.Category.Empty() {
			return fmt.Errorf("panel %s requires category field", spec.ID)
		}
		if spec.Kind == panel.KindStackedBar && spec.Fields.Series.Empty() {
			return fmt.Errorf("panel %s requires series field", spec.ID)
		}
	}
	if spec.Action != nil && strings.TrimSpace(spec.Action.URL) == "" && spec.Action.Kind != action.KindEmitEvent {
		return fmt.Errorf("panel %s action requires url", spec.ID)
	}
	if spec.Action != nil {
		switch spec.Action.Kind {
		case action.KindNavigate, action.KindHtmxSwap, action.KindEmitEvent, action.KindDrill:
		default:
			return fmt.Errorf("panel %s action has unsupported kind %q", spec.ID, spec.Action.Kind)
		}
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
		if spec.Action.Kind == action.KindDrill && spec.Action.Drill != nil && spec.Action.Drill.LabelSource.Kind != "" {
			if err := validateValueSource(spec.ID, "drill_label", spec.Action.Drill.LabelSource); err != nil {
				return err
			}
		}
	}
	return nil
}

func validatePanelFrames(spec panel.Spec, frames *frame.FrameSet) error {
	if frames == nil || frames.Primary() == nil {
		return nil
	}
	primary := frames.Primary()
	if err := validateRequiredPanelFields(spec, primary); err != nil {
		return err
	}
	if spec.Kind == panel.KindTable {
		for _, column := range spec.Columns {
			if column.Field.Empty() {
				return fmt.Errorf("panel %s has table column without field reference", spec.ID)
			}
			if _, ok := primary.Field(column.Field.Name()); !ok {
				return fmt.Errorf("panel %s is missing table column field %q in dataset %s", spec.ID, column.Field.Name(), spec.Dataset)
			}
		}
	}
	if spec.Action != nil {
		for _, param := range spec.Action.Params {
			if err := validateFrameValueSource(spec.ID, spec.Dataset, primary, param.Source); err != nil {
				return err
			}
		}
		for _, source := range spec.Action.Payload {
			if err := validateFrameValueSource(spec.ID, spec.Dataset, primary, source); err != nil {
				return err
			}
		}
		if spec.Action.Kind == action.KindDrill && spec.Action.Drill != nil && spec.Action.Drill.LabelSource.Kind != "" {
			if err := validateFrameValueSource(spec.ID, spec.Dataset, primary, spec.Action.Drill.LabelSource); err != nil {
				return err
			}
		}
	}
	return nil
}

func validateRequiredPanelFields(spec panel.Spec, primary *frame.Frame) error {
	switch spec.Kind {
	case panel.KindStat:
		return requireField(spec, primary, spec.Fields.Value)
	case panel.KindTimeSeries:
		if err := requireField(spec, primary, spec.Fields.Category); err != nil {
			return err
		}
		return requireField(spec, primary, spec.Fields.Value)
	case panel.KindBar, panel.KindHorizontalBar, panel.KindPie, panel.KindDonut, panel.KindGauge:
		if err := requireField(spec, primary, spec.Fields.Value); err != nil {
			return err
		}
		if err := requireOneField(spec, primary, spec.Fields.Label, spec.Fields.Category); err != nil {
			return err
		}
	case panel.KindStackedBar:
		if err := requireField(spec, primary, spec.Fields.Category); err != nil {
			return err
		}
		if err := requireField(spec, primary, spec.Fields.Series); err != nil {
			return err
		}
		return requireField(spec, primary, spec.Fields.Value)
	case panel.KindTable, panel.KindTabs, panel.KindGrid, panel.KindSplit, panel.KindRepeat:
		return nil
	}
	return nil
}

func requireField(spec panel.Spec, primary *frame.Frame, field panel.FieldRef) error {
	if field.Empty() {
		return nil
	}
	if _, ok := primary.Field(field.Name()); ok {
		return nil
	}
	return fmt.Errorf("panel %s is missing field %q in dataset %s", spec.ID, field.Name(), spec.Dataset)
}

func requireOneField(spec panel.Spec, primary *frame.Frame, fields ...panel.FieldRef) error {
	nonEmpty := make([]string, 0, len(fields))
	for _, field := range fields {
		if field.Empty() {
			continue
		}
		nonEmpty = append(nonEmpty, field.Name())
		if _, ok := primary.Field(field.Name()); ok {
			return nil
		}
	}
	if len(nonEmpty) == 0 {
		return nil
	}
	return fmt.Errorf("panel %s is missing field from set %v in dataset %s", spec.ID, nonEmpty, spec.Dataset)
}

func validateFrameValueSource(panelID, dataset string, primary *frame.Frame, source action.ValueSource) error {
	if source.Kind != action.SourceField {
		return nil
	}
	if primary == nil || primary.RowCount == 0 {
		return nil
	}
	if _, ok := primary.Field(source.Name); ok {
		return nil
	}
	return fmt.Errorf("panel %s action references missing field %q in dataset %s", panelID, source.Name, dataset)
}

func validateValueSource(panelID, name string, source action.ValueSource) error {
	switch source.Kind {
	case action.SourceField, action.SourceVariable:
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
