package db

import "github.com/jackc/pgx/v5/pgxpool"

// poolSentinel returns a nil pool value used only for pointer equality
// checks in tests. We do not dial a real pool because that would
// require a running Postgres.
func poolSentinel() *pgxpool.Pool { return &pgxpool.Pool{} }
