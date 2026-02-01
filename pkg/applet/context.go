package applet

import (
	"context"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/csrf"
	"github.com/iota-uz/go-i18n/v2/i18n"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/types"
	"github.com/sirupsen/logrus"
)

// ContextBuilder builds InitialContext for applets by extracting
// user, tenant, locale, and session information from the request context.
type ContextBuilder struct {
	config             Config
	logger             *logrus.Logger
	tenantNameResolver TenantNameResolver
}

// NewContextBuilder creates a new ContextBuilder with the given config and options.
func NewContextBuilder(config Config, opts ...BuilderOption) *ContextBuilder {
	b := &ContextBuilder{
		config: config,
	}
	for _, opt := range opts {
		opt(b)
	}
	return b
}

// Build builds the InitialContext object for the React/Next.js frontend.
// It extracts all necessary information from the request context and
// serializes it for JSON injection into the frontend application.
func (b *ContextBuilder) Build(ctx context.Context, r *http.Request) (*InitialContext, error) {
	// Extract user
	user, err := composables.UseUser(ctx)
	if err != nil {
		return nil, err
	}

	// Extract tenant ID
	tenantID, err := composables.UseTenantID(ctx)
	if err != nil {
		return nil, err
	}

	// Extract page context for locale
	pageCtx := composables.UsePageCtx(ctx)

	// Get all user permissions (not filtered)
	permissions := getUserPermissions(ctx)

	// Get all translations from bundle (not filtered)
	translations := getAllTranslations(pageCtx)

	// Get tenant name
	tenantName := b.getTenantName(ctx, tenantID)

	// Build route context
	router := b.config.Router
	if router == nil {
		router = NewDefaultRouter()
	}
	basePath := ""
	route := router.ParseRoute(r, basePath)

	// Build session context
	session := SessionContext{
		ExpiresAt:  time.Now().Add(24 * time.Hour).UnixMilli(), // Default 24h expiry
		RefreshURL: "/auth/refresh",
		CSRFToken:  csrf.Token(r),
	}

	// Build initial context
	initialContext := &InitialContext{
		User: UserContext{
			ID:          int64(user.ID()),
			Email:       user.Email().Value(),
			FirstName:   user.FirstName(),
			LastName:    user.LastName(),
			Permissions: permissions,
		},
		Tenant: TenantContext{
			ID:   tenantID.String(),
			Name: tenantName,
		},
		Locale: LocaleContext{
			Language:     pageCtx.GetLocale().String(),
			Translations: translations,
		},
		Config: AppConfig{
			GraphQLEndpoint: b.config.Endpoints.GraphQL,
			StreamEndpoint:  b.config.Endpoints.Stream,
			RESTEndpoint:    b.config.Endpoints.REST,
		},
		Route:   route,
		Session: session,
	}

	// Apply custom context extender if provided
	if b.config.CustomContext != nil {
		customData, err := b.config.CustomContext(ctx)
		if err != nil {
			if b.logger != nil {
				b.logger.Warnf("Failed to build custom context: %v", err)
			}
		} else {
			initialContext.Custom = customData
		}
	}

	return initialContext, nil
}

// getUserPermissions extracts all permission IDs the user has.
// Returns ALL permissions, not filtered by applet.
func getUserPermissions(ctx context.Context) []string {
	user, err := composables.UseUser(ctx)
	if err != nil {
		return []string{}
	}

	perms := []string{}
	for _, perm := range user.Permissions() {
		// Use Name() which returns the string identifier (e.g., "bichat.access")
		perms = append(perms, perm.Name())
	}
	return perms
}

// getAllTranslations extracts all translations from the i18n bundle.
// Returns ALL translations, not filtered by applet.
//
// Note: This is a simplified implementation that returns a subset of common keys.
// In production, you may want to iterate through bundle.MessageFiles() to get all keys.
func getAllTranslations(pageCtx types.PageContextProvider) map[string]string {
	// For now, return all accessible translations via T() method
	// This is a simplified approach - a full implementation would
	// iterate through bundle.MessageFiles() to get all message IDs

	localizer := pageCtx.GetLocalizer()
	translations := make(map[string]string)

	// Get common translation keys
	// In a full implementation, this would be extracted from the bundle's MessageFiles
	commonKeys := []string{
		"BiChat.Title",
		"BiChat.NewChat",
		"BiChat.SendMessage",
		"BiChat.MessagePlaceholder",
		"BiChat.Loading",
		"BiChat.Error",
		"BiChat.Retry",
		"BiChat.Cancel",
		"BiChat.Submit",
		"BiChat.Export",
		"BiChat.Permissions.Access",
		"BiChat.Permissions.ReadAll",
		"BiChat.Permissions.Export",
		"Common.Save",
		"Common.Delete",
		"Common.Edit",
		"Common.Cancel",
		"Common.Close",
		"Common.Search",
	}

	for _, key := range commonKeys {
		translations[key] = localizer.MustLocalize(&i18n.LocalizeConfig{
			MessageID: key,
		})
	}

	return translations
}

// getTenantName returns the tenant name for the given tenant ID.
// Uses TenantNameResolver if provided, otherwise returns a default value.
func (b *ContextBuilder) getTenantName(ctx context.Context, tenantID uuid.UUID) string {
	if b.tenantNameResolver != nil {
		name, err := b.tenantNameResolver.ResolveTenantName(tenantID.String())
		if err != nil {
			if b.logger != nil {
				b.logger.Warnf("Failed to resolve tenant name for %s: %v", tenantID, err)
			}
			return "Unknown Tenant"
		}
		return name
	}

	// Fallback if resolver not configured
	return "Default Tenant"
}
