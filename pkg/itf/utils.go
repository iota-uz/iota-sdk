package itf

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"fmt"
	"log"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/modules"
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/session"
	"github.com/iota-uz/iota-sdk/pkg/application"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/configuration"
	"github.com/iota-uz/iota-sdk/pkg/eventbus"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	_ "github.com/lib/pq"
)

type TestFixtures struct {
	SQLDB   *sql.DB
	Pool    *pgxpool.Pool
	Context context.Context
	Tx      pgx.Tx
	App     application.Application
}

func MockSession() session.Session {
	return session.New("mock-token", 0, uuid.Nil, "127.0.0.1", "test-agent")
}

func NewPool(dbOpts string) *pgxpool.Pool {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()

	config, err := pgxpool.ParseConfig(dbOpts)
	if err != nil {
		panic(err)
	}

	// With increased PostgreSQL max_connections (500), we can use reasonable limits
	config.MaxConns = 4
	config.MinConns = 1
	config.MaxConnLifetime = time.Minute * 5
	config.MaxConnIdleTime = time.Second * 30

	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		panic(fmt.Errorf("failed to create database pool: %w", err))
	}

	return pool
}

// DatabaseManager handles database lifecycle for tests
type DatabaseManager struct {
	pool   *pgxpool.Pool
	dbName string
}

// NewDatabaseManager creates a new database and returns a manager that handles cleanup automatically
func NewDatabaseManager(t *testing.T) *DatabaseManager {
	t.Helper()

	dbName := t.Name()
	CreateDB(dbName)
	pool := NewPool(DbOpts(dbName))

	dm := &DatabaseManager{
		pool:   pool,
		dbName: dbName,
	}

	// Register cleanup
	t.Cleanup(func() {
		dm.Close()
	})

	return dm
}

// Pool returns the database pool
func (dm *DatabaseManager) Pool() *pgxpool.Pool {
	return dm.pool
}

// Close closes the pool
func (dm *DatabaseManager) Close() {
	if dm.pool != nil {
		dm.pool.Close()
		dm.pool = nil
	}
}

func DefaultParams() *composables.Params {
	return &composables.Params{
		IP:            "",
		UserAgent:     "",
		Authenticated: true,
		Request:       nil,
		Writer:        nil,
	}
}

// CreateTestTenant creates a test tenant for testing
func CreateTestTenant(ctx context.Context, pool *pgxpool.Pool) (*composables.Tenant, error) {
	tenantID := uuid.New()
	testTenant := &composables.Tenant{
		ID:     tenantID,
		Name:   "Test Tenant " + tenantID.String()[:8],
		Domain: tenantID.String()[:8] + ".test.com",
	}

	_, err := pool.Exec(ctx, "INSERT INTO tenants (id, name, domain, created_at, updated_at) VALUES ($1, $2, $3, $4, $5) ON CONFLICT (id) DO NOTHING",
		testTenant.ID,
		testTenant.Name,
		testTenant.Domain,
		time.Now(),
		time.Now(),
	)

	if err != nil {
		return nil, fmt.Errorf("failed to create test tenant: %w", err)
	}

	return testTenant, nil
}

const (
	// PostgreSQL database name maximum length is 63 characters
	maxDBNameLength = 63
	// Reserve space for hash suffix when truncating (8 chars + underscore)
	hashSuffixLength = 9
)

// sanitizeDBName replaces special characters in database names with underscores
// and ensures the name doesn't exceed PostgreSQL's 63-character limit
func sanitizeDBName(name string) string {
	// Convert to lowercase (PostgreSQL convention)
	sanitized := strings.ToLower(name)

	// Replace special characters with underscores
	sanitized = strings.ReplaceAll(sanitized, "/", "_")
	sanitized = strings.ReplaceAll(sanitized, " ", "_")
	sanitized = strings.ReplaceAll(sanitized, "-", "_")
	sanitized = strings.ReplaceAll(sanitized, ".", "_")
	sanitized = strings.ReplaceAll(sanitized, "(", "_")
	sanitized = strings.ReplaceAll(sanitized, ")", "_")
	sanitized = strings.ReplaceAll(sanitized, "[", "_")
	sanitized = strings.ReplaceAll(sanitized, "]", "_")

	// Remove consecutive underscores
	for strings.Contains(sanitized, "__") {
		sanitized = strings.ReplaceAll(sanitized, "__", "_")
	}

	// Trim leading/trailing underscores
	sanitized = strings.Trim(sanitized, "_")

	// Handle edge case where sanitization results in empty string
	if sanitized == "" {
		sanitized = "test_db"
	}

	// If name is within limit, return as-is
	if len(sanitized) <= maxDBNameLength {
		return sanitized
	}

	// Name is too long, need to truncate and add hash for uniqueness
	return truncateWithHash(sanitized, name)
}

