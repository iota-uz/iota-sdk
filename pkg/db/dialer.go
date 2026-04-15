// Package db provides shared DB utilities for SDK consumers.
//
// DialNamedPool is a small env-var-driven pool dialer for callers that
// want to dial a second, role-scoped *pgxpool.Pool (typically a
// read-only role like ai_readonly) alongside the main application pool,
// with graceful fallback when the env var is unset. This lets an
// environment roll out role-isolated pools independently of a code
// change — flip the env var when the role lands in the target
// deployment.
package db

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
)

// DialNamedPool reads envVar from the process environment and, when set,
// dials a new *pgxpool.Pool against that DSN. When the env var is unset
// or empty, it returns `fallback` unchanged — caller uses the main pool
// and the active bool is false.
//
// The second return value reports whether the dial actually happened
// (i.e. the fallback was NOT taken). Callers inspect this to decide if
// they own the pool's lifecycle (and should Close it) vs. just sharing
// the fallback.
//
// Intentionally minimal: no defaults are applied to the parsed config.
// Callers that need custom timeouts, MaxConns, etc. should dial
// explicitly with pgxpool.NewWithConfig.
func DialNamedPool(ctx context.Context, envVar string, fallback *pgxpool.Pool) (*pgxpool.Pool, bool, error) {
	dsn := strings.TrimSpace(os.Getenv(envVar))
	if dsn == "" {
		return fallback, false, nil
	}

	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, false, fmt.Errorf("db.DialNamedPool: parse %s: %w", envVar, err)
	}
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, false, fmt.Errorf("db.DialNamedPool: dial %s: %w", envVar, err)
	}
	return pool, true, nil
}
