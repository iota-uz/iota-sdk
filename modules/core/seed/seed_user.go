package seed

import (
	"context"
	"github.com/go-faster/errors"

	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/role"
	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/user"
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/tab"
	"github.com/iota-uz/iota-sdk/modules/core/infrastructure/persistence"
	"github.com/iota-uz/iota-sdk/pkg/application"
	"github.com/iota-uz/iota-sdk/pkg/types"
	"github.com/nicksnyder/go-i18n/v2/i18n"
)

const (
	adminRoleName    = "Admin"
	adminRoleDesc    = "Administrator"
	defaultFirstName = "Admin"
	defaultLastName  = "User"
)

type userSeeder struct {
	email  string
	pass   string
	uiLang user.UILanguage
}

func UserSeedFunc(email, password string, uiLang user.UILanguage) application.SeedFunc {
	s := &userSeeder{
		email:  email,
		pass:   password,
		uiLang: uiLang,
	}
	return s.CreateUser
}

func (s *userSeeder) CreateUser(ctx context.Context, app application.Application) error {
	r, err := s.getOrCreateRole(ctx, app)
	if err != nil {
		return err
	}

	usr, err := s.getOrCreateUser(ctx, r)
	if err != nil {
		return err
	}

	return s.createUserTabs(ctx, usr, app)
}

func (s *userSeeder) getOrCreateRole(ctx context.Context, app application.Application) (role.Role, error) {
	roleRepository := persistence.NewRoleRepository()
	matches, err := roleRepository.GetPaginated(ctx, &role.FindParams{Name: adminRoleName})
	if err != nil {
		return nil, err
	}
	if len(matches) > 0 {
		return matches[0], nil
	}

	newRole, err := role.New(adminRoleName, adminRoleDesc, app.RBAC().Permissions())
	if err != nil {
		return nil, err
	}
	return roleRepository.Create(ctx, newRole)
}

func (s *userSeeder) getOrCreateUser(ctx context.Context, r role.Role) (user.User, error) {
	userRepository := persistence.NewUserRepository()
	foundUser, err := userRepository.GetByEmail(ctx, s.email)
	if err != nil && !errors.Is(err, persistence.ErrUserNotFound) {
		return nil, err
	}
	if foundUser != nil {
		return foundUser, nil
	}

	usr, err := user.New(
		defaultFirstName,
		defaultLastName,
		"",
		"",
		s.email,
		nil,
		s.uiLang,
		[]role.Role{r},
	).SetPassword(s.pass)
	if err != nil {
		return nil, err
	}

	return userRepository.Create(ctx, usr)
}

func (s *userSeeder) createUserTabs(ctx context.Context, usr user.User, app application.Application) error {
	tabsRepository := persistence.NewTabRepository()
	localizer := i18n.NewLocalizer(app.Bundle(), string(s.uiLang))
	tabs := buildTabsFromNavItems(app.NavItems(localizer), usr.ID())

	for _, t := range tabs {
		if err := tabsRepository.CreateOrUpdate(ctx, t); err != nil {
			return errors.Wrapf(err, "failed to create tab userID :%d | href: %s", t.UserID, t.Href)
		}
	}
	return nil
}

func buildTabsFromNavItems(navItems []types.NavigationItem, userID uint) []*tab.Tab {
	tabs := make([]*tab.Tab, 0, len(navItems)*4)
	var position uint = 1

	var build func(items []types.NavigationItem)
	build = func(items []types.NavigationItem) {
		for _, item := range items {
			tabs = append(tabs, &tab.Tab{
				ID:       position,
				UserID:   userID,
				Position: position,
				Href:     item.Href,
			})
			position++
			build(item.Children)
		}
	}

	build(navItems)
	return tabs
}
