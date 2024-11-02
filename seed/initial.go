package seed

import (
	"context"
	"github.com/iota-agency/iota-erp/internal/domain/entities/permission"

	"github.com/iota-agency/iota-erp/internal/infrastructure/persistence"
)

func CreatePermissions(ctx context.Context) error {
	permissionRepository := persistence.NewPermissionRepository()

	for _, p := range permission.Permissions {
		if err := permissionRepository.CreateOrUpdate(ctx, &p); err != nil {
			return err
		}
	}
	return nil
}
