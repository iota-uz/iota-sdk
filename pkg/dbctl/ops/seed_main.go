package ops

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/user"
	"github.com/iota-uz/iota-sdk/modules/core/domain/value_objects/internet"
	coreseed "github.com/iota-uz/iota-sdk/modules/core/seed"
	"github.com/iota-uz/iota-sdk/modules/website/domain/entities/aichatconfig"
	websiteseed "github.com/iota-uz/iota-sdk/modules/website/seed"
	"github.com/iota-uz/iota-sdk/pkg/application"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/configuration"
	"github.com/iota-uz/iota-sdk/pkg/defaults"
	"github.com/iota-uz/iota-sdk/pkg/eventbus"
)

func SeedMainOperation() OperationSpec {
	return OperationSpec{
		Name: "seed.main",
		Kind: OperationKindSeed,
		Steps: []StepSpec{{
			ID:          "seed_main_dataset",
			Description: "Seed default tenant, users, permissions, and website defaults",
			TxMode:      TxModeOwnTx,
			Handler: func(ctx context.Context, e *ExecutionContext) error {
				return runMainSeed(ctx, e)
			},
		}},
	}
}

func runMainSeed(ctx context.Context, e *ExecutionContext) error {
	conf := configuration.Use()
	seedDeps := &application.SeedDeps{
		Pool:     e.Pool,
		EventBus: eventbus.NewEventPublisher(conf.Logger()),
		Logger:   conf.Logger(),
	}
	coreseed.RegisterProviders(seedDeps)
	websiteseed.RegisterProviders(seedDeps)
	seeder := application.NewSeeder()
	usr, err := user.New(
		"Test",
		"User",
		internet.MustParseEmail("test@gmail.com"),
		user.UILanguageEN,
	).SetPassword("TestPass123!")
	if err != nil {
		return fmt.Errorf("create test user: %w", err)
	}

	defaultTenant := &composables.Tenant{
		ID:     uuid.MustParse("00000000-0000-0000-0000-000000000001"),
		Name:   "Default",
		Domain: "default.localhost",
	}

	allPermissions := defaults.AllPermissions()
	seeder.Register(
		coreseed.CreateDefaultTenant,
		coreseed.CreateCurrencies,
		coreseed.CreatePermissions(allPermissions),
		coreseed.UserSeedFunc(usr, allPermissions),
		coreseed.UserSeedFunc(user.New(
			"AI",
			"User",
			internet.MustParseEmail("ai@llm.com"),
			user.UILanguageEN,
			user.WithTenantID(defaultTenant.ID),
		), allPermissions),
		coreseed.CreateSubscriptionEntitlements,
		websiteseed.AIChatConfigSeedFunc(aichatconfig.MustNew(
			"gemma-12b-it",
			aichatconfig.AIModelTypeOpenAI,
			"https://llm2.eai.uz/v1",
			aichatconfig.WithTenantID(defaultTenant.ID),
		)),
	)

	ctxWithTenant := composables.WithTenantID(composables.WithTx(ctx, e.Tx), defaultTenant.ID)
	if err := seeder.Seed(ctxWithTenant, seedDeps); err != nil {
		return fmt.Errorf("seed main dataset: %w", err)
	}
	return nil
}
