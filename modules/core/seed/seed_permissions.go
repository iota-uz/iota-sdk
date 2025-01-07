package seed

import (
	"context"
	"github.com/iota-uz/iota-sdk/modules/core/infrastructure/persistence"
	"github.com/iota-uz/iota-sdk/pkg/application"
)

func CreatePermissions(ctx context.Context, app application.Application) error {
	permissionRepository := persistence.NewPermissionRepository()

	for _, p := range app.Permissions() {
		if err := permissionRepository.CreateOrUpdate(ctx, p); err != nil {
			return err
		}
	}
	return nil
}
