package middleware

import (
	"context"
	"github.com/gorilla/mux"
	"github.com/iota-agency/iota-erp/sdk/composables"
	"gorm.io/gorm"
	"log"
	"net/http"
	"time"
)

type GenericConstructor func(r *http.Request, w http.ResponseWriter) interface{}
type ParamsConstructor func(r *http.Request, w http.ResponseWriter) *composables.Params

func DefaultParamsConstructor(r *http.Request, w http.ResponseWriter) *composables.Params {
	return &composables.Params{
		Request:   r,
		Writer:    w,
		Ip:        r.RemoteAddr,
		UserAgent: r.UserAgent(),
	}
}

func WithLogger(logger *log.Logger) mux.MiddlewareFunc {
	return ContextKeyValue("logger", func(r *http.Request, w http.ResponseWriter) interface{} {
		return logger
	})
}

func LogRequests() mux.MiddlewareFunc {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			logger, ok := composables.UseLogger(r.Context())
			if !ok {
				panic("logger not found. Add WithLogger middleware up the chain")
			}
			next.ServeHTTP(w, r)
			logger.Printf("%s %s %s", r.Method, r.RequestURI, time.Since(start))
		})
	}
}

func ContextKeyValue(key string, constructor GenericConstructor) mux.MiddlewareFunc {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := context.WithValue(r.Context(), key, constructor(r, w))
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func RequestParams(constructor ParamsConstructor) mux.MiddlewareFunc {
	return ContextKeyValue("params", func(r *http.Request, w http.ResponseWriter) interface{} {
		return constructor(r, w)
	})
}

func Transactions(db *gorm.DB) mux.MiddlewareFunc {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			err := db.Transaction(func(tx *gorm.DB) error {
				ctx := context.WithValue(r.Context(), "tx", tx)
				next.ServeHTTP(w, r.WithContext(ctx))
				return nil
			})
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
			}
		})
	}
}
