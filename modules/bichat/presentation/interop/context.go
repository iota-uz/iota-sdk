package interop

import (
	"context"

	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/types"
)

// BuildInitialContext builds the initial context object for the React frontend
func BuildInitialContext(ctx context.Context) (*InitialContext, error) {
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
			Name: getTenantName(ctx),
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

// getTenantName returns the tenant name (placeholder implementation)
// TODO: Implement actual tenant name lookup when tenant service is available
func getTenantName(ctx context.Context) string {
	// For now, return a placeholder
	// In a real implementation, this would query the tenant service
	return "Default Tenant"
}
