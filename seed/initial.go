package seed

import (
	"context"
	"github.com/iota-agency/iota-erp/internal/domain/aggregates/role"
	"github.com/iota-agency/iota-erp/internal/domain/aggregates/user"
	"github.com/iota-agency/iota-erp/internal/domain/entities/permission"

	"github.com/iota-agency/iota-erp/internal/infrastructure/persistence"
)

func CreateInitialUser(ctx context.Context) error {
	permissionRepository := persistence.NewPermissionRepository()
	roleRepository := persistence.NewRoleRepository()

	for _, p := range permission.Permissions {
		if err := permissionRepository.Create(ctx, &p); err != nil {
			return err
		}
	}

	for _, r := range role.Roles {
		if err := roleRepository.Create(ctx, &r); err != nil {
			return err
		}
	}
	userRepository := persistence.NewUserRepository()
	u := &user.User{
		//nolint:exhaustruct
		ID:        1,
		FirstName: "Admin",
		LastName:  "User",
		Email:     "test@gmail.com",
	}
	if err := u.SetPassword("TestPass123!"); err != nil {
		return err
	}
	return userRepository.CreateOrUpdate(ctx, u)
}
