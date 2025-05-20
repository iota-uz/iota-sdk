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
	"github.com/sirupsen/logrus"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"

	"github.com/iota-uz/iota-sdk/pkg/configuration"
	"github.com/iota-uz/iota-sdk/pkg/constants"
)

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
	return NewLoggerOptions(true, true, 512)
}

type statusResponseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (w *statusResponseWriter) WriteHeader(code int) {
	w.statusCode = code
	w.ResponseWriter.WriteHeader(code)
}

// Status returns the HTTP status code
func (w *statusResponseWriter) Status() int {
	if w.statusCode == 0 {
		return http.StatusOK
	}
	return w.statusCode
}

func (w *statusResponseWriter) Flush() {
	if flusher, ok := w.ResponseWriter.(http.Flusher); ok {
		flusher.Flush()
	}
}

func (w *statusResponseWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	if hijacker, ok := w.ResponseWriter.(http.Hijacker); ok {
		return hijacker.Hijack()
	}
	return nil, nil, fmt.Errorf("underlying ResponseWriter does not implement http.Hijacker")
}

func wrapResponseWriter(w http.ResponseWriter) *statusResponseWriter {
	return &statusResponseWriter{ResponseWriter: w, statusCode: 0}
}

func getRealIP(r *http.Request, conf *configuration.Configuration) string {
	if len(r.Header.Get(conf.RealIPHeader)) > 0 {
		return r.Header.Get(conf.RealIPHeader)
	}
	return r.RemoteAddr
}

func getRequestID(r *http.Request, conf *configuration.Configuration) string {
	if len(r.Header.Get(conf.RequestIDHeader)) > 0 {
		return r.Header.Get(conf.RequestIDHeader)
	}
	return uuid.New().String()
}

var tracer = otel.Tracer("iota-sdk-middleware")

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
			headers[key] = values[0]
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
		strings.Contains(contentType, "application/xml") ||
		strings.Contains(contentType, "text/xml")
}

