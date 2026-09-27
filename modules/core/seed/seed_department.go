// Package seed provides idempotent seeders for the core organizational model
// (departments and user positions), routing every seeded row through the same
// tenant/structural validation the services enforce.
package seed

import (
	"context"

	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/department"
	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/user"
	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/userposition"
	"github.com/iota-uz/iota-sdk/modules/core/infrastructure/query"
	"github.com/iota-uz/iota-sdk/modules/core/services"
	"github.com/iota-uz/iota-sdk/pkg/application"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
	"github.com/sirupsen/logrus"
)

// DepartmentsSeedFunc returns an idempotent seed function that persists the
// given departments, skipping any that already exist. Each department is run
// through the same multilingual + hierarchy validation the service layer
// enforces, using the row's own tenant as the authoritative tenant.
func DepartmentsSeedFunc(departments ...department.Department) application.SeedFunc {
	const op serrors.Op = "seed.DepartmentsSeedFunc"
	return application.Seed(func(
		ctx context.Context,
		departmentRepository department.Repository,
		orgQuery query.OrgQueryRepository,
		logger logrus.FieldLogger,
	) error {
		for _, d := range departments {
			if exists, err := departmentRepository.Exists(ctx, d.ID()); err != nil {
				logger.Errorf("Failed to check if department %s exists: %v", d.Code(), err)
				return err
			} else if exists {
				logger.Infof("Department %s already exists", d.Code())
				continue
			}
			if err := services.ValidateDepartment(ctx, op, d.TenantID(), d, departmentRepository, orgQuery.DepartmentSubtree); err != nil {
				logger.Errorf("Department %s failed validation: %v", d.Code(), err)
				return err
			}
			if _, err := departmentRepository.Save(ctx, d); err != nil {
				logger.Errorf("Failed to save department %s: %v", d.Code(), err)
				return err
			}
			logger.Infof("Department %s saved", d.Code())
		}
		return nil
	})
}

// UserPositionsSeedFunc returns an idempotent seed function that persists the
// given user positions, skipping any that already exist. Each position is run
// through the same multilingual + reference validation the service layer
// enforces, using the row's own tenant as the authoritative tenant.
func UserPositionsSeedFunc(positions ...userposition.UserPosition) application.SeedFunc {
	const op serrors.Op = "seed.UserPositionsSeedFunc"
	return application.Seed(func(
		ctx context.Context,
		positionRepository userposition.Repository,
		departmentRepository department.Repository,
		userRepository user.Repository,
		logger logrus.FieldLogger,
	) error {
		for _, p := range positions {
			if exists, err := positionRepository.Exists(ctx, p.ID()); err != nil {
				logger.Errorf("Failed to check if user position %s exists: %v", p.ID(), err)
				return err
			} else if exists {
				logger.Infof("User position %s already exists", p.ID())
				continue
			}
			if err := services.ValidateUserPosition(ctx, op, p.TenantID(), p, departmentRepository, userRepository); err != nil {
				logger.Errorf("User position %s failed validation: %v", p.ID(), err)
				return err
			}
			if _, err := positionRepository.Save(ctx, p); err != nil {
				logger.Errorf("Failed to save user position %s: %v", p.ID(), err)
				return err
			}
			logger.Infof("User position %s saved", p.ID())
		}
		return nil
	})
}
