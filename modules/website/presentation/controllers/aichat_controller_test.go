package controllers_test

import (
	"net/http"
	"net/url"
	"os"
	"testing"

	"github.com/iota-uz/iota-sdk/modules"
	"github.com/iota-uz/iota-sdk/modules/website/domain/entities/aichatconfig"
	"github.com/iota-uz/iota-sdk/modules/website/presentation/controllers"
	"github.com/iota-uz/iota-sdk/modules/website/services"
	"github.com/iota-uz/iota-sdk/pkg/itf"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

// setupTest creates test suite
func setupTest(t *testing.T) *itf.Suite {
	t.Helper()

	adminUser := itf.User()

	return itf.HTTP(t, modules.BuiltInModules...).
		AsUser(adminUser)
}

func TestAIChatController_SaveConfig_Success(t *testing.T) {
	// Setup test environment
	suite := setupTest(t)
	env := suite.Environment()

	// Register controller
	controller := controllers.NewAIChatController(controllers.AIChatControllerConfig{
		BasePath: BasePath,
		App:      env.App,
	})
	suite.Register(controller)

	// Get service
	configService := env.App.Service(services.AIChatConfigService{}).(*services.AIChatConfigService)

	// Prepare form data
	formData := url.Values{}
	formData.Set("ModelName", "gpt-4")
	formData.Set("ModelType", string(aichatconfig.AIModelTypeOpenAI))
	formData.Set("SystemPrompt", "You are a helpful assistant.")
	formData.Set("Temperature", "0.7")
	formData.Set("MaxTokens", "1024")
	formData.Set("BaseURL", "https://api.openai.com/v1")
	formData.Set("AccessToken", "test-api-key")

	// Execute request
	resp := suite.POST("/website/ai-chat/config").Form(formData).HTMX().Expect(t)

	resp.Status(http.StatusOK)
	assert.Equal(t, BasePath, resp.Header("Hx-Redirect"))

	configs, err := configService.List(env.Ctx)
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
	suite := setupTest(t)
	env := suite.Environment()

	// Register controller
	controller := controllers.NewAIChatController(controllers.AIChatControllerConfig{
		BasePath: BasePath,
		App:      env.App,
	})
	suite.Register(controller)

	// Get service
	configService := env.App.Service(services.AIChatConfigService{}).(*services.AIChatConfigService)

	// Prepare form data with invalid MaxTokens
	formData := url.Values{}
	formData.Set("ModelName", "gpt-4")
	formData.Set("ModelType", string(aichatconfig.AIModelTypeOpenAI))
	formData.Set("SystemPrompt", "You are a helpful assistant.")
	formData.Set("Temperature", "0.7")
	formData.Set("MaxTokens", "abc") // Invalid integer
	formData.Set("BaseURL", "https://api.openai.com/v1")
	formData.Set("AccessToken", "test-api-key")

	// Execute request
	resp := suite.POST("/website/ai-chat/config").Form(formData).HTMX().Expect(t)

	// Should return 400 Bad Request for validation errors
	resp.Status(http.StatusBadRequest)

	// Check that no config was saved
	configs, err := configService.List(env.Ctx)
	require.NoError(t, err)
	require.Empty(t, configs, "No config should be saved with validation error")
}

func TestAIChatController_SaveConfig_UpdateExisting(t *testing.T) {
	// Setup test environment
	suite := setupTest(t)
	env := suite.Environment()

	// Register controller
	controller := controllers.NewAIChatController(controllers.AIChatControllerConfig{
		BasePath: BasePath,
		App:      env.App,
	})
	suite.Register(controller)

	// Get service
	configService := env.App.Service(services.AIChatConfigService{}).(*services.AIChatConfigService)

	// First, create an initial configuration
	options := []aichatconfig.Option{
		aichatconfig.WithSystemPrompt("Initial prompt"),
		aichatconfig.WithTemperature(0.5),
		aichatconfig.WithMaxTokens(512),
		aichatconfig.WithAccessToken("initial-key"),
		aichatconfig.WithIsDefault(true),
		aichatconfig.WithTenantID(env.Tenant.ID),
	}

	initialConfig, err := aichatconfig.New(
		"initial-model",
		aichatconfig.AIModelTypeOpenAI,
		"https://api.openai.com/v1",
		options...,
	)
	require.NoError(t, err)

	_, err = configService.Save(env.Ctx, initialConfig)
	require.NoError(t, err)

	formData := url.Values{}
	formData.Set("ModelName", "updated-model")
	formData.Set("ModelType", string(aichatconfig.AIModelTypeOpenAI))
	formData.Set("SystemPrompt", "Updated prompt")
	formData.Set("Temperature", "0.8")
	formData.Set("MaxTokens", "2048")
	formData.Set("BaseURL", "https://api.openai.com/v1")
	formData.Set("AccessToken", "updated-key")

	// Execute request
	resp := suite.POST("/website/ai-chat/config").Form(formData).HTMX().Expect(t)

	resp.Status(http.StatusOK)
	assert.Equal(t, BasePath, resp.Header("Hx-Redirect"))

	updatedConfig, err := configService.GetDefault(env.Ctx)
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
	suite := setupTest(t)
	env := suite.Environment()

	// Register controller
	controller := controllers.NewAIChatController(controllers.AIChatControllerConfig{
		BasePath: BasePath,
		App:      env.App,
	})
	suite.Register(controller)

	// Get service
	configService := env.App.Service(services.AIChatConfigService{}).(*services.AIChatConfigService)

	// Ensure no configs exist initially
	configs, err := configService.List(env.Ctx)
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

	// Execute request
	resp := suite.POST("/website/ai-chat/config").Form(formData).HTMX().Expect(t)

	resp.Status(http.StatusOK)
	assert.Equal(t, BasePath, resp.Header("Hx-Redirect"))

	configs, err = configService.List(env.Ctx)
	require.NoError(t, err)
	require.Len(t, configs, 1, "One config should be created")

	// Get default config and verify it was set
	defaultConfig, err := configService.GetDefault(env.Ctx)
	require.NoError(t, err)
	assert.Equal(t, "gpt-4", defaultConfig.ModelName())
	assert.True(t, defaultConfig.IsDefault())
}
