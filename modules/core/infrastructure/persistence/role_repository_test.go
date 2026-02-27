package persistence_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/role"
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/permission"
	corepermissions "github.com/iota-uz/iota-sdk/modules/core/permissions"
	"github.com/iota-uz/iota-sdk/modules/core/infrastructure/persistence"
	warehousepermissions "github.com/iota-uz/iota-sdk/modules/warehouse/permissions"
)

func TestGormRoleRepository_CRUD(t *testing.T) {
	f := setupTest(t)

	permissionRepository := persistence.NewPermissionRepository()
	roleRepository := persistence.NewRoleRepository()
	if err := permissionRepository.Save(f.Ctx, warehousepermissions.PositionCreate); err != nil {
		t.Fatal(err)
	}

	data := role.New(
		"test",
		role.WithDescription("test"),
		role.WithPermissions([]permission.Permission{warehousepermissions.PositionCreate}),
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

func TestGormRoleRepository_CreateUpsertsMissingPermission(t *testing.T) {
	f := setupTest(t)

	roleRepository := persistence.NewRoleRepository()

	customPermission := permission.New(
		permission.WithID(uuid.New()),
		permission.WithName(fmt.Sprintf("Role.Custom.%s", uuid.NewString())),
		permission.WithResource(corepermissions.ResourceRole),
		permission.WithAction(permission.ActionRead),
		permission.WithModifier(permission.ModifierAll),
	)

	data := role.New(
		"role-with-missing-permission-row",
		role.WithDescription("role with missing permission row"),
		role.WithPermissions([]permission.Permission{customPermission}),
	)

	createdRole, err := roleRepository.Create(f.Ctx, data)
	if err != nil {
		t.Fatal(err)
	}

	if len(createdRole.Permissions()) != 1 {
		t.Fatalf("expected 1 permission, got %d", len(createdRole.Permissions()))
	}

	if createdRole.Permissions()[0].Name() != customPermission.Name() {
		t.Fatalf("expected permission name %s, got %s", customPermission.Name(), createdRole.Permissions()[0].Name())
	}
}

func TestGormRoleRepository_CreateAndUpdateUseLegacyPermissionIDByName(t *testing.T) {
	f := setupTest(t)

	permissionRepository := persistence.NewPermissionRepository()
	roleRepository := persistence.NewRoleRepository()

	permissionName := fmt.Sprintf("Role.Legacy.%s", uuid.NewString())
	legacyID := uuid.New()

	legacyPermission := permission.New(
		permission.WithID(legacyID),
		permission.WithName(permissionName),
		permission.WithResource(corepermissions.ResourceRole),
		permission.WithAction(permission.ActionRead),
		permission.WithModifier(permission.ModifierAll),
	)

	if err := permissionRepository.Save(f.Ctx, legacyPermission); err != nil {
		t.Fatal(err)
	}

	createPermission := permission.New(
		permission.WithID(uuid.New()),
		permission.WithName(permissionName),
		permission.WithResource(corepermissions.ResourceRole),
		permission.WithAction(permission.ActionUpdate),
		permission.WithModifier(permission.ModifierAll),
	)

	createdRole, err := roleRepository.Create(
		f.Ctx,
		role.New(
			"legacy-id-role",
			role.WithDescription("legacy id role"),
			role.WithPermissions([]permission.Permission{createPermission}),
		),
	)
	if err != nil {
		t.Fatal(err)
	}

	if len(createdRole.Permissions()) != 1 {
		t.Fatalf("expected 1 permission after create, got %d", len(createdRole.Permissions()))
	}

	createdPermission := createdRole.Permissions()[0]
	if createdPermission.ID() != legacyID {
		t.Fatalf("expected legacy permission ID %s, got %s", legacyID, createdPermission.ID())
	}

	if createdPermission.Action() != permission.ActionUpdate {
		t.Fatalf("expected action %s, got %s", permission.ActionUpdate, createdPermission.Action())
	}

	updatePermission := permission.New(
		permission.WithID(uuid.New()),
		permission.WithName(permissionName),
		permission.WithResource(corepermissions.ResourceRole),
		permission.WithAction(permission.ActionDelete),
		permission.WithModifier(permission.ModifierAll),
	)

	updatedRole, err := roleRepository.Update(
		f.Ctx,
		createdRole.SetPermissions([]permission.Permission{updatePermission}),
	)
	if err != nil {
		t.Fatal(err)
	}

	if len(updatedRole.Permissions()) != 1 {
		t.Fatalf("expected 1 permission after update, got %d", len(updatedRole.Permissions()))
	}

	updatedPermission := updatedRole.Permissions()[0]
	if updatedPermission.ID() != legacyID {
		t.Fatalf("expected legacy permission ID %s after update, got %s", legacyID, updatedPermission.ID())
	}

	if updatedPermission.Action() != permission.ActionDelete {
		t.Fatalf("expected action %s after update, got %s", permission.ActionDelete, updatedPermission.Action())
	}
}
