package seed

import (
	"context"

	"github.com/go-faster/errors"
	"github.com/google/uuid"

	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/role"
	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/user"
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/permission"
	"github.com/iota-uz/iota-sdk/modules/core/infrastructure/persistence"
	"github.com/iota-uz/iota-sdk/pkg/application"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/configuration"
	"github.com/iota-uz/iota-sdk/pkg/repo"
)

const (
	adminRoleName = "Admin"
	adminRoleDesc = "Administrator"
)

type userSeeder struct {
	user        user.User
	permissions []permission.Permission
}

func UserSeedFunc(usr user.User, permissions []permission.Permission) application.SeedFunc {
	s := &userSeeder{
		user:        usr,
		permissions: permissions,
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

	_, err = s.getOrCreateUser(ctx, r, tenantID)
	if err != nil {
		return err
	}

	return nil
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
		role.WithPermissions(s.permissions),
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