func WithLogger(logger *logrus.Logger, opts LoggerOptions) mux.MiddlewareFunc {
	conf := configuration.Use()
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(
			func(w http.ResponseWriter, r *http.Request) {
				start := time.Now()
				requestID := getRequestID(r, conf)

				fieldsLogger := logger.WithFields(logrus.Fields{
					"request-id": requestID,
					"path":       r.RequestURI,
					"method":     r.Method,
				})

				fieldsLogger.WithFields(logrus.Fields{
					"timestamp":       start.UnixNano(),
					"host":            r.Host,
					"ip":              getRealIP(r, conf),
					"user-agent":      r.UserAgent(),
					"request-headers": formatHeaders(r.Header),
				}).Info("request started")

				reqContentType := r.Header.Get("Content-Type")
				logReqBody := opts.LogRequestBody && shouldLogBody(reqContentType)
				if logReqBody && r.Body != nil {
					bodyBuf := new(bytes.Buffer)
					if _, err := io.Copy(bodyBuf, r.Body); err != nil {
						fieldsLogger.WithError(err).Error("failed to read request-body")
						http.Error(w, "failed to read request-body", http.StatusInternalServerError)
						return
					}
					r.Body = io.NopCloser(bytes.NewBuffer(bodyBuf.Bytes()))
					switch {
					case strings.Contains(reqContentType, "application/json"):
						var jsonRequestBody interface{}
						if err := json.Unmarshal(bodyBuf.Bytes(), &jsonRequestBody); err != nil {
							fieldsLogger.WithError(err).Error("failed to parse JSON request-body")
							http.Error(w, "failed to parse JSON request-body", http.StatusBadRequest)
							return
						}
						fieldsLogger.WithField("request-body", jsonRequestBody).Info("JSON request-body parsed")
					case strings.Contains(reqContentType, "application/x-www-form-urlencoded"):
						if err := r.ParseForm(); err != nil {
							fieldsLogger.WithError(err).Error("failed to parse form-urlencoded request-body")
							http.Error(w, "failed to parse form-urlencoded request-body", http.StatusBadRequest)
							return
						}
						fieldsLogger.WithField("request-body", formatFormValues(r.Form)).Info("form-urlencoded request-body parsed")
					case strings.Contains(reqContentType, "application/xml"), strings.Contains(reqContentType, "text/xml"):
						var xmlRequestBody interface{}
						if err := xml.Unmarshal(bodyBuf.Bytes(), &xmlRequestBody); err != nil {
							fieldsLogger.WithError(err).Error("failed to parse XML request-body")
							http.Error(w, "failed to parse XML request-body", http.StatusBadRequest)
							return
						}
						fieldsLogger.WithField("request-body", xmlRequestBody).Info("XML request-body parsed")
					default:
						rawBody := bodyBuf.String()
						fieldsLogger.WithField("request-body", rawBody).Info("request-body parsed")
					}
				}

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
				defer span.End()

				ctx = context.WithValue(ctx, constants.LoggerKey, fieldsLogger)
				ctx = context.WithValue(ctx, constants.RequestStart, start)

				propagator.Inject(ctx, propagation.HeaderCarrier(w.Header()))

				if spanContext := span.SpanContext(); spanContext.HasTraceID() {
					traceID := spanContext.TraceID().String()
					spanID := spanContext.SpanID().String()

					w.Header().Set("X-Trace-Id", traceID)
					w.Header().Set("X-Span-Id", spanID)

					fieldsLogger = fieldsLogger.WithFields(logrus.Fields{
						"trace-id": traceID,
						"span-id":  spanID,
					})
				}

				w.Header().Set("X-Request-Id", requestID)

				wrappedWriter := wrapResponseWriter(w)
				next.ServeHTTP(wrappedWriter, r.WithContext(ctx))

				// Log the status code
				statusCode := wrappedWriter.Status()
				duration := time.Since(start)
				fieldsLogger.WithFields(logrus.Fields{
					"duration":         duration,
					"completed":        true,
					"status-code":      statusCode,
					"status-class":     statusCode / 100,
					"response-headers": formatHeaders(wrappedWriter.Header()),
				}).Info("request completed")

				span.SetAttributes(
					attribute.Int64("http.request_duration_ms", duration.Milliseconds()),
					attribute.Int("http.status_code", statusCode),
				)

				respContentType := wrappedWriter.Header().Get("Content-Type")
				logRespBody := opts.LogResponseBody && shouldLogBody(respContentType)
				if logRespBody {
					bodyBuf := new(bytes.Buffer)
					wrappedWriter.Flush()

					// Check if the underlying ResponseWriter implements io.Reader
					if reader, ok := wrappedWriter.ResponseWriter.(io.Reader); ok {
						if _, err := io.Copy(bodyBuf, reader); err != nil {
							fieldsLogger.WithError(err).Error("failed to read response-body")
							return
						}
						switch {
						case strings.Contains(respContentType, "application/json"):
							var jsonResponseBody interface{}
							if err := json.Unmarshal(bodyBuf.Bytes(), &jsonResponseBody); err != nil {
								fieldsLogger.WithError(err).Error("failed to parse JSON response-body")
								return
							}
							fieldsLogger.WithField("response-body", jsonResponseBody).Info("JSON response-body parsed")
						case strings.Contains(respContentType, "application/xml"), strings.Contains(respContentType, "text/xml"):
							var xmlResponseBody interface{}
							if err := xml.Unmarshal(bodyBuf.Bytes(), &xmlResponseBody); err != nil {
								fieldsLogger.WithError(err).Error("failed to parse XML response-body")
								return
							}
							fieldsLogger.WithField("response-body", xmlResponseBody).Info("XML response-body parsed")
						default:
							rawBody := bodyBuf.String()
							fieldsLogger.WithField("response-body", rawBody).Info("response-body parsed")
						}
					} else {
						fieldsLogger.Error("underlying ResponseWriter does not implement io.Reader")
					}
				}
			},
		)
	}
}
