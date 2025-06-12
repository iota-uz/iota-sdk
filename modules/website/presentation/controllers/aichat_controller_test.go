package controllers_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"

	"github.com/gorilla/mux"
	i18n "github.com/iota-uz/go-i18n/v2/i18n"
	"github.com/iota-uz/iota-sdk/modules"
	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/user"
	"github.com/iota-uz/iota-sdk/modules/core/domain/value_objects/internet"
	"github.com/iota-uz/iota-sdk/modules/website/domain/entities/aichatconfig"
	"github.com/iota-uz/iota-sdk/modules/website/presentation/controllers"
	"github.com/iota-uz/iota-sdk/modules/website/services"
	"github.com/iota-uz/iota-sdk/pkg/application"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/constants"
	"github.com/iota-uz/iota-sdk/pkg/testutils"
	"github.com/iota-uz/iota-sdk/pkg/types"
	"github.com/jackc/pgx/v5"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/text/language"
)

var (
	BasePath = "/website/ai-chat"
)

func TestMain(m *testing.M) {
	if err := os.Chdir("../../../../"); err != nil {
		panic(err)
	}
	os.Exit(m.Run())
}

// testFixtures contains common test dependencies
type testFixtures struct {
	ctx     context.Context
	tx      pgx.Tx
	app     application.Application
	router  *mux.Router
	tenant  *composables.Tenant
	service *services.AIChatConfigService
}

// setupTest initializes test dependencies
func setupTest(t *testing.T) *testFixtures {
	t.Helper()

	// Create test database
	testutils.CreateDB(t.Name())
	pool := testutils.NewPool(testutils.DbOpts(t.Name()))

	// Setup real application with required modules
	app, err := testutils.SetupApplication(pool, modules.BuiltInModules...)
	require.NoError(t, err, "Failed to setup application")

	ctx := context.Background()
	// Create tenant first before starting transaction
	tenant, err := testutils.CreateTestTenant(ctx, pool)
	require.NoError(t, err, "Failed to create test tenant")

	// Begin transaction after tenant creation
	tx, err := pool.Begin(ctx)
	require.NoError(t, err, "Failed to begin transaction")

	// Add cleanup to rollback transaction
	t.Cleanup(func() {
		err := tx.Rollback(ctx)
		if err != nil && err != pgx.ErrTxClosed {
			t.Logf("Warning: failed to rollback transaction: %v", err)
		}
	})

	// Create context with transaction and tenant
	ctx = composables.WithTx(ctx, tx)
	ctx = composables.WithTenantID(ctx, tenant.ID)
	ctx = composables.WithParams(ctx, testutils.DefaultParams())

	// Create admin user
	adminEmail, _ := internet.NewEmail("admin@example.com")
	adminUser := user.New(
		"Admin",
		"User",
		adminEmail,
		user.UILanguageEN,
		user.WithID(1),
		user.WithTenantID(tenant.ID),
	)

	// Get service from application
	configService := app.Service(services.AIChatConfigService{}).(*services.AIChatConfigService)

	// Create controller with real application
	controller := controllers.NewAIChatController(controllers.AIChatControllerConfig{
		BasePath: BasePath,
		App:      app,
	})

	// Create router
	router := mux.NewRouter()

	// Apply test middleware to simulate auth and context
	router.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Add test context values that middleware would normally add
			reqCtx := r.Context()
			reqCtx = composables.WithUser(reqCtx, adminUser)
			reqCtx = context.WithValue(reqCtx, constants.UserKey, adminUser)
			reqCtx = context.WithValue(reqCtx, constants.TxKey, tx)
			reqCtx = composables.WithTx(reqCtx, tx)
			reqCtx = context.WithValue(reqCtx, constants.AppKey, app)

			// Important: Add tenant to context
			reqCtx = composables.WithTenantID(reqCtx, tenant.ID)

			// Add logger to context
			logger := logrus.New()
			fieldsLogger := logger.WithFields(logrus.Fields{
				"test": true,
				"path": r.URL.Path,
			})
			reqCtx = context.WithValue(reqCtx, constants.LoggerKey, fieldsLogger)

			// Add params to context (required for auth middleware)
			params := &composables.Params{
				IP:            "127.0.0.1",
				UserAgent:     "test-agent",
				Authenticated: true, // Important for RedirectNotAuthenticated middleware
				Request:       r,
				Writer:        w,
			}
			reqCtx = composables.WithParams(reqCtx, params)

			// Mock localizer context
			localizer := i18n.NewLocalizer(app.Bundle(), "en")

			// Add PageContext
			parsedURL, _ := url.Parse("/website/ai-chat")
			reqCtx = composables.WithPageCtx(reqCtx, &types.PageContext{
				Locale:    language.English,
				URL:       parsedURL,
				Localizer: localizer,
			})

			next.ServeHTTP(w, r.WithContext(reqCtx))
		})
	})

	controller.Register(router)

	return &testFixtures{
		ctx:     ctx,
		tx:      tx,
		app:     app,
		tenant:  tenant,
		router:  router,
		service: configService,
	}
}

