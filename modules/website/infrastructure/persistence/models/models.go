package models

import (
	"time"
)

type AIChatConfig struct {
	ID           string
	TenantID     string
	ModelName    string
	ModelType    string
	SystemPrompt string
	Temperature  float32
	MaxTokens    int
	BaseURL      string
	AccessToken  string
	IsDefault    bool
	CreatedAt    time.Time
	UpdatedAt    time.Time
}
