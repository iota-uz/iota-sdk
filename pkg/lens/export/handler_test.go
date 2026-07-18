package export

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/iota-uz/iota-sdk/pkg/lens"
	"github.com/iota-uz/iota-sdk/pkg/lens/exportmeta"
	"github.com/iota-uz/iota-sdk/pkg/lens/panel"
	"github.com/iota-uz/iota-sdk/pkg/lens/runtime"
	"github.com/stretchr/testify/require"
)

type resolverFunc func(context.Context, ResolveRequest) (*runtime.DashboardResult, error)

func (f resolverFunc) ResolveLensExport(ctx context.Context, req ResolveRequest) (*runtime.DashboardResult, error) {
	return f(ctx, req)
}

func TestHandlerSignalsWhenDownloadStarts(t *testing.T) {
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

func TestHandlerNamesPanelExportFromLocalizedTitle(t *testing.T) {
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

func TestHandlerSignalsExportFailure(t *testing.T) {
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

func TestHandlerIgnoresInvalidDownloadToken(t *testing.T) {
	t.Parallel()

	handler := Handler{Resolver: resolverFunc(func(context.Context, ResolveRequest) (*runtime.DashboardResult, error) {
		return nil, errors.New("generation failed")
	})}
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/export?dashboard=profitability&lens_download_token=not%20valid", nil)

	handler.ServeHTTP(recorder, request)

	require.Empty(t, recorder.Result().Cookies())
}

func responseCookieValue(response *http.Response, name string) string {
	for _, cookie := range response.Cookies() {
		if cookie.Name == name {
			return cookie.Value
		}
	}
	return ""
}
