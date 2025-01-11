package seed

import (
	"context"
	"errors"
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

func createOrUpdateUser(ctx context.Context, r role.Role) (*user.User, error) {
	userRepository := persistence.NewUserRepository()
	email := "test@gmail.com"
	foundUser, err := userRepository.GetByEmail(ctx, email)
	if err != nil && !errors.Is(err, persistence.ErrUserNotFound) {
		return nil, err
	}
	if foundUser != nil {
		return foundUser, nil
	}
	usr := &user.User{
		ID:         1,
		FirstName:  "Admin",
		LastName:   "User",
		Email:      email,
		UILanguage: user.UILanguageEN,
		Roles:      []role.Role{r},
	}
	if err := usr.SetPassword("TestPass123!"); err != nil {
		return nil, err
	}
	return userRepository.Create(ctx, usr)
}

func createOrUpdateRole(ctx context.Context, app application.Application) (role.Role, error) {
	roleRepository := persistence.NewRoleRepository()
	roleName := "Admin"
	matches, err := roleRepository.GetPaginated(ctx, &role.FindParams{
		Name: roleName,
	})
	if err != nil {
		return nil, err
	}
	if len(matches) > 0 {
		return matches[0], nil
	}
	newRole, err := role.New(
		roleName,
		"Administrator",
		app.Permissions(),
	)
	if err != nil {
		return nil, err
	}
	return roleRepository.Create(ctx, newRole)
}

func CreateUser(ctx context.Context, app application.Application) error {
	tabsRepository := persistence.NewTabRepository()

	r, err := createOrUpdateRole(ctx, app)
	if err != nil {
		return err
	}
	usr, err := createOrUpdateUser(ctx, r)
	if err != nil {
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
