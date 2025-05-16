package main

import (
	"context"
	"log"
	"os"
	"runtime/debug"
	"time"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/modules"
	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/user"
	"github.com/iota-uz/iota-sdk/modules/core/domain/value_objects/internet"
	coreseed "github.com/iota-uz/iota-sdk/modules/core/seed"
	"github.com/iota-uz/iota-sdk/modules/website/domain/entities/aichatconfig"
	websiteseed "github.com/iota-uz/iota-sdk/modules/website/seed"
	"github.com/iota-uz/iota-sdk/pkg/application"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/configuration"
	"github.com/iota-uz/iota-sdk/pkg/constants"
	"github.com/iota-uz/iota-sdk/pkg/eventbus"

	"github.com/jackc/pgx/v5/pgxpool"
)

func pgxPool() *pgxpool.Pool {
	conf := configuration.Use()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()
	pool, err := pgxpool.New(ctx, conf.Database.Opts)
	if err != nil {
		panicWithStack(err)
	}
	return pool
}

func panicWithStack(err error) {
	errorWithStack := string(debug.Stack()) + "\n\nError: " + err.Error()
	panic(errorWithStack)
}

func main() {
	defer func() {
		if r := recover(); r != nil {
			configuration.Use().Unload()
			log.Println(r)
			debug.PrintStack()
			os.Exit(1)
		}
	}()

	conf := configuration.Use()
	ctx := context.Background()
	pool := pgxPool()
	app := application.New(pool, eventbus.NewEventPublisher(conf.Logger()))
	if err := modules.Load(app, modules.BuiltInModules...); err != nil {
		panicWithStack(err)
	}
	app.RegisterNavItems(modules.NavLinks...)
	tx, err := pool.Begin(ctx)
	if err != nil {
		panicWithStack(err)
	}
	seeder := application.NewSeeder()
	usr, err := user.New(
		"Test",
		"User",
		internet.MustParseEmail("test@gmail.com"),
		user.UILanguageEN,
	).SetPassword("TestPass123!")
	if err != nil {
		panicWithStack(err)
	}
	seeder.Register(
		coreseed.CreateDefaultTenant,
		coreseed.CreateCurrencies,
		coreseed.CreatePermissions,
		coreseed.UserSeedFunc(usr),
		coreseed.UserSeedFunc(user.New(
			"AI",
			"User",
			internet.MustParseEmail("ai@llm.com"),
			user.UILanguageEN,
		)),
		websiteseed.AIChatConfigSeedFunc(aichatconfig.MustNew(
			"gemma-12b-it",
			aichatconfig.AIModelTypeOpenAI,
			"https://llm2.eai.uz/v1",
		)),
	)

	// Add default tenant to context
	defaultTenant := &composables.Tenant{
		ID:     uuid.MustParse("00000000-0000-0000-0000-000000000001"),
		Name:   "Default",
		Domain: "default.localhost",
	}
	ctxWithTenant := context.WithValue(
		composables.WithTx(ctx, tx),
		constants.TenantKey,
		defaultTenant,
	)

	if err := seeder.Seed(ctxWithTenant, app); err != nil {
		panicWithStack(err)
	}
	if err := tx.Commit(ctxWithTenant); err != nil {
		panicWithStack(err)
	}
}
