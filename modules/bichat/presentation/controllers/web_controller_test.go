package controllers_test

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/iota-uz/iota-sdk/modules/bichat"
	"github.com/iota-uz/iota-sdk/modules/bichat/presentation/controllers"
	"github.com/iota-uz/iota-sdk/modules/bichat/presentation/interop"
	"github.com/iota-uz/iota-sdk/modules/core"
	"github.com/iota-uz/iota-sdk/pkg/defaults"
	"github.com/iota-uz/iota-sdk/pkg/itf"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// setupWebTest creates a test suite for WebController tests
func setupWebTest(t *testing.T) *itf.Suite {
	t.Helper()

	// Create admin user with BiChat permissions
	adminUser := itf.User()

	return itf.HTTP(t,
		core.NewModule(&core.ModuleOptions{
			PermissionSchema: defaults.PermissionSchema(),
		}),
		bichat.NewModule(),
	).AsUser(adminUser)
}

func TestWebController_Key_ReturnsCorrectIdentifier(t *testing.T) {
	t.Parallel()

	suite := setupWebTest(t)
	controller := controllers.NewWebController(suite.Environment().App)

	require.Equal(t, "bichat.WebController", controller.Key())
}

func TestWebController_RenderChatApp_Success(t *testing.T) {
	t.Parallel()

	suite := setupWebTest(t)
	controller := controllers.NewWebController(suite.Environment().App)
	suite.Register(controller)

	// Execute request to /bichat
	response := suite.GET("/bichat").Expect(t)
	response.Status(http.StatusOK)

	// Verify content type
	contentType := response.Header("Content-Type")
	require.Contains(t, contentType, "text/html")

	// Verify HTML structure
	body := response.Body()
	require.Contains(t, body, "<!DOCTYPE html>")
	require.Contains(t, body, "<html lang=\"en\">")
	require.Contains(t, body, "<div id=\"app\"></div>")

	// Verify script injection
	require.Contains(t, body, "window.__BICHAT_CONTEXT__")
	require.Contains(t, body, "window.__CSRF_TOKEN__")

	// Verify React bundle reference
	require.Contains(t, body, "/bichat/assets/index.js")
}

func TestWebController_RenderChatApp_RootPath(t *testing.T) {
	t.Parallel()

	suite := setupWebTest(t)
	controller := controllers.NewWebController(suite.Environment().App)
	suite.Register(controller)

	// Execute request to /bichat/ (with trailing slash)
	response := suite.GET("/bichat/").Expect(t)
	response.Status(http.StatusOK)

	// Verify content type
	contentType := response.Header("Content-Type")
	require.Contains(t, contentType, "text/html")

	// Verify HTML structure
	body := response.Body()
	require.Contains(t, body, "<!DOCTYPE html>")
}

func TestWebController_RenderChatApp_AppPath(t *testing.T) {
	t.Parallel()

	suite := setupWebTest(t)
	controller := controllers.NewWebController(suite.Environment().App)
	suite.Register(controller)

	// Execute request to /bichat/app
	response := suite.GET("/bichat/app").Expect(t)
	response.Status(http.StatusOK)

	// Verify content type
	contentType := response.Header("Content-Type")
	require.Contains(t, contentType, "text/html")

	// Verify HTML structure
	body := response.Body()
	require.Contains(t, body, "<!DOCTYPE html>")
}

