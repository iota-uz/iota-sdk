package persistence_test

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/modules/website/domain/entities/aichatconfig"
	"github.com/iota-uz/iota-sdk/modules/website/infrastructure/persistence"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAIChatConfigRepository_Save_Create(t *testing.T) {
	t.Parallel()
	fixtures := setupTest(t)

	repo := persistence.NewAIChatConfigRepository()

	config, err := aichatconfig.New(
		"gpt-4",
		aichatconfig.AIModelTypeOpenAI,
		"You are a helpful assistant",
		aichatconfig.WithTemperature(0.8),
		aichatconfig.WithMaxTokens(2048),
	)
	require.NoError(t, err)
	require.NotNil(t, config)

	savedConfig, err := repo.Save(fixtures.ctx, config)
	require.NoError(t, err)
	require.NotNil(t, savedConfig)

	assert.NotEqual(t, uuid.Nil, savedConfig.ID())
	assert.Equal(t, "gpt-4", savedConfig.ModelName())
	assert.Equal(t, aichatconfig.AIModelTypeOpenAI, savedConfig.ModelType())
	assert.Equal(t, "You are a helpful assistant", savedConfig.SystemPrompt())
	assert.Equal(t, float32(0.8), savedConfig.Temperature())
	assert.Equal(t, 2048, savedConfig.MaxTokens())
	assert.False(t, savedConfig.CreatedAt().IsZero())
	assert.False(t, savedConfig.UpdatedAt().IsZero())
}

func TestAIChatConfigRepository_Save_Update(t *testing.T) {
	t.Parallel()
	fixtures := setupTest(t)

	// Create a new repository instance
	repo := persistence.NewAIChatConfigRepository()

	// Create and save a new config
	originalConfig, err := aichatconfig.New(
		"gpt-3.5-turbo",
		aichatconfig.AIModelTypeOpenAI,
		"Original system prompt",
		aichatconfig.WithTemperature(0.7),
		aichatconfig.WithMaxTokens(1024),
	)
	require.NoError(t, err)

	savedConfig, err := repo.Save(fixtures.ctx, originalConfig)
	require.NoError(t, err)
	require.NotNil(t, savedConfig)

	// Wait a short time to ensure timestamps will differ
	time.Sleep(10 * time.Millisecond)

	// Update the config
	updatedConfigTemp, err := savedConfig.WithSystemPrompt("Updated system prompt")
	require.NoError(t, err)

	updatedConfigFinal, err := updatedConfigTemp.WithTemperature(0.9)
	require.NoError(t, err)

	// Save the updated config
	finalConfig, err := repo.Save(fixtures.ctx, updatedConfigFinal)
	require.NoError(t, err)
	require.NotNil(t, finalConfig)

	// Verify the updated config
	assert.Equal(t, savedConfig.ID(), finalConfig.ID())
	assert.Equal(t, "gpt-3.5-turbo", finalConfig.ModelName())
	assert.Equal(t, aichatconfig.AIModelTypeOpenAI, finalConfig.ModelType())
	assert.Equal(t, "Updated system prompt", finalConfig.SystemPrompt())
	assert.Equal(t, float32(0.9), finalConfig.Temperature())
	assert.Equal(t, 1024, finalConfig.MaxTokens())
	assert.Equal(t, savedConfig.CreatedAt(), finalConfig.CreatedAt())
	assert.True(t, finalConfig.UpdatedAt().After(savedConfig.UpdatedAt()))
}

func TestAIChatConfigRepository_GetByID(t *testing.T) {
	fixtures := setupTest(t)

	// Create a new repository instance
	repo := persistence.NewAIChatConfigRepository()

	// Create and save a new config
	config, err := aichatconfig.New(
		"gpt-4",
		aichatconfig.AIModelTypeOpenAI,
		"Test system prompt",
		aichatconfig.WithTemperature(0.8),
		aichatconfig.WithMaxTokens(2048),
	)
	require.NoError(t, err)

	savedConfig, err := repo.Save(fixtures.ctx, config)
	require.NoError(t, err)
	require.NotNil(t, savedConfig)

	// Get the config by ID
	retrievedConfig, err := repo.GetByID(fixtures.ctx, savedConfig.ID())
	require.NoError(t, err)
	require.NotNil(t, retrievedConfig)

	// Verify the retrieved config
	assert.Equal(t, savedConfig.ID(), retrievedConfig.ID())
	assert.Equal(t, savedConfig.ModelName(), retrievedConfig.ModelName())
	assert.Equal(t, savedConfig.ModelType(), retrievedConfig.ModelType())
	assert.Equal(t, savedConfig.SystemPrompt(), retrievedConfig.SystemPrompt())
	assert.Equal(t, savedConfig.Temperature(), retrievedConfig.Temperature())
	assert.Equal(t, savedConfig.MaxTokens(), retrievedConfig.MaxTokens())
	assert.Equal(t, savedConfig.CreatedAt().Unix(), retrievedConfig.CreatedAt().Unix())
	assert.Equal(t, savedConfig.UpdatedAt().Unix(), retrievedConfig.UpdatedAt().Unix())
}

