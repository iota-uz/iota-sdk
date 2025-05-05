package persistence

import (
	"fmt"

	"github.com/go-faster/errors"
	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/modules/website/domain/entities/aichatconfig"
	"github.com/iota-uz/iota-sdk/modules/website/infrastructure/persistence/models"
)

// ToDBConfig maps a domain entity to a database model
func ToDBConfig(config aichatconfig.AIConfig) models.AIChatConfig {
	return models.AIChatConfig{
		ID:           config.ID().String(),
		ModelName:    config.ModelName(),
		ModelType:    string(config.ModelType()),
		SystemPrompt: config.SystemPrompt(),
		Temperature:  config.Temperature(),
		MaxTokens:    config.MaxTokens(),
		CreatedAt:    config.CreatedAt(),
		UpdatedAt:    config.UpdatedAt(),
	}
}

// ToDomainConfig maps a database model to a domain entity
func ToDomainConfig(model models.AIChatConfig) (aichatconfig.AIConfig, error) {
	id, err := uuid.Parse(model.ID)
	if err != nil {
		return nil, errors.Wrap(err, fmt.Sprintf("failed to parse UUID from string: %s", model.ID))
	}

	return aichatconfig.New(
		model.ModelName,
		aichatconfig.AIModelType(model.ModelType),
		model.SystemPrompt,
		aichatconfig.WithID(id),
		aichatconfig.WithTemperature(model.Temperature),
		aichatconfig.WithMaxTokens(model.MaxTokens),
		aichatconfig.WithCreatedAt(model.CreatedAt),
		aichatconfig.WithUpdatedAt(model.UpdatedAt),
	)
}
