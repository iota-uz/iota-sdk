package export

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/iota-uz/iota-sdk/pkg/lens"
	"github.com/iota-uz/iota-sdk/pkg/lens/explore"
	"github.com/iota-uz/iota-sdk/pkg/lens/exportmeta"
	"github.com/iota-uz/iota-sdk/pkg/lens/panel"
	"github.com/iota-uz/iota-sdk/pkg/lens/runtime"
	"github.com/stretchr/testify/require"
)

type resolverFunc func(context.Context, ResolveRequest) (*runtime.DashboardResult, error)

func (f resolverFunc) ResolveLensExport(ctx context.Context, req ResolveRequest) (*runtime.DashboardResult, error) {
	return f(ctx, req)
}

func TestHandlerServeHTTP_SignalsWhenDownloadStarts(t *testing.T) {
	t.Parallel()

	handler := Handler{
		Resolver: resolverFunc(func(context.Context, ResolveRequest) (*runtime.DashboardResult, error) {
			return &runtime.DashboardResult{Spec: lens.DashboardSpec{Title: "Premium report"}, Locale: "en"}, nil
		}),
		Now: func() time.Time { return time.Date(2026, time.July, 18, 15, 10, 0, 0, time.UTC) },
	}
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/export?dashboard=profitability&lens_download_token=download-1", nil)

	handler.ServeHTTP(recorder, request)

	response := recorder.Result()
	require.Equal(t, http.StatusOK, response.StatusCode)
	require.Equal(t, "started", responseCookieValue(response, "lens_export_download-1"))
	require.Equal(t, `attachment; filename="Premium-report-20260718-1510.xlsx"; filename*=UTF-8''Premium-report-20260718-1510.xlsx`, response.Header.Get("Content-Disposition"))
}

func TestHandlerServeHTTP_NamesPanelExportFromLocalizedTitle(t *testing.T) {
	t.Parallel()

	handler := Handler{
		Resolver: resolverFunc(func(context.Context, ResolveRequest) (*runtime.DashboardResult, error) {
			return &runtime.DashboardResult{
				Spec: lens.DashboardSpec{Title: "Прибыльность", Export: exportmeta.Spec{Filename: "profitability"}},
				Panels: map[string]*runtime.PanelResult{
					"written-premium": {Panel: panel.Spec{ID: "written-premium", Title: "Брутто подписанная премия", Kind: panel.KindStat}},
				},
			}, nil
		}),
		Now: func() time.Time { return time.Date(2026, time.July, 18, 15, 11, 0, 0, time.UTC) },
	}
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/export?dashboard=profitability&panel=written-premium", nil)

	handler.ServeHTTP(recorder, request)

	response := recorder.Result()
	require.Equal(t, http.StatusOK, response.StatusCode)
	require.Equal(t,
		`attachment; filename="profitability-20260718-1511.xlsx"; filename*=UTF-8''profitability-%D0%91%D1%80%D1%83%D1%82%D1%82%D0%BE-%D0%BF%D0%BE%D0%B4%D0%BF%D0%B8%D1%81%D0%B0%D0%BD%D0%BD%D0%B0%D1%8F-%D0%BF%D1%80%D0%B5%D0%BC%D0%B8%D1%8F-20260718-1511.xlsx`,
		response.Header.Get("Content-Disposition"),
	)
}

func TestHandlerServeHTTP_SignalsResolverFailure(t *testing.T) {
	t.Parallel()

	handler := Handler{Resolver: resolverFunc(func(context.Context, ResolveRequest) (*runtime.DashboardResult, error) {
		return nil, errors.New("generation failed")
	})}
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/export?dashboard=profitability&lens_download_token=download-2", nil)

	handler.ServeHTTP(recorder, request)

	response := recorder.Result()
	require.Equal(t, http.StatusInternalServerError, response.StatusCode)
	require.Equal(t, "error", responseCookieValue(response, "lens_export_download-2"))
}

func TestHandlerServeHTTP_BuffersWorkbookBeforeStartingResponse(t *testing.T) {
	t.Parallel()
	handler := Handler{Resolver: resolverFunc(func(context.Context, ResolveRequest) (*runtime.DashboardResult, error) {
		return &runtime.DashboardResult{}, nil
	})}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	request := httptest.NewRequest(http.MethodGet, "/export?dashboard=profitability&lens_download_token=download-3", nil).WithContext(ctx)
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	response := recorder.Result()
	require.Equal(t, http.StatusInternalServerError, response.StatusCode)
	require.Equal(t, "error", responseCookieValue(response, "lens_export_download-3"))
	require.Empty(t, response.Header.Get("Content-Disposition"))
}

func TestHandlerServeHTTP_IgnoresInvalidDownloadToken(t *testing.T) {
	t.Parallel()

	handler := Handler{Resolver: resolverFunc(func(context.Context, ResolveRequest) (*runtime.DashboardResult, error) {
		return nil, errors.New("generation failed")
	})}
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/export?dashboard=profitability&lens_download_token=not%20valid", nil)

	handler.ServeHTTP(recorder, request)

	require.Empty(t, recorder.Result().Cookies())
}

func TestParseExplorationExportRequest_Scenario(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		name   string
		values map[string][]string
		mode   explore.ExportMode
		path   []string
		steps  []explore.PathStep
	}{
		{"current view", map[string][]string{ExplorationIDQuery: {"premium"}, ExplorationBranchQuery: {"unearned"}, ExplorationPerspectiveQuery: {"products"}, ExplorationPathQuery: {"root", "property"}, ExplorationPointQuery: {"", "other"}, ExplorationNodeQuery: {"property"}}, explore.ExportCurrentView, []string{"root", "property"}, []explore.PathStep{{NodeKey: "root"}, {NodeKey: "property", PointKey: "other"}}},
		{"full export", map[string][]string{ExplorationModeQuery: {string(explore.ExportFull)}, ExplorationIDQuery: {"premium"}, ExplorationBranchQuery: {"unearned"}}, explore.ExportFull, nil, []explore.PathStep{}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			request, ok, err := ParseExplorationExportRequest(tt.values)
			require.NoError(t, err)
			require.True(t, ok)
			require.Equal(t, tt.mode, request.Mode)
			require.Equal(t, tt.path, request.Path)
			require.Equal(t, tt.steps, request.Steps)
		})
	}
}

func TestWorkbookFilename_UsesResolvedExplorationLabel(t *testing.T) {
	t.Parallel()

	result := &runtime.DashboardResult{Spec: lens.DashboardSpec{Export: exportmeta.Spec{Filename: "profitability"}}}
	selection := &explore.ExportRequest{
		Mode:   explore.ExportCurrentView,
		Labels: explore.ExportLabels{Node: "Product mix"},
	}
	filename := WorkbookFilename(result, "", time.Time{}, selection)
	require.Equal(t, "profitability-Product-mix.xlsx", filename)
}

func responseCookieValue(response *http.Response, name string) string {
	for _, cookie := range response.Cookies() {
		if cookie.Name == name {
			return cookie.Value
		}
	}
	return ""
}
