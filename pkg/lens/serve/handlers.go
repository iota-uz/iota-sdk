package serve

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/iota-uz/iota-sdk/pkg/lens"
	"github.com/iota-uz/iota-sdk/pkg/lens/document"
	lensexport "github.com/iota-uz/iota-sdk/pkg/lens/export"
	"github.com/iota-uz/iota-sdk/pkg/lens/panel"
	lensruntime "github.com/iota-uz/iota-sdk/pkg/lens/runtime"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
)

const (
	maxQueryBodyBytes = 1 << 20
	maxQueryPathDepth = 64
	maxQueryPathEntry = 256
	recomputeCooldown = 5 * time.Second
)

type errorResponse = document.QueryErrorResponse
type QueryRequest = document.QueryRequest
type Page = document.QueryPage
type QueryResponse = document.QueryResponse
type PanelRequest = document.PanelRequest
type PanelResponse = document.PanelResponse
type PanelBatchRequest = document.PanelBatchRequest
type PanelBatchResponse = document.PanelBatchResponse
type PanelBatchStreamEvent = document.PanelBatchStreamEvent

// Document executes and returns a new snapshot-backed dashboard document.
func (h *Handlers) Document(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, document.QueryErrorBadRequest, "method must be GET")
		return
	}
	req := h.runtimeRequest(r)
	scope := lensruntime.DashboardScope()
	if h.progressive {
		scope = lensruntime.ShellScope()
	}
	result, err := h.engine.Execute(r.Context(), h.spec, req, scope)
	if err != nil {
		h.writeExecutionError(r.Context(), w, err)
		return
	}
	if result == nil {
		h.writeInternalError(r.Context(), w, "lens/serve.Document", "dashboard execution failed", fmt.Errorf("executor returned a nil dashboard result"))
		return
	}
	if result.Panels == nil {
		result.Panels = make(map[string]*lensruntime.PanelResult)
	}
	frozen := freezeParams(result, req)
	for _, target := range inlineTargets(h.spec, h.inlineDepth) {
		if h.progressive {
			break
		}
		existing := result.Panel(target.panel.ID)
		if existing != nil && existing.Error == nil && existing.Frames != nil && existing.Frames.Primary() != nil {
			continue
		}
		levelResult, execErr := h.executeLevel(r.Context(), req, frozen, target, 0)
		if execErr != nil {
			if levelResult != nil && levelResult.Error != nil {
				continue
			}
			h.writeExecutionError(r.Context(), w, execErr)
			return
		}
		result.Panels[target.panel.ID] = levelResult
	}
	for _, target := range sourceDataTargets(h.spec, h.inlineDepth) {
		if h.progressive {
			break
		}
		existing := result.Panel(target.panel.ID)
		if existing != nil && existing.Error == nil && existing.Frames != nil && existing.Frames.Primary() != nil {
			continue
		}
		sourceResult, execErr := h.executeSourcePanel(r.Context(), req, frozen, target)
		if execErr != nil {
			// A panel-level failure drops the disclosure (document.Build skips
			// declarations without an executed frame) instead of failing the
			// whole document; transport failures still surface.
			if sourceResult != nil && sourceResult.Error != nil {
				continue
			}
			h.writeExecutionError(r.Context(), w, execErr)
			return
		}
		result.Panels[target.panel.ID] = sourceResult
	}
	doc, err := document.Build(h.spec, result, document.BuildOptions{
		Locale: result.Locale, InlineDepth: h.inlineDepth,
		I18n: document.RuntimeI18nDefaults(),
		Endpoints: document.Endpoints{
			Query: h.endpoint("/lens/query"), Export: h.endpoint("/export"),
			Release: h.endpoint("/lens/release"),
			Panel: func() string {
				if h.progressive {
					return h.endpoint("/lens/panel")
				}
				return ""
			}(),
			Drawer: func() string {
				if h.drawerResolver != nil {
					return h.endpoint("/lens/drawer")
				}
				return ""
			}(),
		},
		DeferPanels: h.progressive,
		Filters:     wireVariableFilters(result.Filters),
		URLState:    &document.URLStateContract{Version: document.URLStateVersion, Param: document.URLStateParam, MaxBytes: document.URLStateMaxBytes},
	})
	if err != nil {
		h.writeInternalError(r.Context(), w, "lens/serve.Document", "document build failed", err)
		return
	}
	if err := h.snapshots.Put(r.Context(), &document.Snapshot{
		ID: doc.SnapshotID, Params: frozen, Frames: doc.Frames, Levels: doc.Drill.Edges, CreatedAt: doc.Meta.GeneratedAt,
	}); err != nil {
		if r.Context().Err() != nil {
			return
		}
		h.writeInternalError(r.Context(), w, "lens/serve.Document", "snapshot storage failed", err)
		return
	}
	writeJSON(w, http.StatusOK, doc)
}

