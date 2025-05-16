package seed

import (
	"context"

	"github.com/iota-uz/iota-sdk/modules/website/domain/entities/aichatconfig"
	"github.com/iota-uz/iota-sdk/modules/website/infrastructure/persistence"
	"github.com/iota-uz/iota-sdk/pkg/application"
	"github.com/iota-uz/iota-sdk/pkg/configuration"
)

func AIChatConfigSeedFunc(configs ...aichatconfig.AIConfig) application.SeedFunc {
	return func(ctx context.Context, app application.Application) error {
		logger := configuration.Use().Logger()
		configRepository := persistence.NewAIChatConfigRepository()

		for _, cfg := range configs {
			if _, err := configRepository.Save(ctx, cfg); err != nil {
				logger.Errorf("Failed to save AI chat config %s: %v", cfg.ModelName(), err)
				return err
			}
			logger.Infof("AI chat config %s saved", cfg.ModelName())
		}
		return nil
	}
}
