package middleware

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"github.com/iota-uz/iota-sdk/pkg/configuration"
	"github.com/iota-uz/iota-sdk/pkg/constants"
	"github.com/sirupsen/logrus"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
)

var tracer = otel.Tracer("iota-sdk-middleware")

type LoggerOptions struct {
	LogRequestBody  bool
	LogResponseBody bool
	MaxBodyLength   int
}

func NewLoggerOptions(logRequestBody bool, logResponseBody bool, maxBodyLength int) LoggerOptions {
	return LoggerOptions{
		LogRequestBody:  logRequestBody,
		LogResponseBody: logResponseBody,
		MaxBodyLength:   maxBodyLength,
	}
}

func DefaultLoggerOptions() LoggerOptions {
	return NewLoggerOptions(true, true, 1024)
}

type loggingResponseWriter struct {
	http.ResponseWriter
	statusCode    int
	body          *bytes.Buffer
	maxBodyLength int
	headerWritten bool
}

func newLoggingResponseWriter(w http.ResponseWriter, maxBodyLength int) *loggingResponseWriter {
	return &loggingResponseWriter{
		ResponseWriter: w,
		statusCode:     http.StatusOK,
		body:           new(bytes.Buffer),
		maxBodyLength:  maxBodyLength,
	}
}

func (w *loggingResponseWriter) WriteHeader(code int) {
	if w.headerWritten {
	}
	w.statusCode = code
	w.ResponseWriter.WriteHeader(code)
	w.headerWritten = true
}

func (w *loggingResponseWriter) Write(b []byte) (int, error) {
	if !w.headerWritten {
		w.WriteHeader(http.StatusOK)
	}
	if w.body.Len() < w.maxBodyLength {
		remainingSpace := w.maxBodyLength - w.body.Len()
		if len(b) > remainingSpace {
			w.body.Write(b[:remainingSpace])
		} else {
			w.body.Write(b)
		}
	}
	return w.ResponseWriter.Write(b)
}

func (w *loggingResponseWriter) Status() int {
	return w.statusCode
}

func (w *loggingResponseWriter) Body() []byte {
	return w.body.Bytes()
}

func (w *loggingResponseWriter) Flush() {
	if flusher, ok := w.ResponseWriter.(http.Flusher); ok {
		flusher.Flush()
	}
}

func (w *loggingResponseWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	if hijacker, ok := w.ResponseWriter.(http.Hijacker); ok {
		return hijacker.Hijack()
	}
	return nil, nil, fmt.Errorf("underlying ResponseWriter does not implement http.Hijacker")
}

func getRealIP(r *http.Request, conf *configuration.Configuration) string {
	if len(r.Header.Get(conf.RealIPHeader)) > 0 {
		return r.Header.Get(conf.RealIPHeader)
	}
	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return ip
}

func getRequestID(r *http.Request, conf *configuration.Configuration) string {
	if len(r.Header.Get(conf.RequestIDHeader)) > 0 {
		return r.Header.Get(conf.RequestIDHeader)
	}
	return uuid.New().String()
}