// Release detaches a browser runtime from a snapshot execution session and
// cancels only its speculative queue. The snapshot itself remains available
// for Back/export until the configured SnapshotStore expires it.
func (h *Handlers) Release(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, document.QueryErrorBadRequest, "method must be POST")
		return
	}
	var req document.ReleaseRequest
	if err := decodeJSON(w, r, &req); err != nil {
		writeError(w, http.StatusBadRequest, document.QueryErrorBadRequest, err.Error())
		return
	}
	req.SnapshotID = strings.TrimSpace(req.SnapshotID)
	if req.SnapshotID == "" {
		writeError(w, http.StatusBadRequest, document.QueryErrorBadRequest, "snapshotId is required")
		return
	}
	snapshot, err := h.snapshots.Get(r.Context(), req.SnapshotID)
	if err != nil {
		h.writeSnapshotError(r.Context(), w, err)
		return
	}
	if !sameSnapshotScope(h.runtimeRequest(r), snapshot.Params) {
		h.writeSnapshotError(r.Context(), w, document.ErrSnapshotGone)
		return
	}
	h.releaseSession(req.SnapshotID)
	w.WriteHeader(http.StatusNoContent)
}

type loadedPanel struct {
	frame       document.Frame
	calculation document.PanelCalculation
}

// Panel materialises a bounded batch of top-level panels against one immutable
// snapshot. Snapshot identity and scope reject the whole batch before response
// headers. Each panel result is then flushed as one NDJSON event as soon as it
// completes, so a slow sibling never delays progressive reveal. Validation and
// execution failures stay isolated to their panel result.
func (h *Handlers) Panel(w http.ResponseWriter, r *http.Request) {
	started := time.Now()
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, document.QueryErrorBadRequest, "method must be POST")
		return
	}
	var req PanelBatchRequest
	if err := decodeJSON(w, r, &req); err != nil {
		writeError(w, http.StatusBadRequest, document.QueryErrorBadRequest, err.Error())
		return
	}
	req.SnapshotID = strings.TrimSpace(req.SnapshotID)
	if req.SnapshotID == "" {
		writeError(w, http.StatusBadRequest, document.QueryErrorBadRequest, "snapshotId is required")
		return
	}
	if len(req.Panels) == 0 || len(req.Panels) > 64 {
		writeError(w, http.StatusBadRequest, document.QueryErrorBadRequest, "panels must contain 1 to 64 entries")
		return
	}
	snapshot, err := h.snapshots.Get(r.Context(), req.SnapshotID)
	if err != nil {
		h.writeSnapshotError(r.Context(), w, err)
		return
	}
	current := h.runtimeRequest(r)
	if !sameSnapshotScope(current, snapshot.Params) {
		// Conceal cross-scope snapshot existence exactly like an expired ID.
		h.writeSnapshotError(r.Context(), w, document.ErrSnapshotGone)
		return
	}
	seen := make(map[string]struct{}, len(req.Panels))
	for index := range req.Panels {
		req.Panels[index].PanelID = strings.TrimSpace(req.Panels[index].PanelID)
		req.Panels[index].Search = strings.TrimSpace(req.Panels[index].Search)
		if req.Panels[index].PanelID == "" {
			writeError(w, http.StatusBadRequest, document.QueryErrorBadRequest, "panelId is required")
			return
		}
		if _, duplicate := seen[req.Panels[index].PanelID]; duplicate {
			writeError(w, http.StatusBadRequest, document.QueryErrorBadRequest, "panelId entries must be unique")
			return
		}
		if rank := req.Panels[index].ViewportRank; rank != nil && (*rank < 0 || *rank > 2) {
			writeError(w, http.StatusBadRequest, document.QueryErrorBadRequest, "viewportRank must be between 0 and 2")
			return
		}
		seen[req.Panels[index].PanelID] = struct{}{}
	}
	flusher, ok := w.(http.Flusher)
	if !ok {
		h.writeInternalError(r.Context(), w, "lens/serve.Panel", "streaming is unavailable", fmt.Errorf("response writer does not support flushing"))
		return
	}
	type completedPanel struct {
		panelID  string
		priority int
		order    int
		result   document.PanelBatchResult
	}
	completed := make(chan completedPanel, len(req.Panels))
	priorityCounts := make(map[int]int)
	priorityOrder := make([]int, 0)
	session := h.session(req.SnapshotID)
	priorities := panelExecutionPriorities(h.plan)
	slices.SortStableFunc(req.Panels, func(left, right PanelRequest) int {
		leftPriority, leftOrder := priorities.forRequest(left)
		rightPriority, rightOrder := priorities.forRequest(right)
		if leftPriority != rightPriority {
			return leftPriority - rightPriority
		}
		return leftOrder - rightOrder
	})
	for _, panelReq := range req.Panels {
		priority, order := priorities.forRequest(panelReq)
		priority = h.effectivePanelPriority(req.SnapshotID, r.Header.Get("X-Lens-Prefetch"), priority)
		if priorityCounts[priority] == 0 {
			priorityOrder = append(priorityOrder, priority)
		}
		priorityCounts[priority]++
		queued := h.queuePanelResult(r.Context(), session, priority, order, panelReq, snapshot, current)
		panelID := panelReq.PanelID
		go func() {
			select {
			case result := <-queued:
				select {
				case completed <- completedPanel{panelID: panelID, priority: priority, order: order, result: result}:
				case <-r.Context().Done():
				}
			case <-r.Context().Done():
			}
		}()
	}
	session.prefetchOnce.Do(func() {
		h.startBackgroundPrefetch(r.Context(), session, snapshot, current)
	})
	w.Header().Set("Content-Type", "application/x-ndjson")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)
	encoder := json.NewEncoder(w)
	buffered := make(map[int][]completedPanel)
	emittedByPriority := make(map[int]int)
	emitted := 0
	priorityIndex := 0
	for emitted < len(req.Panels) {
		select {
		case item := <-completed:
			buffered[item.priority] = append(buffered[item.priority], item)
			for priorityIndex < len(priorityOrder) {
				priority := priorityOrder[priorityIndex]
				items := buffered[priority]
				if len(items) == 0 {
					break
				}
				slices.SortStableFunc(items, func(left, right completedPanel) int { return left.order - right.order })
				for _, ready := range items {
					event := PanelBatchStreamEvent{PanelID: ready.panelID, Result: &ready.result}
					if err := encoder.Encode(event); err != nil {
						return
					}
					flusher.Flush()
					emitted++
					emittedByPriority[priority]++
				}
				delete(buffered, priority)
				if emittedByPriority[priority] < priorityCounts[priority] {
					break
				}
				if priorityIndex == 0 {
					session.enableBackground()
					h.observeMetric(r.Context(), Metric{Name: MetricTimeToFirstUsefulKPI, Value: float64(time.Since(started).Microseconds()) / 1000})
				}
				priorityIndex++
			}
		case <-r.Context().Done():
			return
		}
	}
	if err := encoder.Encode(PanelBatchStreamEvent{Complete: true}); err != nil {
		return
	}
	flusher.Flush()
}

