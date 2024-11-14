package seed

import (
	"context"
	"github.com/iota-agency/iota-erp/internal/application"
	"github.com/iota-agency/iota-erp/internal/infrastructure/persistence"
)

func CreatePermissions(ctx context.Context, app *application.Application) error {
	permissionRepository := persistence.NewPermissionRepository()

	for _, p := range app.Rbac.Permissions() {
		if err := permissionRepository.CreateOrUpdate(ctx, &p); err != nil {
			return err
		}
	}
	return nil
}
