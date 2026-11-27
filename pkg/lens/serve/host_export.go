package serve

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"time"

	"golang.org/x/sync/errgroup"

	"github.com/iota-uz/iota-sdk/pkg/lens"
	"github.com/iota-uz/iota-sdk/pkg/lens/document"
	lensexport "github.com/iota-uz/iota-sdk/pkg/lens/export"
	"github.com/iota-uz/iota-sdk/pkg/lens/frame"
	"github.com/iota-uz/iota-sdk/pkg/lens/panel"
	"github.com/iota-uz/iota-sdk/pkg/lens/resource"
	lensruntime "github.com/iota-uz/iota-sdk/pkg/lens/runtime"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
)

const maxConcurrentHostExportPanels = 8

// ExportOptions configures workbook projection, lineage omission, authorization,
// and resource/time bounds for Host.Export.
type ExportOptions struct {
	// VisibleTableColumnsOnly narrows every exported table panel to its declared
	// visible columns in declaration order, headed by each column's Label.
	VisibleTableColumnsOnly bool
	// PanelExportFields, when non-nil, narrows non-table panels to the fields
	// they display and heads exported columns using the returned field->label
	// map (falling back to the panel title).
	PanelExportFields func(context.Context) map[string]string
	// OmitLineage drops the technical Sources lineage sheet and lineage datasets
	// from the exported Excel workbook.
	OmitLineage bool
	// Timeout bounds workbook generation; defaults to HostConfig.ExecutionTimeout.
	Timeout time.Duration
	// Spool optionally overrides export spooling limits. When nil, the host's
	// shared resource Budget is wired automatically.
	Spool *lensexport.SpoolOptions
	// Authorize optionally authorizes an export request for (r, snapshotID, panelID).
	// Returning a non-nil error responds with 403 Forbidden.
	Authorize func(r *http.Request, snapshotID, panelID string) error
}

