package persistence_test

import (
	"errors"
	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/role"
	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/user"
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/permission"
	"github.com/iota-uz/iota-sdk/modules/core/infrastructure/persistence"
	permissions "github.com/iota-uz/iota-sdk/modules/core/permissions"
	"testing"
)

func TestGormUserRepository_CRUD(t *testing.T) {
	t.Parallel()
	f := setupTest(t)

	permissionRepository := persistence.NewPermissionRepository()
	roleRepository := persistence.NewRoleRepository()
	userRepository := persistence.NewUserRepository()
	roleData, err := role.New(
		"test",
		"test",
		[]*permission.Permission{
			permissions.UserRead,
		},
	)
	if err != nil {
		t.Fatal(err)
	}

	if err := permissionRepository.Save(f.ctx, permissions.UserRead); err != nil {
		t.Fatal(err)
	}

	roleEntity, err := roleRepository.Create(f.ctx, roleData)
	if err != nil {
		t.Fatal(err)
	}
	userEntity := user.New(
		"John",
		"Doe",
		"",
		"",
		"test@gmail.com",
		nil,
		0,
		user.UILanguageEN,
		[]role.Role{roleEntity},
	)

	createdUser, err := userRepository.Create(f.ctx, userEntity)
	if err != nil {
		t.Fatal(err)
	}

	t.Run("Get", func(t *testing.T) {
		dbUser, err := userRepository.GetByID(f.ctx, createdUser.ID())
		if err != nil {
			t.Fatal(err)
		}
		if dbUser.FirstName() != "John" {
			t.Errorf("expected %s, got %s", "John", dbUser.FirstName())
		}

		if dbUser.LastName() != "Doe" {
			t.Errorf("expected %s, got %s", "Doe", dbUser.LastName())
		}

		if dbUser.MiddleName() != "" {
			t.Errorf("expected %s, got %s", "", dbUser.MiddleName())
		}

		if len(dbUser.Roles()) != 1 {
			t.Fatalf("expected %d, got %d", 1, len(dbUser.Roles()))
		}

		roles := dbUser.Roles()

		if roles[0].Name() != "test" {
			t.Errorf("expected %s, got %s", "test", roles[0].Name())
		}

		if len(roles[0].Permissions()) != 1 {
			t.Fatalf("expected %d, got %d", 1, len(roles[0].Permissions()))
		}

		if roles[0].Permissions()[0].Name != permissions.UserRead.Name {
			t.Errorf(
				"expected %s, got %s",
				permissions.UserRead.Name,
				roles[0].Permissions()[0].Name,
			)
		}
	})

	t.Run(
		"Update", func(t *testing.T) {
			if err := userRepository.Update(
				f.ctx,
				createdUser.SetName("Alice", "Karen", "Smith"),
			); err != nil {
				t.Fatal(err)
			}
			dbUser, err := userRepository.GetByID(f.ctx, createdUser.ID())
			if err != nil {
				t.Fatal(err)
			}
			if dbUser.FirstName() != "Alice" {
				t.Errorf(
					"expected %s, got %s",
					"Alice",
					dbUser.FirstName(),
				)
			}

			if dbUser.LastName() != "Karen" {
				t.Errorf(
					"expected %s, got %s",
					"Karen",
					dbUser.LastName(),
				)
			}

			if dbUser.MiddleName() != "Smith" {
				t.Errorf(
					"expected %s, got %s",
					"Smith",
					dbUser.MiddleName(),
				)
			}

			if !dbUser.UpdatedAt().After(createdUser.UpdatedAt()) {
				t.Errorf(
					"expected updated at to be after %v, got %v",
					createdUser.UpdatedAt(),
					dbUser.UpdatedAt(),
				)
			}
		},
	)

	t.Run(
		"Delete", func(t *testing.T) {
			if err := userRepository.Delete(f.ctx, 1); err != nil {
				t.Fatal(err)
			}
			_, err := userRepository.GetByID(f.ctx, 1)
			if err == nil {
				t.Fatal("expected error, got nil")
			}

			if !errors.Is(err, persistence.ErrUserNotFound) {
				t.Errorf(
					"expected %v, got %v",
					persistence.ErrUserNotFound,
					err,
				)
			}
		},
	)
}
