package iota

import (
	"context"
	"github.com/benbjohnson/hashfs"
	"github.com/iota-agency/iota-erp/internal/application"
	"github.com/iota-agency/iota-erp/internal/domain/aggregates/role"
	"github.com/iota-agency/iota-erp/internal/domain/aggregates/user"
	"github.com/iota-agency/iota-erp/internal/domain/entities/permission"
	"github.com/iota-agency/iota-erp/internal/infrastructure/persistence"
	"github.com/iota-agency/iota-erp/internal/modules/shared"
	"github.com/iota-agency/iota-erp/internal/presentation/templates/icons"
	"github.com/iota-agency/iota-erp/internal/types"
	"github.com/iota-agency/iota-erp/iota/assets"
	"github.com/iota-agency/iota-erp/iota/controllers"
	"github.com/nicksnyder/go-i18n/v2/i18n"
)

func NewModule() shared.Module {
	return &Module{}
}

type Module struct {
}

func (m *Module) Seed(ctx context.Context, app *application.Application) error {
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

func (m *Module) Register(app *application.Application) error {
	return nil
}

func (m *Module) MigrationDirs() []string {
	return []string{
		"internal/modules/iota/migrations",
	}
}

func (m *Module) Assets() *hashfs.FS {
	return assets.FS
}

func (m *Module) Name() string {
	return "iota"
}

func (m *Module) NavigationItems(localizer *i18n.Localizer) []types.NavigationItem {
	return []types.NavigationItem{
		{
			Name:     localizer.MustLocalize(&i18n.LocalizeConfig{MessageID: "NavigationLinks.Users"}),
			Children: nil,
			Icon:     icons.Users(icons.Props{Size: "20"}),
			Href:     "/users",
		},
		{
			Name: localizer.MustLocalize(&i18n.LocalizeConfig{MessageID: "NavigationLinks.Operations"}),
			Href: "#",
			Children: []types.NavigationItem{
				{
					Name:        localizer.MustLocalize(&i18n.LocalizeConfig{MessageID: "NavigationLinks.Employees"}),
					Href:        "/operations/employees",
					Permissions: []permission.Permission{permission.EmployeeRead},
				},
				{
					Name:        localizer.MustLocalize(&i18n.LocalizeConfig{MessageID: "NavigationLinks.Settings"}),
					Href:        "/settings",
					Permissions: []permission.Permission{permission.SettingsRead},
				},
				{
					Name:        localizer.MustLocalize(&i18n.LocalizeConfig{MessageID: "NavigationLinks.Projects"}),
					Href:        "/projects",
					Permissions: []permission.Permission{permission.ProjectRead},
				},
			},
			Icon:        icons.Pulse(icons.Props{Size: "20"}),
			Permissions: nil,
		},
		{
			Name: localizer.MustLocalize(&i18n.LocalizeConfig{MessageID: "NavigationLinks.Enums"}),
			Href: "#",
			Icon: icons.CheckCircle(icons.Props{Size: "20"}),
			Children: []types.NavigationItem{
				{
					Name:        localizer.MustLocalize(&i18n.LocalizeConfig{MessageID: "NavigationLinks.TaskTypes"}),
					Href:        "/enums/task-types",
					Permissions: nil,
				},
				{
					Name:        localizer.MustLocalize(&i18n.LocalizeConfig{MessageID: "NavigationLinks.Positions"}),
					Href:        "/enums/positions",
					Permissions: nil,
				},
			},
		},
	}
}

func (m *Module) Controllers() []shared.ControllerConstructor {
	return []shared.ControllerConstructor{
		controllers.NewUsersController,
		controllers.NewLoginController,
	}
}

func (m *Module) LocaleFiles() []string {
	return []string{}
}