// Export writes a snapshot-keyed Excel workbook for one panel or the full
// dashboard, applying ExportOptions and bounded admission via BuildLimiter.
func (h *Host) Export(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		lensexport.SetDownloadSignal(w, r, lensexport.DownloadSignalError)
		writeError(w, http.StatusMethodNotAllowed, document.QueryErrorBadRequest, "method must be GET")
		return
	}
	snapshotID := strings.TrimSpace(r.URL.Query().Get("snapshot"))
	if snapshotID == "" {
		lensexport.SetDownloadSignal(w, r, lensexport.DownloadSignalError)
		writeError(w, http.StatusBadRequest, document.QueryErrorBadRequest, "snapshot is required")
		return
	}
	panelID := strings.TrimSpace(r.URL.Query().Get("panel"))
	if h.export.Authorize != nil {
		if err := h.export.Authorize(r, snapshotID, panelID); err != nil {
			lensexport.SetDownloadSignal(w, r, lensexport.DownloadSignalError)
			writeError(w, http.StatusForbidden, document.QueryErrorBadRequest, "export is forbidden")
			return
		}
	}
	entry, err := h.cache.LookupSnapshotRegistration(r.Context(), snapshotID)
	if err != nil {
		lensexport.SetDownloadSignal(w, r, lensexport.DownloadSignalError)
		h.writeSnapshotError(r.Context(), w, err)
		return
	}
	defer entry.Release()

	snapshot, err := h.snapshots.Get(r.Context(), snapshotID)
	if err != nil {
		lensexport.SetDownloadSignal(w, r, lensexport.DownloadSignalError)
		h.writeSnapshotError(r.Context(), w, err)
		return
	}
	current, err := h.resolveRuntimeRequest(r)
	if err != nil {
		lensexport.SetDownloadSignal(w, r, lensexport.DownloadSignalError)
		h.writeInternalError(r.Context(), w, "lens/serve.Host.Export", "request scope resolution failed", err)
		return
	}
	if !sameSnapshotScope(current, snapshot.Params) {
		lensexport.SetDownloadSignal(w, r, lensexport.DownloadSignalError)
		h.writeSnapshotError(r.Context(), w, document.ErrSnapshotGone)
		return
	}
	spec, err := entry.Spec()
	if err != nil {
		lensexport.SetDownloadSignal(w, r, lensexport.DownloadSignalError)
		h.writeInternalError(r.Context(), w, "lens/serve.Host.Export", "snapshot conversion failed", err)
		return
	}
	if entry.State.progressive {
		if err := h.materializeExportPanels(r.Context(), entry, spec, snapshotID, panelID, current); err != nil {
			lensexport.SetDownloadSignal(w, r, lensexport.DownloadSignalError)
			h.writeInternalError(r.Context(), w, "lens/serve.Host.Export", "panel materialization failed", err)
			return
		}
		snapshot, err = h.snapshots.Get(r.Context(), snapshotID)
		if err != nil {
			lensexport.SetDownloadSignal(w, r, lensexport.DownloadSignalError)
			h.writeSnapshotError(r.Context(), w, err)
			return
		}
	}
	thawed := thawRuntimeRequest(current, snapshot.Params)
	result, err := runtimeResultFromSnapshot(spec, snapshot, thawed)
	if err != nil {
		lensexport.SetDownloadSignal(w, r, lensexport.DownloadSignalError)
		h.writeInternalError(r.Context(), w, "lens/serve.Host.Export", "snapshot conversion failed", err)
		return
	}
	if panelID != "" && result.Panel(panelID) == nil {
		lensexport.SetDownloadSignal(w, r, lensexport.DownloadSignalError)
		writeError(w, http.StatusBadRequest, document.QueryErrorBadRequest, "panel is not available in the snapshot")
		return
	}

	timeout := h.export.Timeout
	if timeout <= 0 {
		timeout = h.executionTimeout
	}
	requestCtx := r.Context()
	exportCtx, cancel := context.WithTimeout(requestCtx, timeout)
	defer cancel()

	release, err := h.buildLimiter.Acquire(exportCtx)
	if err != nil {
		lensexport.SetDownloadSignal(w, r, lensexport.DownloadSignalError)
		if errors.Is(requestCtx.Err(), context.Canceled) {
			return
		}
		if errors.Is(exportCtx.Err(), context.DeadlineExceeded) || errors.Is(err, context.DeadlineExceeded) {
			writeError(w, http.StatusGatewayTimeout, document.QueryErrorInternal, "export deadline exceeded")
			return
		}
		h.writeInternalError(requestCtx, w, "lens/serve.Host.Export", "export capacity is unavailable", err)
		return
	}
	defer release()

	if h.export.VisibleTableColumnsOnly {
		if err := ProjectVisibleTableExportColumns(result); err != nil {
			lensexport.SetDownloadSignal(w, r, lensexport.DownloadSignalError)
			h.writeInternalError(requestCtx, w, "lens/serve.Host.Export", "export projection failed", err)
			return
		}
	}
	if h.export.PanelExportFields != nil {
		if err := ProjectPanelExportFields(result, h.export.PanelExportFields(exportCtx)); err != nil {
			lensexport.SetDownloadSignal(w, r, lensexport.DownloadSignalError)
			h.writeInternalError(requestCtx, w, "lens/serve.Host.Export", "export projection failed", err)
			return
		}
	}
	if h.export.OmitLineage {
		result.Spec.Export.OmitLineage = true
	}

	spool := &lensexport.SpoolOptions{Budget: h.budget}
	if h.export.Spool != nil {
		copied := *h.export.Spool
		if copied.Budget == nil {
			copied.Budget = h.budget
		}
		spool = &copied
	}
	prepared, err := lensexport.New().Prepare(exportCtx, lensexport.Request{
		Result:  result,
		PanelID: panelID,
		Spool:   spool,
	})
	if err != nil {
		lensexport.SetDownloadSignal(w, r, lensexport.DownloadSignalError)
		if errors.Is(requestCtx.Err(), context.Canceled) {
			return
		}
		if errors.Is(exportCtx.Err(), context.DeadlineExceeded) || errors.Is(err, context.DeadlineExceeded) {
			h.observer.OnError(requestCtx, "lens/serve.Host.Export", serrors.Wrap("lens/serve.Host.Export", err))
			writeError(w, http.StatusGatewayTimeout, document.QueryErrorInternal, "export deadline exceeded")
			return
		}
		h.writeInternalError(requestCtx, w, "lens/serve.Host.Export", "export failed", err)
		return
	}
	defer func() { _ = prepared.Close() }()

	filename := lensexport.WorkbookFilename(result, panelID, snapshot.CreatedAt)
	lensexport.SetDownloadSignal(w, r, lensexport.DownloadSignalStarted)
	if err := prepared.WriteHTTP(w, filename); err != nil {
		h.observer.OnError(requestCtx, "lens/serve.Host.Export", serrors.Wrap("lens/serve.Host.Export", err))
	}
}

