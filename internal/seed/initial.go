package seed

import (
	"context"
	"github.com/iota-agency/iota-sdk/pkg/application"
	"github.com/iota-agency/iota-sdk/pkg/infrastructure/persistence"
)

func CreatePermissions(ctx context.Context, app application.Application) error {
	permissionRepository := persistence.NewPermissionRepository()

	for _, p := range app.rbac.Permissions() {
		if err := permissionRepository.CreateOrUpdate(ctx, &p); err != nil {
			return err
		}
	}
	return nil
}
