package testutils

import (
	"context"
	"database/sql"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	_ "github.com/lib/pq"

	"github.com/iota-uz/iota-sdk/modules"
	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/role"
	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/user"
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/permission"
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/session"
	"github.com/iota-uz/iota-sdk/pkg/application"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/configuration"
	"github.com/iota-uz/iota-sdk/pkg/event"
)

type TestContext struct {
	SQLDB   *sql.DB
	Pool    *pgxpool.Pool
	Context context.Context
	Tx      pgx.Tx
	App     application.Application
}

func MockUser(permissions ...permission.Permission) *user.User {
	return &user.User{
		ID:         1,
		FirstName:  "",
		LastName:   "",
		MiddleName: "",
		Password:   "",
		Email:      "",
		AvatarID:   nil,
		Avatar:     nil,
		EmployeeID: nil,
		LastIP:     nil,
		UILanguage: "",
		LastLogin:  nil,
		LastAction: nil,
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
		Roles: []role.Role{
			{
				ID:          1,
				Name:        "admin",
				Permissions: permissions,
			},
		},
	}
}

func MockSession() *session.Session {
	return &session.Session{
		Token:     "",
		UserID:    0,
		IP:        "",
		UserAgent: "",
		ExpiresAt: time.Now(),
		CreatedAt: time.Now(),
	}
}

func GetTestContext() *TestContext {
	conf := configuration.Use()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()
	pool, err := pgxpool.New(ctx, conf.DBOpts)
	if err != nil {
		panic(err)
	}
	app := application.New(pool, event.NewEventPublisher())
	if err := modules.Load(app, modules.BuiltInModules...); err != nil {
		panic(err)
	}
	if err := app.RollbackMigrations(); err != nil {
		panic(err)
	}
	if err := app.RunMigrations(); err != nil {
		panic(err)
	}

	sqlDB := stdlib.OpenDB(*pool.Config().ConnConfig)

	tx, err := pool.Begin(ctx)
	if err != nil {
		panic(err)
	}
	ctx = composables.WithTx(ctx, tx)
	ctx = composables.WithParams(
		ctx,
		&composables.Params{
			IP:            "",
			UserAgent:     "",
			Authenticated: true,
			Request:       nil,
			Writer:        nil,
		},
	)

	return &TestContext{
		SQLDB:   sqlDB,
		Pool:    pool,
		Tx:      tx,
		Context: ctx,
		App:     app,
	}
}
