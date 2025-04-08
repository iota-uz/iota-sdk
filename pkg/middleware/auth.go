package middleware

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"

	"github.com/google/uuid"
	"github.com/gorilla/mux"

	"github.com/iota-uz/iota-sdk/modules/core/services"
	"github.com/iota-uz/iota-sdk/pkg/application"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/configuration"
	"github.com/iota-uz/iota-sdk/pkg/constants"
)

func getToken(r *http.Request) (string, error) {
	conf := configuration.Use()
	token, err := r.Cookie(conf.SidCookieKey)
	if errors.Is(err, http.ErrNoCookie) {
		v := r.Header.Get("Authorization")
		if v == "" {
			return "", errors.New("no token found")
		}
		return v, nil
	}
	if err != nil {
		return "", err
	}
	return token.Value, nil
}

func Authorize() mux.MiddlewareFunc {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(
			func(w http.ResponseWriter, r *http.Request) {
				token, err := getToken(r)
				if err != nil {
					next.ServeHTTP(w, r)
					return
				}
				ctx := r.Context()
				app, err := application.UseApp(ctx)
				if err != nil {
					panic(err)
				}
				authService := app.Service(services.AuthService{}).(*services.AuthService)
				sess, err := authService.Authorize(ctx, token)
				if err != nil {
					next.ServeHTTP(w, r)
					return
				}

				// Now that we have the session, let's ensure we have the tenant in the context
				// First check if we already have a tenant
				_, tenantErr := composables.UseTenant(ctx)
				if tenantErr != nil {
					// First try to get tenant from session (already loaded from DB)
					if sess.TenantID != uuid.Nil {
						// Get tenant info directly
						tx, txErr := composables.UseTx(ctx)
						if txErr == nil {
							var name string
							var domain string
							err := tx.QueryRow(ctx, "SELECT name, domain FROM tenants WHERE id = $1 LIMIT 1", sess.TenantID.String()).Scan(&name, &domain)
							if err == nil {
								// Add tenant to context from session
								t := &composables.Tenant{
									ID:     sess.TenantID,
									Name:   name,
									Domain: domain,
								}
								ctx = context.WithValue(ctx, constants.TenantKey, t)
							}
						}
					} else {
						// Fallback: use direct database query to get the tenant ID for the user
						tx, txErr := composables.UseTx(ctx)
						if txErr == nil {
							var tenantIDStr string
							err := tx.QueryRow(ctx, "SELECT tenant_id FROM users WHERE id = $1 LIMIT 1", sess.UserID).Scan(&tenantIDStr)
							if err == nil && tenantIDStr != "" {
								tenantID, uuidErr := uuid.Parse(tenantIDStr)
								if uuidErr == nil {
									// Now query for the tenant info
									var name string
									var domain string
									err := tx.QueryRow(ctx, "SELECT name, domain FROM tenants WHERE id = $1 LIMIT 1", tenantIDStr).Scan(&name, &domain)
									if err == nil {
										// Add tenant to context
										t := &composables.Tenant{
											ID:     tenantID,
											Name:   name,
											Domain: domain,
										}
										ctx = context.WithValue(ctx, constants.TenantKey, t)
									}
								}
							}
						}
					}
				}

				params, ok := composables.UseParams(ctx)
				if !ok {
					panic("params not found. Add RequestParams middleware up the chain")
				}
				params.Authenticated = true
				next.ServeHTTP(w, r.WithContext(context.WithValue(ctx, constants.SessionKey, sess)))
			},
		)
	}
}

func ProvideUser() mux.MiddlewareFunc {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(
			func(w http.ResponseWriter, r *http.Request) {
				ctx := r.Context()
				sess, err := composables.UseSession(ctx)
				if err != nil {
					next.ServeHTTP(w, r)
					return
				}
				app, err := application.UseApp(ctx)
				if err != nil {
					panic(err)
				}
				userService := app.Service(services.UserService{}).(*services.UserService)
				u, err := userService.GetByID(ctx, sess.UserID)
				if err != nil {
					next.ServeHTTP(w, r)
					return
				}
				// Set the user in context
				ctx = context.WithValue(ctx, constants.UserKey, u)

				// Check if we already have a tenant in context
				_, tenantErr := composables.UseTenant(ctx)
				if tenantErr != nil {
					// If not, get it from the user's tenant ID
					tenantService := app.Service(services.TenantService{}).(*services.TenantService)
					t, err := tenantService.GetByID(ctx, u.TenantID())
					if err != nil {
						log.Printf("Error retrieving tenant: %v", err)
						// Don't add tenant to context if we couldn't get it
					} else {
						// Add tenant to context
						ctx = context.WithValue(ctx, constants.TenantKey, t)
					}
				}

				next.ServeHTTP(w, r.WithContext(ctx))
			},
		)
	}
}

func RedirectNotAuthenticated() mux.MiddlewareFunc {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(
			func(w http.ResponseWriter, r *http.Request) {
				params, ok := composables.UseParams(r.Context())
				if !ok {
					panic("params not found. Add RequestParams middleware up the chain")
				}
				if !params.Authenticated {
					http.Redirect(w, r, fmt.Sprintf("/login?next=%s", r.URL), http.StatusFound)
					return
				}
				next.ServeHTTP(w, r)
			},
		)
	}
}

func RequireAuthorization() mux.MiddlewareFunc {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(
			func(w http.ResponseWriter, r *http.Request) {
				params, ok := composables.UseParams(r.Context())
				if !ok {
					panic("params not found. Add RequestParams middleware up the chain")
				}
				if !params.Authenticated {
					http.Error(w, "Unauthorized", http.StatusUnauthorized)
					return
				}
				next.ServeHTTP(w, r)
			},
		)
	}
}
