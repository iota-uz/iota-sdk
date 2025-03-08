package seed

import (
	"context"

	"github.com/iota-uz/iota-sdk/modules/core/infrastructure/persistence"
	"github.com/iota-uz/iota-sdk/pkg/application"
	"github.com/iota-uz/iota-sdk/pkg/configuration"
)

func CreatePermissions(ctx context.Context, app application.Application) error {
	conf := configuration.Use()
	permissionRepository := persistence.NewPermissionRepository()

	permissions := app.RBAC().Permissions()
	conf.Logger().Infof("Seeding %d permissions", len(permissions))

	for _, p := range permissions {
		conf.Logger().Infof("Creating permission: %s", p.Name)
		if err := permissionRepository.Save(ctx, p); err != nil {
			conf.Logger().Errorf("Failed to create permission %s: %v", p.Name, err)
			return err
		}
	}
	return nil
}
