package seed

import (
	"context"

	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/group"
	"github.com/iota-uz/iota-sdk/modules/core/infrastructure/persistence"
	"github.com/iota-uz/iota-sdk/pkg/application"
	"github.com/iota-uz/iota-sdk/pkg/configuration"
)

func GroupsSeedFunc(groups ...group.Group) application.SeedFunc {
	return func(ctx context.Context, app application.Application) error {
		logger := configuration.Use().Logger()
		groupRepository := persistence.NewGroupRepository(
			persistence.NewUserRepository(persistence.NewUploadRepository()),
			persistence.NewRoleRepository(),
		)

		for _, g := range groups {
			logger.Infof("Saving group: %s", g.Name())
			if _, err := groupRepository.Save(ctx, g); err != nil {
				logger.Errorf("Failed to save group %s: %v", g.Name(), err)
				return err
			}
		}
		return nil
	}
}
