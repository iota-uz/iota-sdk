// Package runtime validates and executes Lens dashboard specs.
package runtime

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"math"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/iota-uz/iota-sdk/pkg/lens"
	"github.com/iota-uz/iota-sdk/pkg/lens/action"
	"github.com/iota-uz/iota-sdk/pkg/lens/cube"
	"github.com/iota-uz/iota-sdk/pkg/lens/datasource"
	"github.com/iota-uz/iota-sdk/pkg/lens/explore"
	"github.com/iota-uz/iota-sdk/pkg/lens/filter"
	"github.com/iota-uz/iota-sdk/pkg/lens/frame"
	"github.com/iota-uz/iota-sdk/pkg/lens/panel"
	"github.com/iota-uz/iota-sdk/pkg/lens/transform"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
	"github.com/sirupsen/logrus"
	"golang.org/x/sync/errgroup"
	"golang.org/x/sync/singleflight"
)

type DatasetResult struct {
	Frames   *frame.FrameSet
	Duration time.Duration
	Error    error
}

type PanelResult struct {
	Panel           panel.Spec
	Frames          *frame.FrameSet
	Duration        time.Duration
	Error           error
	Locale          string
	Timezone        string
	Variables       map[string]any
	RequestPath     string
	Request         url.Values
	Drill           *cube.DrillContext
	TablePagination *TablePagination
}

type DashboardResult struct {
	Spec        lens.DashboardSpec
	Variables   map[string]any
	Filters     filter.Model
	Plan        ExecutionPlan
	Datasets    map[string]*DatasetResult
	Panels      map[string]*PanelResult
	Locale      string
	Timezone    string
	RequestPath string
	Request     url.Values
	Drill       *cube.DrillContext
	StartedAt   time.Time
	Duration    time.Duration
	SnapshotID  string
}

type Result = DashboardResult

func (r *DashboardResult) Panel(panelID string) *PanelResult {
	if r == nil || r.Panels == nil {
		return nil
	}
	return r.Panels[panelID]
}

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
	// DataScope is a stable, non-secret identity for tenant/authz/data access.
	// Host applications must change it whenever the visible row set changes.
	DataScope            string
	Namespace            string
	DataSourceIdentities map[string]string
}

type Options struct {
	Store        SnapshotStore
	DefaultTTL   time.Duration
	CacheVersion string
	Observer     Observer
}

type Observer interface {
	SnapshotHit(string)
	SnapshotMiss(string)
	DatasetExecuted(string, time.Duration)
}

// Runtime is a long-lived Lens execution service. A process should normally
// construct one instance and share it across render, fragment and export paths.
type Runtime struct {
	store    SnapshotStore
	ttl      time.Duration
	version  string
	observer Observer
	flights  singleflight.Group
	mergeMu  sync.Mutex
}

func New(opts Options) *Runtime {
	if opts.Store == nil {
		opts.Store = NewMemorySnapshotStore(MemoryStoreOptions{})
	}
	if opts.DefaultTTL <= 0 {
		opts.DefaultTTL = 5 * time.Minute
	}
	if strings.TrimSpace(opts.CacheVersion) == "" {
		opts.CacheVersion = "v1"
	}
	return &Runtime{store: opts.Store, ttl: opts.DefaultTTL, version: opts.CacheVersion, observer: opts.Observer}
}

func (r *Runtime) Store() SnapshotStore { return r.store }

