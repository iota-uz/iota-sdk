package applet

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/csrf"
	"github.com/iota-uz/go-i18n/v2/i18n"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/sirupsen/logrus"
	"golang.org/x/text/language"
)

// ContextBuilder builds InitialContext for applets by extracting
// user, tenant, locale, and session information from the request context.
type ContextBuilder struct {
	config             Config
	bundle             *i18n.Bundle
	sessionConfig      SessionConfig
	logger             *logrus.Logger
	metrics            MetricsRecorder
	tenantNameResolver TenantNameResolver
	errorEnricher      ErrorContextEnricher
	sessionStore       SessionStore
}

// NewContextBuilder creates a new ContextBuilder with required dependencies.
// All parameters are required except opts which provide optional customization.
//
// Required parameters:
//   - config: Applet configuration (endpoints, router, custom context)
//   - bundle: i18n bundle for translation loading
//   - sessionConfig: Session expiry and refresh configuration
//   - logger: Structured logger for operations
//   - metrics: Metrics recorder for performance tracking
//
// Optional via BuilderOption:
//   - WithTenantNameResolver: Custom tenant name resolution
//   - WithErrorEnricher: Custom error context enrichment
//   - WithSessionStore: Custom session store for actual expiry times
func NewContextBuilder(
	config Config,
	bundle *i18n.Bundle,
	sessionConfig SessionConfig,
	logger *logrus.Logger,
	metrics MetricsRecorder,
	opts ...BuilderOption,
) *ContextBuilder {
	b := &ContextBuilder{
		config:        config,
		bundle:        bundle,
		sessionConfig: sessionConfig,
		logger:        logger,
		metrics:       metrics,
	}

	for _, opt := range opts {
		opt(b)
	}

	return b
}

// Build builds the InitialContext object for the React/Next.js frontend.
// It extracts all necessary information from the request context and
// serializes it for JSON injection into the frontend application.
//
// Performance target: <20ms for typical request
// Logs: Entry/exit with user/tenant/duration
// Metrics: build_duration, translation_load_time, tenant_resolution_time
//
// basePath is the applet's base path (e.g., "/bi-chat") used for route parsing.
func (b *ContextBuilder) Build(ctx context.Context, r *http.Request, basePath string) (*InitialContext, error) {
	const op serrors.Op = "ContextBuilder.Build"
	start := time.Now()

	// Extract user
	user, err := composables.UseUser(ctx)
	if err != nil {
		if b.logger != nil {
			b.logger.WithError(err).Error("Failed to extract user for applet context")
		}
		return nil, serrors.E(op, serrors.Internal, "user extraction failed", err)
	}

	// Extract tenant ID
	tenantID, err := composables.UseTenantID(ctx)
	if err != nil {
		if b.logger != nil {
			b.logger.WithError(err).WithField("user_id", user.ID()).Error("Failed to extract tenant ID")
		}
		return nil, serrors.E(op, serrors.Internal, "tenant extraction failed", err)
	}

	// Extract page context for locale
	pageCtx := composables.UsePageCtx(ctx)
	userLocale := pageCtx.GetLocale()

	// Log build start
	if b.logger != nil {
		b.logger.WithFields(logrus.Fields{
			"user_id":   user.ID(),
			"tenant_id": tenantID.String(),
			"locale":    userLocale.String(),
		}).Debug("Building applet context")
	}

	// Get all user permissions (validated)
	permissions := getUserPermissions(ctx)
	permissions = validatePermissions(permissions)

	// Load all translations for user's locale
	translationStart := time.Now()
	translations := b.getAllTranslations(userLocale)
	translationDuration := time.Since(translationStart)
	if b.metrics != nil {
		b.metrics.RecordDuration("applet.translation_load", translationDuration, map[string]string{
			"locale": userLocale.String(),
		})
	}

	// Get tenant name (3-layer fallback)
	tenantResolveStart := time.Now()
	tenantName := b.getTenantName(ctx, tenantID)
	tenantResolveDuration := time.Since(tenantResolveStart)
	if b.metrics != nil {
		b.metrics.RecordDuration("applet.tenant_resolution", tenantResolveDuration, map[string]string{
			"tenant_id": tenantID.String(),
		})
	}

	// Build route context
	router := b.config.Router
	if router == nil {
		router = NewDefaultRouter()
	}
	route := router.ParseRoute(r, basePath)

	// Build session context
	session := buildSessionContext(r, b.sessionConfig, b.sessionStore)

	// Build error context
	errorCtx, err := b.buildErrorContext(ctx, r)
	if err != nil {
		if b.logger != nil {
			b.logger.WithError(err).Warn("Failed to enrich error context, using defaults")
		}
		// Use minimal defaults on error
		errorCtx = &ErrorContext{
			DebugMode: false,
		}
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
			Language:     userLocale.String(),
			Translations: translations,
		},
		Config: AppConfig{
			GraphQLEndpoint: b.config.Endpoints.GraphQL,
			StreamEndpoint:  b.config.Endpoints.Stream,
			RESTEndpoint:    b.config.Endpoints.REST,
		},
		Route:   route,
		Session: session,
		Error:   errorCtx,
	}

	// Apply custom context extender if provided
	if b.config.CustomContext != nil {
		customData, err := b.config.CustomContext(ctx)
		if err != nil {
			if b.logger != nil {
				b.logger.WithError(err).Warn("Failed to build custom context")
			}
		} else if customData != nil {
			// Sanitize custom data to prevent XSS
			initialContext.Extensions = sanitizeForJSON(customData)
		}
	}

	// Log and record metrics
	buildDuration := time.Since(start)
	if b.logger != nil {
		b.logger.WithFields(logrus.Fields{
			"user_id":     user.ID(),
			"tenant_id":   tenantID.String(),
			"duration_ms": buildDuration.Milliseconds(),
		}).Debug("Built applet context")
	}

	if b.metrics != nil {
		b.metrics.RecordDuration("applet.context_build", buildDuration, map[string]string{
			"tenant_id": tenantID.String(),
		})
		b.metrics.IncrementCounter("applet.context_built", map[string]string{
			"tenant_id": tenantID.String(),
		})
	}

	return initialContext, nil
}

