package middleware

import (
	"bufio"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/mux"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// MetricsOptions configures the Prometheus metrics middleware.
type MetricsOptions struct {
	AuthToken string // required bearer token for /metrics endpoint
}

// Metrics holds Prometheus collectors and the metrics HTTP handler.
type Metrics struct {
	requestDuration  *prometheus.HistogramVec
	requestsTotal    *prometheus.CounterVec
	requestsInFlight prometheus.Gauge
	metricsHandler   http.Handler
	authToken        string
}

// NewMetrics creates a new Metrics instance, registers collectors with
// prometheus.DefaultRegisterer, and builds a promhttp handler backed by
// prometheus.DefaultGatherer.
func NewMetrics(opts MetricsOptions) *Metrics {
	requestDuration := prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "http_request_duration_seconds",
			Help:    "Duration of HTTP requests in seconds.",
			Buckets: []float64{0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10},
		},
		[]string{"method", "route", "status_code", "host"},
	)

	requestsTotal := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "http_requests_total",
			Help: "Total number of HTTP requests.",
		},
		[]string{"method", "route", "status_code", "host"},
	)

	requestsInFlight := prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "http_requests_in_flight",
			Help: "Number of HTTP requests currently being served.",
		},
	)

	prometheus.DefaultRegisterer.MustRegister(
		requestDuration,
		requestsTotal,
		requestsInFlight,
	)

	metricsHandler := promhttp.HandlerFor(prometheus.DefaultGatherer, promhttp.HandlerOpts{})

	return &Metrics{
		requestDuration:  requestDuration,
		requestsTotal:    requestsTotal,
		requestsInFlight: requestsInFlight,
		metricsHandler:   metricsHandler,
		authToken:        opts.AuthToken,
	}
}

// Middleware returns a mux.MiddlewareFunc that:
//  1. Intercepts /metrics requests and serves Prometheus metrics (with bearer token auth).
//  2. For all other requests: tracks in-flight gauge, records request duration histogram,
//     and increments the total request counter.
func (m *Metrics) Middleware() mux.MiddlewareFunc {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Intercept /metrics endpoint.
			if r.URL.Path == "/metrics" {
				m.serveMetrics(w, r)
				return
			}

			m.requestsInFlight.Inc()
			defer m.requestsInFlight.Dec()

			wrappedWriter := &metricsResponseWriter{
				ResponseWriter: w,
			}

			start := time.Now()
			next.ServeHTTP(wrappedWriter, r)
			duration := time.Since(start).Seconds()

			route := "unmatched"
			if cr := mux.CurrentRoute(r); cr != nil {
				if tmpl, err := cr.GetPathTemplate(); err == nil {
					route = tmpl
				}
			}

			statusCode := strconv.Itoa(wrappedWriter.Status())
			method := r.Method
			host := r.Host

			m.requestDuration.WithLabelValues(method, route, statusCode, host).Observe(duration)
			m.requestsTotal.WithLabelValues(method, route, statusCode, host).Inc()
		})
	}
}

// serveMetrics handles the /metrics endpoint with bearer token authentication.
func (m *Metrics) serveMetrics(w http.ResponseWriter, r *http.Request) {
	authHeader := r.Header.Get("Authorization")
	const prefix = "Bearer "
	if !strings.HasPrefix(authHeader, prefix) || authHeader[len(prefix):] != m.authToken {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	m.metricsHandler.ServeHTTP(w, r)
}

// metricsResponseWriter is a lightweight response writer wrapper that captures
// the status code. It implements http.Flusher and http.Hijacker for SSE and
// WebSocket compatibility.
type metricsResponseWriter struct {
	http.ResponseWriter
	statusCode    int
	statusWritten bool
}

func (w *metricsResponseWriter) WriteHeader(code int) {
	if !w.statusWritten {
		w.statusCode = code
		w.statusWritten = true
		w.ResponseWriter.WriteHeader(code)
	}
}

// Status returns the HTTP status code written to the response.
// Defaults to 200 if WriteHeader was never called explicitly.
func (w *metricsResponseWriter) Status() int {
	if w.statusCode == 0 {
		return http.StatusOK
	}
	return w.statusCode
}

func (w *metricsResponseWriter) Write(b []byte) (int, error) {
	if !w.statusWritten {
		w.statusCode = http.StatusOK
		w.statusWritten = true
	}
	return w.ResponseWriter.Write(b)
}

// Flush implements http.Flusher for SSE compatibility.
func (w *metricsResponseWriter) Flush() {
	if flusher, ok := w.ResponseWriter.(http.Flusher); ok {
		flusher.Flush()
	}
}

// Hijack implements http.Hijacker for WebSocket compatibility.
func (w *metricsResponseWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	if hijacker, ok := w.ResponseWriter.(http.Hijacker); ok {
		return hijacker.Hijack()
	}
	return nil, nil, fmt.Errorf("underlying ResponseWriter does not implement http.Hijacker")
}
