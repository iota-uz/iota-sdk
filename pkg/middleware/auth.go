package middleware

import (
	"context"
	"github.com/gorilla/mux"
	"github.com/iota-agency/iota-erp/internal/domain/session"
	"github.com/iota-agency/iota-erp/internal/domain/user"
	localComposables "github.com/iota-agency/iota-erp/pkg/composables"
	"github.com/iota-agency/iota-erp/sdk/composables"
	"net/http"
)

type AuthService interface {
	Authorize(ctx context.Context, token string) (*user.User, *session.Session, error)
}

func PermissionCheck() mux.MiddlewareFunc {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, ok := localComposables.UseSession(r.Context())
			if !ok {
				next.ServeHTTP(w, r)
				return
			}
		})
	}
}

func Authorization(authService AuthService) mux.MiddlewareFunc {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			token, err := r.Cookie("token")
			if err != nil {
				next.ServeHTTP(w, r)
				return
			}
			ctx := r.Context()
			u, sess, err := authService.Authorize(ctx, token.Value)
			if err != nil {
				next.ServeHTTP(w, r)
				return
			}
			params, ok := composables.UseParams(ctx)
			if !ok {
				panic("params not found. Add RequestParams middleware up the chain")
			}
			params.Authenticated = true
			ctx = context.WithValue(ctx, "user", u)
			next.ServeHTTP(w, r.WithContext(context.WithValue(ctx, "session", sess)))
		})
	}
}
