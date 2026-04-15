// Package db provides shared DB utilities for SDK consumers.
//
// DialPool dials a *pgxpool.Pool from a DSN string directly, with graceful
// fallback when the DSN is empty. DialNamedPool is a convenience wrapper
// that resolves the DSN from a named environment variable; it is deprecated
// and will be removed in W5 — callers should resolve the env var themselves
// and call DialPool instead.
//
// This design lets the environment control role-isolated pools independently
// of a code change — flip the env var when the role lands in the target
// deployment.
package db

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
)

// DialPool dials a new *pgxpool.Pool against the given DSN. When dsn is empty
// or whitespace-only, it returns `fallback` unchanged and reports active=false.
//
// The second return value reports whether the dial actually happened
// (i.e. the fallback was NOT taken). Callers inspect this to decide if they
// own the pool's lifecycle (and should Close it) vs. just sharing the fallback.
//
// Intentionally minimal: no defaults are applied to the parsed config.
// Callers that need custom timeouts, MaxConns, etc. should dial explicitly
// with pgxpool.NewWithConfig.
func DialPool(ctx context.Context, dsn string, fallback *pgxpool.Pool) (*pgxpool.Pool, bool, error) {
	dsn = strings.TrimSpace(dsn)
	if dsn == "" {
		return fallback, false, nil
	}

	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, false, fmt.Errorf("db.DialPool: parse DSN: %w", err)
	}
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, false, fmt.Errorf("db.DialPool: dial: %w", err)
	}
	return pool, true, nil
}

// DialNamedPool reads envVar from the process environment and calls DialPool
// with the resolved value.
//
// Deprecated: resolve the env var at the call site and call DialPool directly.
// This wrapper will be removed in W5.
func DialNamedPool(ctx context.Context, envVar string, fallback *pgxpool.Pool) (*pgxpool.Pool, bool, error) {
	return DialPool(ctx, os.Getenv(envVar), fallback)
}
