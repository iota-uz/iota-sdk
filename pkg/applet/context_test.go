package applet

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/csrf"
	"github.com/iota-uz/go-i18n/v2/i18n"
	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/role"
	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/user"
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/permission"
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/upload"
	"github.com/iota-uz/iota-sdk/modules/core/domain/value_objects/internet"
	"github.com/iota-uz/iota-sdk/modules/core/domain/value_objects/phone"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/constants"
	"github.com/iota-uz/iota-sdk/pkg/types"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/text/language"
)

// Mock implementations

type mockUser struct {
	id          uint
	email       internet.Email
	firstName   string
	lastName    string
	permissions []permission.Permission
}

func (m *mockUser) ID() uint                                       { return m.id }
func (m *mockUser) Email() internet.Email                          { return m.email }
func (m *mockUser) FirstName() string                              { return m.firstName }
func (m *mockUser) LastName() string                               { return m.lastName }
func (m *mockUser) Permissions() []permission.Permission           { return m.permissions }
func (m *mockUser) Can(perm permission.Permission) bool            { return false }
func (m *mockUser) Type() user.Type                                { return user.TypeUser }
func (m *mockUser) TenantID() uuid.UUID                            { return uuid.Nil }
func (m *mockUser) MiddleName() string                             { return "" }
func (m *mockUser) Password() string                               { return "" }
func (m *mockUser) Avatar() upload.Upload                          { return nil }
func (m *mockUser) AvatarID() uint                                 { return 0 }
func (m *mockUser) Roles() []role.Role                             { return nil }
func (m *mockUser) GroupIDs() []uuid.UUID                          { return nil }
func (m *mockUser) LastIP() string                                 { return "" }
func (m *mockUser) LastLogin() time.Time                           { return time.Time{} }
func (m *mockUser) EmailVerifiedAt() *time.Time                    { return nil }
func (m *mockUser) CreatedBy() uint                                { return 0 }
func (m *mockUser) CreatedAt() time.Time                           { return time.Time{} }
func (m *mockUser) UpdatedAt() time.Time                           { return time.Time{} }
func (m *mockUser) DeletedAt() *time.Time                          { return nil }
func (m *mockUser) SetFirstName(name string) user.User             { return m }
func (m *mockUser) SetLastName(name string) user.User              { return m }
func (m *mockUser) SetMiddleName(name string) user.User            { return m }
func (m *mockUser) SetEmail(email internet.Email) user.User        { return m }
func (m *mockUser) SetPassword(password string) (user.User, error) { return m, nil }
func (m *mockUser) SetAvatar(avatar upload.Upload) user.User       { return m }
func (m *mockUser) SetAvatarID(id uint) user.User                  { return m }
func (m *mockUser) SetRoles(roles []role.Role) user.User           { return m }
func (m *mockUser) SetPermissions(perms []permission.Permission) user.User {
	return m
}
func (m *mockUser) SetGroupIDs(ids []uuid.UUID) user.User                    { return m }
func (m *mockUser) SetLastIP(ip string) user.User                            { return m }
func (m *mockUser) SetLastLogin(t time.Time) user.User                       { return m }
func (m *mockUser) SetEmailVerifiedAt(t *time.Time) user.User                { return m }
func (m *mockUser) SetType(type_ user.Type) user.User                        { return m }
func (m *mockUser) AddGroupID(groupID uuid.UUID) user.User                   { return m }
func (m *mockUser) AddRole(r role.Role) user.User                            { return m }
func (m *mockUser) RemoveRole(r role.Role) user.User                         { return m }
func (m *mockUser) RemoveGroupID(groupID uuid.UUID) user.User                { return m }
func (m *mockUser) AddPermission(perm permission.Permission) user.User       { return m }
func (m *mockUser) RemovePermission(permID uuid.UUID) user.User              { return m }
func (m *mockUser) SetName(firstName, lastName, middleName string) user.User { return m }
func (m *mockUser) SetUILanguage(lang user.UILanguage) user.User             { return m }
func (m *mockUser) SetPasswordUnsafe(password string) user.User              { return m }
func (m *mockUser) SetPhone(p phone.Phone) user.User                         { return m }
func (m *mockUser) Block(reason string, blockedBy uint, blockedByTenantID uuid.UUID) user.User {
	return m
}
func (m *mockUser) Unblock() user.User                 { return m }
func (m *mockUser) UILanguage() user.UILanguage        { return "" }
func (m *mockUser) Phone() phone.Phone                 { return nil }
func (m *mockUser) LastAction() time.Time              { return time.Time{} }
func (m *mockUser) Events() []interface{}              { return nil }
func (m *mockUser) CanUpdate() bool                    { return true }
func (m *mockUser) CanDelete() bool                    { return true }
func (m *mockUser) CheckPassword(password string) bool { return false }
func (m *mockUser) IsBlocked() bool                    { return false }
func (m *mockUser) BlockReason() string                { return "" }
func (m *mockUser) BlockedAt() time.Time               { return time.Time{} }
func (m *mockUser) BlockedBy() uint                    { return 0 }
func (m *mockUser) BlockedByTenantID() uuid.UUID       { return uuid.Nil }
func (m *mockUser) CanBeBlocked() bool                 { return true }

