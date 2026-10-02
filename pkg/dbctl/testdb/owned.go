package testdb

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/iota-uz/iota-sdk/pkg/config/stdconfig/dbconfig"
)

// Create never replaces an existing database. Callers use a fresh owned name.
func Create(ctx context.Context, name string, db dbconfig.Config) error {
	if err := ownedName(name); err != nil {
		return err
	}
	return createDatabase(ctx, name, "", db, false, false)
}

// Clone copies a sealed template without evicting another application's connections.
func Clone(ctx context.Context, name, template string, db dbconfig.Config) error {
	if err := ownedName(name); err != nil {
		return err
	}
	if err := ownedName(template); err != nil {
		return err
	}
	conn, err := sql.Open("postgres", AdminConnectionString(db))
	if err != nil {
		return err
	}
	defer func() { _ = conn.Close() }()
	var allowsConnections bool
	if err := conn.QueryRowContext(ctx, "SELECT datallowconn FROM pg_database WHERE datname=$1", Name(template)).Scan(&allowsConnections); err != nil {
		return err
	}
	if allowsConnections {
		return fmt.Errorf("template must be sealed before cloning")
	}
	return createDatabase(ctx, name, template, db, false, false)
}

// Seal closes a template to new connections. Its builder must close all its
// existing pools first; no other processes are terminated by this operation.
func Seal(ctx context.Context, name string, db dbconfig.Config) error {
	if err := ownedName(name); err != nil {
		return err
	}
	conn, err := sql.Open("postgres", AdminConnectionString(db))
	if err != nil {
		return err
	}
	defer func() { _ = conn.Close() }()
	_, err = conn.ExecContext(ctx, fmt.Sprintf(`ALTER DATABASE "%s" ALLOW_CONNECTIONS false`, Name(name)))
	return err
}

// Drop is restricted by the adapter to names it allocated and recorded.
func Drop(ctx context.Context, name string, db dbconfig.Config) error {
	if err := ownedName(name); err != nil {
		return err
	}
	return dropDBContext(ctx, name, db)
}
func ConnectionString(name string, db dbconfig.Config) string { return DBOpts(name, db) }

func ownedName(name string) error {
	if name == "" || Name(name) != name || name == "postgres" || name == "template0" || name == "template1" {
		return fmt.Errorf("an explicit non-system database identifier is required")
	}
	return nil
}
