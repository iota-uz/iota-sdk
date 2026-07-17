package export

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/iota-uz/iota-sdk/pkg/lens/runtime"
)

const (
	downloadTokenParam   = "lens_download_token"
	downloadCookiePrefix = "lens_export_"
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
		setDownloadSignal(w, r, "error")
		http.Error(w, "dashboard is required", http.StatusBadRequest)
		return
	}
	if h.Authorize != nil {
		if err := h.Authorize(r.Context(), dashboardID, panelID); err != nil {
			setDownloadSignal(w, r, "error")
			http.Error(w, "Forbidden", http.StatusForbidden)
			return
		}
	}
	if h.Resolver == nil {
		setDownloadSignal(w, r, "error")
		http.Error(w, "export resolver is not configured", http.StatusInternalServerError)
		return
	}
	result, err := h.Resolver.ResolveLensExport(r.Context(), ResolveRequest{DashboardID: dashboardID, PanelID: panelID, SnapshotID: snapshotID, Query: r.URL.Query()})
	if err != nil {
		setDownloadSignal(w, r, "error")
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
	setDownloadSignal(w, r, "started")
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

func setDownloadSignal(w http.ResponseWriter, r *http.Request, value string) {
	token := strings.TrimSpace(r.URL.Query().Get(downloadTokenParam))
	if !validDownloadToken(token) {
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name:     downloadCookiePrefix + token,
		Value:    value,
		Path:     "/",
		MaxAge:   600,
		SameSite: http.SameSiteLaxMode,
		Secure:   r.TLS != nil,
	})
}

func validDownloadToken(token string) bool {
	if token == "" || len(token) > 64 {
		return false
	}
	for _, r := range token {
		if r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '-' || r == '_' {
			continue
		}
		return false
	}
	return true
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