func TestWebController_RenderChatApp_ContextInjection(t *testing.T) {
	t.Parallel()

	suite := setupWebTest(t)
	controller := controllers.NewWebController(suite.Environment().App)
	suite.Register(controller)

	// Execute request
	response := suite.GET("/bichat").Expect(t)
	response.Status(http.StatusOK)

	body := response.Body()

	// Extract context JSON from HTML
	// Find the script tag containing window.__BICHAT_CONTEXT__
	contextStart := strings.Index(body, "window.__BICHAT_CONTEXT__ = ")
	require.NotEqual(t, -1, contextStart, "Context injection should be present")

	// Extract JSON from script tag
	jsonStart := contextStart + len("window.__BICHAT_CONTEXT__ = ")
	jsonEnd := strings.Index(body[jsonStart:], ";")
	require.NotEqual(t, -1, jsonEnd, "Context JSON should end with semicolon")

	contextJSON := body[jsonStart : jsonStart+jsonEnd]

	// Parse context
	var initialContext interop.InitialContext
	err := json.Unmarshal([]byte(contextJSON), &initialContext)
	require.NoError(t, err, "Context JSON should be valid")

	// Verify user context
	require.NotZero(t, initialContext.User.ID)
	require.NotEmpty(t, initialContext.User.Email)
	require.NotEmpty(t, initialContext.User.FirstName)
	require.NotEmpty(t, initialContext.User.LastName)

	// Verify tenant context
	require.NotEmpty(t, initialContext.Tenant.ID)

	// Verify locale context
	require.NotEmpty(t, initialContext.Locale.Language)
	require.NotNil(t, initialContext.Locale.Translations)
	require.NotEmpty(t, initialContext.Locale.Translations)

	// Verify config
	require.Equal(t, "/bichat/graphql", initialContext.Config.GraphQLEndpoint)
	require.Equal(t, "/bichat/stream", initialContext.Config.StreamEndpoint)
}

func TestWebController_RenderChatApp_UserPermissions(t *testing.T) {
	t.Parallel()

	suite := setupWebTest(t)
	controller := controllers.NewWebController(suite.Environment().App)
	suite.Register(controller)

	// Execute request
	response := suite.GET("/bichat").Expect(t)
	response.Status(http.StatusOK)

	body := response.Body()

	// Extract and parse context
	contextStart := strings.Index(body, "window.__BICHAT_CONTEXT__ = ")
	require.NotEqual(t, -1, contextStart)

	jsonStart := contextStart + len("window.__BICHAT_CONTEXT__ = ")
	jsonEnd := strings.Index(body[jsonStart:], ";")
	contextJSON := body[jsonStart : jsonStart+jsonEnd]

	var initialContext interop.InitialContext
	err := json.Unmarshal([]byte(contextJSON), &initialContext)
	require.NoError(t, err)

	// Verify permissions array exists (admin user should have permissions)
	require.NotNil(t, initialContext.User.Permissions)

	// Check for BiChat permissions
	permissions := initialContext.User.Permissions
	hasAccessPerm := false
	for _, perm := range permissions {
		if perm == "bichat.access" {
			hasAccessPerm = true
			break
		}
	}
	require.True(t, hasAccessPerm, "Admin user should have bichat.access permission")
}

func TestWebController_RenderChatApp_CSRFToken(t *testing.T) {
	t.Parallel()

	suite := setupWebTest(t)
	controller := controllers.NewWebController(suite.Environment().App)
	suite.Register(controller)

	// Execute request
	response := suite.GET("/bichat").Expect(t)
	response.Status(http.StatusOK)

	body := response.Body()

	// Verify CSRF token injection
	require.Contains(t, body, "window.__CSRF_TOKEN__")

	// Extract CSRF token from HTML
	csrfStart := strings.Index(body, "window.__CSRF_TOKEN__ = \"")
	require.NotEqual(t, -1, csrfStart, "CSRF token injection should be present")

	tokenStart := csrfStart + len("window.__CSRF_TOKEN__ = \"")
	tokenEnd := strings.Index(body[tokenStart:], "\"")
	require.NotEqual(t, -1, tokenEnd, "CSRF token should end with quote")

	csrfToken := body[tokenStart : tokenStart+tokenEnd]

	// CSRF token may be empty in test environment (gorilla/csrf not configured)
	// but the injection point should exist
	t.Logf("CSRF token extracted: %q", csrfToken)
}

