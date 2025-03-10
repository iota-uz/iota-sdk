package seed

import (
	"context"

	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/group"
	"github.com/iota-uz/iota-sdk/modules/core/infrastructure/persistence"
	"github.com/iota-uz/iota-sdk/pkg/application"
	"github.com/iota-uz/iota-sdk/pkg/configuration"
)

func SeederForGroup(groups []group.Group) application.SeedFunc {
	return func(ctx context.Context, app application.Application) error {
		conf := configuration.Use()
		groupRepository := persistence.NewGroupRepository(
			persistence.NewUserRepository(persistence.NewUploadRepository()),
			persistence.NewRoleRepository(),
		)

		for _, g := range groups {
			conf.Logger().Infof("Saving group: %s", g.Name())
			if _, err := groupRepository.Save(ctx, g); err != nil {
				conf.Logger().Errorf("Failed to save group %s: %v", g.Name(), err)
				return err
			}
		}
		return nil
	}
}
