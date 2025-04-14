package middleware

import (
	"context"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"github.com/rs/cors"
	"github.com/sirupsen/logrus"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"

	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/configuration"
	"github.com/iota-uz/iota-sdk/pkg/constants"
)

var AllowMethods = []string{
	http.MethodConnect,
	http.MethodOptions,
	http.MethodGet,
	http.MethodPost,
	http.MethodHead,
	http.MethodPatch,
	http.MethodPut,
	http.MethodDelete,
}

type (
	GenericConstructor func(r *http.Request, w http.ResponseWriter) interface{}
)

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

func getHeaders(r *http.Request) map[string]string {
	headers := make(map[string]string)
	for key, values := range r.Header {
		if len(values) > 0 {
			headers[key] = values[0]
		}
	}
	return headers
}

func WithLogger(logger *logrus.Logger) mux.MiddlewareFunc {
	conf := configuration.Use()
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(
			func(w http.ResponseWriter, r *http.Request) {
				start := time.Now()
				requestID := getRequestID(r, conf)

				logFields := logrus.Fields{
					"timestamp":  start.UnixNano(),
					"path":       r.RequestURI,
					"method":     r.Method,
					"host":       r.Host,
					"ip":         getRealIP(r, conf),
					"user-agent": r.UserAgent(),
					"request-id": requestID,
					"headers":    getHeaders(r),
				}

				fieldsLogger := logger.WithFields(logFields)

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

				next.ServeHTTP(w, r.WithContext(ctx))

				duration := time.Since(start)
				fieldsLogger.WithFields(logrus.Fields{
					"duration":  duration,
					"completed": true,
				}).Info("request completed")

				span.SetAttributes(attribute.Int64("http.request_duration_ms", duration.Milliseconds()))
			},
		)
	}
}

func Cors(allowOrigins ...string) mux.MiddlewareFunc {
	return cors.New(
		cors.Options{
			AllowedOrigins:   allowOrigins,
			AllowedMethods:   AllowMethods,
			AllowCredentials: true,
		},
	).Handler
}

func ContextKeyValue(key interface{}, constructor GenericConstructor) mux.MiddlewareFunc {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(
			func(w http.ResponseWriter, r *http.Request) {
				ctx := context.WithValue(r.Context(), key, constructor(r, w))
				next.ServeHTTP(w, r.WithContext(ctx))
			},
		)
	}
}

func RequestParams() mux.MiddlewareFunc {
	return ContextKeyValue(
		constants.ParamsKey, func(r *http.Request, w http.ResponseWriter) interface{} {
			return &composables.Params{
				Request:   r,
				Writer:    w,
				IP:        r.RemoteAddr,
				UserAgent: r.UserAgent(),
			}
		},
	)
}
