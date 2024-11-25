package seed

import (
	"context"
	"github.com/iota-agency/iota-sdk/pkg/application"
	"github.com/iota-agency/iota-sdk/pkg/domain/aggregates/role"
	"github.com/iota-agency/iota-sdk/pkg/domain/aggregates/user"
	"github.com/iota-agency/iota-sdk/pkg/infrastructure/persistence"
)

var (
	CEO = role.Role{
		ID:          1,
		Name:        "CEO",
		Description: "Chief Executive Officer",
	}
)

func CreateUser(ctx context.Context, app application.Application) error {
	roleRepository := persistence.NewRoleRepository()

	if err := roleRepository.CreateOrUpdate(ctx, &role.Role{
		ID:          CEO.ID,
		Name:        CEO.Name,
		Description: CEO.Description,
		Permissions: app.Permissions(),
	}); err != nil {
		return err
	}
	userRepository := persistence.NewUserRepository()
	usr := &user.User{
		//nolint:exhaustruct
		ID:         1,
		FirstName:  "Admin",
		LastName:   "User",
		Email:      "test@gmail.com",
		UILanguage: user.UILanguageRU,
		Roles: []*role.Role{
			&CEO,
		},
	}
	if err := usr.SetPassword("TestPass123!"); err != nil {
		return err
	}
	return userRepository.CreateOrUpdate(ctx, usr)
}
