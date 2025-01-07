package persistence_test

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/role"
	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/user"
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/permission"
	"github.com/iota-uz/iota-sdk/modules/core/infrastructure/persistence"
	"github.com/iota-uz/iota-sdk/pkg/testutils"
)

func TestGormUserRepository_CRUD(t *testing.T) {
	ctx := testutils.GetTestContext()
	defer func(Tx pgx.Tx, ctx context.Context) {
		if err := Tx.Commit(ctx); err != nil {
			t.Fatal(err)
		}
	}(ctx.Tx, ctx.Context)

	userRepository := persistence.NewUserRepository()
	roleRepository := persistence.NewRoleRepository()
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

	roleEntity, err := roleRepository.Create(ctx.Context, roleData)
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

	if err := userRepository.Create(ctx.Context, userEntity); err != nil {
		t.Fatal(err)
	}

	t.Run("Get", func(t *testing.T) {
		dbUser, err := userRepository.GetByID(ctx.Context, userEntity.ID)
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
				ctx.Context,
				userEntity.SetName("Alice", "Karen", "Smith"),
			); err != nil {
				t.Fatal(err)
			}
			dbUser, err := userRepository.GetByID(ctx.Context, userEntity.ID)
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

			if !dbUser.UpdatedAt.After(userEntity.UpdatedAt) {
				t.Errorf(
					"expected updated at to be after %v, got %v",
					userEntity.UpdatedAt,
					dbUser.UpdatedAt,
				)
			}
		},
	)

	t.Run(
		"Delete", func(t *testing.T) {
			if err := userRepository.Delete(ctx.Context, 1); err != nil {
				t.Fatal(err)
			}
			_, err := userRepository.GetByID(ctx.Context, 1)
			if err == nil {
				t.Fatal("expected error, got nil")
			}
		},
	)
}