type mockPermission struct {
	name string
}

func (m *mockPermission) ID() uuid.UUID                       { return uuid.Nil }
func (m *mockPermission) Name() string                        { return m.name }
func (m *mockPermission) Resource() permission.Resource       { return "" }
func (m *mockPermission) Action() permission.Action           { return "" }
func (m *mockPermission) Modifier() permission.Modifier       { return "" }
func (m *mockPermission) Equals(p permission.Permission) bool { return false }
func (m *mockPermission) Matches(resource permission.Resource, action permission.Action) bool {
	return false
}
func (m *mockPermission) IsValid() bool { return true }

type mockTenantResolver struct {
	name string
	err  error
}

func (m *mockTenantResolver) ResolveTenantName(tenantID string) (string, error) {
	if m.err != nil {
		return "", m.err
	}
	return m.name, nil
}

type mockErrorEnricher struct {
	ctx *ErrorContext
	err error
}

func (m *mockErrorEnricher) EnrichContext(ctx context.Context, r *http.Request) (*ErrorContext, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.ctx, nil
}

type mockMetrics struct {
	durations []struct {
		name     string
		duration time.Duration
		labels   map[string]string
	}
	counters []struct {
		name   string
		labels map[string]string
	}
}

func (m *mockMetrics) RecordDuration(name string, duration time.Duration, labels map[string]string) {
	m.durations = append(m.durations, struct {
		name     string
		duration time.Duration
		labels   map[string]string
	}{name, duration, labels})
}

func (m *mockMetrics) IncrementCounter(name string, labels map[string]string) {
	m.counters = append(m.counters, struct {
		name   string
		labels map[string]string
	}{name, labels})
}

// Helper functions

func createTestBundle() *i18n.Bundle {
	bundle := i18n.NewBundle(language.English)
	_ = bundle.AddMessages(language.English, &i18n.Message{
		ID:    "greeting",
		Other: "Hello",
	})
	_ = bundle.AddMessages(language.English, &i18n.Message{
		ID:    "farewell",
		Other: "Goodbye",
	})
	_ = bundle.AddMessages(language.English, &i18n.Message{
		ID:    "Common.Greeting",
		Other: "Hello (Common)",
	})
	_ = bundle.AddMessages(language.Russian, &i18n.Message{
		ID:    "greeting",
		Other: "Привет",
	})
	return bundle
}

func createTestContext(t *testing.T, opts ...func(*testContextOptions)) context.Context {
	t.Helper()

	options := &testContextOptions{
		userID:    123,
		email:     "test@example.com",
		firstName: "John",
		lastName:  "Doe",
		tenantID:  uuid.New(),
		locale:    language.English,
	}

	for _, opt := range opts {
		opt(options)
	}

	email, err := internet.NewEmail(options.email)
	require.NoError(t, err)

	mockUser := &mockUser{
		id:          options.userID,
		email:       email,
		firstName:   options.firstName,
		lastName:    options.lastName,
		permissions: options.permissions,
	}

	ctx := context.Background()
	ctx = composables.WithUser(ctx, mockUser)
	ctx = context.WithValue(ctx, constants.TenantIDKey, options.tenantID)
	// PageContext kept for backward compatibility; prefer PageContextProvider for new code.
	ctx = composables.WithPageCtx(ctx, &types.PageContext{ //nolint:staticcheck // SA1019: backward compat
		Locale: options.locale,
	})

	return ctx
}

