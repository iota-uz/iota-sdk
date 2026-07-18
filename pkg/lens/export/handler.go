package export

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/iota-uz/iota-sdk/pkg/lens/runtime"
)

const (
	downloadTokenParam   = "lens_download_token"
	downloadCookiePrefix = "lens_export_"
)

type DownloadSignal string

const (
	DownloadSignalStarted DownloadSignal = "started"
	DownloadSignalError   DownloadSignal = "error"
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
	Now       func() time.Time
}

func (h Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	dashboardID := strings.TrimSpace(r.URL.Query().Get("dashboard"))
	panelID := strings.TrimSpace(r.URL.Query().Get("panel"))
	snapshotID := strings.TrimSpace(r.URL.Query().Get("snapshot"))
	if dashboardID == "" {
		SetDownloadSignal(w, r, DownloadSignalError)
		http.Error(w, "dashboard is required", http.StatusBadRequest)
		return
	}
	if h.Authorize != nil {
		if err := h.Authorize(r.Context(), dashboardID, panelID); err != nil {
			SetDownloadSignal(w, r, DownloadSignalError)
			http.Error(w, "Forbidden", http.StatusForbidden)
			return
		}
	}
	if h.Resolver == nil {
		SetDownloadSignal(w, r, DownloadSignalError)
		http.Error(w, "export resolver is not configured", http.StatusInternalServerError)
		return
	}
	result, err := h.Resolver.ResolveLensExport(r.Context(), ResolveRequest{DashboardID: dashboardID, PanelID: panelID, SnapshotID: snapshotID, Query: r.URL.Query()})
	if err != nil {
		SetDownloadSignal(w, r, DownloadSignalError)
		http.Error(w, "export failed", http.StatusInternalServerError)
		return
	}
	now := time.Now()
	if h.Now != nil {
		now = h.Now()
	}
	filename := WorkbookFilename(result, panelID, now)
	SetDownloadSignal(w, r, DownloadSignalStarted)
	w.Header().Set("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	w.Header().Set("Content-Disposition", ContentDisposition(filename))
	exporter := h.Exporter
	if exporter == nil {
		exporter = New()
	}
	if err := exporter.Write(r.Context(), w, Request{Result: result, PanelID: panelID}); err != nil {
		return
	}
}

func WorkbookFilename(result *runtime.DashboardResult, panelID string, generatedAt time.Time) string {
	base := "dashboard"
	if result != nil {
		if configured := safeFilename(result.Spec.Export.Filename); configured != "" {
			base = configured
		} else if title := safeFilename(result.Spec.Title); title != "" {
			base = title
		}
		if panelID = strings.TrimSpace(panelID); panelID != "" {
			panelName := panelID
			if panel := result.Panel(panelID); panel != nil && strings.TrimSpace(panel.Panel.Title) != "" {
				panelName = panel.Panel.Title
			}
			if panelName = safeFilename(panelName); panelName != "" && !strings.EqualFold(panelName, base) {
				base += "-" + panelName
			}
		}
	}
	if !generatedAt.IsZero() {
		base += "-" + generatedAt.Format("20060102-1504")
	}
	return base + ".xlsx"
}

func ContentDisposition(filename string) string {
	filename = strings.TrimSpace(filename)
	if filename == "" {
		filename = "dashboard.xlsx"
	}
	base := strings.TrimSuffix(filename, ".xlsx")
	ascii := safeASCIIFilename(base)
	if ascii == "" {
		ascii = "dashboard"
	}
	ascii += ".xlsx"
	return fmt.Sprintf(`attachment; filename="%s"; filename*=UTF-8''%s`, ascii, url.PathEscape(filename))
}

// SetDownloadSignal completes the browser-side export loading state. Custom
// authorized Lens endpoints must use it as well as the canonical Handler so a
// failed export cannot leave its button busy indefinitely.
func SetDownloadSignal(w http.ResponseWriter, r *http.Request, value DownloadSignal) {
	token := strings.TrimSpace(r.URL.Query().Get(downloadTokenParam))
	if !validDownloadToken(token) {
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name:     downloadCookiePrefix + token,
		Value:    string(value),
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
	result := strings.Trim(b.String(), "-")
	for strings.Contains(result, "--") {
		result = strings.ReplaceAll(result, "--", "-")
	}
	return result
}

func safeASCIIFilename(value string) string {
	value = strings.TrimSpace(value)
	var b strings.Builder
	for _, r := range value {
		if r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '-' || r == '_' {
			b.WriteRune(r)
		} else if r == ' ' {
			b.WriteByte('-')
		}
	}
	result := strings.Trim(b.String(), "-")
	for strings.Contains(result, "--") {
		result = strings.ReplaceAll(result, "--", "-")
	}
	return result
}
