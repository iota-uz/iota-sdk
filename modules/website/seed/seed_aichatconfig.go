// Package seed provides this package.
package seed

import (
	"context"

	"github.com/iota-uz/iota-sdk/modules/website/domain/entities/aichatconfig"
	"github.com/iota-uz/iota-sdk/pkg/application"
	"github.com/sirupsen/logrus"
)

func AIChatConfigSeedFunc(configs ...aichatconfig.AIConfig) application.SeedFunc {
	return application.Seed(func(ctx context.Context, configRepository aichatconfig.Repository, logger logrus.FieldLogger) error {
		for _, cfg := range configs {
			if _, err := configRepository.Save(ctx, cfg); err != nil {
				logger.Errorf("Failed to save AI chat config %s: %v", cfg.ModelName(), err)
				return err
			}
			logger.Infof("AI chat config %s saved", cfg.ModelName())
		}
		return nil
	})
}