func TestAIChatConfigRepository_GetByID_NotFound(t *testing.T) {
	t.Parallel()
	fixtures := setupTest(t)

	// Create a new repository instance
	repo := persistence.NewAIChatConfigRepository()

	// Try to get a non-existent config
	_, err := repo.GetByID(fixtures.ctx, uuid.New())
	assert.ErrorIs(t, err, aichatconfig.ErrConfigNotFound)
}

func TestAIChatConfigRepository_List(t *testing.T) {
	t.Parallel()
	fixtures := setupTest(t)

	// Create a new repository instance
	repo := persistence.NewAIChatConfigRepository()

	// Create and save multiple configs
	config1, err := aichatconfig.New(
		"gpt-3.5-turbo",
		aichatconfig.AIModelTypeOpenAI,
		"Config 1 system prompt",
	)
	require.NoError(t, err)

	config2, err := aichatconfig.New(
		"gpt-4",
		aichatconfig.AIModelTypeOpenAI,
		"Config 2 system prompt",
		aichatconfig.WithTemperature(0.9),
	)
	require.NoError(t, err)

	savedConfig1, err := repo.Save(fixtures.ctx, config1)
	require.NoError(t, err)

	savedConfig2, err := repo.Save(fixtures.ctx, config2)
	require.NoError(t, err)

	// List all configs
	configs, err := repo.List(fixtures.ctx)
	require.NoError(t, err)
	require.NotNil(t, configs)

	// Verify the list contains the saved configs
	assert.GreaterOrEqual(t, len(configs), 2)

	// Create a map of IDs to make it easier to check
	configMap := make(map[string]aichatconfig.AIConfig)
	for _, cfg := range configs {
		configMap[cfg.ID().String()] = cfg
	}

	// Check that our saved configs are in the list
	cfg1, exists := configMap[savedConfig1.ID().String()]
	assert.True(t, exists)
	assert.Equal(t, savedConfig1.ModelName(), cfg1.ModelName())
	assert.Equal(t, savedConfig1.SystemPrompt(), cfg1.SystemPrompt())

	cfg2, exists := configMap[savedConfig2.ID().String()]
	assert.True(t, exists)
	assert.Equal(t, savedConfig2.ModelName(), cfg2.ModelName())
	assert.Equal(t, savedConfig2.SystemPrompt(), cfg2.SystemPrompt())
}

func TestAIChatConfigRepository_SetDefault(t *testing.T) {
	t.Parallel()
	fixtures := setupTest(t)

	// Create a new repository instance
	repo := persistence.NewAIChatConfigRepository()

	// Create and save a new config
	config, err := aichatconfig.New(
		"gpt-4",
		aichatconfig.AIModelTypeOpenAI,
		"Test system prompt",
	)
	require.NoError(t, err)

	savedConfig, err := repo.Save(fixtures.ctx, config)
	require.NoError(t, err)
	require.NotNil(t, savedConfig)

	// Set the config as default
	err = repo.SetDefault(fixtures.ctx, savedConfig.ID())
	require.NoError(t, err)

	// Get the default config
	defaultConfig, err := repo.GetDefault(fixtures.ctx)
	require.NoError(t, err)
	require.NotNil(t, defaultConfig)

	// Verify the default config
	assert.Equal(t, savedConfig.ID(), defaultConfig.ID())
	assert.Equal(t, savedConfig.ModelName(), defaultConfig.ModelName())
}