func (h *Host) materializeExportPanels(
	ctx context.Context,
	entry *ReportRegistration[*hostEntry],
	spec lens.DashboardSpec,
	snapshotID string,
	panelID string,
	current lensruntime.Request,
) error {
	panels := make([]panel.Spec, 0)
	if panelID != "" {
		panelSpec, ok := lens.FindPanel(spec, panelID)
		if !ok || panelSpec.Kind.IsContainer() {
			return nil
		}
		panels = append(panels, panelSpec)
	} else {
		for _, panelSpec := range lens.FlattenPanels(spec) {
			if !panelSpec.Kind.IsContainer() {
				panels = append(panels, panelSpec)
			}
		}
	}
	group, groupCtx := errgroup.WithContext(ctx)
	group.SetLimit(maxConcurrentHostExportPanels)
	for _, candidate := range panels {
		panelSpec := candidate
		group.Go(func() error {
			payload, err := json.Marshal(document.PanelBatchRequest{
				SnapshotID: snapshotID,
				Panels:     []document.PanelRequest{{PanelID: panelSpec.ID}},
			})
			if err != nil {
				return err
			}
			reqCtx := context.WithValue(groupCtx, hostRuntimeRequestContextKey{}, current)
			req := httptest.NewRequest(http.MethodPost, h.endpoint("/panel"), bytes.NewReader(payload)).WithContext(reqCtx)
			recorder := httptest.NewRecorder()
			entry.Handlers.Panel(recorder, req)
			if recorder.Code != http.StatusOK {
				return fmt.Errorf("panel %q returned %d: %s", panelSpec.ID, recorder.Code, strings.TrimSpace(recorder.Body.String()))
			}
			// Check streamed NDJSON result for panel-level errors.
			decoder := json.NewDecoder(recorder.Body)
			for decoder.More() {
				var evt document.PanelBatchStreamEvent
				if err := decoder.Decode(&evt); err != nil {
					break
				}
				if evt.Result != nil && evt.Result.Error != nil {
					if evt.Result.Error.Error == document.QueryErrorResourceExhausted {
						return resource.NewLimitError("export panel", 1, 0)
					}
					return fmt.Errorf("panel %q failed: %s", panelSpec.ID, evt.Result.Error.Message)
				}
			}
			return nil
		})
	}
	return group.Wait()
}

// ProjectVisibleTableExportColumns narrows every table panel in result to its
// declared visible columns in declaration order, headed by each column's Label.
// Action-only columns (empty Field) and hidden fields (such as row IDs or
// action URLs) are excluded.
func ProjectVisibleTableExportColumns(result *lensruntime.DashboardResult) error {
	if result == nil {
		return nil
	}
	for _, rendered := range result.Panels {
		if rendered == nil || rendered.Panel.Kind != panel.KindTable || len(rendered.Panel.Columns) == 0 || rendered.Frames == nil {
			continue
		}
		projected := rendered.Frames.Clone()
		for _, source := range projected.Frames {
			fields := make([]frame.Field, 0, len(rendered.Panel.Columns))
			for _, column := range rendered.Panel.Columns {
				if column.Field == "" {
					continue
				}
				field, ok := source.Field(column.Field.Name())
				if !ok {
					return fmt.Errorf("table %s export column %s is missing", rendered.Panel.ID, column.Field)
				}
				exportField := field.Clone()
				if exportField.Labels == nil {
					exportField.Labels = map[string]string{}
				}
				exportField.Labels["default"] = column.Label
				fields = append(fields, exportField)
			}
			source.Fields = fields
			if err := source.Normalize(); err != nil {
				return fmt.Errorf("table %s export columns: %w", rendered.Panel.ID, err)
			}
		}
		rendered.Frames = projected
	}
	return nil
}

// ProjectPanelExportFields narrows every non-table panel in result to only the
// fields the panel displays (or fields explicitly named in labels) and heads
// exported columns with labels[field] (or the panel title).
func ProjectPanelExportFields(result *lensruntime.DashboardResult, labels map[string]string) error {
	if result == nil {
		return nil
	}
	for _, rendered := range result.Panels {
		if rendered == nil || rendered.Panel.Kind == panel.KindTable || rendered.Frames == nil {
			continue
		}
		mapping := rendered.Panel.Fields
		stat := rendered.Panel.Kind == panel.KindStat
		shown := map[string]bool{mapping.Value.Name(): true}
		if !stat {
			shown[mapping.Label.Name()] = true
			shown[mapping.Category.Name()] = true
			shown[mapping.Series.Name()] = true
		}
		projected := rendered.Frames.Clone()
		for _, source := range projected.Frames {
			fields := make([]frame.Field, 0, len(source.Fields))
			for _, field := range source.Fields {
				label, named := labels[field.Name]
				switch {
				case stat && shown[field.Name]:
					label = rendered.Panel.Title
				case stat, !shown[field.Name] && !named:
					continue
				case !named:
					label = rendered.Panel.Title
				}
				exportField := field.Clone()
				exportField.Labels = map[string]string{"default": label}
				fields = append(fields, exportField)
			}
			source.Fields = fields
			if err := source.Normalize(); err != nil {
				return fmt.Errorf("panel %s export fields: %w", rendered.Panel.ID, err)
			}
		}
		rendered.Frames = projected
	}
	return nil
}