func (h *Handlers) queuePanelResult(
	ctx context.Context,
	session *executionSession,
	priority int,
	order int,
	req PanelRequest,
	snapshot *document.Snapshot,
	current lensruntime.Request,
) <-chan document.PanelBatchResult {
	panelSpec, validationErr := h.validatePanelRequest(req)
	if validationErr != nil {
		return immediatePanelResult(panelError(document.QueryErrorBadRequest, validationErr.Error()))
	}
	if req.Recompute {
		if calculation, ok := snapshot.Panels[panelSpec.ID]; ok && time.Since(calculation.CalculatedAt) < recomputeCooldown {
			return immediatePanelResult(panelError(document.QueryErrorBadRequest, "panel was recomputed recently"))
		}
	}
	ref := document.FrameRef("panel:" + panelSpec.ID)
	if !req.Recompute {
		calculation, materialized := snapshot.Panels[panelSpec.ID]
		if cached, exists := snapshot.Frames[ref]; exists && materialized {
			calculation.CacheHit = true
			if calculation.CalculatedAt.IsZero() {
				calculation.CalculatedAt = snapshot.CreatedAt
			}
			view, summary := tableFrameView(panelSpec, cached, req.Search, req.Sort)
			view, page := paginatePanelFrame(panelSpec, view, req.Page, h.pageSize)
			return immediatePanelResult(successfulPanel(ref, view, calculation, summary, page))
		}
	}
	loaded := h.queuePanelValue(ctx, session, priority, order, req, snapshot, panelSpec, ref, current)
	result := make(chan document.PanelBatchResult, 1)
	go func() {
		defer close(result)
		defer loaded.Cancel()
		select {
		case <-ctx.Done():
			return
		case scheduled := <-loaded.result:
			if scheduled.err != nil {
				if errors.Is(scheduled.err, document.ErrSnapshotGone) {
					result <- panelError(document.QueryErrorSnapshotGone, "snapshot expired or was not found")
					return
				}
				h.observer.OnError(ctx, "lens/serve.Panel", scheduled.err)
				result <- panelError(document.QueryErrorInternal, "panel execution failed")
				return
			}
			panel, ok := scheduled.value.(loadedPanel)
			if !ok {
				h.observer.OnError(ctx, "lens/serve.Panel", fmt.Errorf("panel execution returned %T", scheduled.value))
				result <- panelError(document.QueryErrorInternal, "panel execution failed")
				return
			}
			view, summary := tableFrameView(panelSpec, panel.frame, req.Search, req.Sort)
			view, page := paginatePanelFrame(panelSpec, view, req.Page, h.pageSize)
			result <- successfulPanel(ref, view, panel.calculation, summary, page)
		}
	}()
	return result
}

func immediatePanelResult(value document.PanelBatchResult) <-chan document.PanelBatchResult {
	result := make(chan document.PanelBatchResult, 1)
	result <- value
	close(result)
	return result
}

func (h *Handlers) validatePanelRequest(req PanelRequest) (panel.Spec, error) {
	if req.Recompute && !h.progressive {
		return panel.Spec{}, fmt.Errorf("recompute is available only for progressive dashboards")
	}
	if len([]rune(req.Search)) > 200 {
		return panel.Spec{}, fmt.Errorf("search cannot exceed 200 characters")
	}
	if req.Page < 0 || req.Page > lensruntime.MaxTablePage {
		return panel.Spec{}, fmt.Errorf("page is outside the supported range")
	}
	panelSpec, ok := lens.FindPanel(h.spec, req.PanelID)
	if !ok || panelSpec.Kind.IsContainer() {
		return panel.Spec{}, fmt.Errorf("panel is not available")
	}
	if req.Search != "" && (panelSpec.Kind != panel.KindTable || panelSpec.Table == nil || !panelSpec.Table.Searchable) {
		return panel.Spec{}, fmt.Errorf("panel is not searchable")
	}
	if err := validateTableSort(panelSpec, req.Sort); err != nil {
		return panel.Spec{}, err
	}
	return panelSpec, nil
}

