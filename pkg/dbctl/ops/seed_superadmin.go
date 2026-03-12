package ops

import (
	"context"
	"os"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/user"
	"github.com/iota-uz/iota-sdk/modules/core/domain/value_objects/internet"
	coreseed "github.com/iota-uz/iota-sdk/modules/core/seed"
	"github.com/iota-uz/iota-sdk/pkg/application"
	"github.com/iota-uz/iota-sdk/pkg/commands/common"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/configuration"
	"github.com/iota-uz/iota-sdk/pkg/defaults"
	"github.com/iota-uz/iota-sdk/pkg/eventbus"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
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
	const op serrors.Op = "dbctl.ops.runSuperadminSeed"
	pool, err := common.GetDefaultDatabasePool()
	if err != nil {
		return serrors.E(op, err, "initialize database pool")
	}
	defer pool.Close()

	tx, err := pool.Begin(ctx)
	if err != nil {
		return serrors.E(op, err, "begin transaction")
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	superadminPassword := os.Getenv("SUPERADMIN_PASSWORD")
	if superadminPassword == "" {
		return serrors.E(op, serrors.Invalid, "SUPERADMIN_PASSWORD is required for seed.superadmin")
	}

	superadminUser, err := user.New(
		"Super",
		"Admin",
		internet.MustParseEmail("admin@superadmin.local"),
		user.UILanguageEN,
		user.WithType(user.TypeSuperAdmin),
	).SetPassword(superadminPassword)
	if err != nil {
		return serrors.E(op, err, "create superadmin user")
	}

	defaultTenant := &composables.Tenant{
		ID:     uuid.MustParse("00000000-0000-0000-0000-000000000001"),
		Name:   "Default",
		Domain: "default.localhost",
	}

	allPermissions := defaults.AllPermissions()
	conf := configuration.Use()
	seedDeps := &application.SeedDeps{
		Pool:     pool,
		EventBus: eventbus.NewEventPublisher(conf.Logger()),
		Logger:   conf.Logger(),
	}
	coreseed.RegisterProviders(seedDeps)
	seeder := application.NewSeeder()
	seeder.Register(
		coreseed.CreateDefaultTenant,
		coreseed.CreateCurrencies,
		coreseed.CreatePermissions(allPermissions),
		coreseed.UserSeedFunc(superadminUser, allPermissions),
		coreseed.CreateSubscriptionEntitlements,
	)

	ctxWithTenant := composables.WithTenantID(composables.WithTx(ctx, tx), defaultTenant.ID)
	if err := seeder.Seed(ctxWithTenant, seedDeps); err != nil {
		return serrors.E(op, err, "seed superadmin dataset")
	}
	if err := tx.Commit(ctxWithTenant); err != nil {
		return serrors.E(op, err, "commit superadmin seed transaction")
	}
	return nil
}
