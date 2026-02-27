package persistence_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/role"
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/permission"
	"github.com/iota-uz/iota-sdk/modules/core/infrastructure/persistence"
	corepermissions "github.com/iota-uz/iota-sdk/modules/core/permissions"
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
			err := roleRepository.Delete(f.Ctx, 1)
			require.NoError(t, err)
			_, err = roleRepository.GetByID(f.Ctx, 1)
			require.Error(t, err)
		},
	)
}

func TestGormRoleRepository_CreatePermissionResolutionScenarios(t *testing.T) {
	type scenario struct {
		name         string
		setup        func(*testing.T, *testFixture, *persistence.PermissionRepository)
		createRole   role.Role
		mutate       func(role.Role) role.Role
		assertCreate func(*testing.T, role.Role)
		assertUpdate func(*testing.T, role.Role)
	}

	customPermission := permission.New(
		permission.WithID(uuid.New()),
		permission.WithName(fmt.Sprintf("Role.Custom.%s", uuid.NewString())),
		permission.WithResource(corepermissions.ResourceRole),
		permission.WithAction(permission.ActionRead),
		permission.WithModifier(permission.ModifierAll),
	)

	legacyPermissionID := uuid.New()
	legacyPermissionName := fmt.Sprintf("Role.Legacy.%s", uuid.NewString())

	cases := []scenario{
		{
			name: "CreateUpsertsMissingPermission",
			createRole: role.New(
				"role-with-missing-permission-row",
				role.WithDescription("role with missing permission row"),
				role.WithPermissions([]permission.Permission{customPermission}),
			),
			assertCreate: func(t *testing.T, createdRole role.Role) {
				require.Len(t, createdRole.Permissions(), 1)
				assert.Equal(t, customPermission.Name(), createdRole.Permissions()[0].Name())
			},
		},
		{
			name: "CreateAndUpdateUsesLegacyPermissionIDByName",
			setup: func(t *testing.T, f *testFixture, permissionRepository *persistence.PermissionRepository) {
				legacyPermission := permission.New(
					permission.WithID(legacyPermissionID),
					permission.WithName(legacyPermissionName),
					permission.WithResource(corepermissions.ResourceRole),
					permission.WithAction(permission.ActionRead),
					permission.WithModifier(permission.ModifierAll),
				)
				require.NoError(t, permissionRepository.Save(f.Ctx, legacyPermission))
			},
			createRole: role.New(
				"legacy-id-role",
				role.WithDescription("legacy id role"),
				role.WithPermissions([]permission.Permission{
					permission.New(
						permission.WithID(uuid.New()),
						permission.WithName(legacyPermissionName),
						permission.WithResource(corepermissions.ResourceRole),
						permission.WithAction(permission.ActionUpdate),
						permission.WithModifier(permission.ModifierAll),
					),
				}),
			),
			mutate: func(createdRole role.Role) role.Role {
				return createdRole.SetPermissions([]permission.Permission{
					permission.New(
						permission.WithID(uuid.New()),
						permission.WithName(legacyPermissionName),
						permission.WithResource(corepermissions.ResourceRole),
						permission.WithAction(permission.ActionDelete),
						permission.WithModifier(permission.ModifierAll),
					),
				})
			},
			assertCreate: func(t *testing.T, createdRole role.Role) {
				createdPermission := createdRole.Permissions()[0]
				require.Len(t, createdRole.Permissions(), 1)
				assert.Equal(t, legacyPermissionID, createdPermission.ID())
				assert.Equal(t, permission.ActionUpdate, createdPermission.Action())
			},
			assertUpdate: func(t *testing.T, updatedRole role.Role) {
				updatedPermission := updatedRole.Permissions()[0]
				require.Len(t, updatedRole.Permissions(), 1)
				assert.Equal(t, legacyPermissionID, updatedPermission.ID())
				assert.Equal(t, permission.ActionDelete, updatedPermission.Action())
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := setupTest(t)
			permissionRepository := persistence.NewPermissionRepository()
			roleRepository := persistence.NewRoleRepository()

			if tc.setup != nil {
				tc.setup(t, f, permissionRepository)
			}

			createdRole, err := roleRepository.Create(f.Ctx, tc.createRole)
			require.NoError(t, err)

			tc.assertCreate(t, createdRole)

			if tc.mutate != nil {
				updatedRole, err := roleRepository.Update(
					f.Ctx,
					tc.mutate(createdRole),
				)
				require.NoError(t, err)
				tc.assertUpdate(t, updatedRole)
			}
		})
	}
}