func (h *Handlers) queuePanelValue(
	base context.Context,
	session *executionSession,
	priority int,
	order int,
	req PanelRequest,
	snapshot *document.Snapshot,
	panelSpec panel.Spec,
	ref document.FrameRef,
	current lensruntime.Request,
) scheduledCall {
	key := "panel:" + snapshot.ID + ":" + panelSpec.ID
	if req.Recompute {
		key += ":recompute"
	}
	return session.submit(base, key, priority, order, func(workCtx context.Context) (any, error) {
		latest, err := h.snapshots.Get(workCtx, snapshot.ID)
		if err != nil {
			return nil, err
		}
		if !sameSnapshotScope(current, latest.Params) {
			return nil, document.ErrSnapshotGone
		}
		if !req.Recompute {
			calculation, materialized := latest.Panels[panelSpec.ID]
			if cached, exists := latest.Frames[ref]; exists && materialized {
				calculation.CacheHit = true
				return loadedPanel{frame: cached, calculation: calculation}, nil
			}
		}
		base := thawRuntimeRequest(current, latest.Params)
		base.Recompute = req.Recompute
		base.ExecutionClass = datasourceExecutionClass(priority)
		executed, err := h.engine.Execute(workCtx, h.spec, base, lensruntime.PanelScope(panelSpec.ID))
		if err != nil {
			return nil, err
		}
		if executed == nil {
			return nil, fmt.Errorf("panel executor returned a nil dashboard result")
		}
		panelResult := executed.Panel(panelSpec.ID)
		if panelResult == nil {
			return nil, fmt.Errorf("panel %q is missing from runtime result", panelSpec.ID)
		}
		if panelResult.Error != nil {
			return nil, panelResult.Error
		}
		projectionSpec, err := progressiveProjectionSpec(panelSpec, panelResult.Panel)
		if err != nil {
			return nil, err
		}
		wire, err := wireFrame(ref, projectionSpec, nil, panelResult)
		if err != nil {
			return nil, err
		}
		calculation := document.PanelCalculation{
			DurationMS: executed.Duration.Milliseconds(), CacheHit: executed.CacheHit, CalculatedAt: time.Now().UTC(),
		}
		if err := h.snapshots.PutPanel(workCtx, latest.ID, panelSpec.ID, ref, wire, calculation); err != nil {
			return nil, err
		}
		return loadedPanel{frame: wire, calculation: calculation}, nil
	})
}

// progressiveProjectionSpec keeps the document's structural panel contract
// frozen while accepting rendering metadata that can only be known after the
// deferred panel executes against real data.
func progressiveProjectionSpec(frozen panel.Spec, executed panel.Spec) (panel.Spec, error) {
	if executed.ID != frozen.ID {
		return panel.Spec{}, fmt.Errorf("executed panel %q does not match frozen panel %q", executed.ID, frozen.ID)
	}
	if executed.Kind != frozen.Kind {
		return panel.Spec{}, fmt.Errorf("executed panel %q changed kind from %q to %q", frozen.ID, frozen.Kind, executed.Kind)
	}
	result := frozen
	result.TotalBadgeValue = executed.TotalBadgeValue
	result.Presentation = executed.Presentation
	result.Colors = append([]string(nil), executed.Colors...)
	// A ring's Total is the whole its rows reconcile against — the same kind of
	// figure as TotalBadgeValue, and knowable only once the panel has run. Left
	// frozen it came from the layout skeleton, so every slice printed its own
	// amount against a placeholder whole: «874 936 990 251,0 %» where the ring
	// carried 92,01 млрд. The ring set itself stays structural; only the
	// executed totals are adopted, and only for rings the frozen spec declares.
	if result.Radial != nil && executed.Radial != nil {
		radial := *result.Radial
		radial.Rings = append([]panel.RadialRing(nil), result.Radial.Rings...)
		totals := make(map[string]float64, len(executed.Radial.Rings))
		for _, ring := range executed.Radial.Rings {
			totals[ring.Key] = ring.Total
		}
		for index := range radial.Rings {
			if total, ok := totals[radial.Rings[index].Key]; ok {
				radial.Rings[index].Total = total
			}
		}
		result.Radial = &radial
	}
	return result, nil
}

func successfulPanel(ref document.FrameRef, frame document.Frame, calculation document.PanelCalculation, summary *document.TableSummary, page *document.QueryPage) document.PanelBatchResult {
	return document.PanelBatchResult{
		Frames: map[document.FrameRef]document.Frame{ref: frame}, Calculation: &calculation, Summary: summary, Page: page,
	}
}

func validateTableSort(spec panel.Spec, ordering *document.TableSort) error {
	if ordering == nil {
		return nil
	}
	if spec.Kind != panel.KindTable || spec.Presentation.NonSortable {
		return fmt.Errorf("panel is not sortable")
	}
	ordering.Field = strings.TrimSpace(ordering.Field)
	if ordering.Direction != document.SortAscending && ordering.Direction != document.SortDescending {
		return fmt.Errorf("sort direction must be asc or desc")
	}
	for _, column := range spec.Columns {
		if column.Field.Name() == ordering.Field {
			return nil
		}
	}
	return fmt.Errorf("sort field is not a declared table column")
}