func TracedMiddleware(name string) mux.MiddlewareFunc {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			propagator := propagation.TraceContext{}
			ctx := propagator.Extract(r.Context(), propagation.HeaderCarrier(r.Header))

			ctx, span := tracer.Start(
				ctx,
				"middleware."+name,
				trace.WithAttributes(
					attribute.String("middleware.name", name),
					attribute.String("http.method", r.Method),
					attribute.String("http.url", r.URL.String()),
					attribute.String("http.host", r.Host),
				),
			)
			defer span.End()

			propagator.Inject(ctx, propagation.HeaderCarrier(r.Header))

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func formatHeaders(h http.Header) map[string]string {
	headers := make(map[string]string)
	for key, values := range h {
		if len(values) > 0 {
			headers[key] = strings.Join(values, ", ")
		}
	}
	return headers
}

func formatFormValues(f url.Values) map[string]string {
	formValues := make(map[string]string)
	for key, values := range f {
		formValues[key] = strings.Join(values, ",")
	}
	return formValues
}

func shouldLogBody(contentType string) bool {
	contentType = strings.ToLower(contentType)
	return strings.Contains(contentType, "application/json") ||
		strings.Contains(contentType, "application/x-www-form-urlencoded") ||
		strings.Contains(contentType, "application/xml")
}

func logRequestStart(logger *logrus.Entry, r *http.Request, conf *configuration.Configuration, start time.Time) {
	logger.WithFields(logrus.Fields{
		"timestamp":       start.UnixNano(),
		"host":            r.Host,
		"ip":              getRealIP(r, conf),
		"user-agent":      r.UserAgent(),
		"request-headers": formatHeaders(r.Header),
	}).Info("request started")
}

func processAndLogRequestBody(r *http.Request, opts LoggerOptions, logger *logrus.Entry, w http.ResponseWriter) (*http.Request, bool) {
	reqContentType := r.Header.Get("Content-Type")
	if !opts.LogRequestBody || !shouldLogBody(reqContentType) || r.Body == nil || r.Body == http.NoBody {
		return r, false
	}

	bodyBytes, err := io.ReadAll(io.LimitReader(r.Body, int64(opts.MaxBodyLength+1)))
	if err != nil {
		logger.WithError(err).Error("failed to read request-body")
		http.Error(w, "failed to read request-body", http.StatusInternalServerError)
		return r, true
	}
	r.Body.Close()

	r.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))

	actualBodyLen := len(bodyBytes)
	truncated := false
	if actualBodyLen > opts.MaxBodyLength {
		bodyBytes = bodyBytes[:opts.MaxBodyLength]
		truncated = true
	}

	if earlyExit := parseAndLogBodyContent(bodyBytes, reqContentType, "request-body", truncated, logger, w); earlyExit {
		return r, true
	}
	return r, false
}

func parseAndLogBodyContent(bodyBytes []byte, contentType, bodyTypeKey string, truncated bool, logger *logrus.Entry, w http.ResponseWriter) (earlyExit bool) {
	logFields := logrus.Fields{}
	if truncated {
		logFields[bodyTypeKey+"-truncated"] = true
	}

	switch {
	case strings.Contains(contentType, "application/json"):
		var jsonData interface{}
		if err := json.Unmarshal(bodyBytes, &jsonData); err != nil {
			logger.WithError(err).WithFields(logFields).Error("failed to parse JSON " + bodyTypeKey)
			if w != nil {
				http.Error(w, "failed to parse JSON "+bodyTypeKey, http.StatusBadRequest)
				return true
			}
			return false
		}
		logger.WithField(bodyTypeKey, jsonData).
			WithFields(logFields).
			Info("JSON " + bodyTypeKey + " content")
	case strings.Contains(contentType, "application/x-www-form-urlencoded"):
		logger.WithField(bodyTypeKey, string(bodyBytes)).
			WithFields(logFields).
			Info("form-urlencoded " + bodyTypeKey + " content (raw)")
	case strings.Contains(contentType, "application/xml"), strings.Contains(contentType, "text/xml"):
		var xmlData interface{}
		if err := xml.Unmarshal(bodyBytes, &xmlData); err != nil {
			logger.WithError(err).
				WithFields(logFields).
				WithField(bodyTypeKey+"-raw", string(bodyBytes)).
				Error("failed to parse XML " + bodyTypeKey + ", logging raw")
			if w != nil {
				http.Error(w, "failed to parse XML "+bodyTypeKey, http.StatusBadRequest)
				return true
			}
			return false
		}
		logger.WithField(bodyTypeKey, xmlData).
			WithFields(logFields).
			Info("XML " + bodyTypeKey + " content")
	default:
		logger.WithField(bodyTypeKey, string(bodyBytes)).
			WithFields(logFields).
			Info(bodyTypeKey + " content (raw)")
	}
	return false
}

