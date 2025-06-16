package testutils

import (
	"context"
	"net/http"
	"net/url"

	"github.com/a-h/templ"
	"github.com/gorilla/mux"
	i18n "github.com/iota-uz/go-i18n/v2/i18n"
	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/user"
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/session"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/constants"
	"github.com/iota-uz/iota-sdk/pkg/types"
	"github.com/sirupsen/logrus"
	"golang.org/x/text/language"
)

// TestMiddleware creates middleware that adds all required context values for controller tests
func TestMiddleware(env *TestEnv, user user.User) mux.MiddlewareFunc {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()

			// Add composables
			ctx = composables.WithUser(ctx, user)
			ctx = composables.WithPool(ctx, env.Pool)
			ctx = composables.WithTx(ctx, env.Tx)
			ctx = composables.WithTenantID(ctx, env.Tenant.ID)
			ctx = composables.WithSession(ctx, &session.Session{})
			ctx = composables.WithParams(ctx, &composables.Params{
				IP:            "127.0.0.1",
				UserAgent:     "test-agent",
				Authenticated: true,
				Request:       r,
				Writer:        w,
			})

			// Add app constants
			ctx = context.WithValue(ctx, constants.AppKey, env.App)
			ctx = context.WithValue(ctx, constants.HeadKey, templ.NopComponent)
			ctx = context.WithValue(ctx, constants.LogoKey, templ.NopComponent)
			ctx = context.WithValue(ctx, constants.LoggerKey, logrus.WithField("test", true))

			// Add localization
			localizer := i18n.NewLocalizer(env.App.Bundle(), "en")
			parsedURL, _ := url.Parse(r.URL.Path)
			ctx = composables.WithPageCtx(ctx, &types.PageContext{
				Locale:    language.English,
				URL:       parsedURL,
				Localizer: localizer,
			})

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
