// Package server provides this package.
package server

import (
	"bufio"
	"errors"
	"mime"
	"net"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"github.com/NYTimes/gziphandler"
	"github.com/gorilla/mux"
	"github.com/iota-uz/iota-sdk/pkg/application"
	"github.com/sirupsen/logrus"
)

// Timeouts mirrors the net/http server timeouts. A zero or negative value
// disables the corresponding timeout.
//
// WriteTimeout does not apply to Server-Sent Events responses (Content-Type
// text/event-stream) or to hijacked websocket connections: the server clears
// the write deadline for them, because they are expected to stay open.
type Timeouts struct {
	ReadHeader time.Duration
	Read       time.Duration
	Write      time.Duration
	Idle       time.Duration
}

// DefaultTimeouts returns the timeouts used when none are configured.
func DefaultTimeouts() Timeouts {
	return Timeouts{
		ReadHeader: 10 * time.Second,
		Read:       5 * time.Minute,
		Write:      5 * time.Minute,
		Idle:       2 * time.Minute,
	}
}

func NewHTTPServer(
	app application.Application,
	notFoundHandler, methodNotAllowedHandler http.Handler,
) *HTTPServer {
	return &HTTPServer{
		Application:             app,
		NotFoundHandler:         notFoundHandler,
		MethodNotAllowedHandler: methodNotAllowedHandler,
		Timeouts:                DefaultTimeouts(),
	}
}

type HTTPServer struct {
	Application             application.Application
	NotFoundHandler         http.Handler
	MethodNotAllowedHandler http.Handler
	// Timeouts applied to the underlying http.Server by Start and Run.
	Timeouts Timeouts
	// Logger receives lifecycle messages from Run. Optional.
	Logger *logrus.Logger

	mu         sync.Mutex
	httpServer *http.Server
	draining   atomic.Bool
	// runDefaults are applied by Run before the caller's options. server.New
	// fills them from the runtime (stop hooks, shutdown configuration).
	runDefaults []RunOption
}

// Start serves on socketAddress until Shutdown is called. It does not handle
// signals; use Run for a process entry point.
func (s *HTTPServer) Start(socketAddress string) error {
	srv := s.newHTTPServer(socketAddress, nil)
	err := srv.ListenAndServe()
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
}

// Draining reports whether a graceful shutdown has begun.
func (s *HTTPServer) Draining() bool {
	return s.draining.Load()
}

func (s *HTTPServer) newHTTPServer(addr string, readinessPaths []string) *http.Server {
	srv := &http.Server{
		Addr:              addr,
		Handler:           s.serveHandler(s.handler(), readinessPaths),
		ReadHeaderTimeout: s.Timeouts.ReadHeader,
		ReadTimeout:       s.Timeouts.Read,
		WriteTimeout:      s.Timeouts.Write,
		IdleTimeout:       s.Timeouts.Idle,
	}
	s.mu.Lock()
	s.httpServer = srv
	s.mu.Unlock()
	return srv
}

func (s *HTTPServer) currentHTTPServer() *http.Server {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.httpServer
}

// serveHandler wraps the application handler with the server-level concerns:
// readiness reporting while draining and the streaming write-deadline exemption.
func (s *HTTPServer) serveHandler(next http.Handler, readinessPaths []string) http.Handler {
	ready := make(map[string]struct{}, len(readinessPaths))
	for _, p := range readinessPaths {
		ready[p] = struct{}{}
	}
	writeTimeout := s.Timeouts.Write > 0
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if s.draining.Load() {
			if _, ok := ready[r.URL.Path]; ok {
				w.Header().Set("Content-Type", "application/json")
				w.Header().Set("Connection", "close")
				w.WriteHeader(http.StatusServiceUnavailable)
				_, _ = w.Write([]byte(`{"status":"shutting_down"}` + "\n"))
				return
			}
		}
		if writeTimeout {
			w = &streamAwareWriter{ResponseWriter: w, rc: http.NewResponseController(w)}
		}
		next.ServeHTTP(w, r)
	})
}

// streamAwareWriter clears the connection write deadline once a response turns
// out to be an event stream, so WriteTimeout does not cut off SSE.
type streamAwareWriter struct {
	http.ResponseWriter
	rc      *http.ResponseController
	checked bool
}

func (w *streamAwareWriter) check() {
	if w.checked {
		return
	}
	w.checked = true
	mediaType, _, err := mime.ParseMediaType(w.Header().Get("Content-Type"))
	if err == nil && mediaType == "text/event-stream" {
		_ = w.rc.SetWriteDeadline(time.Time{})
	}
}

func (w *streamAwareWriter) WriteHeader(code int) {
	if code >= 200 {
		w.check()
	}
	w.ResponseWriter.WriteHeader(code)
}

func (w *streamAwareWriter) Write(b []byte) (int, error) {
	w.check()
	return w.ResponseWriter.Write(b)
}

func (w *streamAwareWriter) Flush() {
	w.check()
	_ = w.rc.Flush()
}

func (w *streamAwareWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	conn, rw, err := w.rc.Hijack()
	if err == nil {
		// The HTTP server's deadlines would otherwise keep applying to the
		// long-lived hijacked connection (websocket).
		_ = conn.SetDeadline(time.Time{})
	}
	return conn, rw, err
}

func (w *streamAwareWriter) Unwrap() http.ResponseWriter {
	return w.ResponseWriter
}

func (s *HTTPServer) handler() http.Handler {
	r := mux.NewRouter()
	middlewares := s.Application.Middleware()
	r.Use(middlewares...)
	for _, controller := range s.Application.Controllers() {
		controller.Register(r)
	}

	notFoundHandler := s.NotFoundHandler
	notAllowedHandler := s.MethodNotAllowedHandler
	for i := len(middlewares) - 1; i >= 0; i-- {
		notFoundHandler = middlewares[i](notFoundHandler)
		notAllowedHandler = middlewares[i](notAllowedHandler)
	}
	r.NotFoundHandler = notFoundHandler
	r.MethodNotAllowedHandler = notAllowedHandler
	return gziphandler.GzipHandler(r)
}
