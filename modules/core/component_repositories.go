package core

import (
	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/group"
	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/role"
	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/user"
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/currency"
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/permission"
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/session"
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/tenant"
	twofactorentity "github.com/iota-uz/iota-sdk/modules/core/domain/entities/twofactor"
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/upload"
	"github.com/iota-uz/iota-sdk/modules/core/infrastructure/persistence"
)

func newUploadRepository() upload.Repository {
	return persistence.NewUploadRepository()
}

func newUserRepository(uploadRepo upload.Repository) user.Repository {
	return persistence.NewUserRepository(uploadRepo)
}

func newRoleRepository() role.Repository {
	return persistence.NewRoleRepository()
}

func newTenantRepository() tenant.Repository {
	return persistence.NewTenantRepository()
}

func newPermissionRepository() permission.Repository {
	return persistence.NewPermissionRepository()
}

func newSessionRepository() session.Repository {
	return persistence.NewSessionRepository()
}

func newOTPRepository() twofactorentity.OTPRepository {
	return persistence.NewOTPRepository()
}

func newRecoveryCodeRepository() twofactorentity.RecoveryCodeRepository {
	return persistence.NewRecoveryCodeRepository()
}

func newGroupRepository(userRepo user.Repository, roleRepo role.Repository) group.Repository {
	return persistence.NewGroupRepository(userRepo, roleRepo)
}

func newCurrencyRepository() currency.Repository {
	return persistence.NewCurrencyRepository()
}
