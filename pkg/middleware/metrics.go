package middleware

import (
	"bufio"
	"crypto/subtle"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/mux"
	"github.com/iota-uz/iota-sdk/pkg/application"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// MetricsOptions configures the Prometheus metrics middleware.
type MetricsOptions struct {
	AuthToken   string                // required bearer token for /metrics endpoint
	Pool        *pgxpool.Pool         // optional — registers pgxpool connection-pool collector
	Hub         application.Huber     // optional — registers websocket_connections_active gauge
	Registry    prometheus.Registerer // optional — custom registerer (defaults to a new registry that also collects Go/process metrics)
	Gatherer    prometheus.Gatherer   // optional — custom gatherer (must match Registry if provided)
	ConstLabels prometheus.Labels     // optional — applied to every exported metric for service scoping
}

// Metrics holds Prometheus collectors and the metrics HTTP handler.
type Metrics struct {
	requestDuration  *prometheus.HistogramVec
	requestsTotal    *prometheus.CounterVec
	requestsInFlight prometheus.Gauge
	metricsHandler   http.Handler
	authToken        string
}

// NewMetrics creates a new Metrics instance and registers collectors.
// It panics if opts.AuthToken is empty.
func NewMetrics(opts MetricsOptions) *Metrics {
	if opts.AuthToken == "" {
		panic("MetricsOptions.AuthToken must be set")
	}

	registry := opts.Registry
	gatherer := opts.Gatherer
	switch {
	case registry == nil && gatherer == nil:
		r := prometheus.NewRegistry()
		// Include default Go runtime and process metrics.
		r.MustRegister(collectors.NewGoCollector())
		r.MustRegister(collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}))
		registry = r
		gatherer = r
	case registry != nil && gatherer == nil:
		g, ok := registry.(prometheus.Gatherer)
		if !ok {
			panic("MetricsOptions.Gatherer must be set when Registry does not implement prometheus.Gatherer")
		}
		gatherer = g
	case registry == nil && gatherer != nil:
		panic("MetricsOptions.Registry must be set when Gatherer is provided")
	}

	requestDuration := prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:        "http_request_duration_seconds",
			Help:        "Duration of HTTP requests in seconds.",
			Buckets:     []float64{0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10},
			ConstLabels: cloneLabels(opts.ConstLabels),
		},
		[]string{"method", "route", "status_code"},
	)

	requestsTotal := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name:        "http_requests_total",
			Help:        "Total number of HTTP requests.",
			ConstLabels: cloneLabels(opts.ConstLabels),
		},
		[]string{"method", "route", "status_code"},
	)

	requestsInFlight := prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name:        "http_requests_in_flight",
			Help:        "Number of HTTP requests currently being served.",
			ConstLabels: cloneLabels(opts.ConstLabels),
		},
	)

	registry.MustRegister(
		requestDuration,
		requestsTotal,
		requestsInFlight,
	)

	if opts.Pool != nil {
		registry.MustRegister(NewPgxPoolCollector(opts.Pool, opts.ConstLabels))
	}

	if opts.Hub != nil {
		registry.MustRegister(prometheus.NewGaugeFunc(
			prometheus.GaugeOpts{
				Name:        "websocket_connections_active",
				Help:        "Number of currently active WebSocket connections.",
				ConstLabels: cloneLabels(opts.ConstLabels),
			},
			func() float64 { return float64(opts.Hub.ConnectionCount()) },
		))
	}

	metricsHandler := promhttp.HandlerFor(gatherer, promhttp.HandlerOpts{})

	return &Metrics{
		requestDuration:  requestDuration,
		requestsTotal:    requestsTotal,
		requestsInFlight: requestsInFlight,
		metricsHandler:   metricsHandler,
		authToken:        opts.AuthToken,
	}
}

func cloneLabels(labels prometheus.Labels) prometheus.Labels {
	if len(labels) == 0 {
		return nil
	}

	cloned := make(prometheus.Labels, len(labels))
	for key, value := range labels {
		cloned[key] = value
	}

	return cloned
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

			m.requestDuration.WithLabelValues(method, route, statusCode).Observe(duration)
			m.requestsTotal.WithLabelValues(method, route, statusCode).Inc()
		})
	}
}

// serveMetrics handles the /metrics endpoint with bearer token authentication.
func (m *Metrics) serveMetrics(w http.ResponseWriter, r *http.Request) {
	authHeader := r.Header.Get("Authorization")
	const prefix = "Bearer "
	if !strings.HasPrefix(authHeader, prefix) ||
		subtle.ConstantTimeCompare([]byte(authHeader[len(prefix):]), []byte(m.authToken)) != 1 {
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
