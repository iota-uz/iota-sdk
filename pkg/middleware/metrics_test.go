package middleware

import (
	"bufio"
	"bytes"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gorilla/mux"
	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
	require.Panics(t, func() {
		NewMetrics(MetricsOptions{AuthToken: ""})
	})
}

func TestNewMetrics_MultipleCalls(t *testing.T) {
	// Verify that creating multiple Metrics instances doesn't panic
	// (each uses its own registry).
	for i := 0; i < 3; i++ {
		assert.NotPanics(t, func() {
			newTestMetrics(t, "token")
		})
	}
}

func TestNewMetrics_AppliesConstLabels(t *testing.T) {
	reg := prometheus.NewRegistry()
	m := NewMetrics(MetricsOptions{
		AuthToken:   "token",
		Registry:    reg,
		Gatherer:    reg,
		ConstLabels: prometheus.Labels{"service": "eai-back"},
	})

	router := mux.NewRouter()
	router.Use(m.Middleware())
	router.HandleFunc("/health", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)
	require.Equal(t, http.StatusOK, rr.Code)

	families, err := reg.Gather()
	require.NoError(t, err)

	for _, family := range families {
		if family.GetName() != "http_requests_total" {
			continue
		}

		require.NotEmpty(t, family.GetMetric())
		foundServiceLabel := false
		for _, metric := range family.GetMetric() {
			for _, label := range metric.GetLabel() {
				if label.GetName() == "service" && label.GetValue() == "eai-back" {
					foundServiceLabel = true
				}
			}
		}
		assert.True(t, foundServiceLabel, "expected const service label on http_requests_total")
		return
	}

	require.Fail(t, "http_requests_total metric family not found")
}

// metricsHandler wraps the middleware around a no-op handler so /metrics interception
// works without depending on gorilla/mux route matching.
func metricsHandler(m *Metrics) http.Handler {
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	return m.Middleware()(inner)
}

func TestServeMetrics_Auth(t *testing.T) {
	tests := []struct {
		name       string
		authHeader string
		wantStatus int
		wantBody   string // substring to assert in body (empty = skip)
	}{
		{
			name:       "valid token",
			authHeader: "Bearer secret-token",
			wantStatus: http.StatusOK,
			wantBody:   "http_requests_in_flight",
		},
		{
			name:       "invalid token",
			authHeader: "Bearer wrong-token",
			wantStatus: http.StatusUnauthorized,
		},
		{
			name:       "missing header",
			authHeader: "",
			wantStatus: http.StatusUnauthorized,
		},
		{
			name:       "empty bearer value",
			authHeader: "Bearer ",
			wantStatus: http.StatusUnauthorized,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m, _ := newTestMetrics(t, "secret-token")
			handler := metricsHandler(m)

			req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
			if tt.authHeader != "" {
				req.Header.Set("Authorization", tt.authHeader)
			}
			rr := httptest.NewRecorder()
			handler.ServeHTTP(rr, req)

			require.Equal(t, tt.wantStatus, rr.Code)
			if tt.wantBody != "" {
				assert.Contains(t, rr.Body.String(), tt.wantBody)
			}
		})
	}
}

func TestMiddleware_RecordsMetrics(t *testing.T) {
	m, reg := newTestMetrics(t, "tok")
	router := mux.NewRouter()
	router.Use(m.Middleware())
	router.HandleFunc("/api/test", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
	})
	handler := m.Middleware()(router)

	req := httptest.NewRequest(http.MethodPost, "/api/test", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	require.Equal(t, http.StatusCreated, rr.Code)

	families, err := reg.Gather()
	require.NoError(t, err)

	httpRequestsTotal := findMetricFamily(t, families, "http_requests_total")
	assertMetricWithLabels(t, httpRequestsTotal, map[string]string{
		"method":      http.MethodPost,
		"route":       "/api/test",
		"status_code": "201",
	})

	req404 := httptest.NewRequest(http.MethodGet, "/missing", nil)
	rr404 := httptest.NewRecorder()
	handler.ServeHTTP(rr404, req404)
	require.Equal(t, http.StatusNotFound, rr404.Code)

	families, err = reg.Gather()
	require.NoError(t, err)

	httpRequestsTotal = findMetricFamily(t, families, "http_requests_total")
	assertMetricWithLabels(t, httpRequestsTotal, map[string]string{
		"method":      http.MethodGet,
		"route":       "unmatched",
		"status_code": "404",
	})

	httpRequestDuration := findMetricFamily(t, families, "http_request_duration_seconds")
	require.NotNil(t, httpRequestDuration)
	assertMetricWithLabels(t, httpRequestDuration, map[string]string{
		"method":      http.MethodPost,
		"route":       "/api/test",
		"status_code": "201",
	})
	assertMetricWithLabels(t, httpRequestDuration, map[string]string{
		"method":      http.MethodGet,
		"route":       "unmatched",
		"status_code": "404",
	})
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
	require.Equal(t, http.StatusOK, rr.Code)

	families, err := reg.Gather()
	require.NoError(t, err)

	for _, f := range families {
		if f.GetName() != "http_requests_total" {
			continue
		}

		for _, metric := range f.GetMetric() {
			for _, lp := range metric.GetLabel() {
				require.NotEqual(t, "host", lp.GetName())
			}
		}
	}
}

func TestMetricsResponseWriter_DefaultStatus(t *testing.T) {
	w := wrapMetricsResponseWriter(httptest.NewRecorder())
	require.Equal(t, http.StatusOK, w.Status(), "default status should be 200")
}

func TestMetricsResponseWriter_WriteHeaderIdempotent(t *testing.T) {
	w := wrapMetricsResponseWriter(httptest.NewRecorder())
	w.WriteHeader(http.StatusNotFound)
	w.WriteHeader(http.StatusOK) // should be ignored
	require.Equal(t, http.StatusNotFound, w.Status(), "WriteHeader should be idempotent")
}

func TestMetricsResponseWriter_HijackMarksSwitchingProtocols(t *testing.T) {
	w := wrapMetricsResponseWriter(&hijackableResponseWriter{ResponseWriter: httptest.NewRecorder()})
	_, _, err := w.Hijack()
	require.NoError(t, err)
	require.Equal(t, http.StatusSwitchingProtocols, w.Status())
}

func findMetricFamily(t *testing.T, families []*dto.MetricFamily, name string) *dto.MetricFamily {
	t.Helper()

	for _, family := range families {
		if family.GetName() == name {
			return family
		}
	}

	require.FailNowf(t, "metric family missing", "expected metric family %s", name)
	return nil
}

func assertMetricWithLabels(t *testing.T, family *dto.MetricFamily, labels map[string]string) {
	t.Helper()

	for _, metric := range family.GetMetric() {
		if labelsMatch(metric.GetLabel(), labels) {
			return
		}
	}

	require.FailNowf(t, "metric series missing", "expected %s series with labels %v", family.GetName(), labels)
}

func labelsMatch(labelPairs []*dto.LabelPair, expected map[string]string) bool {
	found := make(map[string]string, len(labelPairs))
	for _, pair := range labelPairs {
		found[pair.GetName()] = pair.GetValue()
	}

	for key, value := range expected {
		if found[key] != value {
			return false
		}
	}

	return true
}

type hijackableResponseWriter struct {
	http.ResponseWriter
}

func (w *hijackableResponseWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	serverConn, clientConn := net.Pipe()
	_ = clientConn.Close()
	return serverConn, bufio.NewReadWriter(bufio.NewReader(bytes.NewReader(nil)), bufio.NewWriter(&bytes.Buffer{})), nil
}
