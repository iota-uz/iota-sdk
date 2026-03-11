package ops

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/modules"
	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/user"
	"github.com/iota-uz/iota-sdk/modules/core/domain/value_objects/internet"
	coreseed "github.com/iota-uz/iota-sdk/modules/core/seed"
	"github.com/iota-uz/iota-sdk/pkg/application"
	"github.com/iota-uz/iota-sdk/pkg/commands/common"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/defaults"
)

func SeedSuperadminOperation() OperationSpec {
	return OperationSpec{
		Name: "seed.superadmin",
		Kind: OperationKindSeed,
		Steps: []StepSpec{{
			ID:          "seed_superadmin_dataset",
			Description: "Seed default tenant and superadmin account",
			TxMode:      TxModeNone,
			Handler: func(ctx context.Context, _ *ExecutionContext) error {
				return runSuperadminSeed(ctx)
			},
		}},
	}
}

func runSuperadminSeed(ctx context.Context) error {
	app, pool, err := common.NewApplicationWithDefaultsAndOptions(common.AppBuildOptions{
		RuntimeProfile: application.RuntimeProfileCLI,
	}, modules.BuiltInModules...)
	if err != nil {
		return fmt.Errorf("initialize application: %w", err)
	}
	defer pool.Close()

	tx, err := pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	superadminUser, err := user.New(
		"Super",
		"Admin",
		internet.MustParseEmail("admin@superadmin.local"),
		user.UILanguageEN,
		user.WithType(user.TypeSuperAdmin),
	).SetPassword("SuperAdmin123!")
	if err != nil {
		return fmt.Errorf("create superadmin user: %w", err)
	}

	defaultTenant := &composables.Tenant{
		ID:     uuid.MustParse("00000000-0000-0000-0000-000000000001"),
		Name:   "Default",
		Domain: "default.localhost",
	}

	allPermissions := defaults.AllPermissions()
	seeder := application.NewSeeder()
	seeder.Register(
		coreseed.CreateDefaultTenant,
		coreseed.CreateCurrencies,
		func(ctx context.Context, app application.Application) error {
			return coreseed.CreatePermissions(ctx, app, allPermissions)
		},
		coreseed.UserSeedFunc(superadminUser, allPermissions),
		coreseed.CreateSubscriptionEntitlements,
	)

	ctxWithTenant := composables.WithTenantID(composables.WithTx(ctx, tx), defaultTenant.ID)
	if err := seeder.Seed(ctxWithTenant, app); err != nil {
		return fmt.Errorf("seed superadmin dataset: %w", err)
	}
	if err := tx.Commit(ctxWithTenant); err != nil {
		return fmt.Errorf("commit superadmin seed transaction: %w", err)
	}
	return nil
}