type testContextOptions struct {
	userID      uint
	email       string
	firstName   string
	lastName    string
	tenantID    uuid.UUID
	locale      language.Tag
	permissions []permission.Permission
}

func withUserID(id uint) func(*testContextOptions) {
	return func(o *testContextOptions) {
		o.userID = id
	}
}

func withPermissions(perms ...string) func(*testContextOptions) {
	return func(o *testContextOptions) {
		permissions := make([]permission.Permission, len(perms))
		for i, p := range perms {
			permissions[i] = &mockPermission{name: p}
		}
		o.permissions = permissions
	}
}

// Tests

func TestContextBuilder_Build_Success(t *testing.T) {
	t.Parallel()

	bundle := createTestBundle()
	logger := logrus.New()
	logger.SetLevel(logrus.FatalLevel) // Suppress logs during tests
	metrics := &mockMetrics{}
	sessionConfig := DefaultSessionConfig

	config := Config{
		Endpoints: EndpointConfig{
			GraphQL: "/graphql",
			Stream:  "/stream",
			REST:    "/api",
		},
	}

	builder := NewContextBuilder(config, bundle, sessionConfig, logger, metrics)

	ctx := createTestContext(t,
		withUserID(42),
		withPermissions("bichat.access", "finance.read"),
	)

	// Create CSRF-protected request
	r := httptest.NewRequest(http.MethodGet, "/test", nil)
	csrfMiddleware := csrf.Protect(
		[]byte("32-byte-long-secret-key-for-testing!"),
		csrf.Secure(false), // Disable secure for testing
	)
	handler := csrfMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Request now has CSRF token
		initialCtx, err := builder.Build(ctx, r, "")
		assert.NoError(t, err)
		assert.NotNil(t, initialCtx)
		if err != nil || initialCtx == nil {
			return
		}

		// Verify user context
		assert.Equal(t, int64(42), initialCtx.User.ID)
		assert.Equal(t, "test@example.com", initialCtx.User.Email)
		assert.Equal(t, "John", initialCtx.User.FirstName)
		assert.Equal(t, "Doe", initialCtx.User.LastName)
		assert.Contains(t, initialCtx.User.Permissions, "bichat.access")
		assert.Contains(t, initialCtx.User.Permissions, "finance.read")

		// Verify tenant context
		assert.NotEmpty(t, initialCtx.Tenant.ID)
		assert.Contains(t, initialCtx.Tenant.Name, "Tenant") // Default format

		// Verify locale context
		assert.Equal(t, "en", initialCtx.Locale.Language)
		assert.NotEmpty(t, initialCtx.Locale.Translations)
		assert.Equal(t, "Hello", initialCtx.Locale.Translations["greeting"])

		// Verify config
		assert.Equal(t, "/graphql", initialCtx.Config.GraphQLEndpoint)
		assert.Equal(t, "/stream", initialCtx.Config.StreamEndpoint)
		assert.Equal(t, "/api", initialCtx.Config.RESTEndpoint)

		// Verify route context
		assert.Equal(t, "/test", initialCtx.Route.Path)
		assert.NotNil(t, initialCtx.Route.Params)
		assert.NotNil(t, initialCtx.Route.Query)

		// Verify session context
		assert.Greater(t, initialCtx.Session.ExpiresAt, time.Now().UnixMilli())
		assert.Equal(t, "/auth/refresh", initialCtx.Session.RefreshURL)
		assert.NotEmpty(t, initialCtx.Session.CSRFToken)

		// Verify error context
		assert.NotNil(t, initialCtx.Error)
		assert.False(t, initialCtx.Error.DebugMode)

		// Verify metrics were recorded
		assert.NotEmpty(t, metrics.durations)
		assert.NotEmpty(t, metrics.counters)
	}))

	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)
}

