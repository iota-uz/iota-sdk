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

			user, err := composables.UseUser(ctx)
			if err != nil {
				next.ServeHTTP(w, r.WithContext(ctx))
				return
			}

			tenant, err := tenantService.GetByID(ctx, user.TenantID())
			if err != nil {
				next.ServeHTTP(w, r.WithContext(ctx))
				return
			}

			if tenant.LogoID() == nil && tenant.LogoCompactID() == nil {
				next.ServeHTTP(w, r.WithContext(ctx))
				return
			}

			logoProps := assets.LogoProps{}

			if tenant.LogoID() != nil {
				if upload, err := uploadService.GetByID(ctx, uint(*tenant.LogoID())); err == nil {
					logoProps.LogoUpload = mappers.UploadToViewModel(upload)
				}
			}

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
