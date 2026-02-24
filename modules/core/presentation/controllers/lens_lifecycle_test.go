//go:build dev

package controllers

import (
	"context"
	"fmt"
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
	return fmt.Sprintf("postgres://%s:%s@%s:%s/%s", user, password, host, port, dbName)
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

func TestDashboardController_LazyInitAndClose(t *testing.T) {
	t.Parallel()

	pool := openControllerTestPool(t)
	t.Cleanup(pool.Close)

	controller := NewDashboardController(testApplicationWithPool(pool))
	dashboardController, ok := controller.(*DashboardController)
	require.True(t, ok)
	require.Nil(t, dashboardController.executor)

	exec := dashboardController.ensureExecutor()
	require.NotNil(t, exec)
	require.Same(t, exec, dashboardController.ensureExecutor())

	require.NoError(t, dashboardController.Close())
	require.NoError(t, dashboardController.Close())

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	require.NoError(t, pool.Ping(ctx))
}

func TestShowcaseController_LazyInitAndClose(t *testing.T) {
	t.Parallel()

	pool := openControllerTestPool(t)
	t.Cleanup(pool.Close)

	controller := NewShowcaseController(testApplicationWithPool(pool))
	showcaseController, ok := controller.(*ShowcaseController)
	require.True(t, ok)
	require.Nil(t, showcaseController.executor)

	exec := showcaseController.ensureExecutor()
	require.NotNil(t, exec)
	require.Same(t, exec, showcaseController.ensureExecutor())

	require.NoError(t, showcaseController.Close())
	require.NoError(t, showcaseController.Close())

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	require.NoError(t, pool.Ping(ctx))
}
