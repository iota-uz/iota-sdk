package services

import (
	"context"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/modules/website/domain/entities/aichatconfig"
)

type AIChatConfigService struct {
	repository aichatconfig.Repository
}

func NewAIChatConfigService(repository aichatconfig.Repository) *AIChatConfigService {
	return &AIChatConfigService{
		repository: repository,
	}
}

func (s *AIChatConfigService) GetByID(ctx context.Context, id uuid.UUID) (aichatconfig.AIConfig, error) {
	return s.repository.GetByID(ctx, id)
}

func (s *AIChatConfigService) GetDefault(ctx context.Context) (aichatconfig.AIConfig, error) {
	return s.repository.GetDefault(ctx)
}

func (s *AIChatConfigService) Save(ctx context.Context, config aichatconfig.AIConfig) (aichatconfig.AIConfig, error) {
	return s.repository.Save(ctx, config)
}

func (s *AIChatConfigService) Delete(ctx context.Context, id uuid.UUID) error {
	return s.repository.Delete(ctx, id)
}

func (s *AIChatConfigService) List(ctx context.Context) ([]aichatconfig.AIConfig, error) {
	return s.repository.List(ctx)
}

func (s *AIChatConfigService) SetDefault(ctx context.Context, id uuid.UUID) error {
	return s.repository.SetDefault(ctx, id)
}
