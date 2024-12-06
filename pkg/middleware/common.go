package middleware

import (
	"context"
	"github.com/google/uuid"
	"github.com/iota-agency/iota-sdk/pkg/composables"
	"github.com/sirupsen/logrus"
	"log"
	"net/http"
	"time"

	"github.com/gorilla/mux"
	"github.com/iota-agency/iota-sdk/pkg/constants"
	"github.com/rs/cors"
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
	ParamsConstructor  func(r *http.Request, w http.ResponseWriter) *composables.Params
)

func DefaultParamsConstructor(r *http.Request, w http.ResponseWriter) *composables.Params {
	return &composables.Params{
		//nolint:exhaustruct
		Request:   r,
		Writer:    w,
		IP:        r.RemoteAddr,
		UserAgent: r.UserAgent(),
	}
}

func WithLogger(logger *logrus.Logger) mux.MiddlewareFunc {
	return ContextKeyValue(
		constants.LoggerKey,
		func(r *http.Request, w http.ResponseWriter) interface{} {
			return logger.WithFields(logrus.Fields{
				"timestamp":  time.Now().Format(time.RFC3339),
				"path":       r.RequestURI,
				"method":     r.Method,
				"host":       r.Host,
				"ip":         r.RemoteAddr,
				"user-agent": r.UserAgent(),
				"request-id": uuid.New().String(),
			})
		},
	)
}

func LogRequests() mux.MiddlewareFunc {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(
			func(w http.ResponseWriter, r *http.Request) {
				start := time.Now()
				next.ServeHTTP(w, r)
				log.Printf("%s %s %s", r.Method, r.RequestURI, time.Since(start))
				logger, err := composables.UseLogger(r.Context())
				if err != nil {
					log.Printf("logger not found: %v", err)
				}
				logger.WithField("duration", time.Since(start)).Info("request completed")
			},
		)
	}
}

func Cors(allowOrigins []string) mux.MiddlewareFunc {
	return cors.New(
		cors.Options{
			//nolint:exhaustruct
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

func RequestParams(constructor ParamsConstructor) mux.MiddlewareFunc {
	return ContextKeyValue(
		constants.ParamsKey, func(r *http.Request, w http.ResponseWriter) interface{} {
			return constructor(r, w)
		},
	)
}
