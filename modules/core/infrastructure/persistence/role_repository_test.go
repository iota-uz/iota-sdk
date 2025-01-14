package persistence_test

import (
	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/role"
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/permission"
	"github.com/iota-uz/iota-sdk/modules/core/infrastructure/persistence"
	"github.com/iota-uz/iota-sdk/modules/warehouse/permissions"
	"testing"
)

func TestGormRoleRepository_CRUD(t *testing.T) {
	f := setupTest(t)

	permissionRepository := persistence.NewPermissionRepository()
	roleRepository := persistence.NewRoleRepository()
	if err := permissionRepository.Save(f.ctx, permissions.PositionCreate); err != nil {
		t.Fatal(err)
	}

	data, err := role.New(
		"test",
		"test",
		[]*permission.Permission{permissions.PositionCreate},
	)
	if err != nil {
		t.Fatal(err)
	}
	roleEntity, err := roleRepository.Create(f.ctx, data)
	if err != nil {
		t.Fatal(err)
	}

	t.Run(
		"Update", func(t *testing.T) {
			updatedRole, err := roleRepository.Update(f.ctx, roleEntity.SetName("updated"))
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

			if !updatedRole.UpdatedAt().After(roleEntity.UpdatedAt()) {
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
			if err := roleRepository.Delete(f.ctx, 1); err != nil {
				t.Fatal(err)
			}
			_, err := roleRepository.GetByID(f.ctx, 1)
			if err == nil {
				t.Fatal("expected error, got nil")
			}
		},
	)
}