func TestContextBuilder_Build_MissingUser(t *testing.T) {
	t.Parallel()

	bundle := createTestBundle()
	logger := logrus.New()
	logger.SetLevel(logrus.FatalLevel)
	metrics := &mockMetrics{}
	sessionConfig := DefaultSessionConfig

	config := Config{}
	builder := NewContextBuilder(config, bundle, sessionConfig, logger, metrics)

	// Context without user
	ctx := context.Background()
	r := httptest.NewRequest(http.MethodGet, "/test", nil)

	initialCtx, err := builder.Build(ctx, r, "")
	require.Error(t, err)
	assert.Nil(t, initialCtx)
	assert.Contains(t, err.Error(), "user extraction failed")
}

func TestContextBuilder_Build_MissingTenant(t *testing.T) {
	t.Parallel()

	bundle := createTestBundle()
	logger := logrus.New()
	logger.SetLevel(logrus.FatalLevel)
	metrics := &mockMetrics{}
	sessionConfig := DefaultSessionConfig

	config := Config{}
	builder := NewContextBuilder(config, bundle, sessionConfig, logger, metrics)

	// Context with user but no tenant ID
	email, err := internet.NewEmail("test@example.com")
	require.NoError(t, err)
	mockUser := &mockUser{
		id:        123,
		email:     email,
		firstName: "John",
		lastName:  "Doe",
	}

	ctx := context.Background()
	ctx = composables.WithUser(ctx, mockUser)
	// No tenant ID in context

	r := httptest.NewRequest(http.MethodGet, "/test", nil)

	initialCtx, err := builder.Build(ctx, r, "")
	require.Error(t, err)
	assert.Nil(t, initialCtx)
	assert.Contains(t, err.Error(), "tenant extraction failed")
}

func TestGetAllTranslations_Success(t *testing.T) {
	t.Parallel()

	bundle := createTestBundle()
	logger := logrus.New()
	logger.SetLevel(logrus.FatalLevel)
	metrics := &mockMetrics{}
	sessionConfig := DefaultSessionConfig

	config := Config{}
	builder := NewContextBuilder(config, bundle, sessionConfig, logger, metrics)

	// Test English locale
	translations := builder.getAllTranslations(language.English)
	assert.NotEmpty(t, translations)
	assert.Equal(t, "Hello", translations["greeting"])
	assert.Equal(t, "Goodbye", translations["farewell"])

	// Test Russian locale
	translationsRu := builder.getAllTranslations(language.Russian)
	assert.NotEmpty(t, translationsRu)
	assert.Equal(t, "Привет", translationsRu["greeting"])
}

func TestGetAllTranslations_LocaleNotFound(t *testing.T) {
	t.Parallel()

	bundle := createTestBundle()
	logger := logrus.New()
	logger.SetLevel(logrus.FatalLevel)
	metrics := &mockMetrics{}
	sessionConfig := DefaultSessionConfig

	config := Config{}
	builder := NewContextBuilder(config, bundle, sessionConfig, logger, metrics)

	// Test unsupported locale
	translations := builder.getAllTranslations(language.Japanese)
	assert.Empty(t, translations)
}

func TestGetAllTranslations_PrefixesMode(t *testing.T) {
	t.Parallel()

	bundle := createTestBundle()
	logger := logrus.New()
	logger.SetLevel(logrus.FatalLevel)
	metrics := &mockMetrics{}
	sessionConfig := DefaultSessionConfig

	config := Config{
		I18n: I18nConfig{
			Mode:     TranslationModePrefixes,
			Prefixes: []string{"Common."},
		},
	}
	builder := NewContextBuilder(config, bundle, sessionConfig, logger, metrics)

	translations := builder.getAllTranslations(language.English)
	assert.Equal(t, "Hello (Common)", translations["Common.Greeting"])
	_, ok := translations["greeting"]
	assert.False(t, ok)
}

func TestGetAllTranslations_NoneMode(t *testing.T) {
	t.Parallel()

	bundle := createTestBundle()
	logger := logrus.New()
	logger.SetLevel(logrus.FatalLevel)
	metrics := &mockMetrics{}
	sessionConfig := DefaultSessionConfig

	config := Config{
		I18n: I18nConfig{
			Mode: TranslationModeNone,
		},
	}
	builder := NewContextBuilder(config, bundle, sessionConfig, logger, metrics)

	translations := builder.getAllTranslations(language.English)
	assert.Empty(t, translations)
}

