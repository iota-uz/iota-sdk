package persistence_test

import (
	"context"
	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/role"
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/permission"
	"github.com/iota-uz/iota-sdk/modules/core/infrastructure/persistence"
	"github.com/iota-uz/iota-sdk/pkg/testutils"
	"github.com/jackc/pgx/v5"
	"testing"
)

func TestGormPositionRepository_CRUD(t *testing.T) {
	ctx := testutils.GetTestContext()
	defer func(Tx pgx.Tx, ctx context.Context) {
		if err := Tx.Commit(ctx); err != nil {
			t.Fatal(err)
		}
	}(ctx.Tx, ctx.Context)

	roleRepository := persistence.NewRoleRepository()
	roleEntity, err := role.New(
		"test",
		"test",
		[]permission.Permission{},
	)
	if err != nil {
		t.Fatal(err)
	}
	if err := roleRepository.Create(ctx.Context, roleEntity); err != nil {
		t.Fatal(err)
	}

	t.Run(
		"Update", func(t *testing.T) {
			if err := roleRepository.Update(ctx.Context, roleEntity.SetName("updated")); err != nil {
				t.Fatal(err)
			}
			dbRole, err := roleRepository.GetByID(ctx.Context, roleEntity.ID())
			if err != nil {
				t.Fatal(err)
			}
			if dbRole.Name() != "updated" {
				t.Errorf("expected %s, got %s", "updated", dbRole.Name())
			}

			if !dbRole.UpdatedAt().After(roleEntity.UpdatedAt()) {
				t.Errorf("expected updated at to be after %v, got %v", roleEntity.UpdatedAt(), dbRole.UpdatedAt())
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
