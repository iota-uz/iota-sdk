package composables

import (
	"context"
	"errors"
	"github.com/iota-uz/iota-sdk/pkg/constants"
	"gorm.io/gorm"
)

var (
	ErrNoTx = errors.New("no transaction found in context")
	ErrNoDB = errors.New("no database found in context")
)

// WithTx returns a new context with the database transaction.
func WithTx(ctx context.Context, tx *gorm.DB) context.Context {
	return context.WithValue(ctx, constants.TxKey, tx)
}

// UseTx returns the database transaction from the context.
func UseTx(ctx context.Context) (*gorm.DB, bool) {
	tx, ok := ctx.Value(constants.TxKey).(*gorm.DB)
	if !ok {
		return nil, false
	}
	return tx, true
}

// WithDB returns a new context with the database.
func WithDB(ctx context.Context, db *gorm.DB) context.Context {
	return context.WithValue(ctx, constants.DBKey, db)
}

// UseDB returns the database from the context.
func UseDB(ctx context.Context) (*gorm.DB, error) {
	db := ctx.Value(constants.DBKey)
	if db == nil {
		return nil, ErrNoDB
	}
	return db.(*gorm.DB), nil
}