func TestGetTenantName_ResolverSuccess(t *testing.T) {
	t.Parallel()

	bundle := createTestBundle()
	logger := logrus.New()
	logger.SetLevel(logrus.FatalLevel)
	metrics := &mockMetrics{}
	sessionConfig := DefaultSessionConfig

	resolver := &mockTenantResolver{
		name: "ACME Corporation",
		err:  nil,
	}

	config := Config{}
	builder := NewContextBuilder(config, bundle, sessionConfig, logger, metrics,
		WithTenantNameResolver(resolver),
	)

	tenantID := uuid.New()
	ctx := context.Background()

	name := builder.getTenantName(ctx, tenantID)
	assert.Equal(t, "ACME Corporation", name)
}

func TestGetTenantName_ResolverError(t *testing.T) {
	t.Parallel()

	bundle := createTestBundle()
	logger := logrus.New()
	logger.SetLevel(logrus.FatalLevel)
	metrics := &mockMetrics{}
	sessionConfig := DefaultSessionConfig

	// Resolver that fails
	resolver := &mockTenantResolver{
		err: assert.AnError,
	}

	config := Config{}
	builder := NewContextBuilder(config, bundle, sessionConfig, logger, metrics,
		WithTenantNameResolver(resolver),
	)

	tenantID := uuid.MustParse("12345678-1234-1234-1234-123456789012")
	ctx := context.Background()
	// No pool in context, so it should fall back to default format

	name := builder.getTenantName(ctx, tenantID)
	// Should fall back to default format when resolver fails and no DB
	assert.Equal(t, "Tenant 12345678", name)
}

func TestGetTenantName_AllFallbacksToDefault(t *testing.T) {
	t.Parallel()

	bundle := createTestBundle()
	logger := logrus.New()
	logger.SetLevel(logrus.FatalLevel)
	metrics := &mockMetrics{}
	sessionConfig := DefaultSessionConfig

	config := Config{}
	builder := NewContextBuilder(config, bundle, sessionConfig, logger, metrics)
	// No resolver, no database

	tenantID := uuid.MustParse("12345678-1234-1234-1234-123456789012")
	ctx := context.Background()

	name := builder.getTenantName(ctx, tenantID)
	assert.Equal(t, "Tenant 12345678", name) // First 8 chars of UUID
}

func TestGetUserPermissions_Success(t *testing.T) {
	t.Parallel()

	ctx := createTestContext(t,
		withPermissions("bichat.access", "finance.read", "core.admin"),
	)

	permissions := getUserPermissions(ctx)
	assert.Len(t, permissions, 3)
	assert.Contains(t, permissions, "bichat.access")
	assert.Contains(t, permissions, "finance.read")
	assert.Contains(t, permissions, "core.admin")
}

func TestGetUserPermissions_IncludesRolePermissions(t *testing.T) {
	t.Parallel()

	p := permission.New(
		permission.WithID(uuid.New()),
		permission.WithName("BiChat.Access"),
		permission.WithResource("bichat"),
		permission.WithAction(permission.ActionRead),
		permission.WithModifier(permission.ModifierAll),
	)
	r := role.New("Admin", role.WithPermissions([]permission.Permission{p}))
	u := user.New("T", "U", internet.MustParseEmail("role@example.com"), user.UILanguageEN, user.WithID(1))
	u = u.AddRole(r)

	ctx := composables.WithUser(context.Background(), u)
	permissions := getUserPermissions(ctx)
	assert.Contains(t, permissions, "bichat.access")
}

func TestGetUserPermissions_ErrorCase(t *testing.T) {
	t.Parallel()

	// Context without user
	ctx := context.Background()

	permissions := getUserPermissions(ctx)
	assert.Empty(t, permissions)
}