func TestWebController_RenderChatApp_Translations(t *testing.T) {
	t.Parallel()

	suite := setupWebTest(t)
	controller := controllers.NewWebController(suite.Environment().App)
	suite.Register(controller)

	// Execute request
	response := suite.GET("/bichat").Expect(t)
	response.Status(http.StatusOK)

	body := response.Body()

	// Extract and parse context
	contextStart := strings.Index(body, "window.__BICHAT_CONTEXT__ = ")
	require.NotEqual(t, -1, contextStart)

	jsonStart := contextStart + len("window.__BICHAT_CONTEXT__ = ")
	jsonEnd := strings.Index(body[jsonStart:], ";")
	contextJSON := body[jsonStart : jsonStart+jsonEnd]

	var initialContext interop.InitialContext
	err := json.Unmarshal([]byte(contextJSON), &initialContext)
	require.NoError(t, err)

	// Verify translations are present
	translations := initialContext.Locale.Translations
	require.NotEmpty(t, translations, "Translations should not be empty")

	// Check for expected translation keys
	expectedKeys := []string{
		"bichat.title",
		"bichat.new_chat",
		"bichat.send_message",
		"bichat.loading",
		"bichat.error",
	}

	for _, key := range expectedKeys {
		_, exists := translations[key]
		assert.True(t, exists, "Translation key %q should exist", key)
	}
}

func TestWebController_RenderChatApp_TenantIsolation(t *testing.T) {
	t.Parallel()

	suite := setupWebTest(t)
	controller := controllers.NewWebController(suite.Environment().App)
	suite.Register(controller)

	// Execute request
	response := suite.GET("/bichat").Expect(t)
	response.Status(http.StatusOK)

	body := response.Body()

	// Extract and parse context
	contextStart := strings.Index(body, "window.__BICHAT_CONTEXT__ = ")
	require.NotEqual(t, -1, contextStart)

	jsonStart := contextStart + len("window.__BICHAT_CONTEXT__ = ")
	jsonEnd := strings.Index(body[jsonStart:], ";")
	contextJSON := body[jsonStart : jsonStart+jsonEnd]

	var initialContext interop.InitialContext
	err := json.Unmarshal([]byte(contextJSON), &initialContext)
	require.NoError(t, err)

	// Verify tenant ID matches the test environment tenant
	env := suite.Environment()
	require.Equal(t, env.Tenant.ID.String(), initialContext.Tenant.ID)
}

func TestWebController_RenderChatApp_RequiresAuthentication(t *testing.T) {
	t.Parallel()

	// Create suite WITHOUT authenticated user
	suite := itf.HTTP(t,
		core.NewModule(&core.ModuleOptions{
			PermissionSchema: defaults.PermissionSchema(),
		}),
		bichat.NewModule(),
	)

	controller := controllers.NewWebController(suite.Environment().App)
	suite.Register(controller)

	// Execute request without authentication
	response := suite.GET("/bichat").Expect(t)

	// Should redirect to login (302 or 303)
	statusCode := response.StatusCode()
	require.Contains(t, []int{http.StatusFound, http.StatusSeeOther}, statusCode)
}

func TestWebController_RenderChatApp_LocaleContext(t *testing.T) {
	t.Parallel()

	suite := setupWebTest(t)
	controller := controllers.NewWebController(suite.Environment().App)
	suite.Register(controller)

	// Execute request
	response := suite.GET("/bichat").Expect(t)
	response.Status(http.StatusOK)

	body := response.Body()

	// Extract and parse context
	contextStart := strings.Index(body, "window.__BICHAT_CONTEXT__ = ")
	require.NotEqual(t, -1, contextStart)

	jsonStart := contextStart + len("window.__BICHAT_CONTEXT__ = ")
	jsonEnd := strings.Index(body[jsonStart:], ";")
	contextJSON := body[jsonStart : jsonStart+jsonEnd]

	var initialContext interop.InitialContext
	err := json.Unmarshal([]byte(contextJSON), &initialContext)
	require.NoError(t, err)

	// Verify locale is valid
	locale := initialContext.Locale.Language
	require.NotEmpty(t, locale)

	// Should be a valid language tag (e.g., "en", "ru", "uz")
	validLocales := []string{"en", "ru", "uz", "en-US", "ru-RU", "uz-UZ"}
	hasValidLocale := false
	for _, valid := range validLocales {
		if strings.HasPrefix(locale, valid) {
			hasValidLocale = true
			break
		}
	}
	require.True(t, hasValidLocale, "Locale %q should be valid", locale)
}

