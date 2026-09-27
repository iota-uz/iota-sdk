// Package seed provides this package.
package seed

import (
	"context"

	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/permission"
	"github.com/iota-uz/iota-sdk/pkg/application"
	"github.com/sirupsen/logrus"
)

func CreatePermissions(permissions []permission.Permission) application.SeedFunc {
	return application.Seed(
		func(ctx context.Context, permissionRepository permission.Repository, logger logrus.FieldLogger) error {
			logger.Infof("Seeding %d permissions", len(permissions))
			for _, p := range permissions {
				logger.Infof("Creating permission: %s", p.Name())
				if err := permissionRepository.Save(ctx, p); err != nil {
					logger.Errorf("Failed to create permission %s: %v", p.Name(), err)
					return err
				}
			}
			return nil
		},
	)
}
