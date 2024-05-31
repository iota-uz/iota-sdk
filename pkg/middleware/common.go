package middleware

import (
	"context"
	"github.com/gorilla/mux"
	"github.com/iota-agency/iota-erp/models"
	"github.com/iota-agency/iota-erp/pkg/authentication"
	"log"
	"net/http"
	"time"
)

type RequestParams struct {
	Ip            string
	UserAgent     string
	Writer        http.ResponseWriter
	Authenticated bool
	User          *models.User
	Token         string
}

func LoggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		log.Printf("%s %s %s", r.Method, r.RequestURI, time.Since(start))
	})
}

func AuthMiddleware(authService *authentication.Service) mux.MiddlewareFunc {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			params := &RequestParams{
				Ip:            r.RemoteAddr,
				UserAgent:     r.UserAgent(),
				Writer:        w,
				Authenticated: false,
			}
			ctx := context.WithValue(r.Context(), "params", params)
			token, err := r.Cookie("token")
			if err != nil {
				next.ServeHTTP(w, r.WithContext(ctx))
				return
			}
			user, err := authService.Authorize(token.Value)
			if err != nil {
				next.ServeHTTP(w, r.WithContext(ctx))
				return
			}
			params.Authenticated = true
			params.User = user
			params.Token = token.Value
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
