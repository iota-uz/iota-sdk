package elxolding

import (
	"context"
	"github.com/iota-agency/iota-erp/internal/domain/aggregates/role"
	"github.com/iota-agency/iota-erp/internal/domain/aggregates/user"
	"github.com/iota-agency/iota-erp/internal/infrastructure/persistence"
	"github.com/iota-agency/iota-erp/internal/modules/shared"
	"github.com/iota-agency/iota-erp/internal/presentation/templates/icons"
)

func NewModule() shared.Module {
	return &Module{}
}

type Module struct {
}

func (m *Module) Seed(ctx context.Context) error {
	roleRepository := persistence.NewRoleRepository()

	for _, r := range Roles {
		if err := roleRepository.CreateOrUpdate(ctx, &r); err != nil {
			return err
		}
	}
	userRepository := persistence.NewUserRepository()
	usr := &user.User{
		//nolint:exhaustruct
		ID:        1,
		FirstName: "Admin",
		LastName:  "User",
		Email:     "test@gmail.com",
		Roles: []*role.Role{
			&CEO,
		},
	}
	if err := usr.SetPassword("TestPass123!"); err != nil {
		return err
	}
	return userRepository.CreateOrUpdate(ctx, usr)
}

func (m *Module) Name() string {
	return "elxolding"
}

func (m *Module) NavigationItems() []shared.NavigationItem {
	return []shared.NavigationItem{
		{
			Name:     "Users",
			Children: nil,
			Icon:     icons.Users(icons.Props{Size: "20"}),
			Href:     "/users",
		},
	}
}

func (m *Module) Controllers() []shared.ControllerConstructor {
	return []shared.ControllerConstructor{
		NewUsersController,
	}
}

func (m *Module) LocaleFiles() []string {
	return []string{}
}
