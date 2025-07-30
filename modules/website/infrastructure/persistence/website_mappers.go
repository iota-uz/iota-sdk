package persistence

import (
	"fmt"

	"github.com/go-faster/errors"
	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/modules/website/domain/entities/aichatconfig"
	"github.com/iota-uz/iota-sdk/modules/website/domain/entities/chatthread"
	"github.com/iota-uz/iota-sdk/modules/website/infrastructure/persistence/models"
)

// ToDBConfig maps a domain entity to a database model
func ToDBConfig(config aichatconfig.AIConfig) models.AIChatConfig {
	return models.AIChatConfig{
		ID:           config.ID().String(),
		TenantID:     config.TenantID().String(),
		ModelName:    config.ModelName(),
		ModelType:    string(config.ModelType()),
		SystemPrompt: config.SystemPrompt(),
		Temperature:  config.Temperature(),
		MaxTokens:    config.MaxTokens(),
		BaseURL:      config.BaseURL(),
		AccessToken:  config.AccessToken(),
		IsDefault:    config.IsDefault(),
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

	tenantID, err := uuid.Parse(model.TenantID)
	if err != nil {
		return nil, errors.Wrap(err, fmt.Sprintf("failed to parse tenant UUID from string: %s", model.TenantID))
	}

	options := []aichatconfig.Option{
		aichatconfig.WithID(id),
		aichatconfig.WithTenantID(tenantID),
		aichatconfig.WithTemperature(model.Temperature),
		aichatconfig.WithMaxTokens(model.MaxTokens),
		aichatconfig.WithAccessToken(model.AccessToken),
		aichatconfig.WithIsDefault(model.IsDefault),
		aichatconfig.WithCreatedAt(model.CreatedAt),
		aichatconfig.WithUpdatedAt(model.UpdatedAt),
	}

	if model.SystemPrompt != "" {
		options = append(options, aichatconfig.WithSystemPrompt(model.SystemPrompt))
	}

	return aichatconfig.New(
		model.ModelName,
		aichatconfig.AIModelType(model.ModelType),
		model.BaseURL,
		options...,
	)
}

func ToDomainChatThread(model models.ChatThread) (chatthread.ChatThread, error) {
	id, err := uuid.Parse(model.ID)
	if err != nil {
		return nil, errors.Wrap(err, fmt.Sprintf("failed to parse UUID from string: %s", model.ID))
	}
	return chatthread.New(model.ChatID, nil, chatthread.WithTimestamp(model.Timestamp), chatthread.WithID(id)), nil
}

func ToDBChatThread(thread chatthread.ChatThread) models.ChatThread {
	return models.ChatThread{
		ID:        thread.ID().String(),
		ChatID:    thread.ChatID(),
		Timestamp: thread.Timestamp(),
	}
}
