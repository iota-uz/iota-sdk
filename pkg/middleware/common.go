package middleware

import (
	"context"
	"github.com/gorilla/mux"
	"github.com/iota-agency/iota-erp/models"
	"github.com/iota-agency/iota-erp/pkg/authentication"
	"gorm.io/gorm"
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
	Tx            *gorm.DB
}

func LoggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		log.Printf("%s %s %s", r.Method, r.RequestURI, time.Since(start))
	})
}

func AuthMiddleware(db *gorm.DB, authService *authentication.Service) mux.MiddlewareFunc {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			err := db.Transaction(func(tx *gorm.DB) error {
				params := &RequestParams{
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
				user, err := authService.Authorize(token.Value)
				if err != nil {
					next.ServeHTTP(w, r.WithContext(ctx))
					return nil
				}
				params.Authenticated = true
				params.User = user
				params.Token = token.Value
				next.ServeHTTP(w, r.WithContext(ctx))
				return nil
			})
			if err != nil {
				log.Println(err)
			}
		})
	}
}
