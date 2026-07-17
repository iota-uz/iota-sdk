package export

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/iota-uz/iota-sdk/pkg/lens"
	"github.com/iota-uz/iota-sdk/pkg/lens/runtime"
	"github.com/stretchr/testify/require"
)

type resolverFunc func(context.Context, ResolveRequest) (*runtime.DashboardResult, error)

func (f resolverFunc) ResolveLensExport(ctx context.Context, req ResolveRequest) (*runtime.DashboardResult, error) {
	return f(ctx, req)
}

func TestHandlerSignalsWhenDownloadStarts(t *testing.T) {
	t.Parallel()

	handler := Handler{Resolver: resolverFunc(func(context.Context, ResolveRequest) (*runtime.DashboardResult, error) {
		return &runtime.DashboardResult{Spec: lens.DashboardSpec{Title: "Premium report"}, Locale: "en"}, nil
	})}
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/export?dashboard=profitability&lens_download_token=download-1", nil)

	handler.ServeHTTP(recorder, request)

	response := recorder.Result()
	require.Equal(t, http.StatusOK, response.StatusCode)
	require.Equal(t, "started", responseCookieValue(response, "lens_export_download-1"))
	require.Equal(t, "attachment; filename=\"Premium-report.xlsx\"", response.Header.Get("Content-Disposition"))
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