func TestBuildSessionContext(t *testing.T) {
	t.Parallel()

	sessionConfig := SessionConfig{
		ExpiryDuration: 2 * time.Hour,
		RefreshURL:     "/custom/refresh",
		RenewBefore:    10 * time.Minute,
	}

	r := httptest.NewRequest(http.MethodGet, "/test", nil)

	// Apply CSRF middleware to get token
	csrfMiddleware := csrf.Protect(
		[]byte("32-byte-long-secret-key-for-testing!"),
		csrf.Secure(false),
	)
	handler := csrfMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sessionCtx := buildSessionContext(r, sessionConfig, nil)

		// Verify expiry is approximately 2 hours from now
		expectedExpiry := time.Now().Add(2 * time.Hour).UnixMilli()
		assert.InDelta(t, expectedExpiry, sessionCtx.ExpiresAt, float64(1000)) // 1 second tolerance

		assert.Equal(t, "/custom/refresh", sessionCtx.RefreshURL)
		assert.NotEmpty(t, sessionCtx.CSRFToken)
	}))

	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)
}

func TestBuildErrorContext_WithEnricher(t *testing.T) {
	t.Parallel()

	bundle := createTestBundle()
	logger := logrus.New()
	logger.SetLevel(logrus.FatalLevel)
	metrics := &mockMetrics{}
	sessionConfig := DefaultSessionConfig

	enricher := &mockErrorEnricher{
		ctx: &ErrorContext{
			SupportEmail: "support@example.com",
			DebugMode:    true,
			ErrorCodes: map[string]string{
				"NOT_FOUND": "Resource not found",
			},
		},
	}

	config := Config{}
	builder := NewContextBuilder(config, bundle, sessionConfig, logger, metrics,
		WithErrorEnricher(enricher),
	)

	ctx := context.Background()
	r := httptest.NewRequest(http.MethodGet, "/test", nil)

	errorCtx, err := builder.buildErrorContext(ctx, r)
	require.NoError(t, err)
	require.NotNil(t, errorCtx)

	assert.Equal(t, "support@example.com", errorCtx.SupportEmail)
	assert.True(t, errorCtx.DebugMode)
	assert.Equal(t, "Resource not found", errorCtx.ErrorCodes["NOT_FOUND"])
}

func TestBuildErrorContext_WithoutEnricher(t *testing.T) {
	t.Parallel()

	bundle := createTestBundle()
	logger := logrus.New()
	logger.SetLevel(logrus.FatalLevel)
	metrics := &mockMetrics{}
	sessionConfig := DefaultSessionConfig

	config := Config{}
	builder := NewContextBuilder(config, bundle, sessionConfig, logger, metrics)
	// No enricher

	ctx := context.Background()
	r := httptest.NewRequest(http.MethodGet, "/test", nil)

	errorCtx, err := builder.buildErrorContext(ctx, r)
	require.NoError(t, err)
	require.NotNil(t, errorCtx)

	// Should return minimal defaults
	assert.False(t, errorCtx.DebugMode)
	assert.Empty(t, errorCtx.SupportEmail)
}

func TestContextBuilder_Build_WithCustomContext(t *testing.T) {
	t.Parallel()

	bundle := createTestBundle()
	logger := logrus.New()
	logger.SetLevel(logrus.FatalLevel)
	metrics := &mockMetrics{}
	sessionConfig := DefaultSessionConfig

	config := Config{
		CustomContext: func(ctx context.Context) (map[string]interface{}, error) {
			return map[string]interface{}{
				"customField": "customValue",
				"userId":      42,
			}, nil
		},
	}

	builder := NewContextBuilder(config, bundle, sessionConfig, logger, metrics)

	ctx := createTestContext(t)
	r := httptest.NewRequest(http.MethodGet, "/test", nil)

	// Apply CSRF
	csrfMiddleware := csrf.Protect(
		[]byte("32-byte-long-secret-key-for-testing!"),
		csrf.Secure(false),
	)
	var capturedCtx *InitialContext
	var capturedErr error

	handler := csrfMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedCtx, capturedErr = builder.Build(ctx, r, "")
	}))

	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)

	// Assert after handler completes
	require.NoError(t, capturedErr)
	require.NotNil(t, capturedCtx)
	assert.NotNil(t, capturedCtx.Extensions)
	assert.Equal(t, "customValue", capturedCtx.Extensions["customField"])
	assert.Equal(t, 42, capturedCtx.Extensions["userId"])
}

