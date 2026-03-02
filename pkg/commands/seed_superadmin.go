// Package commands provides this package.
package commands

import (
	"context"
	"fmt"
	"os"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/user"
	"github.com/iota-uz/iota-sdk/modules/core/domain/value_objects/internet"
	coreseed "github.com/iota-uz/iota-sdk/modules/core/seed"
	"github.com/iota-uz/iota-sdk/pkg/application"
	"github.com/iota-uz/iota-sdk/pkg/commands/common"
	"github.com/iota-uz/iota-sdk/pkg/commands/safety"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/configuration"
	"github.com/iota-uz/iota-sdk/pkg/defaults"
)

// SeedSuperadmin seeds the database with a superadmin user
func SeedSuperadmin(opts safety.RunOptions, mods ...application.Module) error {
	conf := configuration.Use()
	ctx := context.Background()

	app, pool, err := common.NewApplicationWithDefaults(mods...)
	if err != nil {
		return fmt.Errorf("failed to initialize application: %w", err)
	}
	defer pool.Close()

	preflight, err := safety.RunPreflight(ctx, pool, safety.OperationSeedSuperadmin)
	if err != nil {
		return fmt.Errorf("failed to run seed preflight: %w", err)
	}
	safety.PrintPreflight(os.Stdout, preflight, opts)
	if err := safety.EnforceSafety(opts, preflight, os.Stdin, os.Stdout); err != nil {
		return err
	}
	credential := safety.CredentialStatus{
		Label:    "Superadmin User",
		Email:    "admin@superadmin.local",
		Password: "SuperAdmin123!",
	}
	existedBefore, err := userExistsByEmail(ctx, credential.Email)
	if err != nil {
		return err
	}
	if existedBefore {
		credential.Status = "already_existed (seed skipped/left unchanged)"
	} else {
		credential.Status = "created_now"
	}
	if opts.DryRun {
		if existedBefore {
			credential.Status = "would_keep_existing"
		} else {
			credential.Status = "would_create"
		}
		safety.PrintCredentialSummary(os.Stdout, []safety.CredentialStatus{credential}, true)
		safety.PrintOutcomeSummary(os.Stdout, "superadmin seed", false, true)
		return nil
	}

	tx, err := pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}

	seeder := application.NewSeeder()

	// Create superadmin user with all permissions
	superadminUser, err := user.New(
		"Super",
		"Admin",
		internet.MustParseEmail("admin@superadmin.local"),
		user.UILanguageEN,
		user.WithType(user.TypeSuperAdmin),
	).SetPassword("SuperAdmin123!")
	if err != nil {
		return fmt.Errorf("failed to create superadmin user: %w", err)
	}

	// Add default tenant to context
	defaultTenant := &composables.Tenant{
		ID:     uuid.MustParse("00000000-0000-0000-0000-000000000001"),
		Name:   "Default",
		Domain: "default.localhost",
	}

	allPermissions := defaults.AllPermissions()

	// Register seeder functions
	seeder.Register(
		coreseed.CreateDefaultTenant,
		coreseed.CreateCurrencies,
		func(ctx context.Context, app application.Application) error {
			return coreseed.CreatePermissions(ctx, app, allPermissions)
		},
		coreseed.UserSeedFunc(superadminUser, allPermissions),
		coreseed.CreateSubscriptionEntitlements,
	)

	// Create context with tenant ID (we use the default tenant)
	ctxWithTenant := composables.WithTenantID(
		composables.WithTx(ctx, tx),
		defaultTenant.ID,
	)

	if err := seeder.Seed(ctxWithTenant, app); err != nil {
		if rollbackErr := tx.Rollback(ctx); rollbackErr != nil {
			return fmt.Errorf("rollback failed: %w (original error: %w)", rollbackErr, err)
		}
		return fmt.Errorf("failed to seed superadmin: %w", err)
	}

	if err := tx.Commit(ctxWithTenant); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	existsAfter, err := userExistsByEmail(ctx, credential.Email)
	if err != nil {
		return err
	}
	if existsAfter && !existedBefore {
		credential.Status = "created_now"
	} else {
		credential.Status = "already_existed (seed skipped/left unchanged)"
	}

	safety.PrintCredentialSummary(os.Stdout, []safety.CredentialStatus{credential}, false)
	safety.PrintOutcomeSummary(os.Stdout, "superadmin seed", true, false)
	conf.Logger().Info("Superadmin user seeded successfully!")
	conf.Logger().Info("Email: admin@superadmin.local")
	conf.Logger().Info("Password: SuperAdmin123!")
	return nil
}
