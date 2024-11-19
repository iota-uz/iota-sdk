package middleware

import (
	"context"
	"github.com/iota-agency/iota-sdk/pkg/composables"
	"log"
	"net/http"
	"time"

	"github.com/gorilla/mux"
	"github.com/iota-agency/iota-sdk/pkg/constants"
	"github.com/rs/cors"
	"gorm.io/gorm"
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

func WithLogger(logger *log.Logger) mux.MiddlewareFunc {
	return ContextKeyValue(
		constants.LoggerKey, func(*http.Request, http.ResponseWriter) interface{} {
			return logger
		},
	)
}

func LogRequests() mux.MiddlewareFunc {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(
			func(w http.ResponseWriter, r *http.Request) {
				start := time.Now()
				logger, ok := composables.UseLogger(r.Context())
				if !ok {
					panic("logger not found. Add WithLogger middleware up the chain")
				}
				next.ServeHTTP(w, r)
				logger.Printf("%s %s %s", r.Method, r.RequestURI, time.Since(start))
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

func Transactions(db *gorm.DB) mux.MiddlewareFunc {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(
			func(w http.ResponseWriter, r *http.Request) {
				err := db.Transaction(
					func(tx *gorm.DB) error { //nolint:contextcheck
						ctx := context.WithValue(r.Context(), constants.TxKey, tx)
						next.ServeHTTP(w, r.WithContext(ctx))
						return nil
					},
				)
				if err != nil {
					http.Error(w, err.Error(), http.StatusInternalServerError)
				}
			},
		)
	}
}
