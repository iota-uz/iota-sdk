package seed

import (
	"github.com/iota-uz/iota-sdk/modules/core/infrastructure/persistence"
	"github.com/iota-uz/iota-sdk/pkg/application"
)

func RegisterProviders(deps *application.SeedDeps) {
	uploadRepository := persistence.NewUploadRepository()
	roleRepository := persistence.NewRoleRepository()
	userRepository := persistence.NewUserRepository(uploadRepository)

	deps.RegisterValues(
		persistence.NewTenantRepository(),
		persistence.NewCurrencyRepository(),
		persistence.NewPermissionRepository(),
		uploadRepository,
		roleRepository,
		userRepository,
		persistence.NewGroupRepository(userRepository, roleRepository),
	)
}
