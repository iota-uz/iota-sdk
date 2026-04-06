// Package seed provides this package.
package seed

import (
	"context"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/tenant"
	"github.com/iota-uz/iota-sdk/pkg/application"
	"github.com/sirupsen/logrus"
)

var createDefaultTenant = application.Seed(
	func(ctx context.Context, tenantRepository tenant.Repository, logger logrus.FieldLogger) error {
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
	},
)

func CreateDefaultTenant(ctx context.Context, deps *application.SeedDeps) error {
	return createDefaultTenant(ctx, deps)
}