func TestContextBuilder_Build_WithMuxRouter(t *testing.T) {
	t.Parallel()

	bundle := createTestBundle()
	logger := logrus.New()
	logger.SetLevel(logrus.FatalLevel)
	metrics := &mockMetrics{}
	sessionConfig := DefaultSessionConfig

	config := Config{
		Router: NewMuxRouter(),
	}

	builder := NewContextBuilder(config, bundle, sessionConfig, logger, metrics)

	ctx := createTestContext(t)
	r := httptest.NewRequest(http.MethodGet, "/sessions/123?tab=history", nil)

	// Apply CSRF
	csrfMiddleware := csrf.Protect(
		[]byte("32-byte-long-secret-key-for-testing!"),
		csrf.Secure(false),
	)
	var capturedCtx *InitialContext
	var capturedErr error

	handler := csrfMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedCtx, capturedErr = builder.Build(ctx, r, "")
	}))

	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)

	// Assert after handler completes
	require.NoError(t, capturedErr)
	require.NotNil(t, capturedCtx)
	assert.Equal(t, "history", capturedCtx.Route.Query["tab"])
}

func TestContextBuilder_Build_MetricsRecorded(t *testing.T) {
	t.Parallel()

	bundle := createTestBundle()
	logger := logrus.New()
	logger.SetLevel(logrus.FatalLevel)
	metrics := &mockMetrics{}
	sessionConfig := DefaultSessionConfig

	config := Config{}
	builder := NewContextBuilder(config, bundle, sessionConfig, logger, metrics)

	ctx := createTestContext(t)
	r := httptest.NewRequest(http.MethodGet, "/test", nil)

	// Apply CSRF
	csrfMiddleware := csrf.Protect(
		[]byte("32-byte-long-secret-key-for-testing!"),
		csrf.Secure(false),
	)
	handler := csrfMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, err := builder.Build(ctx, r, "")
		assert.NoError(t, err)
		if err != nil {
			return
		}

		// Verify metrics were recorded
		assert.NotEmpty(t, metrics.durations)
		assert.NotEmpty(t, metrics.counters)

		// Check for specific metrics
		var foundContextBuild, foundTranslation, foundTenant bool
		for _, d := range metrics.durations {
			switch d.name {
			case "applet.context_build":
				foundContextBuild = true
			case "applet.translation_load":
				foundTranslation = true
			case "applet.tenant_resolution":
				foundTenant = true
			}
		}
		assert.True(t, foundContextBuild, "context_build metric not recorded")
		assert.True(t, foundTranslation, "translation_load metric not recorded")
		assert.True(t, foundTenant, "tenant_resolution metric not recorded")

		// Verify counter
		var foundCounter bool
		for _, c := range metrics.counters {
			if c.name == "applet.context_built" {
				foundCounter = true
			}
		}
		assert.True(t, foundCounter, "context_built counter not recorded")
	}))

	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)
}

// Mock session store for testing

type mockSessionStore struct {
	expiresAt time.Time
}

func (m *mockSessionStore) GetSessionExpiry(r *http.Request) time.Time {
	return m.expiresAt
}

func TestBuildSessionContext_WithSessionStore(t *testing.T) {
	t.Parallel()

	sessionConfig := SessionConfig{
		ExpiryDuration: 2 * time.Hour,
		RefreshURL:     "/custom/refresh",
		RenewBefore:    10 * time.Minute,
	}

	// Mock session store with actual expiry 3 hours from now
	actualExpiry := time.Now().Add(3 * time.Hour)
	store := &mockSessionStore{
		expiresAt: actualExpiry,
	}

	r := httptest.NewRequest(http.MethodGet, "/test", nil)

	// Apply CSRF middleware to get token
	csrfMiddleware := csrf.Protect(
		[]byte("32-byte-long-secret-key-for-testing!"),
		csrf.Secure(false),
	)
	handler := csrfMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sessionCtx := buildSessionContext(r, sessionConfig, store)

		// Verify it uses actual expiry from store (3 hours), not config (2 hours)
		expectedExpiry := actualExpiry.UnixMilli()
		assert.Equal(t, expectedExpiry, sessionCtx.ExpiresAt)

		assert.Equal(t, "/custom/refresh", sessionCtx.RefreshURL)
		assert.NotEmpty(t, sessionCtx.CSRFToken)
	}))

	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)
}

