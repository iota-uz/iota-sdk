package testutils

import (
	"context"
	"database/sql"
	"log"
	"os"
	"time"

	"github.com/iota-uz/iota-sdk/modules"
	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/role"
	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/user"
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/permission"
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/session"
	"github.com/iota-uz/iota-sdk/pkg/application"
	"github.com/iota-uz/iota-sdk/pkg/application/dbutils"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/configuration"
	"github.com/iota-uz/iota-sdk/pkg/event"
	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/lib/pq"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

type TestContext struct {
	SQLDB   *sql.DB
	GormDB  *gorm.DB
	Context context.Context
	Tx      *gorm.DB
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
		Roles: []*role.Role{
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
	db, err := dbutils.ConnectDB(
		conf.DBOpts,
		gormlogger.New(
			log.New(os.Stdout, "\r\n", log.LstdFlags),
			gormlogger.Config{
				SlowThreshold:             0,
				LogLevel:                  gormlogger.Error,
				IgnoreRecordNotFoundError: false,
				Colorful:                  true,
				ParameterizedQueries:      true,
			},
		),
	)
	if err != nil {
		panic(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()
	pool, err := pgxpool.New(ctx, conf.DBOpts)
	app := application.New(db, pool, event.NewEventPublisher())
	if err := modules.Load(app, modules.BuiltInModules...); err != nil {
		panic(err)
	}
	if err := app.RollbackMigrations(); err != nil {
		panic(err)
	}
	if err := app.RunMigrations(); err != nil {
		panic(err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		panic(err)
	}

	tx := db.Begin()
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
		GormDB:  db,
		Tx:      tx,
		Context: ctx,
		App:     app,
	}
}
