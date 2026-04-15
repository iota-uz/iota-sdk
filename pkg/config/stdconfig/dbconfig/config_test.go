package dbconfig_test

import (
	"testing"
	"time"

	"github.com/iota-uz/iota-sdk/pkg/config"
	"github.com/iota-uz/iota-sdk/pkg/config/providers/static"
	"github.com/iota-uz/iota-sdk/pkg/config/stdconfig/dbconfig"
)

// validValues returns a static map that produces a fully valid Config.
func validValues() map[string]any {
	return map[string]any{
		"db.name":                       "testdb",
		"db.host":                       "127.0.0.1",
		"db.port":                       "5432",
		"db.user":                       "alice",
		"db.password":                   "s3cret",
		"db.migrationsdir":              "migrations",
		"db.pool.maxconns":              int32(32),
		"db.pool.minconns":              int32(8),
		"db.pool.maxconnlifetime":       time.Hour,
		"db.pool.maxconnlifetimejitter": 6 * time.Minute,
		"db.pool.maxconnidletime":       15 * time.Minute,
		"db.pool.healthcheckperiod":     time.Minute,
		"db.pool.connecttimeout":        10 * time.Second,
	}
}

func buildSource(t *testing.T, values map[string]any) config.Source {
	t.Helper()
	src, err := config.Build(static.New(values))
	if err != nil {
		t.Fatalf("config.Build: %v", err)
	}
	return src
}

func TestUnmarshalRoundTrip(t *testing.T) {
	t.Parallel()

	src := buildSource(t, validValues())

	var cfg dbconfig.Config
	if err := src.Unmarshal("db", &cfg); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	if cfg.Name != "testdb" {
		t.Errorf("Name: got %q, want %q", cfg.Name, "testdb")
	}
	if cfg.Host != "127.0.0.1" {
		t.Errorf("Host: got %q, want %q", cfg.Host, "127.0.0.1")
	}
	if cfg.Port != "5432" {
		t.Errorf("Port: got %q, want %q", cfg.Port, "5432")
	}
	if cfg.User != "alice" {
		t.Errorf("User: got %q, want %q", cfg.User, "alice")
	}
	if cfg.Password != "s3cret" {
		t.Errorf("Password: got %q, want %q", cfg.Password, "s3cret")
	}
	if cfg.MigrationsDir != "migrations" {
		t.Errorf("MigrationsDir: got %q, want %q", cfg.MigrationsDir, "migrations")
	}
	if cfg.Pool.MaxConns != 32 {
		t.Errorf("Pool.MaxConns: got %d, want 32", cfg.Pool.MaxConns)
	}
	if cfg.Pool.MinConns != 8 {
		t.Errorf("Pool.MinConns: got %d, want 8", cfg.Pool.MinConns)
	}
	if cfg.Pool.MaxConnLifetime != time.Hour {
		t.Errorf("Pool.MaxConnLifetime: got %v, want 1h", cfg.Pool.MaxConnLifetime)
	}
	if cfg.Pool.ConnectTimeout != 10*time.Second {
		t.Errorf("Pool.ConnectTimeout: got %v, want 10s", cfg.Pool.ConnectTimeout)
	}
}

func TestConnectionString(t *testing.T) {
	t.Parallel()

	cfg := dbconfig.Config{
		Name:     "mydb",
		Host:     "db.example.com",
		Port:     "5433",
		User:     "bob",
		Password: "hunter2",
	}

	want := "host=db.example.com port=5433 user=bob dbname=mydb password=hunter2 sslmode=disable"
	got := cfg.ConnectionString()
	if got != want {
		t.Errorf("ConnectionString:\n got  %q\n want %q", got, want)
	}
}

func TestPoolConfig_HappyPath(t *testing.T) {
	t.Parallel()

	cfg := dbconfig.Config{
		Name:     "testdb",
		Host:     "localhost",
		Port:     "5432",
		User:     "postgres",
		Password: "postgres",
		Pool: dbconfig.PoolTuning{
			MaxConns:       32,
			MinConns:       4,
			ConnectTimeout: 10 * time.Second,
		},
	}

	poolCfg, err := cfg.PoolConfig()
	if err != nil {
		t.Fatalf("PoolConfig: unexpected error: %v", err)
	}
	if poolCfg.MaxConns != 32 {
		t.Errorf("MaxConns: got %d, want 32", poolCfg.MaxConns)
	}
	if poolCfg.MinConns != 4 {
		t.Errorf("MinConns: got %d, want 4", poolCfg.MinConns)
	}
	if poolCfg.AfterConnect == nil {
		t.Error("AfterConnect hook must be set")
	}
}

