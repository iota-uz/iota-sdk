package middleware

import (
	"context"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"github.com/rs/cors"
	"github.com/sirupsen/logrus"

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
				logFields := logrus.Fields{
					"timestamp":  start.UnixNano(),
					"path":       r.RequestURI,
					"method":     r.Method,
					"host":       r.Host,
					"ip":         getRealIP(r, conf),
					"user-agent": r.UserAgent(),
					"request-id": getRequestID(r, conf),
					"headers":    getHeaders(r),
				}

				fieldsLogger := logger.WithFields(logFields)

				ctx := context.WithValue(r.Context(), constants.LoggerKey, fieldsLogger)
				ctx = context.WithValue(ctx, constants.RequestStart, start)
				next.ServeHTTP(w, r.WithContext(ctx))

				fieldsLogger.WithField("duration", time.Since(start)).Info("request completed")
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

