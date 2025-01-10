package seed

import (
	"context"
	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/role"
	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/user"
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/tab"
	"github.com/iota-uz/iota-sdk/modules/core/infrastructure/persistence"
	"github.com/iota-uz/iota-sdk/pkg/application"
	"github.com/iota-uz/iota-sdk/pkg/types"
	"github.com/nicksnyder/go-i18n/v2/i18n"
)

func navItems2Tabs(navItems []types.NavigationItem) []*tab.Tab {
	tabs := make([]*tab.Tab, 0, len(navItems)*4)
	for _, navItem := range navItems {
		tabs = append(tabs, &tab.Tab{
			Href: navItem.Href,
		})
		tabs = append(tabs, navItems2Tabs(navItem.Children)...)
	}
	return tabs
}

func CreateUser(ctx context.Context, app application.Application) error {
	userRepository := persistence.NewUserRepository()
	roleRepository := persistence.NewRoleRepository()
	tabsRepository := persistence.NewTabRepository()

	CEO, err := role.New(
		"CEO",
		"Chief Executive Officer",
		app.Permissions(),
	)
	if err != nil {
		return err
	}
	createdRole, err := roleRepository.Create(ctx, CEO)
	if err != nil {
		return err
	}

	usr := &user.User{
		ID:         1,
		FirstName:  "Admin",
		LastName:   "User",
		Email:      "test@gmail.com",
		UILanguage: user.UILanguageEN,
		Roles:      []role.Role{createdRole},
	}
	if err := usr.SetPassword("TestPass123!"); err != nil {
		return err
	}
	if err := userRepository.CreateOrUpdate(ctx, usr); err != nil {
		return err
	}
	localizer := i18n.NewLocalizer(app.Bundle(), "ru")
	tabs := navItems2Tabs(app.NavItems(localizer))
	for i, t := range tabs {
		t.ID = uint(i + 1)
		t.UserID = usr.ID
		t.Position = uint(i + 1)
		if err := tabsRepository.CreateOrUpdate(ctx, t); err != nil {
			return err
		}
	}

	return nil
}
