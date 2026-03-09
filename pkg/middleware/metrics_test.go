package middleware

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gorilla/mux"
	"github.com/prometheus/client_golang/prometheus"
)

func newTestMetrics(t *testing.T, token string) (*Metrics, *prometheus.Registry) {
	t.Helper()
	reg := prometheus.NewRegistry()
	m := NewMetrics(MetricsOptions{
		AuthToken: token,
		Registry:  reg,
		Gatherer:  reg,
	})
	return m, reg
}

func TestNewMetrics_PanicsOnEmptyToken(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic for empty AuthToken")
		}
	}()
	NewMetrics(MetricsOptions{AuthToken: ""})
}

func TestNewMetrics_MultipleCalls(t *testing.T) {
	// Verify that creating multiple Metrics instances doesn't panic
	// (each uses its own registry).
	for i := 0; i < 3; i++ {
		newTestMetrics(t, "token")
	}
}

// metricsHandler wraps the middleware around a no-op handler so /metrics interception
// works without depending on gorilla/mux route matching.
func metricsHandler(m *Metrics) http.Handler {
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	return m.Middleware()(inner)
}

func TestServeMetrics_ValidToken(t *testing.T) {
	m, _ := newTestMetrics(t, "secret-token")
	handler := metricsHandler(m)

	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	req.Header.Set("Authorization", "Bearer secret-token")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	if !strings.Contains(rr.Body.String(), "http_requests_in_flight") {
		t.Fatal("expected prometheus metrics output")
	}
}

func TestServeMetrics_InvalidToken(t *testing.T) {
	m, _ := newTestMetrics(t, "secret-token")
	handler := metricsHandler(m)

	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	req.Header.Set("Authorization", "Bearer wrong-token")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rr.Code)
	}
}

func TestServeMetrics_MissingHeader(t *testing.T) {
	m, _ := newTestMetrics(t, "secret-token")
	handler := metricsHandler(m)

	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rr.Code)
	}
}

func TestServeMetrics_EmptyBearerValue(t *testing.T) {
	m, _ := newTestMetrics(t, "secret-token")
	handler := metricsHandler(m)

	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	req.Header.Set("Authorization", "Bearer ")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for empty bearer, got %d", rr.Code)
	}
}

func TestMiddleware_RecordsMetrics(t *testing.T) {
	m, reg := newTestMetrics(t, "tok")
	router := mux.NewRouter()
	router.Use(m.Middleware())
	router.HandleFunc("/api/test", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
	})

	req := httptest.NewRequest(http.MethodPost, "/api/test", nil)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d", rr.Code)
	}

	// Verify metrics were recorded.
	families, err := reg.Gather()
	if err != nil {
		t.Fatalf("gather: %v", err)
	}

	found := map[string]bool{
		"http_requests_total":            false,
		"http_request_duration_seconds":  false,
	}
	for _, f := range families {
		if _, ok := found[f.GetName()]; ok {
			found[f.GetName()] = true
		}
	}
	for name, ok := range found {
		if !ok {
			t.Errorf("metric %q not found after request", name)
		}
	}
}

func TestMiddleware_NoHostLabel(t *testing.T) {
	m, reg := newTestMetrics(t, "tok")
	router := mux.NewRouter()
	router.Use(m.Middleware())
	router.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}

	families, err := reg.Gather()
	if err != nil {
		t.Fatalf("gather: %v", err)
	}

	for _, f := range families {
		if f.GetName() == "http_requests_total" {
			for _, m := range f.GetMetric() {
				for _, lp := range m.GetLabel() {
					if lp.GetName() == "host" {
						t.Fatal("unexpected 'host' label found on http_requests_total")
					}
				}
			}
		}
	}
}

func TestMetricsResponseWriter_DefaultStatus(t *testing.T) {
	w := &metricsResponseWriter{ResponseWriter: httptest.NewRecorder()}
	if w.Status() != http.StatusOK {
		t.Fatalf("default status should be 200, got %d", w.Status())
	}
}

func TestMetricsResponseWriter_WriteHeaderIdempotent(t *testing.T) {
	w := &metricsResponseWriter{ResponseWriter: httptest.NewRecorder()}
	w.WriteHeader(http.StatusNotFound)
	w.WriteHeader(http.StatusOK) // should be ignored
	if w.Status() != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Status())
	}
}
