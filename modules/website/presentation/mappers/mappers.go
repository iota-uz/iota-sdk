package mappers

import (
	"github.com/iota-uz/iota-sdk/modules/website/domain/entities/aichatconfig"
	"github.com/iota-uz/iota-sdk/modules/website/presentation/viewmodels"
)

// AIConfigToViewModel maps an AI chat configuration domain entity to a view model
func AIConfigToViewModel(config aichatconfig.AIConfig) *viewmodels.AIConfig {
	return &viewmodels.AIConfig{
		ID:           config.ID().String(),
		ModelName:    config.ModelName(),
		ModelType:    string(config.ModelType()),
		SystemPrompt: config.SystemPrompt(),
		Temperature:  config.Temperature(),
		MaxTokens:    config.MaxTokens(),
		BaseURL:      config.BaseURL(),
		CreatedAt:    config.CreatedAt().Format("2006-01-02 15:04:05"),
		UpdatedAt:    config.UpdatedAt().Format("2006-01-02 15:04:05"),
	}
}
