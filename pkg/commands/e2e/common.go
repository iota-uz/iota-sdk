package e2e

import (
	"context"
	"fmt"
	"time"

	"github.com/iota-uz/iota-sdk/pkg/commands/common"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	E2E_DB_NAME     = "iota_erp_e2e"
	E2E_SERVER_PORT = "3201"
	E2E_SERVER_HOST = "localhost"
)

// GetE2EPool creates a database connection pool for e2e tests
func GetE2EPool() (*pgxpool.Pool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()

	pool, err := common.GetDatabasePool(ctx, E2E_DB_NAME)
	if err != nil {
		// If connection to e2e database fails, try connecting to postgres database to create it
		postgresPool, postgresErr := common.GetDatabasePool(ctx, "postgres")
		if postgresErr != nil {
			return nil, fmt.Errorf("failed to connect to PostgreSQL: %w", postgresErr)
		}
		defer postgresPool.Close()

		// Create the e2e database
		_, createErr := postgresPool.Exec(ctx, fmt.Sprintf("CREATE DATABASE %s", E2E_DB_NAME))
		if createErr != nil {
			// Database might already exist, try to connect again
		}

		// Try connecting to e2e database again
		pool, err = common.GetDatabasePool(ctx, E2E_DB_NAME)
		if err != nil {
			return nil, err
		}
	}
	return pool, nil
}
