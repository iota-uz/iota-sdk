package persistence_test

import (
	"testing"
	"time"

	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/role"
	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/user"
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/permission"
	"github.com/iota-uz/iota-sdk/modules/core/infrastructure/persistence"
)

func TestGormUserRepository_CRUD(t *testing.T) {
	f := setupTest(t)

	permissionRepository := persistence.NewPermissionRepository()
	roleRepository := persistence.NewRoleRepository()
	userRepository := persistence.NewUserRepository()
	roleData, err := role.New(
		"test",
		"test",
		[]*permission.Permission{
			permission.UserRead,
		},
	)
	if err != nil {
		t.Fatal(err)
	}

	if err := permissionRepository.Create(f.ctx, permission.UserRead); err != nil {
		t.Fatal(err)
	}

	roleEntity, err := roleRepository.Create(f.ctx, roleData)
	if err != nil {
		t.Fatal(err)
	}
	userEntity := &user.User{
		ID:         0,
		FirstName:  "John",
		LastName:   "Doe",
		MiddleName: "",
		Password:   "",
		Email:      "",
		AvatarID:   nil,
		Avatar:     nil,
		EmployeeID: nil,
		LastIP:     nil,
		UILanguage: "",
		LastLogin:  nil,
		LastAction: nil,
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
		Roles:      []role.Role{roleEntity},
	}

	createdUser, err := userRepository.Create(f.ctx, userEntity)
	if err != nil {
		t.Fatal(err)
	}

	t.Run("Get", func(t *testing.T) {
		dbUser, err := userRepository.GetByID(f.ctx, createdUser.ID)
		if err != nil {
			t.Fatal(err)
		}
		if dbUser.FirstName != "John" {
			t.Errorf("expected %s, got %s", "John", dbUser.FirstName)
		}

		if dbUser.LastName != "Doe" {
			t.Errorf("expected %s, got %s", "Doe", dbUser.LastName)
		}

		if dbUser.MiddleName != "" {
			t.Errorf("expected %s, got %s", "", dbUser.MiddleName)
		}

		if len(dbUser.Roles) != 1 {
			t.Errorf("expected %d, got %d", 1, len(dbUser.Roles))
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
			dbUser, err := userRepository.GetByID(f.ctx, createdUser.ID)
			if err != nil {
				t.Fatal(err)
			}
			if dbUser.FirstName != "Alice" {
				t.Errorf(
					"expected %s, got %s",
					"Alice",
					dbUser.FirstName,
				)
			}

			if dbUser.LastName != "Smith" {
				t.Errorf(
					"expected %s, got %s",
					"Smith",
					dbUser.LastName,
				)
			}

			if dbUser.MiddleName != "Karen" {
				t.Errorf(
					"expected %s, got %s",
					"Karen",
					dbUser.MiddleName,
				)
			}

			if !dbUser.UpdatedAt.After(createdUser.UpdatedAt) {
				t.Errorf(
					"expected updated at to be after %v, got %v",
					createdUser.UpdatedAt,
					dbUser.UpdatedAt,
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
		},
	)
}