func TestPoolConfig_MaxConnsZero(t *testing.T) {
	t.Parallel()

	cfg := dbconfig.Config{
		Pool: dbconfig.PoolTuning{
			MaxConns:       0,
			ConnectTimeout: 10 * time.Second,
		},
	}
	_, err := cfg.PoolConfig()
	if err == nil {
		t.Fatal("expected error for MaxConns=0, got nil")
	}
}

func TestPoolConfig_MinConnsExceedsMax(t *testing.T) {
	t.Parallel()

	cfg := dbconfig.Config{
		Pool: dbconfig.PoolTuning{
			MaxConns:       4,
			MinConns:       8,
			ConnectTimeout: 10 * time.Second,
		},
	}
	_, err := cfg.PoolConfig()
	if err == nil {
		t.Fatal("expected error when MinConns > MaxConns, got nil")
	}
}

func TestPoolConfig_ZeroConnectTimeout(t *testing.T) {
	t.Parallel()

	cfg := dbconfig.Config{
		Pool: dbconfig.PoolTuning{
			MaxConns:       8,
			MinConns:       2,
			ConnectTimeout: 0,
		},
	}
	_, err := cfg.PoolConfig()
	if err == nil {
		t.Fatal("expected error for ConnectTimeout=0, got nil")
	}
}

func TestFromLegacy(t *testing.T) {
	t.Parallel()

	legacy := buildLegacyConfiguration()
	got := dbconfig.FromLegacy(legacy)

	if got.Name != legacy.Database.Name {
		t.Errorf("Name: got %q, want %q", got.Name, legacy.Database.Name)
	}
	if got.Host != legacy.Database.Host {
		t.Errorf("Host: got %q, want %q", got.Host, legacy.Database.Host)
	}
	if got.Port != legacy.Database.Port {
		t.Errorf("Port: got %q, want %q", got.Port, legacy.Database.Port)
	}
	if got.User != legacy.Database.User {
		t.Errorf("User: got %q, want %q", got.User, legacy.Database.User)
	}
	if got.Password != legacy.Database.Password {
		t.Errorf("Password: got %q, want %q", got.Password, legacy.Database.Password)
	}
	if got.MigrationsDir != legacy.MigrationsDir {
		t.Errorf("MigrationsDir: got %q, want %q", got.MigrationsDir, legacy.MigrationsDir)
	}
	if got.Pool.MaxConns != legacy.Database.MaxConns {
		t.Errorf("Pool.MaxConns: got %d, want %d", got.Pool.MaxConns, legacy.Database.MaxConns)
	}
	if got.Pool.MinConns != legacy.Database.MinConns {
		t.Errorf("Pool.MinConns: got %d, want %d", got.Pool.MinConns, legacy.Database.MinConns)
	}
	if got.Pool.MaxConnLifetime != legacy.Database.MaxConnLifetime {
		t.Errorf("Pool.MaxConnLifetime: got %v, want %v", got.Pool.MaxConnLifetime, legacy.Database.MaxConnLifetime)
	}
	if got.Pool.MaxConnLifetimeJitter != legacy.Database.MaxConnLifetimeJitter {
		t.Errorf("Pool.MaxConnLifetimeJitter: got %v, want %v", got.Pool.MaxConnLifetimeJitter, legacy.Database.MaxConnLifetimeJitter)
	}
	if got.Pool.MaxConnIdleTime != legacy.Database.MaxConnIdleTime {
		t.Errorf("Pool.MaxConnIdleTime: got %v, want %v", got.Pool.MaxConnIdleTime, legacy.Database.MaxConnIdleTime)
	}
	if got.Pool.HealthCheckPeriod != legacy.Database.HealthCheckPeriod {
		t.Errorf("Pool.HealthCheckPeriod: got %v, want %v", got.Pool.HealthCheckPeriod, legacy.Database.HealthCheckPeriod)
	}
	if got.Pool.ConnectTimeout != legacy.Database.ConnectTimeout {
		t.Errorf("Pool.ConnectTimeout: got %v, want %v", got.Pool.ConnectTimeout, legacy.Database.ConnectTimeout)
	}
}
