package middleware

import (
	"context"
	"github.com/iota-agency/iota-erp/internal/domain/aggregates/user"
	"github.com/iota-agency/iota-erp/pkg/composables"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/iota-agency/iota-erp/internal/configuration"
	"github.com/iota-agency/iota-erp/internal/domain/entities/session"
	"github.com/iota-agency/iota-erp/pkg/constants"
)

type AuthService interface {
	Authorize(ctx context.Context, token string) (*user.User, *session.Session, error)
}

func Authorization(authService AuthService) mux.MiddlewareFunc {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(
			func(w http.ResponseWriter, r *http.Request) {
				conf := configuration.Use()
				token, err := r.Cookie(conf.SidCookieKey)
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
				ctx = context.WithValue(ctx, constants.UserKey, u)
				next.ServeHTTP(w, r.WithContext(context.WithValue(ctx, constants.SessionKey, sess)))
			},
		)
	}
}
