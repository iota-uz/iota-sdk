package middleware

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// serve runs handler through HTMXCacheControl and returns the recorded response.
func serve(t *testing.T, req *http.Request, handler http.HandlerFunc) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	HTMXCacheControl()(handler).ServeHTTP(rec, req)
	return rec
}

func TestHTMXCacheControl_HTMLContentType(t *testing.T) {
	rec := serve(t, httptest.NewRequest(http.MethodGet, "/", nil), func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte("<html></html>"))
	})

	assert.Equal(t, "Hx-Request", rec.Header().Get("Vary"))
	assert.Equal(t, "no-store", rec.Header().Get("Cache-Control"))
}

// The regression this guards: a handler that writes an HTML body without setting
// Content-Type. net/http sniffs it as text/html, so the middleware must too —
// otherwise the cache-safety headers are silently skipped.
func TestHTMXCacheControl_SniffsEmptyContentType(t *testing.T) {
	rec := serve(t, httptest.NewRequest(http.MethodGet, "/", nil), func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("<!DOCTYPE html><html><body>hi</body></html>"))
	})

	assert.Equal(t, "Hx-Request", rec.Header().Get("Vary"))
	assert.Equal(t, "no-store", rec.Header().Get("Cache-Control"))
}

// Regression for the WriteHeader-before-Write path: a handler that explicitly
// commits a 200 with no Content-Type, then writes an HTML body. This must run
// against a real server — httptest.ResponseRecorder never freezes its header map
// so it cannot reproduce the lost-headers bug. The middleware buffers the status
// until the body can be sniffed.
func TestHTMXCacheControl_SniffsAfterExplicitWriteHeader(t *testing.T) {
	h := HTMXCacheControl()(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("<!DOCTYPE html><html><body>hi</body></html>"))
	}))
	srv := httptest.NewServer(h)
	defer srv.Close()

	resp, err := http.Get(srv.URL)
	require.NoError(t, err)
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body)

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "Hx-Request", resp.Header.Get("Vary"))
	assert.Equal(t, "no-store", resp.Header.Get("Cache-Control"))
}

// A buffered status with no body (here a 204) must still be written.
func TestHTMXCacheControl_BufferedStatusNoBody(t *testing.T) {
	h := HTMXCacheControl()(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	srv := httptest.NewServer(h)
	defer srv.Close()

	resp, err := http.Get(srv.URL)
	require.NoError(t, err)
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body)

	assert.Equal(t, http.StatusNoContent, resp.StatusCode)
	assert.Empty(t, resp.Header.Get("Vary"))
}

func TestHTMXCacheControl_NonHTMLUntouched(t *testing.T) {
	rec := serve(t, httptest.NewRequest(http.MethodGet, "/", nil), func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true}`))
	})

	assert.Empty(t, rec.Header().Get("Vary"))
	assert.Empty(t, rec.Header().Get("Cache-Control"))
}

func TestHTMXCacheControl_PreservesHandlerCacheControl(t *testing.T) {
	rec := serve(t, httptest.NewRequest(http.MethodGet, "/", nil), func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.Header().Set("Cache-Control", "public, max-age=3600")
		_, _ = w.Write([]byte("<html></html>"))
	})

	assert.Equal(t, "Hx-Request", rec.Header().Get("Vary"))
	assert.Equal(t, "public, max-age=3600", rec.Header().Get("Cache-Control"))
}

func TestHTMXCacheControl_HistoryRestoreStripsMarkers(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Hx-History-Restore-Request", "true")
	req.Header.Set("Hx-Request", "true")
	req.Header.Set("Hx-Boosted", "true")

	var sawHxRequest, sawBoosted string
	serve(t, req, func(_ http.ResponseWriter, r *http.Request) {
		sawHxRequest = r.Header.Get("Hx-Request")
		sawBoosted = r.Header.Get("Hx-Boosted")
	})

	assert.Empty(t, sawHxRequest, "history-restore must render as a full navigation")
	assert.Empty(t, sawBoosted)
}
