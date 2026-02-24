//go:build dev

package controllers

import (
	"context"
	"fmt"
	"net"
	"net/url"
	"os"
	"testing"
	"time"

	"github.com/iota-uz/iota-sdk/pkg/application"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
)

func controllerTestConnString(defaultDB string) string {
	host := os.Getenv("DB_HOST")
	if host == "" {
		host = "localhost"
	}
	port := os.Getenv("DB_PORT")
	if port == "" {
		port = "5432"
	}
	user := os.Getenv("DB_USER")
	if user == "" {
		user = "postgres"
	}
	password := os.Getenv("DB_PASSWORD")
	if password == "" {
		password = "postgres"
	}
	dbName := os.Getenv("DB_NAME")
	if dbName == "" {
		dbName = defaultDB
	}
	return fmt.Sprintf("postgres://%s@%s/%s", url.UserPassword(user, password).String(), net.JoinHostPort(host, port), dbName)
}

func openControllerTestPool(t *testing.T) *pgxpool.Pool {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	pool, err := pgxpool.New(ctx, controllerTestConnString("postgres"))
	if err != nil {
		t.Skipf("skipping test: cannot connect to database: %v", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		t.Skipf("skipping test: database ping failed: %v", err)
	}
	return pool
}

func testApplicationWithPool(pool *pgxpool.Pool) application.Application {
	return application.New(&application.ApplicationOptions{
		Pool:   pool,
		Bundle: application.LoadBundle(),
	})
}

func TestController_LazyInitAndClose(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		run  func(t *testing.T, pool *pgxpool.Pool)
	}{
		{
			name: "Dashboard",
			run: func(t *testing.T, pool *pgxpool.Pool) {
				t.Helper()
				controller := NewDashboardController(testApplicationWithPool(pool))
				c, ok := controller.(*DashboardController)
				require.True(t, ok)
				require.Nil(t, c.executor)

				exec := c.ensureExecutor()
				require.NoError(t, c.executorInitErr)
				require.NotNil(t, exec)
				require.Same(t, exec, c.ensureExecutor())

				require.NoError(t, c.Close())
				require.NoError(t, c.Close())

				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()
				require.NoError(t, pool.Ping(ctx))
			},
		},
		{
			name: "Showcase",
			run: func(t *testing.T, pool *pgxpool.Pool) {
				t.Helper()
				controller := NewShowcaseController(testApplicationWithPool(pool))
				c, ok := controller.(*ShowcaseController)
				require.True(t, ok)
				require.Nil(t, c.executor)

				exec := c.ensureExecutor()
				require.NoError(t, c.executorInitErr)
				require.NotNil(t, exec)
				require.Same(t, exec, c.ensureExecutor())

				require.NoError(t, c.Close())
				require.NoError(t, c.Close())

				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()
				require.NoError(t, pool.Ping(ctx))
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			pool := openControllerTestPool(t)
			t.Cleanup(pool.Close)
			tc.run(t, pool)
		})
	}
}
