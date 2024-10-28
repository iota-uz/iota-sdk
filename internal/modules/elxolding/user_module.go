package elxolding

import (
	"github.com/iota-agency/iota-erp/internal/modules/shared"
	"github.com/iota-agency/iota-erp/internal/presentation/templates/icons"
)

func NewUserModule() shared.Module {
	return &UserModule{}
}

type UserModule struct {
}

func (u *UserModule) Name() string {
	return "elxolding-user-module"
}

func (u *UserModule) NavigationItems() []shared.NavigationItem {
	return []shared.NavigationItem{
		{
			Name:     "Users",
			Children: nil,
			Icon:     icons.Users(icons.Props{Size: "20"}),
			Href:     "/users",
		},
	}
}

func (u *UserModule) Controllers() []shared.ControllerConstructor {
	return []shared.ControllerConstructor{
		NewUsersController,
	}
}

func (u *UserModule) LocaleFiles() []string {
	return []string{}
}
