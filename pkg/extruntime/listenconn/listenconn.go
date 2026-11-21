// Package listenconn opens the dedicated connections the extension core holds
// LISTEN on. A long-lived LISTEN must not take a pool slot, yet it must be
// opened the way the pool opens its own connections: with the pool's
// BeforeConnect (credentials, role, host chosen per connection) and
// AfterConnect (session setup) hooks.
package listenconn

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Connect opens a connection outside the pool with the pool's settings and
// connect hooks. A connection whose AfterConnect fails is closed.
func Connect(ctx context.Context, pool *pgxpool.Pool) (*pgx.Conn, error) {
	cfg := pool.Config()
	cc := cfg.ConnConfig.Copy()
	if cfg.BeforeConnect != nil {
		if err := cfg.BeforeConnect(ctx, cc); err != nil {
			return nil, err
		}
	}
	conn, err := pgx.ConnectConfig(ctx, cc)
	if err != nil {
		return nil, err
	}
	if cfg.AfterConnect != nil {
		if err := cfg.AfterConnect(ctx, conn); err != nil {
			_ = conn.Close(context.WithoutCancel(ctx))
			return nil, err
		}
	}
	return conn, nil
}