func TestBuildSessionContext_WithSessionStoreReturnsZero(t *testing.T) {
	t.Parallel()

	sessionConfig := SessionConfig{
		ExpiryDuration: 2 * time.Hour,
		RefreshURL:     "/custom/refresh",
		RenewBefore:    10 * time.Minute,
	}

	// Mock session store that returns zero time (session not found)
	store := &mockSessionStore{
		expiresAt: time.Time{},
	}

	r := httptest.NewRequest(http.MethodGet, "/test", nil)

	// Apply CSRF middleware to get token
	csrfMiddleware := csrf.Protect(
		[]byte("32-byte-long-secret-key-for-testing!"),
		csrf.Secure(false),
	)
	handler := csrfMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sessionCtx := buildSessionContext(r, sessionConfig, store)

		// Verify it falls back to configured duration (2 hours)
		expectedExpiry := time.Now().Add(2 * time.Hour).UnixMilli()
		assert.InDelta(t, expectedExpiry, sessionCtx.ExpiresAt, float64(1000)) // 1 second tolerance

		assert.Equal(t, "/custom/refresh", sessionCtx.RefreshURL)
		assert.NotEmpty(t, sessionCtx.CSRFToken)
	}))

	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)
}

func TestBuildSessionContext_WithoutSessionStore(t *testing.T) {
	t.Parallel()

	sessionConfig := SessionConfig{
		ExpiryDuration: 2 * time.Hour,
		RefreshURL:     "/custom/refresh",
		RenewBefore:    10 * time.Minute,
	}

	r := httptest.NewRequest(http.MethodGet, "/test", nil)

	// Apply CSRF middleware to get token
	csrfMiddleware := csrf.Protect(
		[]byte("32-byte-long-secret-key-for-testing!"),
		csrf.Secure(false),
	)
	handler := csrfMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sessionCtx := buildSessionContext(r, sessionConfig, nil)

		// Verify it uses configured duration (2 hours)
		expectedExpiry := time.Now().Add(2 * time.Hour).UnixMilli()
		assert.InDelta(t, expectedExpiry, sessionCtx.ExpiresAt, float64(1000)) // 1 second tolerance

		assert.Equal(t, "/custom/refresh", sessionCtx.RefreshURL)
		assert.NotEmpty(t, sessionCtx.CSRFToken)
	}))

	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)
}

func TestContextBuilder_Build_WithSessionStore(t *testing.T) {
	t.Parallel()

	bundle := createTestBundle()
	logger := logrus.New()
	logger.SetLevel(logrus.FatalLevel)
	metrics := &mockMetrics{}
	sessionConfig := DefaultSessionConfig

	// Mock session store with actual expiry 48 hours from now
	actualExpiry := time.Now().Add(48 * time.Hour)
	store := &mockSessionStore{
		expiresAt: actualExpiry,
	}

	config := Config{
		Endpoints: EndpointConfig{
			GraphQL: "/graphql",
			Stream:  "/stream",
			REST:    "/api",
		},
	}

	builder := NewContextBuilder(config, bundle, sessionConfig, logger, metrics,
		WithSessionStore(store),
	)

	ctx := createTestContext(t, withUserID(42))
	r := httptest.NewRequest(http.MethodGet, "/test", nil)

	// Apply CSRF
	csrfMiddleware := csrf.Protect(
		[]byte("32-byte-long-secret-key-for-testing!"),
		csrf.Secure(false),
	)
	handler := csrfMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		initialCtx, err := builder.Build(ctx, r, "")
		assert.NoError(t, err)
		assert.NotNil(t, initialCtx)
		if err != nil || initialCtx == nil {
			return
		}

		// Verify session uses actual expiry from store (48 hours)
		expectedExpiry := actualExpiry.UnixMilli()
		assert.Equal(t, expectedExpiry, initialCtx.Session.ExpiresAt)
		assert.NotEmpty(t, initialCtx.Session.CSRFToken)
	}))

	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)
}
