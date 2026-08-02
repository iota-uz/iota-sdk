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
		seen[req.Panels[index].PanelID] = struct{}{}
	}
	flusher, ok := w.(http.Flusher)
	if !ok {
		h.writeInternalError(r.Context(), w, "lens/serve.Panel", "streaming is unavailable", fmt.Errorf("response writer does not support flushing"))
		return
	}
	type completedPanel struct {
		panelID string
		result  document.PanelBatchResult
	}
	completed := make(chan completedPanel, len(req.Panels))
	for _, panelReq := range req.Panels {
		panelReq := panelReq
		go func() {
			result := h.panelResult(r.Context(), panelReq, snapshot, current)
			select {
			case completed <- completedPanel{panelID: panelReq.PanelID, result: result}:
			case <-r.Context().Done():
			}
		}()
	}
	w.Header().Set("Content-Type", "application/x-ndjson")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)
	encoder := json.NewEncoder(w)
	for range req.Panels {
		select {
		case item := <-completed:
			event := PanelBatchStreamEvent{PanelID: item.panelID, Result: &item.result}
			if err := encoder.Encode(event); err != nil {
				return
			}
			flusher.Flush()
		case <-r.Context().Done():
			return
		}
	}
	if err := encoder.Encode(PanelBatchStreamEvent{Complete: true}); err != nil {
		return
	}
	flusher.Flush()
}

func (h *Handlers) panelResult(
	ctx context.Context,
	req PanelRequest,
	snapshot *document.Snapshot,
	current lensruntime.Request,
) document.PanelBatchResult {
	panelSpec, validationErr := h.validatePanelRequest(req)
	if validationErr != nil {
		return panelError(document.QueryErrorBadRequest, validationErr.Error())
	}
	if req.Recompute {
		if calculation, ok := snapshot.Panels[panelSpec.ID]; ok && time.Since(calculation.CalculatedAt) < recomputeCooldown {
			return panelError(document.QueryErrorBadRequest, "panel was recomputed recently")
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
			return successfulPanel(ref, view, calculation, summary, page)
		}
	}
	loaded, err := h.loadPanelValue(ctx, req, snapshot, panelSpec, ref, current)
	if err != nil {
		if errors.Is(err, document.ErrSnapshotGone) {
			return panelError(document.QueryErrorSnapshotGone, "snapshot expired or was not found")
		}
		h.observer.OnError(ctx, "lens/serve.Panel", err)
		return panelError(document.QueryErrorInternal, "panel execution failed")
	}
	view, summary := tableFrameView(panelSpec, loaded.frame, req.Search, req.Sort)
	view, page := paginatePanelFrame(panelSpec, view, req.Page, h.pageSize)
	return successfulPanel(ref, view, loaded.calculation, summary, page)
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

func (h *Handlers) loadPanelValue(
	ctx context.Context,
	req PanelRequest,
	snapshot *document.Snapshot,
	panelSpec panel.Spec,
	ref document.FrameRef,
	current lensruntime.Request,
) (loadedPanel, error) {
	key := "panel:" + snapshot.ID + ":" + panelSpec.ID
	if req.Recompute {
		key += ":recompute"
	}
	result := h.loads.DoChan(key, func() (any, error) {
		workCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), h.workTimeout)
		defer cancel()
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
	select {
	case <-ctx.Done():
		return loadedPanel{}, ctx.Err()
	case loaded := <-result:
		if loaded.Err != nil {
			return loadedPanel{}, loaded.Err
		}
		panel, ok := loaded.Val.(loadedPanel)
		if !ok {
			return loadedPanel{}, fmt.Errorf("panel execution returned %T", loaded.Val)
		}
		return panel, nil
	}
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
	writeJSON(w, http.StatusOK, document.DrawerResolveResponse{URL: target})
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
		// The cache key carries the path's point selections: a level entered
		// through year 2024 must not be served the frame cached for 2025.
		if cached, ok := snapshot.Frames[target.cacheRef()]; ok {
			cached, _ = tableFrameView(target.panel, cached, "", req.Sort)
			writeJSON(w, http.StatusOK, QueryResponse{Frames: map[document.FrameRef]document.Frame{target.ref: cached}})
			return
		}
		h.queryAggregate(w, r, req, snapshot, target)
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

func (h *Handlers) queryAggregate(w http.ResponseWriter, r *http.Request, req QueryRequest, snapshot *document.Snapshot, target levelTarget) {
	ctx := r.Context()
	base := h.runtimeRequest(r)
	key := snapshot.ID + ":" + string(target.cacheRef())
	result := h.loads.DoChan(key, func() (any, error) {
		workCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), h.workTimeout)
		defer cancel()
		latest, err := h.snapshots.Get(workCtx, snapshot.ID)
		if err != nil {
			return nil, err
		}
		if cached, ok := latest.Frames[target.cacheRef()]; ok {
			return cached, nil
		}
		panelResult, err := h.executeLevel(workCtx, thawRuntimeRequest(base, latest.Params), latest.Params, target, 0)
		if err != nil {
			return nil, err
		}
		wire, err := wireFrame(target.ref, target.panel, target.dynamicChildren, panelResult)
		if err != nil {
			return nil, err
		}
		if err := document.ResolveDynamicChildren(&wire, latest.Levels[target.levelKey]); err != nil {
			return nil, err
		}
		if err := document.ValidateResolvedChildren(latest.Levels[target.levelKey], wire, latest.Levels); err != nil {
			return nil, err
		}
		if err := h.snapshots.Append(workCtx, snapshot.ID, map[document.FrameRef]document.Frame{target.cacheRef(): wire}); err != nil {
			return nil, err
		}
		return wire, nil
	})
	select {
	case <-ctx.Done():
		return
	case loaded := <-result:
		if loaded.Err != nil {
			if errors.Is(loaded.Err, document.ErrSnapshotGone) {
				h.writeSnapshotError(ctx, w, loaded.Err)
				return
			}
			h.writeExecutionError(ctx, w, loaded.Err)
			return
		}
		frame, ok := loaded.Val.(document.Frame)
		if !ok {
			h.writeInternalError(ctx, w, "lens/serve.Query", "level execution failed", fmt.Errorf("level execution returned %T", loaded.Val))
			return
		}
		frame, _ = tableFrameView(target.panel, frame, "", req.Sort)
		writeJSON(w, http.StatusOK, QueryResponse{Frames: map[document.FrameRef]document.Frame{target.ref: frame}})
	}
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
		if panelID != "" {
			if spec, ok := lens.FindPanel(h.spec, panelID); !ok || spec.Kind.IsContainer() {
				writeError(w, http.StatusBadRequest, document.QueryErrorBadRequest, "panel is not available in the snapshot")
				return
			}
			scope = lensruntime.PanelExportScope(panelID)
		}
		result, err = h.engine.Execute(r.Context(), h.spec, request, scope)
		if err != nil {
			h.writeExecutionError(r.Context(), w, err)
			return
		}
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
