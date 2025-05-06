package dtos

import (
	"context"
	"strconv"

	"github.com/iota-uz/iota-sdk/modules/website/domain/entities/aichatconfig"
)

// AIConfigDTO represents a data transfer object for AI chat configuration
type AIConfigDTO struct {
	ModelName    string
	ModelType    string
	SystemPrompt string
	Temperature  string
	MaxTokens    string
	BaseURL      string
	AccessToken  string
}

// ToEntity converts the DTO to a domain entity
func (dto *AIConfigDTO) ToEntity() (aichatconfig.AIConfig, error) {
	// Parse Temperature
	var temperature float32
	if dto.Temperature != "" {
		temp, err := strconv.ParseFloat(dto.Temperature, 32)
		if err != nil {
			return nil, err
		}
		temperature = float32(temp)
	} else {
		temperature = 0.7 // Default value
	}

	// Parse MaxTokens
	var maxTokens int
	if dto.MaxTokens != "" {
		tokens, err := strconv.Atoi(dto.MaxTokens)
		if err != nil {
			return nil, err
		}
		maxTokens = tokens
	} else {
		maxTokens = 1024 // Default value
	}

	// Set default base URL if not provided
	baseURL := dto.BaseURL
	if baseURL == "" {
		baseURL = "https://api.openai.com/v1"
	}

	// Create the entity
	options := []aichatconfig.Option{
		aichatconfig.WithTemperature(temperature),
		aichatconfig.WithMaxTokens(maxTokens),
	}

	// Add access token if provided
	if dto.AccessToken != "" {
		options = append(options, aichatconfig.WithAccessToken(dto.AccessToken))
	}

	modelType := aichatconfig.AIModelType(dto.ModelType)

	return aichatconfig.New(
		dto.ModelName,
		modelType,
		dto.SystemPrompt,
		baseURL,
		options...,
	)
}

// Ok validates the DTO
func (dto *AIConfigDTO) Ok(ctx context.Context) (map[string]string, bool) {
	errors := make(map[string]string)

	// Validate ModelName
	if dto.ModelName == "" {
		errors["ModelName"] = "Model name is required"
	}

	// Validate ModelType
	if dto.ModelType == "" {
		errors["ModelType"] = "Model type is required"
	} else if dto.ModelType != string(aichatconfig.AIModelTypeOpenAI) {
		errors["ModelType"] = "Invalid model type"
	}

	// Validate BaseURL
	if dto.BaseURL == "" {
		errors["BaseURL"] = "Base URL is required"
	}

	// Validate Temperature
	if dto.Temperature != "" {
		temp, err := strconv.ParseFloat(dto.Temperature, 32)
		if err != nil {
			errors["Temperature"] = "Temperature must be a valid number"
		} else if temp < 0.0 || temp > 2.0 {
			errors["Temperature"] = "Temperature must be between 0.0 and 2.0"
		}
	}

	// Validate MaxTokens
	if dto.MaxTokens != "" {
		tokens, err := strconv.Atoi(dto.MaxTokens)
		if err != nil {
			errors["MaxTokens"] = "Max tokens must be a valid number"
		} else if tokens <= 0 {
			errors["MaxTokens"] = "Max tokens must be positive"
		}
	}

	return errors, len(errors) == 0
}
