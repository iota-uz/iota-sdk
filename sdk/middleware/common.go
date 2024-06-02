package middleware

import (
	"context"
	"github.com/gorilla/mux"
	"gorm.io/gorm"
	"log"
	"net/http"
	"time"
)

type RequestParams[U, S any] struct {
	Ip            string
	UserAgent     string
	Writer        http.ResponseWriter
	Authenticated bool
	User          *U
	Session       *S
	Tx            *gorm.DB
}

func LoggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		log.Printf("%s %s %s", r.Method, r.RequestURI, time.Since(start))
	})
}

type AuthService[U, S any] interface {
	Authorize(token string) (*U, *S, error)
}

func AuthMiddleware[U any, S any](db *gorm.DB, authService AuthService[U, S]) mux.MiddlewareFunc {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			err := db.Transaction(func(tx *gorm.DB) error {
				params := &RequestParams[U, S]{
					Ip:            r.RemoteAddr,
					UserAgent:     r.UserAgent(),
					Writer:        w,
					Authenticated: false,
					Tx:            tx,
				}
				ctx := context.WithValue(r.Context(), "params", params)
				token, err := r.Cookie("token")
				if err != nil {
					next.ServeHTTP(w, r.WithContext(ctx))
					return nil
				}
				user, session, err := authService.Authorize(token.Value)
				if err != nil {
					next.ServeHTTP(w, r.WithContext(ctx))
					return nil
				}
				params.Authenticated = true
				params.User = user
				params.Session = session
				next.ServeHTTP(w, r.WithContext(ctx))
				return nil
			})
			if err != nil {
				log.Println(err)
			}
		})
	}
}
