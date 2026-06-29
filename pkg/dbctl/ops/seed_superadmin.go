package ops

import (
	"context"
	"fmt"
	"os"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/user"
	"github.com/iota-uz/iota-sdk/modules/core/domain/value_objects/internet"
	coreseed "github.com/iota-uz/iota-sdk/modules/core/seed"
	"github.com/iota-uz/iota-sdk/pkg/application"
	"github.com/iota-uz/iota-sdk/pkg/composables"
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
			TxMode:      TxModeOwnTx,
			Handler: func(ctx context.Context, e *ExecutionContext) error {
				return runSuperadminSeed(ctx, e)
			},
		}},
	}
}

func runSuperadminSeed(ctx context.Context, e *ExecutionContext) error {
	const op serrors.Op = "dbctl.ops.runSuperadminSeed"

	// SUPERADMIN_PASSWORD is intentionally read from the environment here.
	// It is a CLI-only install-time credential that is not appropriate for
	// the typed config subsystem. Allowed by the W4 grep gate.
	superadminPassword := os.Getenv("SUPERADMIN_PASSWORD")
	if superadminPassword == "" {
		return serrors.E(op, serrors.Invalid, "SUPERADMIN_PASSWORD is required for seed.superadmin")
	}

	// SUPERADMIN_LANGUAGE is optional. Defaults to "en" for backward
	// compatibility. Set to a regional code (e.g. "pt-BR") to seed the
	// superadmin with that UI language from the start.
	uiLanguage := os.Getenv("SUPERADMIN_LANGUAGE")
	if uiLanguage == "" {
		uiLanguage = string(user.UILanguageEN)
	}
	parsedLanguage, err := user.NewUILanguage(uiLanguage)
	if err != nil {
		return serrors.E(op, serrors.Invalid,
			fmt.Sprintf("SUPERADMIN_LANGUAGE=%q is not a supported locale", uiLanguage))
	}

	superadminUser, err := user.New(
		"Super",
		"Admin",
		internet.MustParseEmail("admin@superadmin.local"),
		parsedLanguage,
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
	seedDeps := &application.SeedDeps{
		Pool:     e.Pool,
		EventBus: eventbus.NewEventPublisher(e.Logger),
		Logger:   e.Logger,
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

	ctxWithTenant := composables.WithTenantID(composables.WithTx(ctx, e.Tx), defaultTenant.ID)
	if err := seeder.Seed(ctxWithTenant, seedDeps); err != nil {
		return serrors.E(op, err, "seed superadmin dataset")
	}
	return nil
}
