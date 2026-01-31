package interop

import (
	"context"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/modules/core/services"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/types"
	"github.com/sirupsen/logrus"
)

// ContextBuilder holds optional dependencies for building initial context
type ContextBuilder struct {
	tenantService *services.TenantService
	logger        *logrus.Logger
}

// NewContextBuilder creates a new context builder with optional dependencies
func NewContextBuilder(tenantService *services.TenantService, logger *logrus.Logger) *ContextBuilder {
	return &ContextBuilder{
		tenantService: tenantService,
		logger:        logger,
	}
}

// Build builds the initial context object for the React frontend
func (b *ContextBuilder) Build(ctx context.Context) (*InitialContext, error) {
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

	// Get all bichat.* translation keys
	translations := getTranslations(pageCtx)

	// Build context
	initialContext := &InitialContext{
		User: UserContext{
			ID:          int64(user.ID()),
			Email:       user.Email().Value(),
			FirstName:   user.FirstName(),
			LastName:    user.LastName(),
			Permissions: getUserPermissions(ctx),
		},
		Tenant: TenantContext{
			ID:   tenantID.String(),
			Name: b.getTenantName(ctx, tenantID),
		},
		Locale: LocaleContext{
			Language:     pageCtx.GetLocale().String(),
			Translations: translations,
		},
		Config: AppConfig{
			GraphQLEndpoint: "/bichat/graphql",
			StreamEndpoint:  "/bichat/stream",
		},
	}

	return initialContext, nil
}

// BuildInitialContext builds the initial context object for the React frontend
// This is a backward-compatible wrapper that uses default behavior (no tenant service)
func BuildInitialContext(ctx context.Context) (*InitialContext, error) {
	builder := NewContextBuilder(nil, nil)
	return builder.Build(ctx)
}

// getTranslations extracts all BiChat.* translation keys
func getTranslations(pageCtx types.PageContextProvider) map[string]string {
	// Translation keys matching the JSON structure
	keys := map[string]string{
		"bichat.title":                "BiChat.Title",
		"bichat.new_chat":             "BiChat.NewChat",
		"bichat.send_message":         "BiChat.SendMessage",
		"bichat.message_placeholder":  "BiChat.MessagePlaceholder",
		"bichat.loading":              "BiChat.Loading",
		"bichat.error":                "BiChat.Error",
		"bichat.retry":                "BiChat.Retry",
		"bichat.cancel":               "BiChat.Cancel",
		"bichat.submit":               "BiChat.Submit",
		"bichat.export":               "BiChat.Export",
		"bichat.permissions.access":   "BiChat.Permissions.Access",
		"bichat.permissions.read_all": "BiChat.Permissions.ReadAll",
		"bichat.permissions.export":   "BiChat.Permissions.Export",
	}

	translations := make(map[string]string)
	for jsKey, i18nKey := range keys {
		translations[jsKey] = pageCtx.T(i18nKey)
	}

	return translations
}

// getTenantName returns the tenant name for the given tenant ID
func (b *ContextBuilder) getTenantName(ctx context.Context, tenantID uuid.UUID) string {
	// If tenant service is available, use it
	if b.tenantService != nil {
		tenant, err := b.tenantService.GetByID(ctx, tenantID)
		if err != nil {
			// Log warning but don't fail - use fallback
			if b.logger != nil {
				b.logger.Warnf("Failed to get tenant name for %s: %v", tenantID, err)
			}
			return "Unknown Tenant"
		}
		return tenant.Name()
	}

	// Fallback if service not configured
	return "Default Tenant"
}
