package export

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/iota-uz/iota-sdk/pkg/lens/runtime"
)

type ResolveRequest struct {
	DashboardID, PanelID, SnapshotID string
	Query                            url.Values
}
type Resolver interface {
	ResolveLensExport(context.Context, ResolveRequest) (*runtime.DashboardResult, error)
}
type Authorize func(context.Context, string, string) error

type Handler struct {
	Resolver  Resolver
	Authorize Authorize
	Exporter  *Exporter
}

func (h Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	dashboardID := strings.TrimSpace(r.URL.Query().Get("dashboard"))
	panelID := strings.TrimSpace(r.URL.Query().Get("panel"))
	snapshotID := strings.TrimSpace(r.URL.Query().Get("snapshot"))
	if dashboardID == "" {
		http.Error(w, "dashboard is required", http.StatusBadRequest)
		return
	}
	if h.Authorize != nil {
		if err := h.Authorize(r.Context(), dashboardID, panelID); err != nil {
			http.Error(w, "Forbidden", http.StatusForbidden)
			return
		}
	}
	if h.Resolver == nil {
		http.Error(w, "export resolver is not configured", http.StatusInternalServerError)
		return
	}
	result, err := h.Resolver.ResolveLensExport(r.Context(), ResolveRequest{DashboardID: dashboardID, PanelID: panelID, SnapshotID: snapshotID, Query: r.URL.Query()})
	if err != nil {
		http.Error(w, "export failed", http.StatusInternalServerError)
		return
	}
	filename := safeFilename(result.Spec.Export.Filename)
	if filename == "" {
		filename = safeFilename(result.Spec.Title)
	}
	if filename == "" {
		filename = "dashboard"
	}
	filename += ".xlsx"
	w.Header().Set("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))
	exporter := h.Exporter
	if exporter == nil {
		exporter = New()
	}
	if err := exporter.Write(r.Context(), w, Request{Result: result, PanelID: panelID}); err != nil {
		return
	}
}

func safeFilename(value string) string {
	value = strings.TrimSpace(value)
	var b strings.Builder
	for _, r := range value {
		if r > 127 || r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '-' || r == '_' {
			b.WriteRune(r)
		} else if r == ' ' {
			b.WriteByte('-')
		}
	}
	return strings.Trim(b.String(), "-")
}