// truncateWithHash truncates a database name and adds a hash suffix for uniqueness
func truncateWithHash(sanitized, original string) string {
	// Calculate hash of the original name for uniqueness
	hasher := sha256.New()
	hasher.Write([]byte(original))
	hash := fmt.Sprintf("%x", hasher.Sum(nil))[:8] // Use first 8 chars of hash

	// Calculate available space for the name part
	maxNameLength := maxDBNameLength - hashSuffixLength

	// Truncate intelligently - try to keep meaningful parts
	truncated := intelligentTruncate(sanitized, maxNameLength)

	// Combine truncated name with hash
	return fmt.Sprintf("%s_%s", truncated, hash)
}

// intelligentTruncate tries to keep the most meaningful parts of a test name
func intelligentTruncate(name string, maxLength int) string {
	if len(name) <= maxLength {
		return name
	}

	// Split by underscores to identify segments
	parts := strings.Split(name, "_")

	// If we have multiple parts, try to keep the most important ones
	if len(parts) > 1 {
		// Keep the first and last parts if possible, as they're often most meaningful
		first := parts[0]
		last := parts[len(parts)-1]

		// If first and last alone fit, use them
		combined := first + "_" + last
		if len(combined) <= maxLength && first != last {
			return combined
		}

		// If first part is reasonable length, start with it
		if len(first) <= maxLength/2 {
			result := first
			remaining := maxLength - len(first) - 1 // -1 for underscore

			// Add as many subsequent parts as we can fit
			for i := 1; i < len(parts) && len(result) < maxLength; i++ {
				part := parts[i]
				if len(part)+1 <= remaining { // +1 for underscore
					result += "_" + part
					remaining -= len(part) + 1
				} else {
					// If we can fit a truncated version of this part, do it
					if remaining > 4 { // Minimum meaningful length
						result += "_" + part[:remaining-1]
					}
					break
				}
			}
			return result
		}
	}

	// Fallback: simple truncation
	return name[:maxLength]
}

func CreateDB(name string) {
	sanitizedName := sanitizeDBName(name)

	c := configuration.Use()
	// Create connection string for postgres admin database
	adminConnStr := fmt.Sprintf(
		"host=%s port=%s user=%s dbname=postgres password=%s sslmode=disable",
		c.Database.Host, c.Database.Port, c.Database.User, c.Database.Password,
	)
	db, err := sql.Open("postgres", adminConnStr)
	if err != nil {
		panic(err)
	}
	defer func() {
		if err := db.Close(); err != nil {
			log.Printf("[WARNING] Error closing CreateDB connection: %v", err)
		}
	}()
	_, err = db.ExecContext(context.Background(), fmt.Sprintf("DROP DATABASE IF EXISTS %s", sanitizedName))
	if err != nil {
		panic(err)
	}
	_, err = db.ExecContext(context.Background(), fmt.Sprintf("CREATE DATABASE %s", sanitizedName))
	if err != nil {
		panic(err)
	}
}

func DbOpts(name string) string {
	sanitizedName := sanitizeDBName(name)

	c := configuration.Use()
	return fmt.Sprintf(
		"host=%s port=%s user=%s dbname=%s password=%s sslmode=disable",
		c.Database.Host, c.Database.Port, c.Database.User, strings.ToLower(sanitizedName), c.Database.Password,
	)
}

func SetupApplication(pool *pgxpool.Pool, mods ...application.Module) (application.Application, error) {
	conf := configuration.Use()
	bundle := application.LoadBundle()
	app := application.New(&application.ApplicationOptions{
		Pool:     pool,
		Bundle:   bundle,
		EventBus: eventbus.NewEventPublisher(conf.Logger()),
		Logger:   conf.Logger(),
	})
	if err := modules.Load(app, mods...); err != nil {
		return nil, err
	}
	if err := app.Migrations().Run(); err != nil {
		return nil, err
	}
	return app, nil
}

func GetTestContext() *TestFixtures {
	conf := configuration.Use()
	pool := NewPool(conf.Database.Opts)
	bundle := application.LoadBundle()
	app := application.New(&application.ApplicationOptions{
		Pool:     pool,
		Bundle:   bundle,
		EventBus: eventbus.NewEventPublisher(conf.Logger()),
		Logger:   conf.Logger(),
	})
	if err := modules.Load(app, modules.BuiltInModules...); err != nil {
		panic(err)
	}
	if err := app.Migrations().Rollback(); err != nil {
		panic(err)
	}
	if err := app.Migrations().Run(); err != nil {
		panic(err)
	}

	sqlDB := stdlib.OpenDB(*pool.Config().ConnConfig)
	ctx := context.Background()
	tx, err := pool.Begin(ctx)
	if err != nil {
		panic(err)
	}
	ctx = composables.WithTx(ctx, tx)
	ctx = composables.WithParams(
		ctx,
		DefaultParams(),
	)

	return &TestFixtures{
		SQLDB:   sqlDB,
		Pool:    pool,
		Tx:      tx,
		Context: ctx,
		App:     app,
	}
}
