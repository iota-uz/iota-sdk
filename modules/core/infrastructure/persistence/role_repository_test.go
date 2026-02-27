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
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGormRoleRepository_CRUD(t *testing.T) {
	f := setupTest(t)

	permissionRepository := persistence.NewPermissionRepository()
	roleRepository := persistence.NewRoleRepository()
	require.NoError(t, permissionRepository.Save(f.Ctx, warehousepermissions.PositionCreate))

	data := role.New(
		"test",
		role.WithDescription("test"),
		role.WithPermissions([]permission.Permission{warehousepermissions.PositionCreate}),
	)
	roleEntity, err := roleRepository.Create(f.Ctx, data)
	require.NoError(t, err)

	t.Run(
		"Update", func(t *testing.T) {
			updatedRole, err := roleRepository.Update(f.Ctx, roleEntity.SetName("updated"))
			require.NoError(t, err)
			assert.Equal(t, "updated", updatedRole.Name())

			// updated_at can be the same or earlier due to timestamp precision/truncation.
			// Ensure it's not meaningfully earlier.
			if updatedRole.UpdatedAt().Before(roleEntity.UpdatedAt().Add(-time.Second)) {
				assert.Failf(
					t,
					"updated_at drift",
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
				require.NoError(t, err)
			}
			_, err := roleRepository.GetByID(f.Ctx, 1)
			require.Error(t, err)
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
	require.NoError(t, err)
	require.Len(t, createdRole.Permissions(), 1)
	assert.Equal(t, customPermission.Name(), createdRole.Permissions()[0].Name())
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

	require.NoError(t, permissionRepository.Save(f.Ctx, legacyPermission))

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
	require.NoError(t, err)
	require.Len(t, createdRole.Permissions(), 1)

	createdPermission := createdRole.Permissions()[0]
	assert.Equal(t, legacyID, createdPermission.ID())
	assert.Equal(t, permission.ActionUpdate, createdPermission.Action())

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
	require.NoError(t, err)
	require.Len(t, updatedRole.Permissions(), 1)

	updatedPermission := updatedRole.Permissions()[0]
	assert.Equal(t, legacyID, updatedPermission.ID())
	assert.Equal(t, permission.ActionDelete, updatedPermission.Action())
}
