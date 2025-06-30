package middleware

import (
	"context"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/iota-uz/iota-sdk/internal/assets"
	"github.com/iota-uz/iota-sdk/modules/core/presentation/mappers"
	"github.com/iota-uz/iota-sdk/modules/core/services"
	"github.com/iota-uz/iota-sdk/pkg/application"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/constants"
)

func ProvideDynamicLogo(app application.Application) mux.MiddlewareFunc {
	tenantService := app.Service(services.TenantService{}).(*services.TenantService)
	uploadService := app.Service(services.UploadService{}).(*services.UploadService)

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()

			// Try to get user from context
			user, err := composables.UseUser(ctx)
			if err != nil {
				// If no user, provide default logo
				ctx = context.WithValue(ctx, constants.LogoKey, assets.DefaultLogo())
				next.ServeHTTP(w, r.WithContext(ctx))
				return
			}

			// Get tenant
			tenant, err := tenantService.GetByID(ctx, user.TenantID())
			if err != nil {
				// If tenant error, provide default logo
				ctx = context.WithValue(ctx, constants.LogoKey, assets.DefaultLogo())
				next.ServeHTTP(w, r.WithContext(ctx))
				return
			}

			// Build logo props
			logoProps := &assets.LogoProps{}

			// Load main logo if exists
			if tenant.LogoID() != nil {
				if upload, err := uploadService.GetByID(ctx, uint(*tenant.LogoID())); err == nil {
					logoProps.LogoUpload = mappers.UploadToViewModel(upload)
				}
			}

			// Load compact logo if exists
			if tenant.LogoCompactID() != nil {
				if upload, err := uploadService.GetByID(ctx, uint(*tenant.LogoCompactID())); err == nil {
					logoProps.LogoCompactUpload = mappers.UploadToViewModel(upload)
				}
			}

			// Provide dynamic logo component
			ctx = context.WithValue(ctx, constants.LogoKey, assets.DynamicLogo(logoProps))
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
