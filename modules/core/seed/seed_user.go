package seed

import (
	"context"

	"github.com/go-faster/errors"
	"github.com/google/uuid"

	"github.com/iota-uz/go-i18n/v2/i18n"
	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/role"
	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/user"
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/tab"
	"github.com/iota-uz/iota-sdk/modules/core/infrastructure/persistence"
	"github.com/iota-uz/iota-sdk/pkg/application"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/configuration"
	"github.com/iota-uz/iota-sdk/pkg/repo"
	"github.com/iota-uz/iota-sdk/pkg/types"
)

const (
	adminRoleName = "Admin"
	adminRoleDesc = "Administrator"
)

type userSeeder struct {
	user user.User
}

func UserSeedFunc(usr user.User) application.SeedFunc {
	s := &userSeeder{
		user: usr,
	}
	return s.CreateUser
}

func (s *userSeeder) CreateUser(ctx context.Context, app application.Application) error {
	tenantID, err := composables.UseTenantID(ctx)
	if err != nil {
		return errors.Wrapf(err, "failed to get tenant from context")
	}

	r, err := s.getOrCreateRole(ctx, app)
	if err != nil {
		return err
	}

	usr, err := s.getOrCreateUser(ctx, r, tenantID)
	if err != nil {
		return err
	}

	return s.createUserTabs(ctx, usr, app)
}

func (s *userSeeder) getOrCreateRole(ctx context.Context, app application.Application) (role.Role, error) {
	roleRepository := persistence.NewRoleRepository()
	matches, err := roleRepository.GetPaginated(ctx, &role.FindParams{
		Filters: []role.Filter{
			{
				Column: role.NameField,
				Filter: repo.Eq(adminRoleName),
			},
		},
	})
	if err != nil {
		return nil, err
	}
	logger := configuration.Use().Logger()
	if len(matches) > 0 {
		logger.Infof("Role %s already exists", adminRoleName)
		return matches[0], nil
	}

	tenantID, err := composables.UseTenantID(ctx)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to get tenant from context")
	}

	newRole := role.New(
		adminRoleName,
		role.WithDescription(adminRoleDesc),
		role.WithPermissions(app.RBAC().Permissions()),
		role.WithType(role.TypeSystem),
		role.WithTenantID(tenantID),
	)
	logger.Infof("Creating role %s", adminRoleName)
	return roleRepository.Create(ctx, newRole)
}

func (s *userSeeder) getOrCreateUser(ctx context.Context, r role.Role, tenantID uuid.UUID) (user.User, error) {
	uploadRepository := persistence.NewUploadRepository()
	userRepository := persistence.NewUserRepository(uploadRepository)
	foundUser, err := userRepository.GetByEmail(ctx, s.user.Email().Value())
	if err != nil && !errors.Is(err, persistence.ErrUserNotFound) {
		return nil, err
	}

	logger := configuration.Use().Logger()
	if foundUser != nil {
		logger.Infof("User %s already exists", s.user.Email().Value())
		return foundUser, nil
	}

	newUser := user.New(
		s.user.FirstName(),
		s.user.LastName(),
		s.user.Email(),
		s.user.UILanguage(),
		user.WithTenantID(tenantID),
		user.WithPassword(s.user.Password()),
		user.WithMiddleName(s.user.MiddleName()),
		user.WithPhone(s.user.Phone()),
	)

	logger.Infof("Creating user %s", s.user.Email().Value())
	return userRepository.Create(ctx, newUser.AddRole(r))
}

func (s *userSeeder) createUserTabs(
	ctx context.Context,
	usr user.User,
	app application.Application,
) error {
	tabsRepository := persistence.NewTabRepository()
	localizer := i18n.NewLocalizer(app.Bundle(), string(s.user.UILanguage()))

	tenantID, err := composables.UseTenantID(ctx)
	if err != nil {
		return errors.Wrapf(err, "failed to get tenant from context")
	}

	tabs := buildTabsFromNavItems(app.NavItems(localizer), usr.ID(), tenantID)

	for _, t := range tabs {
		if err := tabsRepository.CreateOrUpdate(ctx, t); err != nil {
			return errors.Wrapf(err, "failed to create tab userID :%d | href: %s", t.UserID, t.Href)
		}
	}
	return nil
}

func buildTabsFromNavItems(navItems []types.NavigationItem, userID uint, tenantID uuid.UUID) []*tab.Tab {
	tabs := make([]*tab.Tab, 0, len(navItems)*4)
	var position uint = 1

	var build func(items []types.NavigationItem)
	build = func(items []types.NavigationItem) {
		for _, item := range items {
			tabs = append(tabs, &tab.Tab{
				ID:       position,
				UserID:   userID,
				TenantID: tenantID,
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
