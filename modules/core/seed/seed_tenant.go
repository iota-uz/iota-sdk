package seed

import (
	"context"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/tenant"
	"github.com/iota-uz/iota-sdk/modules/core/infrastructure/persistence"
	"github.com/iota-uz/iota-sdk/pkg/application"
	"github.com/iota-uz/iota-sdk/pkg/configuration"
)

func CreateDefaultTenant(ctx context.Context, app application.Application) error {
	conf := configuration.Use()
	logger := conf.Logger()
	tenantRepository := persistence.NewTenantRepository()
	// Create a new tenant with a fixed UUID for the default tenant
	defaultTenant := tenant.New(
		"Default",
		tenant.WithID(uuid.MustParse("00000000-0000-0000-0000-000000000001")),
		tenant.WithDomain("default.localhost"),
	)
	existingTenants, err := tenantRepository.List(ctx)
	if err != nil {
		logger.Errorf("Failed to list tenants: %v", err)
		return err
	}

	if len(existingTenants) > 0 {
		logger.Infof("Default tenant already exists")
		return nil
	}

	logger.Infof("Creating default tenant")
	_, err = tenantRepository.Create(ctx, defaultTenant)
	if err != nil {
		logger.Errorf("Failed to create default tenant: %v", err)
		return err
	}
	return nil
}