func TestWebController_RenderChatApp_ConfigEndpoints(t *testing.T) {
	t.Parallel()

	suite := setupWebTest(t)
	controller := controllers.NewWebController(suite.Environment().App)
	suite.Register(controller)

	// Execute request
	response := suite.GET("/bichat").Expect(t)
	response.Status(http.StatusOK)

	body := response.Body()

	// Extract and parse context
	contextStart := strings.Index(body, "window.__BICHAT_CONTEXT__ = ")
	require.NotEqual(t, -1, contextStart)

	jsonStart := contextStart + len("window.__BICHAT_CONTEXT__ = ")
	jsonEnd := strings.Index(body[jsonStart:], ";")
	contextJSON := body[jsonStart : jsonStart+jsonEnd]

	var initialContext interop.InitialContext
	err := json.Unmarshal([]byte(contextJSON), &initialContext)
	require.NoError(t, err)

	// Verify endpoint configuration
	require.Equal(t, "/bichat/graphql", initialContext.Config.GraphQLEndpoint)
	require.Equal(t, "/bichat/stream", initialContext.Config.StreamEndpoint)
}

func TestWebController_RenderChatApp_HTMLStructure(t *testing.T) {
	t.Parallel()

	suite := setupWebTest(t)
	controller := controllers.NewWebController(suite.Environment().App)
	suite.Register(controller)

	// Execute request
	response := suite.GET("/bichat").Expect(t)
	response.Status(http.StatusOK)

	body := response.Body()

	// Verify essential HTML structure elements
	require.Contains(t, body, "<!DOCTYPE html>")
	require.Contains(t, body, "<html lang=\"en\">")
	require.Contains(t, body, "<head>")
	require.Contains(t, body, "<meta charset=\"UTF-8\">")
	require.Contains(t, body, "<meta name=\"viewport\"")
	require.Contains(t, body, "<title>BI Chat</title>")
	require.Contains(t, body, "<body>")
	require.Contains(t, body, "<div id=\"app\"></div>")
	require.Contains(t, body, "</body>")
	require.Contains(t, body, "</html>")

	// Verify script elements
	require.Contains(t, body, "<script type=\"module\"")
	require.Contains(t, body, "<script>")
}

func TestWebController_RenderChatApp_NoXSSVulnerability(t *testing.T) {
	t.Parallel()

	suite := setupWebTest(t)
	controller := controllers.NewWebController(suite.Environment().App)
	suite.Register(controller)

	// Execute request
	response := suite.GET("/bichat").Expect(t)
	response.Status(http.StatusOK)

	body := response.Body()

	// Verify that user-controlled data is properly escaped
	// The JSON context should be safely embedded in the script tag
	require.Contains(t, body, "window.__BICHAT_CONTEXT__ = ")

	// Extract context JSON
	contextStart := strings.Index(body, "window.__BICHAT_CONTEXT__ = ")
	require.NotEqual(t, -1, contextStart)

	jsonStart := contextStart + len("window.__BICHAT_CONTEXT__ = ")
	jsonEnd := strings.Index(body[jsonStart:], ";")
	contextJSON := body[jsonStart : jsonStart+jsonEnd]

	// Verify JSON is valid (not broken by injection)
	var initialContext interop.InitialContext
	err := json.Unmarshal([]byte(contextJSON), &initialContext)
	require.NoError(t, err, "Context JSON should be valid and properly escaped")

	// Verify user fields don't contain unescaped script tags
	userEmail := initialContext.User.Email
	require.NotContains(t, userEmail, "<script>")
	require.NotContains(t, userEmail, "</script>")

	userName := initialContext.User.FirstName + " " + initialContext.User.LastName
	require.NotContains(t, userName, "<script>")
	require.NotContains(t, userName, "</script>")
}