func paginatePanelFrame(spec panel.Spec, source document.Frame, requested, pageSize int) (document.Frame, *document.QueryPage) {
	if spec.Kind != panel.KindTable || (requested <= 0 && len(source.Rows) <= pageSize) {
		return source, nil
	}
	page := requested
	if page <= 0 {
		page = lensruntime.DefaultTablePage
	}
	start := (page - 1) * pageSize
	if start >= len(source.Rows) {
		source.Rows = nil
		return source, &document.QueryPage{Number: page, Size: pageSize}
	}
	end := min(start+pageSize, len(source.Rows))
	hasNext := end < len(source.Rows)
	source.Rows = source.Rows[start:end]
	return source, &document.QueryPage{Number: page, Size: pageSize, HasNext: hasNext}
}

func panelError(code document.QueryErrorCode, message string) document.PanelBatchResult {
	return document.PanelBatchResult{Error: &document.QueryErrorResponse{Error: code, Message: message}}
}

// Drawer resolves a compact metric key against frozen, scope-checked snapshot
// state. The host mints any short-lived authorization token only at open time.
func (h *Handlers) Drawer(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, document.QueryErrorBadRequest, "method must be POST")
		return
	}
	if h.drawerResolver == nil {
		writeError(w, http.StatusNotFound, document.QueryErrorBadRequest, "drawer resolver is unavailable")
		return
	}
	var req document.DrawerResolveRequest
	if err := decodeJSON(w, r, &req); err != nil {
		writeError(w, http.StatusBadRequest, document.QueryErrorBadRequest, err.Error())
		return
	}
	req.SnapshotID = strings.TrimSpace(req.SnapshotID)
	req.MetricKey = strings.TrimSpace(req.MetricKey)
	if req.SnapshotID == "" || req.MetricKey == "" || len(req.MetricKey) > 120 {
		writeError(w, http.StatusBadRequest, document.QueryErrorBadRequest, "bounded snapshotId and metricKey are required")
		return
	}
	if len(req.Params) > 32 || !validDrawerParams(req.Params) {
		writeError(w, http.StatusBadRequest, document.QueryErrorBadRequest, "drawer params exceed the bounded scalar contract")
		return
	}
	encoded, err := json.Marshal(req.Params)
	if err != nil || len(encoded) > 8*1024 {
		writeError(w, http.StatusBadRequest, document.QueryErrorBadRequest, "drawer params cannot exceed 8192 bytes")
		return
	}
	snapshot, err := h.snapshots.Get(r.Context(), req.SnapshotID)
	if err != nil {
		h.writeSnapshotError(r.Context(), w, err)
		return
	}
	current := h.runtimeRequest(r)
	if !sameSnapshotScope(current, snapshot.Params) {
		h.writeSnapshotError(r.Context(), w, document.ErrSnapshotGone)
		return
	}
	target, err := h.drawerResolver(r, req, thawRuntimeRequest(current, snapshot.Params))
	if err != nil {
		h.writeExecutionError(r.Context(), w, err)
		return
	}
	parsed, err := url.Parse(strings.TrimSpace(target))
	if err != nil || !strings.HasPrefix(target, "/") || strings.HasPrefix(target, "//") || strings.Contains(target, `\`) || parsed.IsAbs() || parsed.Host != "" || parsed.User != nil {
		h.writeInternalError(r.Context(), w, "lens/serve.Drawer", "drawer resolution failed", fmt.Errorf("resolver returned a non-relative URL"))
		return
	}
	query := parsed.Query()
	query.Set("_lens_snapshot", snapshot.ID)
	parsed.RawQuery = query.Encode()
	writeJSON(w, http.StatusOK, document.DrawerResolveResponse{URL: parsed.String()})
}

func validDrawerParams(params map[string]any) bool {
	for key, value := range params {
		if key = strings.TrimSpace(key); key == "" || len(key) > 64 {
			return false
		}
		switch typed := value.(type) {
		case nil, string, float64, bool:
			if text, ok := typed.(string); ok && len(text) > 512 {
				return false
			}
		case []any:
			if len(typed) > 20 {
				return false
			}
			for _, item := range typed {
				text, ok := item.(string)
				if !ok || len(text) > 512 {
					return false
				}
			}
		default:
			return false
		}
	}
	return true
}

// Query returns a cached aggregate level or executes one level using frozen
// snapshot parameters. Evidence levels are always executed live.
func (h *Handlers) Query(w http.ResponseWriter, r *http.Request) {
	started := time.Now()
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, document.QueryErrorBadRequest, "method must be POST")
		return
	}
	var req QueryRequest
	if err := decodeJSON(w, r, &req); err != nil {
		writeError(w, http.StatusBadRequest, document.QueryErrorBadRequest, err.Error())
		return
	}
	req.SnapshotID = strings.TrimSpace(req.SnapshotID)
	if req.SnapshotID == "" {
		writeError(w, http.StatusBadRequest, document.QueryErrorBadRequest, "snapshotId is required")
		return
	}
	if len(req.Path) == 0 {
		writeError(w, http.StatusBadRequest, document.QueryErrorBadRequest, "path is required")
		return
	}
	if len(req.Path) > maxQueryPathDepth {
		writeError(w, http.StatusBadRequest, document.QueryErrorBadRequest, "path cannot exceed 64 entries")
		return
	}
	for _, entry := range req.Path {
		if len([]rune(entry)) > maxQueryPathEntry {
			writeError(w, http.StatusBadRequest, document.QueryErrorBadRequest, "path entries cannot exceed 256 characters")
			return
		}
	}
	if req.Page < 0 {
		writeError(w, http.StatusBadRequest, document.QueryErrorBadRequest, "page cannot be negative")
		return
	}
	if req.Page > lensruntime.MaxTablePage {
		writeError(w, http.StatusBadRequest, document.QueryErrorBadRequest, "page exceeds the maximum")
		return
	}
	if req.IdlePrefetch && !req.Prefetch {
		writeError(w, http.StatusBadRequest, document.QueryErrorBadRequest, "idlePrefetch requires prefetch")
		return
	}
	snapshot, err := h.snapshots.Get(r.Context(), req.SnapshotID)
	if err != nil {
		h.writeSnapshotError(r.Context(), w, err)
		return
	}
	current := h.runtimeRequest(r)
	if !sameSnapshotScope(current, snapshot.Params) {
		h.writeSnapshotError(r.Context(), w, document.ErrSnapshotGone)
		return
	}
	target, err := resolveTarget(h.spec, req.Path, req.Perspective)
	if err != nil {
		writeError(w, http.StatusBadRequest, document.QueryErrorBadRequest, err.Error())
		return
	}
	if err := validateTableSort(target.panel, req.Sort); err != nil {
		writeError(w, http.StatusBadRequest, document.QueryErrorBadRequest, err.Error())
		return
	}
	if !target.evidence {
		key := "level:" + snapshot.ID + ":" + string(target.cacheRef())
		session := h.session(snapshot.ID)
		session.advanceRevision(req.Revision)
		// The cache key carries the path's point selections: a level entered
		// through year 2024 must not be served the frame cached for 2025.
		if cached, ok := snapshot.Frames[target.cacheRef()]; ok {
			cached, _ = tableFrameView(target.panel, cached, "", req.Sort)
			writeJSON(w, http.StatusOK, QueryResponse{Frames: map[document.FrameRef]document.Frame{target.ref: cached}})
			if !req.Prefetch {
				h.observeMetric(r.Context(), Metric{Name: MetricPrefetchHit, Value: boolMetric(session.wasPrefetched(key))})
				h.observeMetric(r.Context(), Metric{Name: MetricTimeToFirstChild, Value: float64(time.Since(started).Microseconds()) / 1000})
			}
			return
		}
		h.queryAggregate(w, r, req, snapshot, target, started)
		return
	}
	page := req.Page
	if page == 0 {
		page = lensruntime.DefaultTablePage
	}
	base := thawRuntimeRequest(current, snapshot.Params)
	applySortRequest(&base, req.Sort)
	panelResult, err := h.executeLevel(r.Context(), base, snapshot.Params, target, page)
	if err != nil {
		h.writeExecutionError(r.Context(), w, err)
		return
	}
	wire, err := wireFrame(target.ref, target.panel, target.dynamicChildren, panelResult)
	if err != nil {
		h.writeInternalError(r.Context(), w, "lens/serve.Query", "level result conversion failed", err)
		return
	}
	wire, _ = tableFrameView(target.panel, wire, "", req.Sort)
	if err := document.ResolveDynamicChildren(&wire, snapshot.Levels[target.levelKey]); err != nil {
		h.writeInternalError(r.Context(), w, "lens/serve.Query", "dynamic children resolution failed", err)
		return
	}
	if err := document.ValidateResolvedChildren(snapshot.Levels[target.levelKey], wire, snapshot.Levels); err != nil {
		h.writeInternalError(r.Context(), w, "lens/serve.Query", "dynamic children validation failed", err)
		return
	}
	hasNext, err := h.evidenceHasNext(panelResult, &wire)
	if err != nil {
		h.writeExecutionError(r.Context(), w, err)
		return
	}
	writeJSON(w, http.StatusOK, QueryResponse{
		Frames: map[document.FrameRef]document.Frame{target.ref: wire},
		Page:   &Page{Number: page, Size: h.pageSize, HasNext: hasNext},
	})
}

func (h *Handlers) evidenceHasNext(
	result *lensruntime.PanelResult,
	wire *document.Frame,
) (bool, error) {
	if len(wire.Rows) > h.pageSize {
		wire.Rows = wire.Rows[:h.pageSize]
		return true, nil
	}
	if result.TablePagination != nil {
		return result.TablePagination.HasMore, nil
	}
	if len(wire.Rows) < h.pageSize {
		return false, nil
	}
	return false, fmt.Errorf("full evidence page requires authoritative table pagination metadata")
}

func (h *Handlers) queryAggregate(w http.ResponseWriter, r *http.Request, req QueryRequest, snapshot *document.Snapshot, target levelTarget, started time.Time) {
	ctx := r.Context()
	base := h.runtimeRequest(r)
	key := "level:" + snapshot.ID + ":" + string(target.cacheRef())
	session := h.session(snapshot.ID)
	session.advanceRevision(req.Revision)
	priority := priorityInteractive
	if req.Prefetch {
		priority = priorityIntent
		if req.IdlePrefetch {
			priority = priorityIdlePrefetch
		}
	}
	result := session.submit(ctx, key, priority, 0, func(workCtx context.Context) (any, error) {
		return h.materializeAggregate(workCtx, snapshot.ID, base, target)
	}, req.Revision)
	defer result.Cancel()
	select {
	case <-ctx.Done():
		return
	case loaded := <-result.result:
		if loaded.err != nil {
			if errors.Is(loaded.err, document.ErrSnapshotGone) {
				h.writeSnapshotError(ctx, w, loaded.err)
				return
			}
			h.writeExecutionError(ctx, w, loaded.err)
			return
		}
		frame, ok := loaded.value.(document.Frame)
		if !ok {
			h.writeInternalError(ctx, w, "lens/serve.Query", "level execution failed", fmt.Errorf("level execution returned %T", loaded.value))
			return
		}
		frame, _ = tableFrameView(target.panel, frame, "", req.Sort)
		if req.Prefetch {
			session.markPrefetched(key)
		}
		writeJSON(w, http.StatusOK, QueryResponse{Frames: map[document.FrameRef]document.Frame{target.ref: frame}})
		if !req.Prefetch {
			h.observeMetric(ctx, Metric{Name: MetricPrefetchHit, Value: boolMetric(session.wasPrefetched(key))})
			h.observeMetric(ctx, Metric{Name: MetricTimeToFirstChild, Value: float64(time.Since(started).Microseconds()) / 1000})
		}
	}
}

func boolMetric(value bool) float64 {
	if value {
		return 1
	}
	return 0
}

func (h *Handlers) materializeAggregate(
	ctx context.Context,
	snapshotID string,
	current lensruntime.Request,
	target levelTarget,
) (document.Frame, error) {
	latest, err := h.snapshots.Get(ctx, snapshotID)
	if err != nil {
		return document.Frame{}, err
	}
	if !sameSnapshotScope(current, latest.Params) {
		return document.Frame{}, document.ErrSnapshotGone
	}
	if cached, ok := latest.Frames[target.cacheRef()]; ok {
		return cached, nil
	}
	panelResult, err := h.executeLevel(ctx, thawRuntimeRequest(current, latest.Params), latest.Params, target, 0)
	if err != nil {
		return document.Frame{}, err
	}
	wire, err := wireFrame(target.ref, target.panel, target.dynamicChildren, panelResult)
	if err != nil {
		return document.Frame{}, err
	}
	level := latest.Levels[target.levelKey]
	if err := document.ResolveDynamicChildren(&wire, level); err != nil {
		return document.Frame{}, err
	}
	if err := document.ValidateResolvedChildren(level, wire, latest.Levels); err != nil {
		return document.Frame{}, err
	}
	if err := h.snapshots.Append(ctx, snapshotID, map[document.FrameRef]document.Frame{target.cacheRef(): wire}); err != nil {
		return document.Frame{}, err
	}
	return wire, nil
}

func (h *Handlers) startBackgroundPrefetch(
	base context.Context,
	session *executionSession,
	snapshot *document.Snapshot,
	current lensruntime.Request,
) {
	// Materialize a child only after its parent root completed successfully.
	// Besides enforcing the graph activation gate, the sequential producer keeps
	// speculative fan-out bounded even when the session has spare foreground
	// capacity; the scheduler still owns the single background slot and can
	// promote a queued target when an interactive request joins it.
	go func() {
		parentReady := false
		for order, target := range backgroundPrefetchTargets(h.spec) {
			isRoot := len(target.points) == 0
			if isRoot {
				parentReady = false
			} else if !parentReady {
				continue
			}
			key := "level:" + snapshot.ID + ":" + string(target.cacheRef())
			result := session.submit(context.WithoutCancel(base), key, priorityIdlePrefetch, order, func(ctx context.Context) (any, error) {
				return h.materializeAggregate(ctx, snapshot.ID, current, target)
			})
			loaded := <-result.result
			if loaded.err == nil {
				session.markPrefetched(key)
				if isRoot {
					parentReady = true
				}
				continue
			}
			if isRoot {
				parentReady = false
			}
			if !errors.Is(loaded.err, document.ErrSnapshotGone) && !errors.Is(loaded.err, context.Canceled) {
				h.observer.OnError(context.Background(), "lens/serve.Prefetch", loaded.err)
			}
		}
	}()
}

func applySortRequest(request *lensruntime.Request, ordering *document.TableSort) {
	if ordering == nil {
		return
	}
	request.Request = cloneValues(request.Request)
	request.Request.Set(lensruntime.TableSortFieldQuery, ordering.Field)
	request.Request.Set(lensruntime.TableSortDirectionQuery, string(ordering.Direction))
}

// Export writes a snapshot-keyed workbook for one panel or the full document.
func (h *Handlers) Export(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, document.QueryErrorBadRequest, "method must be GET")
		return
	}
	snapshotID := strings.TrimSpace(r.URL.Query().Get("snapshot"))
	if snapshotID == "" {
		writeError(w, http.StatusBadRequest, document.QueryErrorBadRequest, "snapshot is required")
		return
	}
	snapshot, err := h.snapshots.Get(r.Context(), snapshotID)
	if err != nil {
		h.writeSnapshotError(r.Context(), w, err)
		return
	}
	panelID := strings.TrimSpace(r.URL.Query().Get("panel"))
	current := h.runtimeRequest(r)
	if !sameSnapshotScope(current, snapshot.Params) {
		h.writeSnapshotError(r.Context(), w, document.ErrSnapshotGone)
		return
	}
	request := thawRuntimeRequest(current, snapshot.Params)
	var result *lensruntime.DashboardResult
	if h.progressive {
		scope := lensruntime.DashboardExportScope()
		exportKey := "dashboard"
		if panelID != "" {
			if spec, ok := lens.FindPanel(h.spec, panelID); !ok || spec.Kind.IsContainer() {
				writeError(w, http.StatusBadRequest, document.QueryErrorBadRequest, "panel is not available in the snapshot")
				return
			}
			scope = lensruntime.PanelExportScope(panelID)
			exportKey = "panel:" + panelID
		}
		loaded, executeErr := h.ExecuteExportSurface(r.Context(), snapshot.ID, exportKey, func(workCtx context.Context) (any, error) {
			return h.engine.Execute(workCtx, h.spec, request, scope)
		})
		err = executeErr
		if err != nil {
			h.writeExecutionError(r.Context(), w, err)
			return
		}
		result, _ = loaded.(*lensruntime.DashboardResult)
		if result == nil {
			h.writeInternalError(r.Context(), w, "lens/serve.Export", "export execution failed", fmt.Errorf("executor returned a nil dashboard result"))
			return
		}
		// Workbook identity belongs to the immutable document snapshot rather
		// than the runtime's internal dataset-cache key.
		result.SnapshotID = snapshot.ID
		result.StartedAt = snapshot.CreatedAt
	} else {
		result, err = runtimeResultFromSnapshot(h.spec, snapshot, request)
		if err != nil {
			h.writeInternalError(r.Context(), w, "lens/serve.Export", "snapshot conversion failed", err)
			return
		}
		if panelID != "" && result.Panel(panelID) == nil {
			writeError(w, http.StatusBadRequest, document.QueryErrorBadRequest, "panel is not available in the snapshot")
			return
		}
	}
	var workbook bytes.Buffer
	if err := lensexport.New().Write(r.Context(), &workbook, lensexport.Request{Result: result, PanelID: panelID}); err != nil {
		if r.Context().Err() != nil {
			return
		}
		h.writeInternalError(r.Context(), w, "lens/serve.Export", "export failed", err)
		return
	}
	filename := lensexport.WorkbookFilename(result, panelID, snapshot.CreatedAt)
	w.Header().Set("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	w.Header().Set("Content-Disposition", lensexport.ContentDisposition(filename))
	w.Header().Set("Content-Length", strconv.Itoa(workbook.Len()))
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(workbook.Bytes())
}

func (h *Handlers) writeSnapshotError(ctx context.Context, w http.ResponseWriter, err error) {
	if ctx.Err() != nil {
		return
	}
	if errors.Is(err, document.ErrSnapshotGone) {
		writeError(w, http.StatusGone, document.QueryErrorSnapshotGone, "snapshot is unknown or expired")
		return
	}
	h.writeInternalError(ctx, w, "lens/serve.writeSnapshotError", "snapshot lookup failed", err)
}

func (h *Handlers) writeExecutionError(ctx context.Context, w http.ResponseWriter, err error) {
	if ctx.Err() != nil {
		return
	}
	h.writeInternalError(ctx, w, "lens/serve.writeExecutionError", "lens execution failed", err)
}

func (h *Handlers) writeInternalError(ctx context.Context, w http.ResponseWriter, op, message string, err error) {
	if ctx.Err() != nil {
		return
	}
	wrapped := serrors.E(serrors.Op(op), err)
	h.observer.OnError(ctx, op, wrapped)
	writeError(w, http.StatusInternalServerError, document.QueryErrorInternal, message)
}

func decodeJSON(w http.ResponseWriter, r *http.Request, target any) error {
	r.Body = http.MaxBytesReader(w, r.Body, maxQueryBodyBytes)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return fmt.Errorf("invalid JSON body: %w", err)
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		if err == nil {
			return fmt.Errorf("request body must contain one JSON object")
		}
		return fmt.Errorf("invalid JSON body: %w", err)
	}
	return nil
}

func writeError(w http.ResponseWriter, status int, code document.QueryErrorCode, message string) {
	writeJSON(w, status, errorResponse{Error: code, Message: message})
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	payload, err := json.Marshal(value)
	if err != nil {
		payload = []byte(`{"error":"internal","message":"JSON encoding failed"}`)
		status = http.StatusInternalServerError
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = w.Write(append(payload, '\n'))
}

func cloneParams(values map[string]any) map[string]any {
	result := make(map[string]any, len(values))
	for key, value := range values {
		result[key] = value
	}
	return result
}
