package serve

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/iota-uz/iota-sdk/pkg/lens/document"
	lensexport "github.com/iota-uz/iota-sdk/pkg/lens/export"
	lensruntime "github.com/iota-uz/iota-sdk/pkg/lens/runtime"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
)

const maxQueryBodyBytes = 1 << 20

type errorResponse struct {
	Error   string `json:"error"`
	Message string `json:"message"`
}

// QueryRequest is the snapshot-scoped level request accepted by Query.
type QueryRequest struct {
	SnapshotID  string            `json:"snapshotId"`
	Path        document.NodePath `json:"path"`
	Perspective string            `json:"perspective,omitempty"`
	Page        int               `json:"page,omitempty"`
}

// Page describes an Evidence response page.
type Page struct {
	Number int `json:"number"`
	Size   int `json:"size"`
}

// QueryResponse contains the frames materialized for one requested level.
type QueryResponse struct {
	Frames map[document.FrameRef]document.Frame `json:"frames"`
	Page   *Page                                `json:"page,omitempty"`
}

// Document executes and returns a new snapshot-backed dashboard document.
func (h *Handlers) Document(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "bad_request", "method must be GET")
		return
	}
	req := h.runtimeRequest(r)
	result, err := h.engine.Execute(r.Context(), h.spec, req, lensruntime.DashboardScope())
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
	doc, err := document.Build(h.spec, result, document.BuildOptions{
		Locale: result.Locale, InlineDepth: h.inlineDepth,
		Endpoints: document.Endpoints{Query: h.endpoint("/lens/query"), Export: h.endpoint("/export")},
	})
	if err != nil {
		h.writeInternalError(r.Context(), w, "lens/serve.Document", "document build failed", err)
		return
	}
	if err := h.snapshots.Put(r.Context(), &document.Snapshot{
		ID: doc.SnapshotID, Params: frozen, Frames: doc.Frames, CreatedAt: doc.Meta.GeneratedAt,
	}); err != nil {
		if r.Context().Err() != nil {
			return
		}
		h.writeInternalError(r.Context(), w, "lens/serve.Document", "snapshot storage failed", err)
		return
	}
	writeJSON(w, http.StatusOK, doc)
}

// Query returns a cached aggregate level or executes one level using frozen
// snapshot parameters. Evidence levels are always executed live.
func (h *Handlers) Query(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "bad_request", "method must be POST")
		return
	}
	var req QueryRequest
	if err := decodeJSON(w, r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}
	req.SnapshotID = strings.TrimSpace(req.SnapshotID)
	if req.SnapshotID == "" {
		writeError(w, http.StatusBadRequest, "bad_request", "snapshotId is required")
		return
	}
	if len(req.Path) == 0 {
		writeError(w, http.StatusBadRequest, "bad_request", "path is required")
		return
	}
	if req.Page < 0 {
		writeError(w, http.StatusBadRequest, "bad_request", "page cannot be negative")
		return
	}
	snapshot, err := h.snapshots.Get(r.Context(), req.SnapshotID)
	if err != nil {
		h.writeSnapshotError(r.Context(), w, err)
		return
	}
	target, err := resolveTarget(h.spec, req.Path, req.Perspective)
	if err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}
	if !target.evidence {
		if cached, ok := snapshot.Frames[target.ref]; ok {
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
	panelResult, err := h.executeLevel(r.Context(), thawRuntimeRequest(h.runtimeRequest(r), snapshot.Params), snapshot.Params, target, page)
	if err != nil {
		h.writeExecutionError(r.Context(), w, err)
		return
	}
	wire, err := wireFrame(target.ref, panelResult)
	if err != nil {
		h.writeInternalError(r.Context(), w, "lens/serve.Query", "level result conversion failed", err)
		return
	}
	writeJSON(w, http.StatusOK, QueryResponse{
		Frames: map[document.FrameRef]document.Frame{target.ref: wire},
		Page:   &Page{Number: page, Size: h.pageSize},
	})
}

func (h *Handlers) queryAggregate(w http.ResponseWriter, r *http.Request, req QueryRequest, snapshot *document.Snapshot, target levelTarget) {
	ctx := r.Context()
	base := h.runtimeRequest(r)
	key := snapshot.ID + ":" + string(target.ref)
	result := h.loads.DoChan(key, func() (any, error) {
		workCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), h.workTimeout)
		defer cancel()
		latest, err := h.snapshots.Get(workCtx, snapshot.ID)
		if err != nil {
			return nil, err
		}
		if cached, ok := latest.Frames[target.ref]; ok {
			return cached, nil
		}
		panelResult, err := h.executeLevel(workCtx, thawRuntimeRequest(base, latest.Params), latest.Params, target, 0)
		if err != nil {
			return nil, err
		}
		wire, err := wireFrame(target.ref, panelResult)
		if err != nil {
			return nil, err
		}
		if err := h.snapshots.Append(workCtx, snapshot.ID, map[document.FrameRef]document.Frame{target.ref: wire}); err != nil {
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
		writeJSON(w, http.StatusOK, QueryResponse{Frames: map[document.FrameRef]document.Frame{target.ref: frame}})
	}
}

// Export writes a snapshot-keyed workbook for one panel or the full document.
func (h *Handlers) Export(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "bad_request", "method must be GET")
		return
	}
	snapshotID := strings.TrimSpace(r.URL.Query().Get("snapshot"))
	if snapshotID == "" {
		writeError(w, http.StatusBadRequest, "bad_request", "snapshot is required")
		return
	}
	snapshot, err := h.snapshots.Get(r.Context(), snapshotID)
	if err != nil {
		h.writeSnapshotError(r.Context(), w, err)
		return
	}
	request := thawRuntimeRequest(h.runtimeRequest(r), snapshot.Params)
	result, err := runtimeResultFromSnapshot(h.spec, snapshot, request)
	if err != nil {
		h.writeInternalError(r.Context(), w, "lens/serve.Export", "snapshot conversion failed", err)
		return
	}
	panelID := strings.TrimSpace(r.URL.Query().Get("panel"))
	if panelID != "" && result.Panel(panelID) == nil {
		writeError(w, http.StatusBadRequest, "bad_request", "panel is not available in the snapshot")
		return
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
		writeError(w, http.StatusGone, "snapshot_gone", "snapshot is unknown or expired")
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
	writeError(w, http.StatusInternalServerError, "internal", message)
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

func writeError(w http.ResponseWriter, status int, code, message string) {
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
