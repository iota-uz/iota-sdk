package models

import (
	"time"
)

type AIChatConfig struct {
	ID           string
	ModelName    string
	ModelType    string
	SystemPrompt string
	Temperature  float32
	MaxTokens    int
	IsDefault    bool
	CreatedAt    time.Time
	UpdatedAt    time.Time
}
