package services

import (
	"context"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/modules/website/domain/entities/aichatconfig"
)

// AIChatConfigService handles operations for AI chat configurations
type AIChatConfigService struct {
	repository aichatconfig.Repository
}

// NewAIChatConfigService creates a new instance of AIChatConfigService
func NewAIChatConfigService(repository aichatconfig.Repository) *AIChatConfigService {
	return &AIChatConfigService{
		repository: repository,
	}
}

// GetByID retrieves an AI chat configuration by its ID
func (s *AIChatConfigService) GetByID(ctx context.Context, id uuid.UUID) (aichatconfig.AIConfig, error) {
	return s.repository.GetByID(ctx, id)
}

// GetDefault retrieves the default AI chat configuration
func (s *AIChatConfigService) GetDefault(ctx context.Context) (aichatconfig.AIConfig, error) {
	return s.repository.GetDefault(ctx)
}

// Save creates or updates an AI chat configuration
func (s *AIChatConfigService) Save(ctx context.Context, config aichatconfig.AIConfig) (aichatconfig.AIConfig, error) {
	return s.repository.Save(ctx, config)
}

// Delete removes an AI chat configuration
func (s *AIChatConfigService) Delete(ctx context.Context, id uuid.UUID) error {
	return s.repository.Delete(ctx, id)
}

// List retrieves all AI chat configurations
func (s *AIChatConfigService) List(ctx context.Context) ([]aichatconfig.AIConfig, error) {
	return s.repository.List(ctx)
}

// SetDefault sets an AI chat configuration as the default
func (s *AIChatConfigService) SetDefault(ctx context.Context, id uuid.UUID) error {
	return s.repository.SetDefault(ctx, id)
}
