// Package seed provides this package.
package seed

import (
	"context"

	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/group"
	"github.com/iota-uz/iota-sdk/pkg/application"
	"github.com/sirupsen/logrus"
)

func GroupsSeedFunc(groups ...group.Group) application.SeedFunc {
	return application.Seed(func(ctx context.Context, groupRepository group.Repository, logger logrus.FieldLogger) error {
		for _, g := range groups {
			if exists, err := groupRepository.Exists(ctx, g.ID()); err != nil {
				logger.Errorf("Failed to check if group %s exists: %v", g.Name(), err)
			} else if exists {
				logger.Infof("Group %s already exists", g.Name())
				continue
			}
			if _, err := groupRepository.Save(ctx, g); err != nil {
				logger.Errorf("Failed to save group %s: %v", g.Name(), err)
				return err
			}
			logger.Infof("Group %s saved", g.Name())
		}
		return nil
	})
}