func TestAIChatController_SaveConfig_Success(t *testing.T) {
	// Setup test environment
	fixtures := setupTest(t)

	// Prepare form data
	formData := url.Values{}
	formData.Set("ModelName", "gpt-4")
	formData.Set("ModelType", string(aichatconfig.AIModelTypeOpenAI))
	formData.Set("SystemPrompt", "You are a helpful assistant.")
	formData.Set("Temperature", "0.7")
	formData.Set("MaxTokens", "1024")
	formData.Set("BaseURL", "https://api.openai.com/v1")
	formData.Set("AccessToken", "test-api-key")

	req := httptest.NewRequest(http.MethodPost, "/website/ai-chat/config", strings.NewReader(formData.Encode()))
	req = req.WithContext(fixtures.ctx)
	req.Header.Set("Hx-Request", "true")
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	// Create response recorder
	rr := httptest.NewRecorder()

	// Execute request
	fixtures.router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Logf("Response body: %s", rr.Body.String())
		t.Logf("Response headers: %v", rr.Header())
	}
	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Equal(t, BasePath, rr.Header().Get("Hx-Redirect"))

	configs, err := fixtures.service.List(fixtures.ctx)
	require.NoError(t, err)
	require.Len(t, configs, 1, "One config should be saved")

	savedConfig := configs[0]
	assert.Equal(t, "gpt-4", savedConfig.ModelName())
	assert.Equal(t, aichatconfig.AIModelTypeOpenAI, savedConfig.ModelType())
	assert.Equal(t, "You are a helpful assistant.", savedConfig.SystemPrompt())
	assert.InEpsilon(t, float32(0.7), savedConfig.Temperature(), 0.01)
	assert.Equal(t, 1024, savedConfig.MaxTokens())
	assert.Equal(t, "https://api.openai.com/v1", savedConfig.BaseURL())
	assert.Equal(t, "test-api-key", savedConfig.AccessToken())
	assert.True(t, savedConfig.IsDefault())
}

