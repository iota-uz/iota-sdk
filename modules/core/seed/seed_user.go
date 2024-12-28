package seed

import (
	"context"
	"github.com/iota-agency/iota-sdk/modules/core/infrastructure/persistence"
	"github.com/iota-agency/iota-sdk/pkg/application"
	"github.com/iota-agency/iota-sdk/pkg/domain/aggregates/role"
	"github.com/iota-agency/iota-sdk/pkg/domain/aggregates/user"
	"github.com/iota-agency/iota-sdk/pkg/domain/entities/tab"
	"github.com/iota-agency/iota-sdk/pkg/types"
	"github.com/nicksnyder/go-i18n/v2/i18n"
)

var (
	CEO = role.Role{
		ID:          1,
		Name:        "CEO",
		Description: "Chief Executive Officer",
	}
)

func navItems2Tabs(navItems []types.NavigationItem) []*tab.Tab {
	tabs := make([]*tab.Tab, len(navItems))
	for i, navItem := range navItems {
		tabs[i] = &tab.Tab{
			Href: navItem.Href,
		}
		tabs = append(tabs, navItems2Tabs(navItem.Children)...)
	}
	return tabs
}

func CreateUser(ctx context.Context, app application.Application) error {
	userRepository := persistence.NewUserRepository()
	roleRepository := persistence.NewRoleRepository()
	tabsRepository := persistence.NewTabRepository()

	if err := roleRepository.CreateOrUpdate(ctx, &role.Role{
		ID:          CEO.ID,
		Name:        CEO.Name,
		Description: CEO.Description,
		Permissions: app.Permissions(),
	}); err != nil {
		return err
	}

	usr := &user.User{
		//nolint:exhaustruct
		ID:         1,
		FirstName:  "Admin",
		LastName:   "User",
		Email:      "test@gmail.com",
		UILanguage: user.UILanguageEN,
		Roles: []*role.Role{
			&CEO,
		},
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