// getUserPermissions extracts all permission IDs the user has.
// Returns ALL permissions (not filtered by applet).
// Returns empty slice if user extraction fails.
func getUserPermissions(ctx context.Context) []string {
	user, err := composables.UseUser(ctx)
	if err != nil {
		return []string{}
	}

	perms := make([]string, 0, len(user.Permissions()))
	for _, perm := range user.Permissions() {
		// Use Name() which returns the string identifier (e.g., "bichat.access")
		perms = append(perms, perm.Name())
	}
	return perms
}

// getAllTranslations extracts ALL translations from the i18n bundle for the user's locale.
// Uses bundle.Messages() to iterate all message IDs and localize each one.
// NO caching - loads fresh each time from bundle for correctness.
//
// Performance: Pre-allocates map with estimated size for efficiency.
func (b *ContextBuilder) getAllTranslations(locale language.Tag) map[string]string {
	// Get all messages for the user's locale
	messages := b.bundle.Messages()
	localeMessages, exists := messages[locale]
	if !exists {
		// Locale not found, return empty map
		if b.logger != nil {
			b.logger.WithField("locale", locale.String()).Warn("No translations found for locale")
		}
		return make(map[string]string)
	}

	// Pre-allocate with exact size
	translations := make(map[string]string, len(localeMessages))

	// Create localizer for the user's locale
	localizer := i18n.NewLocalizer(b.bundle, locale.String())

	// Iterate all message IDs and localize
	for messageID := range localeMessages {
		translation := localizer.MustLocalize(&i18n.LocalizeConfig{
			MessageID: messageID,
		})
		translations[messageID] = translation
	}

	return translations
}

// getTenantName returns the tenant name using multi-layer fallback:
// 1. Try TenantNameResolver (if provided via WithTenantNameResolver)
// 2. Try direct database query (if pool available in context)
// 3. Fall back to "Tenant {short-uuid}" format (first 8 chars)
//
// Logs warnings on resolver/database failures but always returns a valid name.
func (b *ContextBuilder) getTenantName(ctx context.Context, tenantID uuid.UUID) string {
	tenantIDStr := tenantID.String()

	// Layer 1: Custom resolver
	if b.tenantNameResolver != nil {
		name, err := b.tenantNameResolver.ResolveTenantName(tenantIDStr)
		if err == nil && name != "" {
			return name
		}
		if b.logger != nil {
			b.logger.WithError(err).WithField("tenant_id", tenantIDStr).Warn("Tenant resolver failed, trying database")
		}
	}

	// Layer 2: Direct database query
	pool, err := composables.UsePool(ctx)
	if err == nil {
		name := queryTenantNameFromDB(ctx, pool, tenantID)
		if name != "" {
			return name
		}
		if b.logger != nil {
			b.logger.WithField("tenant_id", tenantIDStr).Warn("Database tenant query failed, using fallback")
		}
	}

	// Layer 3: Default format with short UUID (first 8 chars)
	return fmt.Sprintf("Tenant %s", tenantIDStr[:8])
}

// queryTenantNameFromDB queries tenant name directly from database.
// Returns empty string on error (caller will use fallback).
func queryTenantNameFromDB(ctx context.Context, pool *pgxpool.Pool, tenantID uuid.UUID) string {
	const query = "SELECT name FROM tenants WHERE id = $1"
	var name string
	if err := pool.QueryRow(ctx, query, tenantID).Scan(&name); err != nil {
		return ""
	}
	return name
}

// buildSessionContext creates SessionContext from request and session configuration.
// Uses actual session expiry from SessionStore if available, otherwise falls back to
// configured ExpiryDuration.
//
// Fallback strategy:
//  1. Try reading from session store (if provided)
//  2. Use configured duration if store not available or returns zero time
func buildSessionContext(r *http.Request, config SessionConfig, store SessionStore) SessionContext {
	var expiresAt time.Time

	// Try reading actual expiry from session store
	if store != nil {
		storeExpiry := store.GetSessionExpiry(r)
		if !storeExpiry.IsZero() {
			expiresAt = storeExpiry
		}
	}

	// Fallback to configured duration if no store or zero time returned
	if expiresAt.IsZero() {
		expiresAt = time.Now().Add(config.ExpiryDuration)
	}

	return SessionContext{
		ExpiresAt:  expiresAt.UnixMilli(),
		RefreshURL: config.RefreshURL,
		CSRFToken:  csrf.Token(r),
	}
}

// buildErrorContext builds ErrorContext using optional ErrorContextEnricher.
// Returns minimal defaults if enricher not provided or fails.
func (b *ContextBuilder) buildErrorContext(ctx context.Context, r *http.Request) (*ErrorContext, error) {
	const op serrors.Op = "ContextBuilder.buildErrorContext"

	// If enricher provided, use it
	if b.errorEnricher != nil {
		errorCtx, err := b.errorEnricher.EnrichContext(ctx, r)
		if err != nil {
			return nil, serrors.E(op, err)
		}
		return errorCtx, nil
	}

	// Default minimal error context
	return &ErrorContext{
		DebugMode: false,
	}, nil
}
