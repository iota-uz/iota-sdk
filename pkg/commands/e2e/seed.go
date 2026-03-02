// Package e2e provides this package.
package e2e

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

// Seed populates the e2e database with test data
func Seed(opts safety.RunOptions) error {
	// Set environment variable for e2e database
	_ = os.Setenv("DB_NAME", E2EDBName)

	conf := configuration.Use()
	ctx := context.Background()

	pool, err := GetE2EPool()
	if err != nil {
		return fmt.Errorf("failed to connect to e2e database: %w", err)
	}
	defer pool.Close()

	preflight, err := safety.RunPreflight(ctx, pool, safety.OperationE2ESeed)
	if err != nil {
		return fmt.Errorf("failed to run e2e seed preflight: %w", err)
	}
	safety.PrintPreflight(os.Stdout, preflight, opts)
	if err := safety.EnforceSafety(opts, preflight, os.Stdin, os.Stdout); err != nil {
		return err
	}

	credentials := []safety.CredentialStatus{
		{Label: "E2E Test User", Email: "test@gmail.com", Password: "TestPass123!"},
		{Label: "E2E AI User", Email: "ai@llm.com"},
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
			if preExisting[credentials[i].Email] {
				credentials[i].Status = "would_keep_existing"
			} else {
				credentials[i].Status = "would_create"
			}
		}
		safety.PrintCredentialSummary(os.Stdout, credentials, true)
		safety.PrintOutcomeSummary(os.Stdout, "e2e seed", false, true)
		return nil
	}

	app, err := common.NewApplication(pool, modules.BuiltInModules...)
	if err != nil {
		return fmt.Errorf("failed to create application: %w", err)
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
		return err
	}

	if err := tx.Commit(ctxWithTenant); err != nil {
		return err
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
	safety.PrintOutcomeSummary(os.Stdout, "e2e seed", true, false)
	conf.Logger().Info("Seeded e2e database with test data")
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