func TestAIChatConfigRepository_SetDefault_MultipleTimes(t *testing.T) {
	t.Parallel()
	fixtures := setupTest(t)

	// Create a new repository instance
	repo := persistence.NewAIChatConfigRepository()

	// Create and save two configs
	config1, err := aichatconfig.New(
		"gpt-3.5-turbo",
		aichatconfig.AIModelTypeOpenAI,
		"Config 1 system prompt",
	)
	require.NoError(t, err)

	config2, err := aichatconfig.New(
		"gpt-4",
		aichatconfig.AIModelTypeOpenAI,
		"Config 2 system prompt",
	)
	require.NoError(t, err)

	savedConfig1, err := repo.Save(fixtures.ctx, config1)
	require.NoError(t, err)

	savedConfig2, err := repo.Save(fixtures.ctx, config2)
	require.NoError(t, err)

	// Set the first config as default
	err = repo.SetDefault(fixtures.ctx, savedConfig1.ID())
	require.NoError(t, err)

	// Verify the first config is default
	defaultConfig, err := repo.GetDefault(fixtures.ctx)
	require.NoError(t, err)
	assert.Equal(t, savedConfig1.ID(), defaultConfig.ID())

	// Set the second config as default
	err = repo.SetDefault(fixtures.ctx, savedConfig2.ID())
	require.NoError(t, err)

	// Verify the second config is now default
	defaultConfig, err = repo.GetDefault(fixtures.ctx)
	require.NoError(t, err)
	assert.Equal(t, savedConfig2.ID(), defaultConfig.ID())
}

func TestAIChatConfigRepository_SetDefault_NonExistentConfig(t *testing.T) {
	t.Parallel()
	fixtures := setupTest(t)

	// Create a new repository instance
	repo := persistence.NewAIChatConfigRepository()

	// Try to set a non-existent config as default
	err := repo.SetDefault(fixtures.ctx, uuid.New())
	assert.ErrorIs(t, err, aichatconfig.ErrConfigNotFound)
}

func TestAIChatConfigRepository_Delete(t *testing.T) {
	t.Parallel()
	fixtures := setupTest(t)

	// Create a new repository instance
	repo := persistence.NewAIChatConfigRepository()

	// Create and save a new config
	config, err := aichatconfig.New(
		"gpt-4",
		aichatconfig.AIModelTypeOpenAI,
		"Test system prompt",
	)
	require.NoError(t, err)

	savedConfig, err := repo.Save(fixtures.ctx, config)
	require.NoError(t, err)
	require.NotNil(t, savedConfig)

	// Delete the config
	err = repo.Delete(fixtures.ctx, savedConfig.ID())
	require.NoError(t, err)

	// Try to get the deleted config
	_, err = repo.GetByID(fixtures.ctx, savedConfig.ID())
	assert.ErrorIs(t, err, aichatconfig.ErrConfigNotFound)
}

func TestAIChatConfigRepository_Delete_DefaultConfig(t *testing.T) {
	t.Parallel()
	fixtures := setupTest(t)

	// Create a new repository instance
	repo := persistence.NewAIChatConfigRepository()

	// Create and save a new config
	config, err := aichatconfig.New(
		"gpt-4",
		aichatconfig.AIModelTypeOpenAI,
		"Test system prompt",
	)
	require.NoError(t, err)

	savedConfig, err := repo.Save(fixtures.ctx, config)
	require.NoError(t, err)
	require.NotNil(t, savedConfig)

	// Set the config as default
	err = repo.SetDefault(fixtures.ctx, savedConfig.ID())
	require.NoError(t, err)

	// Try to delete the default config
	err = repo.Delete(fixtures.ctx, savedConfig.ID())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "cannot delete default config")

	// Verify the config still exists
	retrievedConfig, err := repo.GetByID(fixtures.ctx, savedConfig.ID())
	require.NoError(t, err)
	assert.Equal(t, savedConfig.ID(), retrievedConfig.ID())
}

func TestAIChatConfigRepository_Delete_NonExistentConfig(t *testing.T) {
	t.Parallel()
	fixtures := setupTest(t)

	// Create a new repository instance
	repo := persistence.NewAIChatConfigRepository()

	// Try to delete a non-existent config
	err := repo.Delete(fixtures.ctx, uuid.New())
	assert.ErrorIs(t, err, aichatconfig.ErrConfigNotFound)
}

func TestAIChatConfigRepository_GetDefault_NoDefaultConfig(t *testing.T) {
	t.Parallel()
	fixtures := setupTest(t)

	// Create a new repository instance
	repo := persistence.NewAIChatConfigRepository()

	// Create and save a new config without setting it as default
	config, err := aichatconfig.New(
		"gpt-4",
		aichatconfig.AIModelTypeOpenAI,
		"Test system prompt",
	)
	require.NoError(t, err)

	_, err = repo.Save(fixtures.ctx, config)
	require.NoError(t, err)

	// Try to get the default config when none is set
	_, err = repo.GetDefault(fixtures.ctx)
	assert.ErrorIs(t, err, aichatconfig.ErrConfigNotFound)
}