func setupOpenTelemetrySpan(r *http.Request, requestID string, conf *configuration.Configuration) (context.Context, trace.Span) {
	propagator := propagation.TraceContext{}
	ctx := propagator.Extract(r.Context(), propagation.HeaderCarrier(r.Header))

	ctx, span := tracer.Start(
		ctx,
		"http.request",
		trace.WithAttributes(
			attribute.String("http.method", r.Method),
			attribute.String("http.url", r.URL.String()),
			attribute.String("http.route", r.URL.Path),
			attribute.String("http.user_agent", r.UserAgent()),
			attribute.String("http.request_id", requestID),
			attribute.String("net.host.name", r.Host),
			attribute.String("net.peer.ip", getRealIP(r, conf)),
		),
	)
	return ctx, span
}

func enrichContextAndPrepareResponseHeaders(ctx context.Context, w http.ResponseWriter, span trace.Span, logger *logrus.Entry, requestID string, start time.Time) (context.Context, *logrus.Entry) {
	ctx = context.WithValue(ctx, constants.LoggerKey, logger)
	ctx = context.WithValue(ctx, constants.RequestStart, start)

	propagator := propagation.TraceContext{}
	propagator.Inject(ctx, propagation.HeaderCarrier(w.Header()))

	if spanContext := span.SpanContext(); spanContext.IsValid() && spanContext.HasTraceID() {
		traceID := spanContext.TraceID().String()
		spanID := spanContext.SpanID().String()

		w.Header().Set("X-Trace-Id", traceID)
		w.Header().Set("X-Span-Id", spanID)

		logger = logger.WithFields(logrus.Fields{
			"trace-id": traceID,
			"span-id":  spanID,
		})
	}
	w.Header().Set("X-Request-Id", requestID)
	return ctx, logger
}

func logRequestCompletion(lrw *loggingResponseWriter, start time.Time, logger *logrus.Entry, span trace.Span) {
	duration := time.Since(start)
	statusCode := lrw.Status()

	logger.WithFields(logrus.Fields{
		"duration":         duration.String(),
		"duration_ms":      duration.Milliseconds(),
		"completed":        true,
		"status-code":      statusCode,
		"status-class":     fmt.Sprintf("%dxx", statusCode/100),
		"response-headers": formatHeaders(lrw.Header()),
	}).Info("request completed")

	span.SetAttributes(
		attribute.Int64("http.response_duration_ms", duration.Milliseconds()),
		attribute.Int("http.status_code", statusCode),
	)
}

func logResponseBody(lrw *loggingResponseWriter, opts LoggerOptions, logger *logrus.Entry) {
	respContentType := lrw.Header().Get("Content-Type")
	if !opts.LogResponseBody || !shouldLogBody(respContentType) {
		return
	}

	bodyBytes := lrw.Body()
	if len(bodyBytes) == 0 {
		logger.Info("empty response-body or not captured")
		return
	}

	truncated := len(bodyBytes) == opts.MaxBodyLength && lrw.body.Len() == opts.MaxBodyLength
	parseAndLogBodyContent(bodyBytes, respContentType, "response-body", truncated, logger, nil)
}

func WithLogger(logger *logrus.Logger, opts LoggerOptions) mux.MiddlewareFunc {
	conf := configuration.Use()
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			requestID := getRequestID(r, conf)

			baseLogger := logger.WithFields(logrus.Fields{
				"request-id": requestID,
				"path":       r.RequestURI,
				"method":     r.Method,
			})

			logRequestStart(baseLogger, r, conf, start)

			var earlyExit bool
			r, earlyExit = processAndLogRequestBody(r, opts, baseLogger, w)
			if earlyExit {
				return
			}

			ctx, span := setupOpenTelemetrySpan(r, requestID, conf)
			defer span.End()

			var fieldsLogger *logrus.Entry
			ctx, fieldsLogger = enrichContextAndPrepareResponseHeaders(ctx, w, span, baseLogger, requestID, start)

			lrw := newLoggingResponseWriter(w, opts.MaxBodyLength)

			next.ServeHTTP(lrw, r.WithContext(ctx))

			logRequestCompletion(lrw, start, fieldsLogger, span)
			logResponseBody(lrw, opts, fieldsLogger)
		})
	}
}