func TestAIChatController_SaveConfig_ValidationError(t *testing.T) {
	// Setup test environment
	fixtures := setupTest(t)

	// Prepare invalid form data
	formData := url.Values{}
	formData.Set("ModelName", "") // Required field is empty
	formData.Set("ModelType", string(aichatconfig.AIModelTypeOpenAI))
	formData.Set("SystemPrompt", "You are a helpful assistant.")
	formData.Set("Temperature", "3.0") // Invalid temperature (should be 0.0-2.0)
	formData.Set("MaxTokens", "abc")   // Invalid number
	formData.Set("BaseURL", "")        // Required field is empty

	// Create request with form data
	req := httptest.NewRequest(http.MethodPost, "/website/ai-chat/config", strings.NewReader(formData.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	// Create response recorder
	rr := httptest.NewRecorder()

	// Execute request
	fixtures.router.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Logf("Response body: %s", rr.Body.String())
		t.Logf("Response headers: %v", rr.Header())
	}
	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestAIChatController_SaveConfig_UpdateExisting(t *testing.T) {
	fixtures := setupTest(t)

	// First, create an initial configuration
	options := []aichatconfig.Option{
		aichatconfig.WithSystemPrompt("Initial prompt"),
		aichatconfig.WithTemperature(0.5),
		aichatconfig.WithMaxTokens(512),
		aichatconfig.WithAccessToken("initial-key"),
		aichatconfig.WithIsDefault(true),
		aichatconfig.WithTenantID(fixtures.tenant.ID),
	}

	initialConfig, err := aichatconfig.New(
		"initial-model",
		aichatconfig.AIModelTypeOpenAI,
		"https://api.openai.com/v1",
		options...,
	)
	require.NoError(t, err)

	_, err = fixtures.service.Save(fixtures.ctx, initialConfig)
	require.NoError(t, err)

	formData := url.Values{}
	formData.Set("ModelName", "updated-model")
	formData.Set("ModelType", string(aichatconfig.AIModelTypeOpenAI))
	formData.Set("SystemPrompt", "Updated prompt")
	formData.Set("Temperature", "0.8")
	formData.Set("MaxTokens", "2048")
	formData.Set("BaseURL", "https://api.openai.com/v1")
	formData.Set("AccessToken", "updated-key")

	// Create request with form data
	req := httptest.NewRequest(http.MethodPost, "/website/ai-chat/config", strings.NewReader(formData.Encode()))
	req.Header.Set("Hx-Request", "true")
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	rr := httptest.NewRecorder()

	fixtures.router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Logf("Response body: %s", rr.Body.String())
		t.Logf("Response headers: %v", rr.Header())
	}
	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Equal(t, BasePath, rr.Header().Get("Hx-Redirect"))

	updatedConfig, err := fixtures.service.GetDefault(fixtures.ctx)
	require.NoError(t, err)

	assert.Equal(t, "updated-model", updatedConfig.ModelName())
	assert.Equal(t, "Updated prompt", updatedConfig.SystemPrompt())
	assert.InEpsilon(t, float32(0.8), updatedConfig.Temperature(), 0.01)
	assert.Equal(t, 2048, updatedConfig.MaxTokens())
	assert.Equal(t, "updated-key", updatedConfig.AccessToken())
	assert.True(t, updatedConfig.IsDefault())
}

func TestAIChatController_SaveConfig_FirstConfigSetsDefault(t *testing.T) {
	// Setup test environment
	fixtures := setupTest(t)

	// Ensure no configs exist initially
	configs, err := fixtures.service.List(fixtures.ctx)
	require.NoError(t, err)
	require.Empty(t, configs, "No configs should exist initially")

	// Prepare form data
	formData := url.Values{}
	formData.Set("ModelName", "gpt-4")
	formData.Set("ModelType", string(aichatconfig.AIModelTypeOpenAI))
	formData.Set("SystemPrompt", "You are a helpful assistant.")
	formData.Set("Temperature", "0.7")
	formData.Set("MaxTokens", "1024")
	formData.Set("BaseURL", "https://api.openai.com/v1")
	formData.Set("AccessToken", "test-api-key")

	// Create request with form data
	req := httptest.NewRequest(http.MethodPost, "/website/ai-chat/config", strings.NewReader(formData.Encode()))
	req.Header.Set("Hx-Request", "true")
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	rr := httptest.NewRecorder()
	fixtures.router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Logf("Response body: %s", rr.Body.String())
		t.Logf("Response headers: %v", rr.Header())
	}
	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Equal(t, BasePath, rr.Header().Get("Hx-Redirect"))

	configs, err = fixtures.service.List(fixtures.ctx)
	require.NoError(t, err)
	require.Len(t, configs, 1, "One config should be created")

	// Get default config and verify it was set
	defaultConfig, err := fixtures.service.GetDefault(fixtures.ctx)
	require.NoError(t, err)
	assert.Equal(t, "gpt-4", defaultConfig.ModelName())
	assert.True(t, defaultConfig.IsDefault())
}
