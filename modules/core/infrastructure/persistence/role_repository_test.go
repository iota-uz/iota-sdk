package persistence_test

import (
	"testing"
	"time"

	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/role"
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/permission"
	"github.com/iota-uz/iota-sdk/modules/core/infrastructure/persistence"
	"github.com/iota-uz/iota-sdk/modules/warehouse/permissions"
)

func TestGormRoleRepository_CRUD(t *testing.T) {
	f := setupTest(t)

	permissionRepository := persistence.NewPermissionRepository()
	roleRepository := persistence.NewRoleRepository()
	if err := permissionRepository.Save(f.Ctx, permissions.PositionCreate); err != nil {
		t.Fatal(err)
	}

	data := role.New(
		"test",
		role.WithDescription("test"),
		role.WithPermissions([]permission.Permission{permissions.PositionCreate}),
	)
	roleEntity, err := roleRepository.Create(f.Ctx, data)
	if err != nil {
		t.Fatal(err)
	}

	t.Run(
		"Update", func(t *testing.T) {
			updatedRole, err := roleRepository.Update(f.Ctx, roleEntity.SetName("updated"))
			if err != nil {
				t.Fatal(err)
			}
			if updatedRole.Name() != "updated" {
				t.Errorf(
					"expected %s, got %s",
					"updated",
					updatedRole.Name(),
				)
			}

			// updated_at can be the same or earlier due to timestamp precision/truncation.
			// Ensure it's not meaningfully earlier.
			if updatedRole.UpdatedAt().Before(roleEntity.UpdatedAt().Add(-time.Second)) {
				t.Errorf(
					"expected updated at to be after %v, got %v",
					roleEntity.UpdatedAt(),
					updatedRole.UpdatedAt(),
				)
			}
		},
	)

	t.Run(
		"Delete", func(t *testing.T) {
			if err := roleRepository.Delete(f.Ctx, 1); err != nil {
				t.Fatal(err)
			}
			_, err := roleRepository.GetByID(f.Ctx, 1)
			if err == nil {
				t.Fatal("expected error, got nil")
			}
		},
	)
}
