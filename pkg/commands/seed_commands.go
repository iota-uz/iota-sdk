// Package commands provides this package.
package commands

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/modules"
	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/user"
	"github.com/iota-uz/iota-sdk/modules/core/domain/value_objects/internet"
	"github.com/iota-uz/iota-sdk/modules/core/infrastructure/persistence"
	coreseed "github.com/iota-uz/iota-sdk/modules/core/seed"
	"github.com/iota-uz/iota-sdk/modules/website/domain/entities/aichatconfig"
	websiteseed "github.com/iota-uz/iota-sdk/modules/website/seed"
	"github.com/iota-uz/iota-sdk/pkg/application"
	"github.com/iota-uz/iota-sdk/pkg/commands/common"
	"github.com/iota-uz/iota-sdk/pkg/commands/safety"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/configuration"
	"github.com/iota-uz/iota-sdk/pkg/defaults"
)

// SeedDatabase seeds the main database with initial data
func SeedDatabase(opts safety.RunOptions, mods ...application.Module) error {
	conf := configuration.Use()
	ctx := context.Background()

	app, pool, err := common.NewApplicationWithDefaults(mods...)
	if err != nil {
		return fmt.Errorf("failed to initialize application: %w", err)
	}
	defer pool.Close()

	preflight, err := safety.RunPreflight(ctx, pool, safety.OperationSeedMain)
	if err != nil {
		return fmt.Errorf("failed to run seed preflight: %w", err)
	}
	safety.PrintPreflight(os.Stdout, preflight, opts)
	if err := safety.EnforceSafety(opts, preflight, os.Stdin, os.Stdout); err != nil {
		return err
	}

	credentials := []safety.CredentialStatus{
		{Label: "Default Test User", Email: "test@gmail.com", Password: "TestPass123!"},
		{Label: "Default AI User", Email: "ai@llm.com"},
	}
	preExisting := make(map[string]bool, len(credentials))
	for i := range credentials {
		exists, existsErr := userExistsByEmail(ctx, credentials[i].Email)
		if existsErr != nil {
			return existsErr
		}
		preExisting[credentials[i].Email] = exists
		if exists {
			credentials[i].Status = "already_existed (seed skipped/left unchanged)"
		} else {
			credentials[i].Status = "created_now"
		}
	}
	if opts.DryRun {
		for i := range credentials {
			if credentials[i].Status == "created_now" {
				credentials[i].Status = "would_create"
			} else {
				credentials[i].Status = "would_keep_existing"
			}
		}
		safety.PrintCredentialSummary(os.Stdout, credentials, true)
		safety.PrintOutcomeSummary(os.Stdout, "main seed", false, true)
		return nil
	}

	app.RegisterNavItems(modules.NavLinks...)

	tx, err := pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}

	seeder := application.NewSeeder()

	// Create test user
	usr, err := user.New(
		"Test",
		"User",
		internet.MustParseEmail("test@gmail.com"),
		user.UILanguageEN,
	).SetPassword("TestPass123!")
	if err != nil {
		return fmt.Errorf("failed to create test user: %w", err)
	}

	// Add default tenant to context
	defaultTenant := &composables.Tenant{
		ID:     uuid.MustParse("00000000-0000-0000-0000-000000000001"),
		Name:   "Default",
		Domain: "default.localhost",
	}

	allPermissions := defaults.AllPermissions()
	seeder.Register(
		coreseed.CreateDefaultTenant,
		coreseed.CreateCurrencies,
		func(ctx context.Context, app application.Application) error {
			return coreseed.CreatePermissions(ctx, app, allPermissions)
		},
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

	ctxWithTenant := composables.WithTenantID(
		composables.WithTx(ctx, tx),
		defaultTenant.ID,
	)

	if err := seeder.Seed(ctxWithTenant, app); err != nil {
		if rollbackErr := tx.Rollback(ctx); rollbackErr != nil {
			return fmt.Errorf("rollback failed: %w (original error: %w)", rollbackErr, err)
		}
		return fmt.Errorf("failed to seed database: %w", err)
	}

	if err := tx.Commit(ctxWithTenant); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	for i := range credentials {
		exists, existsErr := userExistsByEmail(ctx, credentials[i].Email)
		if existsErr != nil {
			return existsErr
		}
		if exists && !preExisting[credentials[i].Email] {
			credentials[i].Status = "created_now"
			continue
		}
		credentials[i].Status = "already_existed (seed skipped/left unchanged)"
	}
	safety.PrintCredentialSummary(os.Stdout, credentials, false)
	safety.PrintOutcomeSummary(os.Stdout, "main seed", true, false)
	conf.Logger().Info("Database seeded successfully!")
	return nil
}

func userExistsByEmail(ctx context.Context, email string) (bool, error) {
	uploadRepository := persistence.NewUploadRepository()
	userRepository := persistence.NewUserRepository(uploadRepository)
	u, err := userRepository.GetByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, persistence.ErrUserNotFound) {
			return false, nil
		}
		return false, fmt.Errorf("check existing user %s: %w", email, err)
	}
	return u != nil, nil
}