type Scope struct {
	PanelIDs              []string
	IncludeExportEvidence bool
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

// DashboardExportScope executes the visible dashboard datasets plus every
// evidence dataset declared by the dashboard and its panels. Ordinary
// DashboardScope deliberately excludes export-only evidence so loading a page
// never pays the cost of materialising large audit tables.
func DashboardExportScope() Scope {
	return Scope{IncludeExportEvidence: true}
}

// PanelsExportScope is the export-aware counterpart of PanelsScope.
func PanelsExportScope(panelIDs ...string) Scope {
	return Scope{PanelIDs: append([]string(nil), panelIDs...), IncludeExportEvidence: true}
}

// PanelExportScope is the export-aware counterpart of PanelScope.
func PanelExportScope(panelID string) Scope {
	return PanelsExportScope(panelID)
}

func (r *Runtime) Execute(ctx context.Context, spec lens.DashboardSpec, req Request, scope Scope) (*DashboardResult, error) {
	op := serrors.Op("lens/runtime.Run")
	if err := Validate(spec); err != nil {
		return nil, serrors.E(op, err)
	}
	startedAt := time.Now()
	variables, err := resolveVariables(spec.Variables, req)
	if err != nil {
		return nil, serrors.E(op, err)
	}
	internalPlan, err := compileExecutionPlan(spec, scope)
	if err != nil {
		return nil, serrors.E(op, err)
	}

	result := &DashboardResult{
		Spec:        spec,
		Variables:   variables,
		Filters:     filter.Build(spec.Variables, variables),
		Plan:        internalPlan.view,
		Datasets:    make(map[string]*DatasetResult, len(spec.Datasets)),
		Panels:      make(map[string]*PanelResult),
		Locale:      req.Locale,
		Timezone:    req.Timezone,
		RequestPath: strings.TrimSpace(req.Path),
		Request:     sanitizedRequestValues(req.Request),
		Drill:       parseDrillContext(req.Request),
		StartedAt:   startedAt,
	}

	state := plannedExecutor{
		spec:      spec,
		runtime:   req,
		executor:  r,
		variables: variables,
		drill:     result.Drill,
	}
	state.snapshotKey, state.specFingerprint = r.executionIdentity(spec, req, variables)
	result.SnapshotID = state.snapshotKey
	if spec.Cache.Mode != lens.CacheDisabled {
		if snapshot, ok := r.store.Load(ctx, state.snapshotKey); ok {
			if r.observer != nil {
				r.observer.SnapshotHit(snapshot.ID)
			}
			for _, stage := range internalPlan.datasetStages {
				for _, datasetSpec := range stage {
					if frames := snapshot.Datasets[datasetSpec.Name]; frames != nil {
						result.Datasets[datasetSpec.Name] = &DatasetResult{Frames: frames.Clone()}
					}
				}
			}
		} else if r.observer != nil {
			r.observer.SnapshotMiss(state.snapshotKey)
		}
	}

	if err := state.executeDatasets(ctx, internalPlan.datasetStages, result.Datasets); err != nil {
		return nil, serrors.E(op, err)
	}
	if err := state.executePanels(ctx, internalPlan.panels, result.Datasets, result.Panels); err != nil {
		return nil, serrors.E(op, err)
	}
	result.Duration = time.Since(startedAt)
	if spec.Cache.Mode != lens.CacheDisabled {
		r.saveSnapshot(ctx, state, result)
	}
	return result, nil
}

type plannedExecutor struct {
	spec            lens.DashboardSpec
	runtime         Request
	executor        *Runtime
	variables       map[string]any
	drill           *cube.DrillContext
	snapshotKey     string
	specFingerprint string
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
	plan, err := compileExecutionPlan(spec, scope)
	if err != nil {
		return ExecutionPlan{}, serrors.E(op, err)
	}
	return plan.view, nil
}

func compileExecutionPlan(spec lens.DashboardSpec, scope Scope) (executionPlan, error) {
	datasets := indexDatasets(spec.Datasets)
	targetPanels, err := scopedPanels(spec, scope.PanelIDs)
	if err != nil {
		return executionPlan{}, err
	}
	required := requiredDatasetNames(spec, targetPanels, datasets, scope.IncludeExportEvidence)
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
			if _, cached := results[datasetSpec.Name]; cached {
				continue
			}
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
				Panel:       panelSpec,
				Duration:    time.Since(start),
				Locale:      s.runtime.Locale,
				Timezone:    s.runtime.Timezone,
				Variables:   s.variables,
				RequestPath: strings.TrimSpace(s.runtime.Path),
				Request:     sanitizedRequestValues(s.runtime.Request),
				Drill:       s.drill,
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
	flightKey := s.snapshotKey + ":dataset:" + spec.Name + ":" + queryCacheKey(request)
	value, err, _ := s.executor.flights.Do(flightKey, func() (any, error) {
		started := time.Now()
		frames, err := ds.Run(ctx, request)
		if err == nil && s.executor.observer != nil {
			s.executor.observer.DatasetExecuted(spec.Name, time.Since(started))
		}
		return frames, err
	})
	if err != nil {
		return nil, err
	}
	frames, ok := value.(*frame.FrameSet)
	if !ok {
		return nil, fmt.Errorf("dataset %q returned unexpected result %T", spec.Name, value)
	}
	return frames.Clone(), nil
}

func (r *Runtime) executionIdentity(spec lens.DashboardSpec, req Request, variables map[string]any) (string, string) {
	specBytes, err := json.Marshal(spec) //nolint:musttag // Runtime specs intentionally retain their Go field names in the fingerprint.
	if err != nil {
		specBytes = []byte(fmt.Sprintf("%#v", spec))
	}
	specSum := sha256.Sum256(specBytes)
	specFingerprint := fmt.Sprintf("%x", specSum[:])
	identity := struct {
		Version     string            `json:"version"`
		Namespace   string            `json:"namespace"`
		Spec        string            `json:"spec"`
		Locale      string            `json:"locale"`
		Timezone    string            `json:"timezone"`
		DataScope   string            `json:"dataScope"`
		Variables   map[string]any    `json:"variables"`
		DataSources map[string]string `json:"dataSources"`
	}{r.version, req.Namespace, specFingerprint, req.Locale, req.Timezone, req.DataScope, variables, req.DataSourceIdentities}
	payload, err := json.Marshal(identity)
	if err != nil {
		payload = []byte(fmt.Sprintf("%#v", identity))
	}
	sum := sha256.Sum256(payload)
	return strings.TrimSpace(req.Namespace) + ":" + fmt.Sprintf("%x", sum[:]), specFingerprint
}

func (r *Runtime) saveSnapshot(ctx context.Context, state plannedExecutor, result *DashboardResult) {
	if result == nil {
		return
	}
	r.mergeMu.Lock()
	defer r.mergeMu.Unlock()
	datasets := map[string]*frame.FrameSet{}
	provenance := map[string]DatasetProvenance{}
	if existing, ok := r.store.Load(ctx, state.snapshotKey); ok {
		for name, frames := range existing.Datasets {
			datasets[name] = frames.Clone()
		}
		for name, item := range existing.Provenance {
			provenance[name] = item
		}
	}
	byName := indexDatasets(state.spec.Datasets)
	for name, item := range result.Datasets {
		datasetSpec := byName[name]
		if datasetSpec.Cache.Mode == lens.CacheDisabled || item == nil || item.Error != nil || item.Frames == nil {
			continue
		}
		datasets[name] = item.Frames.Clone()
		provenance[name] = DatasetProvenance{Dataset: name, Source: datasetSpec.Source, DependsOn: append([]string(nil), datasetSpec.DependsOn...), Duration: item.Duration}
	}
	ttl := state.spec.Cache.TTL
	if ttl <= 0 {
		ttl = r.ttl
	}
	now := time.Now()
	r.store.Save(ctx, state.snapshotKey, &ExecutionSnapshot{ID: state.snapshotKey, SpecFingerprint: state.specFingerprint, Variables: cloneMap(state.variables), DataScope: state.runtime.DataScope, Locale: state.runtime.Locale, Timezone: state.runtime.Timezone, Datasets: datasets, Provenance: provenance, CreatedAt: now, ExpiresAt: now.Add(ttl)}, ttl)
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

func requiredDatasetNames(spec lens.DashboardSpec, panels []panel.Spec, datasets map[string]lens.DatasetSpec, includeExportEvidence bool) []string {
	seen := make(map[string]struct{})
	for _, panelSpec := range panels {
		if isCompositePanel(panelSpec.Kind) || strings.TrimSpace(panelSpec.Dataset) == "" {
			continue
		}
		markRequiredDataset(panelSpec.Dataset, datasets, seen)
	}
	if includeExportEvidence {
		for _, name := range spec.Export.EvidenceDatasets {
			markRequiredDataset(name, datasets, seen)
		}
		for _, panelSpec := range panels {
			evidence := panelSpec.Export.EvidenceDatasets
			if len(evidence) == 0 {
				if datasetSpec, ok := datasets[panelSpec.Dataset]; ok {
					evidence = datasetSpec.Export.EvidenceDatasets
				}
			}
			for _, name := range evidence {
				markRequiredDataset(name, datasets, seen)
			}
		}
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
	return kind.IsContainer()
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

func parseDrillContext(values url.Values) *cube.DrillContext {
	ctx := cube.ParseDrillContext(values)
	return &ctx
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

func sanitizedRequestValues(values url.Values) url.Values {
	cloned := cloneValues(values)
	delete(cloned, TablePaginationPanelQuery)
	delete(cloned, TablePaginationPageQuery)
	delete(cloned, TablePaginationLimitQuery)
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
	if err := validateExplorers(spec, datasets, panelIDs); err != nil {
		return wrap(err)
	}
	return nil
}

func validateExplorers(spec lens.DashboardSpec, datasets map[string]lens.DatasetSpec, panelIDs map[string]struct{}) error {
	seen := make(map[string]struct{}, len(spec.Explorers))
	explorers := make(map[string]explore.Spec, len(spec.Explorers))
	for _, explorerSpec := range spec.Explorers {
		if err := explorerSpec.Validate(); err != nil {
			return err
		}
		if _, ok := seen[explorerSpec.ID]; ok {
			return fmt.Errorf("duplicate explorer %s", explorerSpec.ID)
		}
		seen[explorerSpec.ID] = struct{}{}
		explorers[explorerSpec.ID] = explorerSpec
		if _, ok := panelIDs[explorerSpec.HostPanelID]; !ok {
			return fmt.Errorf("explorer %s references missing host panel %s", explorerSpec.ID, explorerSpec.HostPanelID)
		}
		for _, branch := range explorerSpec.Branches {
			for _, perspective := range branch.Perspectives {
				for _, node := range perspective.Nodes {
					if node.Panel != nil {
						if err := validatePanel(*node.Panel, datasets, make(map[string]struct{})); err != nil {
							return fmt.Errorf("explorer %s branch %s perspective %s node %s: %w", explorerSpec.ID, branch.Key, perspective.Key, node.Key, err)
						}
					}
					for _, edge := range node.Edges {
						if err := validateAction("explorer "+explorerSpec.ID+" edge "+edge.PointKey, edge.Action, actionValidationOptions{allowFieldSources: true}); err != nil {
							return err
						}
					}
				}
			}
		}
	}
	for _, row := range spec.Rows {
		for _, panelSpec := range row.Panels {
			if err := validatePanelExploreReferences(panelSpec, explorers); err != nil {
				return err
			}
		}
	}
	return nil
}

func validatePanelExploreReferences(panelSpec panel.Spec, explorers map[string]explore.Spec) error {
	if err := validateExploreReference("panel "+panelSpec.ID, panelSpec.Action, explorers); err != nil {
		return err
	}
	for _, column := range panelSpec.Columns {
		if err := validateExploreReference("panel "+panelSpec.ID+" column "+column.Field.Name(), column.Action, explorers); err != nil {
			return err
		}
	}
	for _, child := range panelSpec.Children {
		if err := validatePanelExploreReferences(child, explorers); err != nil {
			return err
		}
	}
	return nil
}

func validateExploreReference(owner string, actionSpec *action.Spec, explorers map[string]explore.Spec) error {
	if actionSpec == nil || actionSpec.Kind != action.KindExplore || actionSpec.Explore == nil {
		return nil
	}
	explorerSpec, ok := explorers[actionSpec.Explore.ExplorerID]
	if !ok {
		return fmt.Errorf("%s references missing explorer %q", owner, actionSpec.Explore.ExplorerID)
	}
	if actionSpec.Explore.Branch.Kind != action.SourceLiteral {
		return nil
	}
	branchKey := fmt.Sprint(actionSpec.Explore.Branch.Value)
	branch, ok := explorerSpec.Branch(branchKey)
	if !ok {
		return fmt.Errorf("%s references missing explorer branch %q", owner, branchKey)
	}
	if actionSpec.Explore.Perspective != "" {
		if _, ok := branch.Perspective(actionSpec.Explore.Perspective); !ok {
			return fmt.Errorf("%s references missing explorer perspective %q", owner, actionSpec.Explore.Perspective)
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
	switch {
	case spec.Kind.IsContainer():
		for _, child := range spec.Children {
			if err := validatePanel(child, datasets, panelIDs); err != nil {
				return err
			}
		}
		return nil
	case spec.Kind.IsChart() || spec.Kind.RendersNatively():
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
	case panel.KindStat, panel.KindTable, panel.KindTabs, panel.KindGrid, panel.KindSplit, panel.KindRepeat, panel.KindStatGroup:
		// These panel kinds do not require label/category validation here.
	case panel.KindBar, panel.KindHorizontalBar, panel.KindSegmentBar, panel.KindCascade, panel.KindPie, panel.KindDonut, panel.KindGauge:
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
	if err := validateAction("panel "+spec.ID, spec.Action, actionValidationOptions{allowCubeDrill: true, allowFieldSources: true}); err != nil {
		return err
	}
	if err := validateDrillTree(spec); err != nil {
		return err
	}
	if math.IsNaN(spec.CircularScale) || math.IsInf(spec.CircularScale, 0) || spec.CircularScale < 0 {
		return fmt.Errorf("panel %s circular scale must be zero or a positive finite value", spec.ID)
	}
	return nil
}

type actionValidationOptions struct {
	allowCubeDrill    bool
	allowFieldSources bool
}

func validateAction(owner string, spec *action.Spec, opts actionValidationOptions) error {
	if spec == nil {
		return nil
	}
	if strings.TrimSpace(spec.URL) == "" && spec.URLSource == nil && spec.Kind != action.KindEmitEvent && spec.Kind != action.KindExplore {
		return fmt.Errorf("%s action requires url", owner)
	}
	switch spec.Kind {
	case action.KindNavigate, action.KindHtmxSwap, action.KindEmitEvent:
	case action.KindCubeDrill:
		if !opts.allowCubeDrill {
			return fmt.Errorf("%s action has unsupported kind %q", owner, spec.Kind)
		}
	case action.KindExplore:
		if spec.Explore == nil {
			return fmt.Errorf("%s explore action requires explore spec", owner)
		}
		if strings.TrimSpace(spec.Explore.ExplorerID) == "" {
			return fmt.Errorf("%s explore action requires explorer id", owner)
		}
		if err := validateActionValueSource(owner, "branch", spec.Explore.Branch, opts); err != nil {
			return err
		}
	default:
		return fmt.Errorf("%s action has unsupported kind %q", owner, spec.Kind)
	}
	if spec.Kind == action.KindEmitEvent && strings.TrimSpace(spec.Event) == "" {
		return fmt.Errorf("%s emit event action requires event name", owner)
	}
	if spec.Kind == action.KindHtmxSwap && strings.TrimSpace(spec.Target) == "" {
		return fmt.Errorf("%s htmx action requires target", owner)
	}
	if spec.URLSource != nil {
		if err := validateActionValueSource(owner, "url", *spec.URLSource, opts); err != nil {
			return err
		}
	}
	for _, param := range spec.Params {
		if err := validateActionKey(owner, "parameter name", param.Name); err != nil {
			return err
		}
		if err := validateActionValueSource(owner, param.Name, param.Source, opts); err != nil {
			return err
		}
	}
	for name, source := range spec.Payload {
		if err := validateActionKey(owner, "payload key", name); err != nil {
			return err
		}
		if err := validateActionValueSource(owner, name, source, opts); err != nil {
			return err
		}
	}
	return nil
}

func validateActionKey(owner, kind, key string) error {
	trimmed := strings.TrimSpace(key)
	if trimmed == "" {
		return fmt.Errorf("%s action %s cannot be blank", owner, kind)
	}
	if trimmed != key {
		return fmt.Errorf("%s action %s %q has surrounding whitespace", owner, kind, key)
	}
	return nil
}

func validateActionValueSource(owner, name string, source action.ValueSource, opts actionValidationOptions) error {
	if source.Kind == action.SourceField && !opts.allowFieldSources {
		return fmt.Errorf("%s action value %s cannot use a field source", owner, name)
	}
	return validateValueSource(owner, name, source)
}

func validateDrillTree(spec panel.Spec) error {
	if spec.DrillTree == nil {
		return nil
	}
	if spec.DrillHierarchy != nil {
		return fmt.Errorf("panel %s drill tree cannot be combined with bar drill hierarchy", spec.ID)
	}
	if spec.Kind != panel.KindPie && spec.Kind != panel.KindDonut {
		return fmt.Errorf("panel %s drill tree is unsupported for kind %q", spec.ID, spec.Kind)
	}
	if spec.Fields.ID.Empty() {
		return fmt.Errorf("panel %s drill tree requires id field", spec.ID)
	}
	if len(spec.DrillTree.Branches) == 0 {
		return fmt.Errorf("panel %s drill tree requires at least one branch", spec.ID)
	}
	if spec.DrillTree.ExpandedSpan < 0 || spec.DrillTree.ExpandedSpan > 12 {
		return fmt.Errorf("panel %s drill tree expanded span must be between 1 and 12 when configured", spec.ID)
	}

	branchKeys := make(map[string]struct{}, len(spec.DrillTree.Branches))
	for i, branch := range spec.DrillTree.Branches {
		key := strings.TrimSpace(branch.TriggerKey)
		if key == "" {
			return fmt.Errorf("panel %s drill tree branch %d requires trigger key", spec.ID, i)
		}
		if key != branch.TriggerKey {
			return fmt.Errorf("panel %s drill tree branch key %q has surrounding whitespace", spec.ID, branch.TriggerKey)
		}
		if _, exists := branchKeys[key]; exists {
			return fmt.Errorf("panel %s drill tree has duplicate branch key %q", spec.ID, key)
		}
		branchKeys[key] = struct{}{}
		if strings.TrimSpace(branch.Label) == "" {
			return fmt.Errorf("panel %s drill tree branch %q requires label", spec.ID, key)
		}
		if len(branch.Children) == 0 {
			return fmt.Errorf("panel %s drill tree branch %q requires children", spec.ID, key)
		}
		if err := validateDrillLevelView(spec.ID, "branch "+key, branch.View); err != nil {
			return err
		}
		if err := validateDrillNodes(spec.ID, key, branch.Children); err != nil {
			return err
		}
	}
	return nil
}

func validateDrillLevelView(panelID, owner string, view *panel.DrillLevelView) error {
	if view == nil {
		return nil
	}
	switch view.LegendPosition {
	case "", panel.LegendTop, panel.LegendRight, panel.LegendBottom, panel.LegendLeft:
	default:
		return fmt.Errorf("panel %s drill tree %s has invalid legend position %q", panelID, owner, view.LegendPosition)
	}
	if view.LegendWidthPx < 0 {
		return fmt.Errorf("panel %s drill tree %s legend width cannot be negative", panelID, owner)
	}
	if math.IsNaN(view.CircularScale) || math.IsInf(view.CircularScale, 0) || view.CircularScale < 0 {
		return fmt.Errorf("panel %s drill tree %s circular scale must be zero or a positive finite value", panelID, owner)
	}
	return nil
}

func validateDrillNodes(panelID, parentPath string, nodes []panel.DrillNode) error {
	keys := make(map[string]struct{}, len(nodes))
	total := 0.0
	for i, node := range nodes {
		key := strings.TrimSpace(node.Key)
		if key == "" {
			return fmt.Errorf("panel %s drill tree node %s[%d] requires key", panelID, parentPath, i)
		}
		if key != node.Key {
			return fmt.Errorf("panel %s drill tree node key %q has surrounding whitespace", panelID, node.Key)
		}
		if _, exists := keys[key]; exists {
			return fmt.Errorf("panel %s drill tree node %s has duplicate child key %q", panelID, parentPath, key)
		}
		keys[key] = struct{}{}
		path := parentPath + "/" + key
		if strings.TrimSpace(node.Label) == "" {
			return fmt.Errorf("panel %s drill tree node %s requires label", panelID, path)
		}
		if math.IsNaN(node.Value) || math.IsInf(node.Value, 0) || node.Value < 0 {
			return fmt.Errorf("panel %s drill tree node %s requires finite nonnegative value", panelID, path)
		}
		total += node.Value
		if math.IsNaN(total) || math.IsInf(total, 0) {
			return fmt.Errorf("panel %s drill tree node group %s requires finite total", panelID, parentPath)
		}
		if len(node.Children) > 0 && node.Action != nil {
			return fmt.Errorf("panel %s drill tree node %s cannot have both children and action", panelID, path)
		}
		if err := validateDrillLevelView(panelID, "node "+path, node.View); err != nil {
			return err
		}
		if err := validateAction("panel "+panelID+" drill tree node "+path, node.Action, actionValidationOptions{}); err != nil {
			return err
		}
		if len(node.Children) > 0 {
			if err := validateDrillNodes(panelID, path, node.Children); err != nil {
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
	if spec.DrillTree != nil {
		if err := validateDrillTreeFrame(spec, primary); err != nil {
			return err
		}
	}
	if spec.Kind == panel.KindTable {
		for _, column := range spec.Columns {
			if column.Field.Empty() {
				if column.Action != nil {
					continue // action-only columns (e.g. "View" links) don't need a field
				}
				return fmt.Errorf("panel %s has table column without field reference", spec.ID)
			}
			if _, ok := primary.Field(column.Field.Name()); !ok {
				return fmt.Errorf("panel %s is missing table column field %q in dataset %s", spec.ID, column.Field.Name(), spec.Dataset)
			}
		}
	}
	if spec.Action != nil {
		if spec.Action.URLSource != nil {
			if err := validateFrameValueSource(spec.ID, spec.Dataset, primary, *spec.Action.URLSource); err != nil {
				return err
			}
		}
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
	}
	return nil
}

func validateDrillTreeFrame(spec panel.Spec, primary *frame.Frame) error {
	field, ok := primary.Field(spec.Fields.ID.Name())
	if !ok {
		return fmt.Errorf("panel %s drill tree is missing id field %q in dataset %s", spec.ID, spec.Fields.ID.Name(), spec.Dataset)
	}
	counts := make(map[string]int, len(field.Values))
	for i, value := range field.Values {
		key, ok := value.(string)
		if !ok || strings.TrimSpace(key) == "" || key != strings.TrimSpace(key) {
			return fmt.Errorf("panel %s drill tree id field %q row %d requires a nonblank string", spec.ID, spec.Fields.ID.Name(), i)
		}
		counts[key]++
		if counts[key] > 1 {
			return fmt.Errorf("panel %s drill tree id field %q has duplicate key %q in dataset %s", spec.ID, spec.Fields.ID.Name(), key, spec.Dataset)
		}
	}
	for _, branch := range spec.DrillTree.Branches {
		switch counts[branch.TriggerKey] {
		case 1:
		case 0:
			return fmt.Errorf("panel %s drill tree branch key %q is missing from dataset %s", spec.ID, branch.TriggerKey, spec.Dataset)
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
	case panel.KindBar, panel.KindHorizontalBar, panel.KindSegmentBar, panel.KindPie, panel.KindDonut, panel.KindGauge:
		if err := requireField(spec, primary, spec.Fields.Value); err != nil {
			return err
		}
		if err := requireOneField(spec, primary, spec.Fields.Label, spec.Fields.Category); err != nil {
			return err
		}
	case panel.KindCascade:
		if err := requireField(spec, primary, spec.Fields.Value); err != nil {
			return err
		}
		if err := requireOneField(spec, primary, spec.Fields.Label, spec.Fields.Category); err != nil {
			return err
		}
		for _, field := range []panel.FieldRef{spec.Fields.Cut, spec.Fields.CutLabel, spec.Fields.Final} {
			if err := requireField(spec, primary, field); err != nil {
				return err
			}
		}
	case panel.KindStackedBar:
		if err := requireField(spec, primary, spec.Fields.Category); err != nil {
			return err
		}
		if err := requireField(spec, primary, spec.Fields.Series); err != nil {
			return err
		}
		return requireField(spec, primary, spec.Fields.Value)
	case panel.KindTable, panel.KindTabs, panel.KindGrid, panel.KindSplit, panel.KindRepeat, panel.KindStatGroup:
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

func validateValueSource(owner, name string, source action.ValueSource) error {
	switch source.Kind {
	case action.SourceField, action.SourceVariable:
		if strings.TrimSpace(source.Name) == "" {
			return fmt.Errorf("%s action value %s requires source name", owner, name)
		}
	case action.SourceLiteral:
		if source.Value == nil {
			return fmt.Errorf("%s action value %s requires literal value", owner, name)
		}
	default:
		return fmt.Errorf("%s action value %s has unsupported source kind %q", owner, name, source.Kind)
	}
	return nil
}
