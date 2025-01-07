package persistence_test

import (
	"context"
	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/role"
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/permission"
	"github.com/iota-uz/iota-sdk/modules/core/infrastructure/persistence"
	"github.com/iota-uz/iota-sdk/modules/warehouse/permissions"
	"github.com/iota-uz/iota-sdk/pkg/testutils"
	"github.com/jackc/pgx/v5"
	"testing"
)

func TestGormRoleRepository_CRUD(t *testing.T) {
	ctx := testutils.GetTestContext()
	defer func(Tx pgx.Tx, ctx context.Context) {
		if err := Tx.Commit(ctx); err != nil {
			t.Fatal(err)
		}
	}(ctx.Tx, ctx.Context)

	permissionRepository := persistence.NewPermissionRepository()
	roleRepository := persistence.NewRoleRepository()
	if err := permissionRepository.Create(ctx.Context, permissions.PositionCreate); err != nil {
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
	roleEntity, err := roleRepository.Create(ctx.Context, data)
	if err != nil {
		t.Fatal(err)
	}

	t.Run(
		"Update", func(t *testing.T) {
			updatedRole, err := roleRepository.Update(ctx.Context, roleEntity.SetName("updated"))
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
			if err := roleRepository.Delete(ctx.Context, 1); err != nil {
				t.Fatal(err)
			}
			_, err := roleRepository.GetByID(ctx.Context, 1)
			if err == nil {
				t.Fatal("expected error, got nil")
			}
		},
	)
}
